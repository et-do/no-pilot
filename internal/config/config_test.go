package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
)

// writeFile creates a file at path with the given content, creating any
// necessary parent directories.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestLoad_noConfigFiles verifies that Load succeeds and returns a usable
// Config when neither the user nor project config files exist.
func TestLoad_noConfigFiles(t *testing.T) {
	dir := t.TempDir()
	// Point user config to an empty temp dir so no file is found there either.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil Config")
	}
}

// TestLoad_defaultPolicyIsAllowed verifies that a tool with no explicit policy
// is permitted by default.
func TestLoad_defaultPolicyIsAllowed(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Policy("read_file").IsAllowed() {
		t.Error("Policy(read_file).IsAllowed() = false, want true (default)")
	}
}

// TestLoad_projectConfigDisablesTool verifies that a project-level policy can
// deny a tool.
func TestLoad_projectConfigDisablesTool(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".no-pilot.yaml"), `
tools:
  execute_runInTerminal:
    allowed: false
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Policy("execute_runInTerminal").IsAllowed() {
		t.Error("Policy(execute_runInTerminal).IsAllowed() = true, want false")
	}
}

// TestLoad_projectConfigSetsDenyPaths verifies that deny_paths from the
// project config are surfaced correctly.
func TestLoad_projectConfigSetsDenyPaths(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".no-pilot.yaml"), `
tools:
  read_readFile:
    deny_paths:
      - "**/.env"
      - "**/secrets/**"
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	pol := cfg.Policy("read_readFile")
	if !pol.IsAllowed() {
		t.Error("Policy(read_readFile).IsAllowed() = false, want true")
	}
	want := []string{"**/.env", "**/secrets/**"}
	if len(pol.DenyPaths) != len(want) {
		t.Fatalf("DenyPaths = %v, want %v", pol.DenyPaths, want)
	}
	for i, pattern := range want {
		if pol.DenyPaths[i] != pattern {
			t.Errorf("DenyPaths[%d] = %q, want %q", i, pol.DenyPaths[i], pattern)
		}
	}
}

// TestLoad_userDenyIsStickyOverProject verifies zero-trust merge: once a user
// denies a tool, the project cannot re-enable it.
func TestLoad_userDenyIsStickyOverProject(t *testing.T) {
	userCfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userCfgDir)

	// User config denies the tool.
	writeFile(t, filepath.Join(userCfgDir, "no-pilot", "config.yaml"), `
tools:
  execute_runInTerminal:
    allowed: false
`)

	// Project config attempts to re-enable it — must be ignored.
	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, ".no-pilot.yaml"), `
tools:
  execute_runInTerminal:
    allowed: true
`)

	cfg, err := config.Load(projectDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Policy("execute_runInTerminal").IsAllowed() {
		t.Error("Policy(execute_runInTerminal).IsAllowed() = true, want false (user deny is sticky)")
	}
}

// TestLoad_userPolicyAppliedWhenNoProjectConfig verifies that the user-level
// policy is applied when the project has no config file.
func TestLoad_userPolicyAppliedWhenNoProjectConfig(t *testing.T) {
	userCfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userCfgDir)

	writeFile(t, filepath.Join(userCfgDir, "no-pilot", "config.yaml"), `
tools:
  read_readFile:
    allowed: false
`)

	cfg, err := config.Load(t.TempDir()) // project dir has no .no-pilot.yaml
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Policy("read_readFile").IsAllowed() {
		t.Error("Policy(read_readFile).IsAllowed() = true, want false (user config)")
	}
}

// TestLoad_denyPathsUnion verifies that deny_paths from user and project configs
// are unioned — all patterns from both layers accumulate.
func TestLoad_denyPathsUnion(t *testing.T) {
	userCfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userCfgDir)

	writeFile(t, filepath.Join(userCfgDir, "no-pilot", "config.yaml"), `
tools:
  read_readFile:
    deny_paths:
      - "**/.env"
`)

	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, ".no-pilot.yaml"), `
tools:
  read_readFile:
    deny_paths:
      - "**/secrets/**"
`)

	cfg, err := config.Load(projectDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	pol := cfg.Policy("read_readFile")
	want := map[string]bool{"**/.env": true, "**/secrets/**": true}
	if len(pol.DenyPaths) != len(want) {
		t.Fatalf("DenyPaths len = %d, want %d; got %v", len(pol.DenyPaths), len(want), pol.DenyPaths)
	}
	for _, p := range pol.DenyPaths {
		if !want[p] {
			t.Errorf("unexpected DenyPaths entry %q", p)
		}
	}
}

// TestLoad_allowCommandsLayers verifies that allow_commands from user and project
// configs are accumulated as separate layers in AllowCommandLayers. A command
// must satisfy every layer that defines an allowlist (logical AND across layers,
// not a string-set intersection of patterns).
func TestLoad_allowCommandsLayers(t *testing.T) {
	userCfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userCfgDir)

	writeFile(t, filepath.Join(userCfgDir, "no-pilot", "config.yaml"), `
tools:
  execute_runInTerminal:
    allow_commands:
      - "go *"
      - "git *"
`)

	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, ".no-pilot.yaml"), `
tools:
  execute_runInTerminal:
    allow_commands:
      - "go *"
      - "npm *"
`)

	cfg, err := config.Load(projectDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	pol := cfg.Policy("execute_runInTerminal")

	// Two layers must be accumulated: user's and project's.
	if len(pol.AllowCommandLayers) != 2 {
		t.Fatalf("AllowCommandLayers len = %d, want 2; got %v", len(pol.AllowCommandLayers), pol.AllowCommandLayers)
	}

	// Layer 0: user's allowlist.
	wantUser := map[string]bool{"go *": true, "git *": true}
	if len(pol.AllowCommandLayers[0]) != len(wantUser) {
		t.Errorf("layer[0] = %v, want %v", pol.AllowCommandLayers[0], wantUser)
	}
	for _, p := range pol.AllowCommandLayers[0] {
		if !wantUser[p] {
			t.Errorf("unexpected pattern %q in layer[0]", p)
		}
	}

	// Layer 1: project's allowlist.
	wantProj := map[string]bool{"go *": true, "npm *": true}
	if len(pol.AllowCommandLayers[1]) != len(wantProj) {
		t.Errorf("layer[1] = %v, want %v", pol.AllowCommandLayers[1], wantProj)
	}
	for _, p := range pol.AllowCommandLayers[1] {
		if !wantProj[p] {
			t.Errorf("unexpected pattern %q in layer[1]", p)
		}
	}
}

