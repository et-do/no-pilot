// Package policy provides the enforcement layer that every no-pilot tool handler
// uses to check the merged user+project policy before executing.
//
// Four middleware constructors are provided:
//
//   - Enforce — wraps a tool handler to check whether the tool is allowed at all.
//   - EnforceWithPaths — additionally checks named string arguments against the
//     tool's deny_paths before passing the request to the inner handler.
//   - EnforceWithCommand — additionally checks a command argument against the
//     tool's allow_commands allowlist and deny_commands denylist.
//   - EnforceWithURL — additionally checks a URL argument against the tool's
//     deny_urls denylist.
//
// All four return a [server.ToolHandlerMiddleware] so they compose cleanly with
// any other middleware registered on the MCPServer.
package policy

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/et-do/no-pilot/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Enforce returns a middleware that denies the tool call if the tool is not
// allowed by the merged policy. Argument values are not checked; use
// EnforceWithPaths, EnforceWithCommand, or EnforceWithURL for tools that
// accept constrained arguments.
func Enforce(cfg *config.Config, toolName string) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if result := checkAllowed(cfg, toolName); result != nil {
				return result, nil
			}
			return next(ctx, req)
		}
	}
}

// EnforceWithPaths returns a middleware that:
//  1. Denies the call if the tool is not allowed.
//  2. Denies the call if any of the named string arguments match a deny_paths pattern.
//
// pathArgs lists the argument keys whose values should be matched against
// deny_paths (e.g. "path", "filePath"). Arguments that are absent or not
// strings are skipped silently — mandatory argument validation is left to the
// tool handler itself.
func EnforceWithPaths(cfg *config.Config, toolName string, pathArgs ...string) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if result := checkAllowed(cfg, toolName); result != nil {
				return result, nil
			}

			pol := cfg.Policy(toolName)
			if len(pol.DenyPaths) > 0 && len(pathArgs) > 0 {
				args := req.GetArguments()
				for _, key := range pathArgs {
					val, ok := args[key]
					if !ok {
						continue
					}
					pathStr, ok := val.(string)
					if !ok {
						continue
					}
					if matched, pattern := matchesAny(pathStr, pol.DenyPaths); matched {
						return mcp.NewToolResultError(fmt.Sprintf(
							"path %q is denied by policy pattern %q", pathStr, pattern,
						)), nil
					}
				}
			}

			return next(ctx, req)
		}
	}
}

// EnforceWithCommand returns a middleware that:
//  1. Denies the call if the tool is not allowed.
//  2. Denies the call if the named command argument does not match any pattern
//     in allow_commands (when allow_commands is set).
//  3. Denies the call if the named command argument matches any pattern in
//     deny_commands. Deny is evaluated after allow.
//
// commandArg is the argument key whose value holds the shell command string
// (e.g. "command"). A missing or non-string argument is skipped silently.
func EnforceWithCommand(cfg *config.Config, toolName string, commandArg string) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if result := checkAllowed(cfg, toolName); result != nil {
				return result, nil
			}

			pol := cfg.Policy(toolName)
			args := req.GetArguments()
			val, ok := args[commandArg]
			if !ok {
				return next(ctx, req)
			}
			cmd, ok := val.(string)
			if !ok {
				return next(ctx, req)
			}

			// Allowlist check: if allow_commands is set the command must match.
			if len(pol.AllowCommands) > 0 {
				if matched, _ := matchesAnyCmd(cmd, pol.AllowCommands); !matched {
					return mcp.NewToolResultError(fmt.Sprintf(
						"command %q is not permitted by policy allow_commands", cmd,
					)), nil
				}
			}

			// Denylist check: command must not match any deny pattern.
			if matched, pattern := matchesAnyCmd(cmd, pol.DenyCommands); matched {
				return mcp.NewToolResultError(fmt.Sprintf(
					"command %q is denied by policy pattern %q", cmd, pattern,
				)), nil
			}

			return next(ctx, req)
		}
	}
}

// EnforceWithURL returns a middleware that:
//  1. Denies the call if the tool is not allowed.
//  2. Denies the call if the named URL argument matches any pattern in deny_urls.
//
// urlArg is the argument key whose value holds the URL string (e.g. "url").
// A missing or non-string argument is skipped silently.
func EnforceWithURL(cfg *config.Config, toolName string, urlArg string) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if result := checkAllowed(cfg, toolName); result != nil {
				return result, nil
			}

			pol := cfg.Policy(toolName)
			if len(pol.DenyURLs) > 0 {
				args := req.GetArguments()
				val, ok := args[urlArg]
				if ok {
					urlStr, ok := val.(string)
					if ok {
						if matched, pattern := matchesAnyURL(urlStr, pol.DenyURLs); matched {
							return mcp.NewToolResultError(fmt.Sprintf(
								"URL %q is denied by policy pattern %q", urlStr, pattern,
							)), nil
						}
					}
				}
			}

			return next(ctx, req)
		}
	}
}

// checkAllowed returns a denial result if the tool is not allowed, or nil if
// the call may proceed.
func checkAllowed(cfg *config.Config, toolName string) *mcp.CallToolResult {
	if !cfg.Policy(toolName).IsAllowed() {
		return mcp.NewToolResultError(fmt.Sprintf("tool %q is disabled by policy", toolName))
	}
	return nil
}

// matchesAny reports whether s matches any of the given doublestar glob
// patterns (path semantics: * does not cross /). Use for file-system paths.
// It returns the first matching pattern alongside the boolean.
//
// Invalid patterns are silently skipped.
func matchesAny(s string, patterns []string) (matched bool, pattern string) {
	for _, p := range patterns {
		ok, err := doublestar.Match(p, s)
		if err != nil {
			continue
		}
		if ok {
			return true, p
		}
	}
	return false, ""
}

// matchesAnyCmd matches a command string against shell-style glob patterns
// where * matches any sequence of characters, including spaces and slashes.
// Use for allow_commands / deny_commands values.
func matchesAnyCmd(cmd string, patterns []string) (matched bool, pattern string) {
	for _, p := range patterns {
		ok, err := shellGlobMatch(p, cmd)
		if err != nil {
			continue
		}
		if ok {
			return true, p
		}
	}
	return false, ""
}

// matchesAnyURL extracts the hostname from rawURL and matches it against
// shell-style glob patterns. Falls back to matching the raw URL string if
// parsing fails or yields no host. Use for deny_urls values.
func matchesAnyURL(rawURL string, patterns []string) (matched bool, pattern string) {
	host := rawURL
	if u, err := url.Parse(rawURL); err == nil && u.Hostname() != "" {
		host = u.Hostname()
	}
	return matchesAnyCmd(host, patterns)
}

// shellGlobMatch reports whether s fully matches the shell-style glob pattern p,
// where * expands to any sequence of characters (including / and space) and
// ? expands to any single character. All other regexp meta-characters in p are
// treated as literals.
func shellGlobMatch(p, s string) (bool, error) {
	var re strings.Builder
	re.WriteString("^")
	for _, ch := range p {
		switch ch {
		case '*':
			re.WriteString(".*")
		case '?':
			re.WriteString(".")
		case '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|', '\\':
			re.WriteByte('\\')
			re.WriteRune(ch)
		default:
			re.WriteRune(ch)
		}
	}
	re.WriteString("$")
	return regexp.MatchString(re.String(), s)
}
