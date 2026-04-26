// Package config loads and merges no-pilot policy from two sources:
//
//  1. User-level config: $XDG_CONFIG_HOME/no-pilot/config.yaml (Linux) or
//     ~/Library/Application Support/no-pilot/config.yaml (macOS)
//  2. Project-level config: .no-pilot.yaml in the workspace root
//
// The two configs are merged with zero-trust semantics: restrictions can only
// tighten as layers accumulate. A tool disabled at the user level cannot be
// re-enabled by a project config, and deny lists from both layers are unioned.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

const (
	// ProjectPolicyFileName is the only project-level policy filename loaded by no-pilot.
	ProjectPolicyFileName = ".no-pilot.yaml"

	// StrictSelfProtectEnv enables fail-closed validation that the project policy
	// explicitly denies editing itself via deny_paths. When enabled and invalid,
	// config loading fails and the server will not start.
	StrictSelfProtectEnv = "NO_PILOT_STRICT_SELF_PROTECT"
)

// Provider is the interface satisfied by both *Config (static snapshot) and
// *Watcher (live-reloading). All policy middleware and tool registration
// functions accept Provider so the server can be built against either.
type Provider interface {
	Policy(tool string) ToolPolicy
}

// Config holds the merged no-pilot policy.
type Config struct {
	Tools map[string]ToolPolicy `yaml:"tools"`
}

// ToolPolicy defines whether a tool is enabled and any argument-level restrictions.
type ToolPolicy struct {
	// Allowed controls whether the tool may be invoked at all.
	// A nil pointer means the tool inherits the default (allowed).
	Allowed *bool `yaml:"allowed"`

	// DenyPaths is a list of doublestar glob patterns; any file-path argument
	// matching one of these patterns causes the call to be rejected.
	DenyPaths []string `yaml:"deny_paths"`

	// AllowCommands is an allowlist of shell command glob patterns. When set,
	// a command argument must match at least one pattern to be permitted.
	AllowCommands []string `yaml:"allow_commands"`

	// DenyCommands is a list of shell command glob patterns; any command
	// argument matching one of these patterns causes the call to be rejected.
	// Deny is evaluated after Allow.
	DenyCommands []string `yaml:"deny_commands"`

	// DenyURLs is a list of hostname glob patterns; any URL argument whose
	// hostname matches one of these patterns causes the call to be rejected.
	DenyURLs []string `yaml:"deny_urls"`

	// DenyShellEscape, when true, blocks common interpreter invocations that
	// accept a -c / -e flag (sh, bash, python -c, perl -e, node -e, etc.).
	// These patterns are enforced in addition to DenyCommands and cannot be
	// removed by a later config layer (zero-trust sticky: once true, stays true).
	DenyShellEscape bool `yaml:"deny_shell_escape"`

	// AllowCommandLayers holds the per-config-layer allow_commands lists
	// accumulated during a multi-file merge. A command must match at least one
	// pattern in every layer. Not parsed from YAML; populated by merge only.
	AllowCommandLayers [][]string `yaml:"-"`
}

// IsAllowed reports whether the tool is permitted.
// Tools are allowed by default when no explicit policy is set.
func (p ToolPolicy) IsAllowed() bool {
	if p.Allowed == nil {
		return true
	}
	return *p.Allowed
}

// Policy returns the effective ToolPolicy for the named tool.
// If no policy is configured for the tool, a zero-value ToolPolicy is
// returned, which defaults to allowed with no deny patterns.
func (c *Config) Policy(tool string) ToolPolicy {
	if c.Tools == nil {
		return ToolPolicy{}
	}
	return c.Tools[tool]
}

// Load reads the user-level config and overlays the project-level config.
// Missing config files are silently skipped.
func Load(projectDir string) (*Config, error) {
	cfg := &Config{}

	userCfg, err := loadFile(userConfigPath())
	if err != nil {
		return nil, err
	}
	merge(cfg, userCfg)

	projectCfgPath := filepath.Join(projectDir, ProjectPolicyFileName)
	projectCfgExists := fileExists(projectCfgPath)
	projCfg, err := loadFile(projectCfgPath)
	if err != nil {
		return nil, err
	}
	if projectCfgExists && strictSelfProtectEnabled() {
		if err := validateSelfProtectPolicy(projCfg, projectCfgPath); err != nil {
			return nil, err
		}
	}
	merge(cfg, projCfg)

	return cfg, nil
}

// userConfigPath returns the platform-appropriate path to the user config file.
func userConfigPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "no-pilot", "config.yaml")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func strictSelfProtectEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(StrictSelfProtectEnv)))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func validateSelfProtectPolicy(cfg *Config, path string) error {
	if cfg == nil || len(cfg.Tools) == 0 {
		return fmt.Errorf("config %s: strict self-protect is enabled but tools policy is empty", path)
	}
	hasEnabledEditTool := false
	for tool, pol := range cfg.Tools {
		if !strings.HasPrefix(tool, "edit_") {
			continue
		}
		if pol.Allowed != nil && !*pol.Allowed {
			continue
		}
		hasEnabledEditTool = true
		if hasSelfDenyPath(pol.DenyPaths) {
			return nil
		}
	}
	if !hasEnabledEditTool {
		return nil
	}
	return fmt.Errorf("config %s: strict self-protect requires at least one enabled edit_* tool to deny path %q in deny_paths", path, ProjectPolicyFileName)
}

