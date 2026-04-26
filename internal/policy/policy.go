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
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// shellEscapePatterns are built-in deny patterns applied when DenyShellEscape
// is true. They cover the most common interpreter invocations that accept a
// -c / -e flag, allowing an arbitrary command string to run and bypass
// deny_commands glob matching (e.g. "bash -c 'rm -rf /'").
var shellEscapePatterns = []string{
	"sh -c *", "bash -c *", "zsh -c *", "dash -c *", "ksh -c *", "fish -c *",
	"python -c *", "python2 -c *", "python3 -c *",
	"perl -e *", "perl -E *",
	"ruby -e *",
	"node -e *", "node --eval *", "nodejs -e *",
}

// shellHopPatterns extract the embedded command for common shell/eval wrappers.
var shellHopPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^\s*(?:\S+/)?(?:sh|bash|zsh|dash|ksh|fish)\s+-(?:[A-Za-z]*c[A-Za-z]*)\s+(.+)\s*$`),
	regexp.MustCompile(`^\s*(?:\S+/)?(?:python|python2|python3)\s+-c\s+(.+)\s*$`),
	regexp.MustCompile(`^\s*(?:\S+/)?perl\s+-[eE]\s+(.+)\s*$`),
	regexp.MustCompile(`^\s*(?:\S+/)?ruby\s+-e\s+(.+)\s*$`),
	regexp.MustCompile(`^\s*(?:\S+/)?(?:node|nodejs)\s+(?:-e|--eval)\s+(.+)\s*$`),
}

var (
	policyLogger   = log.New(os.Stderr, "[no-pilot] ", log.LstdFlags)
	policyLogLevel = logging.LevelInfo
	requestSeq     atomic.Uint64
)

const (
	protectedProjectPolicyFile = ".no-pilot.yaml"
	maxCommandHopDepth         = 3
)

type requestIDKey struct{}

// SetLogger configures policy middleware logging behavior.
func SetLogger(logger *log.Logger, level logging.Level) {
	if logger != nil {
		policyLogger = logger
	}
	policyLogLevel = level
}

// RequestIDFromContext returns the correlation ID for a tool call, if present.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}

func ensureRequestID(ctx context.Context) (context.Context, string) {
	if id := RequestIDFromContext(ctx); id != "" {
		return ctx, id
	}
	id := fmt.Sprintf("req-%x", requestSeq.Add(1))
	return context.WithValue(ctx, requestIDKey{}, id), id
}

// Enforce returns a middleware that denies the tool call if the tool is not
// allowed by the merged policy. Argument values are not checked; use
// EnforceWithPaths, EnforceWithCommand, or EnforceWithURL for tools that
// accept constrained arguments.
func Enforce(cfg config.Provider, toolName string) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ctx, reqID := ensureRequestID(ctx)
			started := time.Now()
			if result := checkAllowed(cfg, toolName); result != nil {
				logPolicyDecision(reqID, toolName, "deny", "tool_disabled", started)
				return result, nil
			}
			result, err := next(ctx, req)
			logPolicyResult(reqID, toolName, result, err, started)
			return result, err
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
func EnforceWithPaths(cfg config.Provider, toolName string, pathArgs ...string) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ctx, reqID := ensureRequestID(ctx)
			started := time.Now()
			if result := checkAllowed(cfg, toolName); result != nil {
				logPolicyDecision(reqID, toolName, "deny", "tool_disabled", started)
				return result, nil
			}

			pol := cfg.Policy(toolName)
			if len(pathArgs) > 0 {
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
					// Clean the path before matching so that traversal sequences
					// (e.g. /workspace/secrets/../../etc/passwd -> /etc/passwd)
					// are resolved to the same form the OS will use, preventing
					// deny_paths patterns from being bypassed via ".." components.
					cleanPath := filepath.Clean(pathStr)

					// System-level immutable guard: no edit tool may ever write
					// project policy, regardless of configured deny_paths.
					if isProtectedPolicyWrite(toolName, cleanPath) {
						logPolicyDecision(reqID, toolName, "deny", "protected_policy_file", started)
						return mcp.NewToolResultError(fmt.Sprintf(
							"path %q is protected by system policy", pathStr,
						)), nil
					}

					if matched, pattern := matchesAny(cleanPath, pol.DenyPaths); matched {
						return mcp.NewToolResultError(fmt.Sprintf(
							"path %q is denied by policy pattern %q", pathStr, pattern,
						)), nil
					}
				}
			}

			result, err := next(ctx, req)
			logPolicyResult(reqID, toolName, result, err, started)
			return result, err
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
func EnforceWithCommand(cfg config.Provider, toolName string, commandArg string) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ctx, reqID := ensureRequestID(ctx)
			started := time.Now()
			if result := checkAllowed(cfg, toolName); result != nil {
				logPolicyDecision(reqID, toolName, "deny", "tool_disabled", started)
				return result, nil
			}

			pol := cfg.Policy(toolName)
			args := req.GetArguments()
			val, ok := args[commandArg]
			if !ok {
				result, err := next(ctx, req)
				logPolicyResult(reqID, toolName, result, err, started)
				return result, err
			}
			cmd, ok := val.(string)
			if !ok {
				result, err := next(ctx, req)
				logPolicyResult(reqID, toolName, result, err, started)
				return result, err
			}

			if reason, detail := evaluateCommandPolicy(pol, toolName, cmd, 0); reason != "" {
				logPolicyDecision(reqID, toolName, "deny", reason, started)
				return mcp.NewToolResultError(detail), nil
			}

			result, err := next(ctx, req)
			logPolicyResult(reqID, toolName, result, err, started)
			return result, err
		}
	}
}

// EnforceWithURL returns a middleware that:
//  1. Denies the call if the tool is not allowed.
//  2. Denies the call if the named URL argument matches any pattern in deny_urls.
//
// urlArg is the argument key whose value holds the URL string (e.g. "url").
// A missing or non-string argument is skipped silently.
func EnforceWithURL(cfg config.Provider, toolName string, urlArg string) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ctx, reqID := ensureRequestID(ctx)
			started := time.Now()
			if result := checkAllowed(cfg, toolName); result != nil {
				logPolicyDecision(reqID, toolName, "deny", "tool_disabled", started)
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
							logPolicyDecision(reqID, toolName, "deny", fmt.Sprintf("url_denied:%s", pattern), started)
							return mcp.NewToolResultError(fmt.Sprintf(
								"URL %q is denied by policy pattern %q", urlStr, pattern,
							)), nil
						}
					}
				}
			}

			result, err := next(ctx, req)
			logPolicyResult(reqID, toolName, result, err, started)
			return result, err
		}
	}
}

// checkAllowed returns a denial result if the tool is not allowed, or nil if
// the call may proceed.
func checkAllowed(cfg config.Provider, toolName string) *mcp.CallToolResult {
	if !cfg.Policy(toolName).IsAllowed() {
		return mcp.NewToolResultError(fmt.Sprintf("tool %q is disabled by policy", toolName))
	}
	return nil
}

func logPolicyDecision(reqID, toolName, outcome, reason string, started time.Time) {
	if !logging.Enabled(policyLogLevel, logging.LevelInfo) {
		return
	}
	duration := time.Since(started).Round(time.Millisecond)
	policyLogger.Printf("[policy] request_id=%s tool=%s outcome=%s reason=%s duration=%s", reqID, toolName, outcome, reason, duration)
}

func logPolicyResult(reqID, toolName string, result *mcp.CallToolResult, err error, started time.Time) {
	if err != nil {
		if logging.Enabled(policyLogLevel, logging.LevelError) {
			duration := time.Since(started).Round(time.Millisecond)
			policyLogger.Printf("[policy] request_id=%s tool=%s outcome=%s reason=%s duration=%s", reqID, toolName, "error", err.Error(), duration)
		}
		return
	}
	if result != nil && result.IsError {
		logPolicyDecision(reqID, toolName, "deny", "tool_result_error", started)
		return
	}
	logPolicyDecision(reqID, toolName, "allow", "ok", started)
}

func isProtectedPolicyWrite(toolName, cleanPath string) bool {
	if !strings.HasPrefix(toolName, "edit_") {
		return false
	}
	return filepath.Base(cleanPath) == protectedProjectPolicyFile
}

func evaluateCommandPolicy(pol config.ToolPolicy, toolName, cmd string, hop int) (reason, detail string) {
	// System-level immutable guard: runInTerminal cannot target .no-pilot.yaml,
	// even if the YAML policy would otherwise allow the command.
	if toolName == "execute_runInTerminal" && referencesProtectedPolicyFile(cmd) {
		return "protected_policy_file_command", fmt.Sprintf("command %q targets protected policy file %q", cmd, protectedProjectPolicyFile)
	}

	// Allowlist check: command must match at least one pattern in every
	// configured allow_commands layer (logical AND across config layers).
	// Fall back to AllowCommands for ToolPolicy values built directly
	// (not via config.Load) that have no AllowCommandLayers set.
	layers := pol.AllowCommandLayers
	if len(layers) == 0 && len(pol.AllowCommands) > 0 {
		layers = [][]string{pol.AllowCommands}
	}
	for _, layer := range layers {
		if matched, _ := matchesAnyCmd(cmd, layer); !matched {
			return "command_not_allowed", fmt.Sprintf("command %q is not permitted by policy allow_commands", cmd)
		}
	}

	// Denylist check: shell escape patterns (when deny_shell_escape is
	// enabled) are evaluated before user-defined deny patterns. This
	// ensures that even if an allow_commands entry permits "bash *",
	// the hardened patterns still block "bash -c '<arbitrary command>'".
	if pol.DenyShellEscape {
		if matched, pattern := matchesAnyCmd(cmd, shellEscapePatterns); matched {
			return fmt.Sprintf("deny_shell_escape:%s", pattern), fmt.Sprintf(
				"command %q is blocked by deny_shell_escape (matched built-in pattern %q)", cmd, pattern,
			)
		}
	}

	// Denylist check: command must not match any deny pattern.
	if matched, pattern := matchesAnyCmd(cmd, pol.DenyCommands); matched {
		return fmt.Sprintf("command_denied:%s", pattern), fmt.Sprintf(
			"command %q is denied by policy pattern %q", cmd, pattern,
		)
	}

	// Monotonic attenuation across shell hops: for commands that spawn an
	// embedded command via -c/-e/--eval, recursively require the embedded
	// payload to satisfy the same policy envelope.
	if hop < maxCommandHopDepth {
		if inner, ok := extractEmbeddedCommand(cmd); ok && inner != "" && inner != cmd {
			if nestedReason, nestedDetail := evaluateCommandPolicy(pol, toolName, inner, hop+1); nestedReason != "" {
				return fmt.Sprintf("nested_%s", nestedReason), nestedDetail
			}
		}
	}

	return "", ""
}

func extractEmbeddedCommand(cmd string) (string, bool) {
	for _, re := range shellHopPatterns {
		matches := re.FindStringSubmatch(cmd)
		if len(matches) != 2 {
			continue
		}
		inner := strings.TrimSpace(matches[1])
		if len(inner) >= 2 {
			if (inner[0] == '\'' && inner[len(inner)-1] == '\'') || (inner[0] == '"' && inner[len(inner)-1] == '"') {
				inner = inner[1 : len(inner)-1]
			}
		}
		inner = strings.TrimSpace(inner)
		if inner == "" {
			return "", false
		}
		return inner, true
	}
	return "", false
}

func referencesProtectedPolicyFile(cmd string) bool {
	return strings.Contains(strings.ToLower(cmd), strings.ToLower(protectedProjectPolicyFile))
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
// shell-style glob patterns. Patterns are matched against the hostname only,
// not the full URL — use patterns like "*.internal" or "169.254.*" rather
// than full URL patterns. Falls back to matching the raw string if URL
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
