package config_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/et-do/no-pilot/internal/config"
)

func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal(msg)
}

func hasString(vals []string, needle string) bool {
	for _, v := range vals {
		if v == needle {
			return true
		}
	}
	return false
}

func TestWatcher_initialLoad(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, ".no-pilot.yaml"), `
tools:
  execute_runInTerminal:
    deny_commands:
      - "rm *"
`)

	w, err := config.NewWatcher(projectDir, nil)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Fatalf("Watcher.Close() error = %v", err)
		}
	}()

	pol := w.Policy("execute_runInTerminal")
	if !hasString(pol.DenyCommands, "rm *") {
		t.Fatalf("DenyCommands = %v, want to contain %q", pol.DenyCommands, "rm *")
	}
}

func TestWatcher_reloadProjectConfigOnWrite(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	projectDir := t.TempDir()
	cfgPath := filepath.Join(projectDir, ".no-pilot.yaml")
	writeFile(t, cfgPath, `
tools:
  execute_runInTerminal:
    allow_commands:
      - "go *"
`)

	w, err := config.NewWatcher(projectDir, nil)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Fatalf("Watcher.Close() error = %v", err)
		}
	}()

	writeFile(t, cfgPath, `
tools:
  execute_runInTerminal:
    allow_commands:
      - "make *"
`)

	waitFor(t, 2*time.Second, func() bool {
		pol := w.Policy("execute_runInTerminal")
		return hasString(pol.AllowCommands, "make *")
	}, "watcher did not reload updated project config")
}

func TestWatcher_invalidReloadKeepsLastGoodConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	projectDir := t.TempDir()
	cfgPath := filepath.Join(projectDir, ".no-pilot.yaml")
	writeFile(t, cfgPath, `
tools:
  execute_runInTerminal:
    allow_commands:
      - "go *"
`)

	w, err := config.NewWatcher(projectDir, nil)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Fatalf("Watcher.Close() error = %v", err)
		}
	}()

	// Invalid YAML should be rejected on reload; previous policy must remain.
	writeFile(t, cfgPath, `{invalid: yaml: [`)

	// Give the watcher loop time to attempt reload and ensure snapshot remains usable.
	time.Sleep(150 * time.Millisecond)
	pol := w.Policy("execute_runInTerminal")
	if !hasString(pol.AllowCommands, "go *") {
		t.Fatalf("AllowCommands after invalid reload = %v, want previous value to be retained", pol.AllowCommands)
	}
}

func TestWatcher_reloadUserConfigOnWrite(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	userCfgPath := filepath.Join(xdg, "no-pilot", "config.yaml")
	writeFile(t, userCfgPath, `
tools:
  read_readFile:
    allowed: false
`)

	projectDir := t.TempDir()
	w, err := config.NewWatcher(projectDir, nil)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Fatalf("Watcher.Close() error = %v", err)
		}
	}()

	if w.Policy("read_readFile").IsAllowed() {
		t.Fatal("initial IsAllowed() = true, want false")
	}

	writeFile(t, userCfgPath, `
tools:
  read_readFile:
    allowed: true
`)

	waitFor(t, 2*time.Second, func() bool {
		return w.Policy("read_readFile").IsAllowed()
	}, "watcher did not reload updated user config")
}