// TestLoad_allowCommandsOneNilUsesOther verifies that if one layer has no
// allow_commands restriction the other layer's list is the sole layer in
// AllowCommandLayers, so its allowlist still applies.
func TestLoad_allowCommandsOneNilUsesOther(t *testing.T) {
	userCfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userCfgDir)

	// User has no allow_commands.
	writeFile(t, filepath.Join(userCfgDir, "no-pilot", "config.yaml"), `
tools:
  execute_runInTerminal:
    allowed: true
`)

	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, ".no-pilot.yaml"), `
tools:
  execute_runInTerminal:
    allow_commands:
      - "go *"
      - "make *"
`)

	cfg, err := config.Load(projectDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	pol := cfg.Policy("execute_runInTerminal")

	// Only one layer since the user config had no allow_commands.
	if len(pol.AllowCommandLayers) != 1 {
		t.Fatalf("AllowCommandLayers len = %d, want 1; got %v", len(pol.AllowCommandLayers), pol.AllowCommandLayers)
	}
	want := map[string]bool{"go *": true, "make *": true}
	if len(pol.AllowCommandLayers[0]) != len(want) {
		t.Fatalf("AllowCommandLayers[0] = %v, want %v", pol.AllowCommandLayers[0], want)
	}
	for _, p := range pol.AllowCommandLayers[0] {
		if !want[p] {
			t.Errorf("unexpected AllowCommandLayers[0] entry %q", p)
		}
	}
}

// TestLoad_malformedYAML verifies that a parse error is returned for invalid YAML.
func TestLoad_malformedYAML(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".no-pilot.yaml"), `{invalid: yaml: [`)

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load() error = nil, want parse error")
	}
}

// TestLoad_invalidDenyPathPattern verifies that an invalid doublestar glob in
// deny_paths causes Load to return an error. This ensures fail-closed behaviour:
// a misconfigured deny rule is rejected at startup rather than silently skipped
// at enforcement time (which would be fail-open).
func TestLoad_invalidDenyPathPattern(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".no-pilot.yaml"), `
tools:
  read_readFile:
    deny_paths:
      - "[invalid"
`)

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load() error = nil, want error for invalid glob pattern")
	}
}

// TestLoad_invalidDenyCommandPattern verifies that deny_commands patterns are
// validated at load time to avoid silently ineffective deny rules.
func TestLoad_invalidDenyCommandPattern(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".no-pilot.yaml"), `
tools:
  execute_runInTerminal:
    deny_commands:
      - "  "
`)

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load() error = nil, want error for invalid deny_commands pattern")
	}
}

// TestLoad_invalidDenyURLPattern verifies that deny_urls patterns are
// validated at load time to avoid silently ineffective deny rules.
func TestLoad_invalidDenyURLPattern(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".no-pilot.yaml"), `
tools:
  web_fetch:
    deny_urls:
      - " *.internal"
`)

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load() error = nil, want error for invalid deny_urls pattern")
	}
}

// TestLoad_denyShellEscapeIsSticky verifies that deny_shell_escape follows
// zero-trust OR semantics: once true in any config layer it cannot be unset
// by a subsequent layer.
func TestLoad_denyShellEscapeIsSticky(t *testing.T) {
	userCfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userCfgDir)

	// User config enables deny_shell_escape.
	writeFile(t, filepath.Join(userCfgDir, "no-pilot", "config.yaml"), `
tools:
  execute_runInTerminal:
    deny_shell_escape: true
`)

	// Project config does NOT set deny_shell_escape (defaults to false).
	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, ".no-pilot.yaml"), `
tools:
  execute_runInTerminal:
    allowed: true
`)

	cfg, err := config.Load(projectDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	pol := cfg.Policy("execute_runInTerminal")
	if !pol.DenyShellEscape {
		t.Error("DenyShellEscape = false after merge, want true (sticky once set)")
	}
}

// TestLoad_denyShellEscapeCanBeAddedByProjectConfig verifies that a project
// config can tighten security by enabling deny_shell_escape even when the user
// config does not set it.
func TestLoad_denyShellEscapeCanBeAddedByProjectConfig(t *testing.T) {
	userCfgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userCfgDir)

	// User config does NOT set deny_shell_escape.
	writeFile(t, filepath.Join(userCfgDir, "no-pilot", "config.yaml"), `
tools:
  execute_runInTerminal:
    allowed: true
`)

	// Project config enables deny_shell_escape (tightening).
	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, ".no-pilot.yaml"), `
tools:
  execute_runInTerminal:
    deny_shell_escape: true
`)

	cfg, err := config.Load(projectDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	pol := cfg.Policy("execute_runInTerminal")
	if !pol.DenyShellEscape {
		t.Error("DenyShellEscape = false after merge, want true (project config can add restriction)")
	}
}
