package execute_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/testutil"
)

func TestCreateAndRunTask_writesTaskAndRuns(t *testing.T) {
	resetTerminalState(t)
	workspace := t.TempDir()
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := callExecuteTool(t, c, "execute_createAndRunTask", map[string]any{
		"workspaceFolder": workspace,
		"label":           "echo-task",
		"command":         "echo",
		"args":            []any{"hello-task"},
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", getText(res))
	}
	if !strings.Contains(getText(res), "hello-task") {
		t.Fatalf("expected command output, got %q", getText(res))
	}

	tasksPath := filepath.Join(workspace, ".vscode", "tasks.json")
	b, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("tasks.json not created: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("invalid tasks.json: %v", err)
	}
}

func TestCreateAndRunTask_backgroundReturnsTerminalID(t *testing.T) {
	resetTerminalState(t)
	workspace := t.TempDir()
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := callExecuteTool(t, c, "execute_createAndRunTask", map[string]any{
		"workspaceFolder": workspace,
		"label":           "sleep-task",
		"command":         "sleep",
		"args":            []any{"5"},
		"isBackground":    true,
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", getText(res))
	}
	if !strings.Contains(getText(res), "terminal_id:") {
		t.Fatalf("expected terminal id in background mode, got %q", getText(res))
	}
	id := extractTerminalID(t, getText(res))
	_ = callExecuteTool(t, c, "execute_killTerminal", map[string]any{"id": id})
}

func TestCreateAndRunTask_denyPathBlocked(t *testing.T) {
	resetTerminalState(t)
	tf := true
	cfg := &config.Config{Tools: map[string]config.ToolPolicy{
		"execute_createAndRunTask": {Allowed: &tf, DenyPaths: []string{"**/blocked/**"}},
	}}
	workspace := filepath.Join(t.TempDir(), "blocked", "ws")
	c := testutil.NewClient(t, cfg)
	res := callExecuteTool(t, c, "execute_createAndRunTask", map[string]any{
		"workspaceFolder": workspace,
		"label":           "echo-task",
		"command":         "echo",
	})
	if !res.IsError {
		t.Fatal("IsError = false, want true when deny_paths blocks")
	}
}
