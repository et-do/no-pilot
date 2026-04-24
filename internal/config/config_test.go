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

// TestLoad_allowCommandsIntersection verifies that allow_commands from user and
// project are intersected — only patterns present in both are kept.
func TestLoad_allowCommandsIntersection(t *testing.T) {
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
	// Only "go *" appears in both lists.
	if len(pol.AllowCommands) != 1 || pol.AllowCommands[0] != "go *" {
		t.Errorf("AllowCommands = %v, want [\"go *\"] (intersection)", pol.AllowCommands)
	}
}

// TestLoad_allowCommandsOneNilUsesOther verifies that if one layer has no
// allow_commands restriction the other layer's list applies.
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
	want := map[string]bool{"go *": true, "make *": true}
	if len(pol.AllowCommands) != len(want) {
		t.Fatalf("AllowCommands = %v, want %v", pol.AllowCommands, want)
	}
	for _, p := range pol.AllowCommands {
		if !want[p] {
			t.Errorf("unexpected AllowCommands entry %q", p)
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