func hasSelfDenyPath(patterns []string) bool {
	for _, p := range patterns {
		pat := strings.TrimSpace(p)
		if pat == "" {
			continue
		}
		if filepath.Base(pat) == ProjectPolicyFileName {
			return true
		}
		if ok, err := doublestar.Match(pat, ProjectPolicyFileName); err == nil && ok {
			return true
		}
		if ok, err := doublestar.Match(pat, filepath.ToSlash("workspace/"+ProjectPolicyFileName)); err == nil && ok {
			return true
		}
	}
	return false
}

// loadFile parses a YAML config file. A missing file returns an empty Config
// without error, since both config files are optional.
func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := validatePatterns(&cfg, path); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// validatePatterns checks that every deny_paths pattern in cfg is a valid
// doublestar glob. An invalid pattern would be silently skipped at enforcement
// time (fail-open), so we reject it at load time instead to fail-closed.
func validatePatterns(cfg *Config, path string) error {
	for toolName, pol := range cfg.Tools {
		for _, p := range pol.DenyPaths {
			if _, err := doublestar.Match(p, ""); err != nil {
				return fmt.Errorf("config %s: tool %q: deny_paths: invalid glob pattern %q: %w", path, toolName, p, err)
			}
		}
		for _, p := range pol.DenyCommands {
			if err := validateShellGlobPattern(p); err != nil {
				return fmt.Errorf("config %s: tool %q: deny_commands: invalid pattern %q: %w", path, toolName, p, err)
			}
		}
		for _, p := range pol.DenyURLs {
			if err := validateShellGlobPattern(p); err != nil {
				return fmt.Errorf("config %s: tool %q: deny_urls: invalid pattern %q: %w", path, toolName, p, err)
			}
		}
	}
	return nil
}

// validateShellGlobPattern verifies a shell-style glob pattern can be safely
// interpreted by the command/URL matcher and avoids easy-to-miss mistakes.
func validateShellGlobPattern(pattern string) error {
	if strings.TrimSpace(pattern) == "" {
		return errors.New("pattern must not be empty or whitespace-only")
	}
	if strings.TrimSpace(pattern) != pattern {
		return errors.New("pattern must not have leading or trailing whitespace")
	}
	if strings.ContainsAny(pattern, "\n\r\t") {
		return errors.New("pattern must be a single line without tabs")
	}
	if _, err := regexp.Compile(shellGlobRegex(pattern)); err != nil {
		return fmt.Errorf("compile shell glob: %w", err)
	}
	return nil
}

func shellGlobRegex(pattern string) string {
	var re strings.Builder
	re.WriteString("^")
	for _, ch := range pattern {
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
	return re.String()
}

// merge applies zero-trust semantics when overlaying src onto dst:
//
//   - Allowed: false is sticky — once a tool is denied it cannot be
//     re-enabled by a subsequent config layer.
//   - DenyPaths, DenyCommands, DenyURLs: union — every denial from every
//     config layer accumulates.
//   - AllowCommands: each config layer's list is kept separately in
//     AllowCommandLayers; a command must satisfy the allowlist from every
//     layer that defines one.
func merge(dst, src *Config) {
	if len(src.Tools) == 0 {
		return
	}
	if dst.Tools == nil {
		dst.Tools = make(map[string]ToolPolicy, len(src.Tools))
	}
	for name, srcPol := range src.Tools {
		dstPol, exists := dst.Tools[name]
		if !exists {
			pol := srcPol
			if len(pol.AllowCommands) > 0 {
				pol.AllowCommandLayers = [][]string{pol.AllowCommands}
			}
			dst.Tools[name] = pol
			continue
		}
		dst.Tools[name] = mergePolicy(dstPol, srcPol)
	}
}

// mergePolicy merges two ToolPolicy values according to zero-trust rules.
func mergePolicy(dst, src ToolPolicy) ToolPolicy {
	out := dst

	// Allowed: false is sticky.
	if src.Allowed != nil {
		if dst.Allowed == nil {
			out.Allowed = src.Allowed
		} else if !*src.Allowed {
			// src denies → stays denied regardless of dst.
			f := false
			out.Allowed = &f
		}
		// src.Allowed=true + dst.Allowed=false → stays false (zero-trust).
		// src.Allowed=true + dst.Allowed=true  → stays true (no change needed).
	}

	// Deny lists: union.
	out.DenyPaths = unionStrings(dst.DenyPaths, src.DenyPaths)
	out.DenyCommands = unionStrings(dst.DenyCommands, src.DenyCommands)
	out.DenyURLs = unionStrings(dst.DenyURLs, src.DenyURLs)

	// DenyShellEscape is sticky: once true in any layer it stays true.
	if src.DenyShellEscape {
		out.DenyShellEscape = true
	}

	// AllowCommands: accumulate layers — a command must satisfy each config
	// layer's allowlist independently (logical AND across layers).
	out.AllowCommandLayers = dst.AllowCommandLayers
	if len(src.AllowCommands) > 0 {
		newLayers := make([][]string, len(dst.AllowCommandLayers)+1)
		copy(newLayers, dst.AllowCommandLayers)
		newLayers[len(dst.AllowCommandLayers)] = src.AllowCommands
		out.AllowCommandLayers = newLayers
	}

	return out
}

// unionStrings returns a new slice containing all elements of a followed by
// elements of b that are not already in a.
func unionStrings(a, b []string) []string {
	if len(b) == 0 {
		return append([]string(nil), a...)
	}
	if len(a) == 0 {
		return append([]string(nil), b...)
	}
	seen := make(map[string]struct{}, len(a))
	for _, s := range a {
		seen[s] = struct{}{}
	}
	out := append([]string(nil), a...)
	for _, s := range b {
		if _, ok := seen[s]; !ok {
			out = append(out, s)
		}
	}
	return out
}
