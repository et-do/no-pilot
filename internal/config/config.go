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

	"gopkg.in/yaml.v3"
)

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

	// AllowCommands is a whitelist of shell command glob patterns. When set,
	// a command argument must match at least one pattern to be permitted.
	AllowCommands []string `yaml:"allow_commands"`

	// DenyCommands is a list of shell command glob patterns; any command
	// argument matching one of these patterns causes the call to be rejected.
	// Deny is evaluated after Allow.
	DenyCommands []string `yaml:"deny_commands"`

	// DenyURLs is a list of URL glob patterns; any URL argument matching one
	// of these patterns causes the call to be rejected.
	DenyURLs []string `yaml:"deny_urls"`
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

	projCfg, err := loadFile(filepath.Join(projectDir, ".no-pilot.yaml"))
	if err != nil {
		return nil, err
	}
	merge(cfg, projCfg)

	return cfg, nil
}

// userConfigPath returns the platform-appropriate path to the user config file.
func userConfigPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "no-pilot", "config.yaml")
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
	return &cfg, nil
}

// merge applies zero-trust semantics when overlaying src onto dst:
//
//   - Allowed: false is sticky — once a tool is denied it cannot be
//     re-enabled by a subsequent config layer.
//   - DenyPaths, DenyCommands, DenyURLs: union — every denial from every
//     config layer accumulates.
//   - AllowCommands: intersection — a command must satisfy every allowlist
//     that has been configured; if only one layer restricts, that list applies.
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
			dst.Tools[name] = srcPol
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

	// AllowCommands: intersection (most restrictive of both layers).
	out.AllowCommands = intersectAllowCommands(dst.AllowCommands, src.AllowCommands)

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

// intersectAllowCommands returns the intersection of two allowlists.
// If one side is empty (no restriction), the other side's list applies.
// If both are set, only patterns present in both are kept.
func intersectAllowCommands(a, b []string) []string {
	if len(a) == 0 {
		return append([]string(nil), b...)
	}
	if len(b) == 0 {
		return append([]string(nil), a...)
	}
	set := make(map[string]struct{}, len(b))
	for _, p := range b {
		set[p] = struct{}{}
	}
	var out []string
	for _, p := range a {
		if _, ok := set[p]; ok {
			out = append(out, p)
		}
	}
	return out
}
