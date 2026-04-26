package execute_test

import (
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/testutil"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func callRunInTerminal(t *testing.T, c *client.Client, cmd string) *mcp.CallToolResult {
	t.Helper()
	return callExecuteTool(t, c, "execute_runInTerminal", map[string]any{"command": cmd})
}

func TestRunInTerminal_echo(t *testing.T) {
	resetTerminalState(t)
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	result := callRunInTerminal(t, c, "echo hello world")
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	out := getText(result)
	if !strings.Contains(out, "hello world") {
		t.Errorf("output = %q, want contains 'hello world'", out)
	}
}

func TestRunInTerminal_exitNonZero(t *testing.T) {
	resetTerminalState(t)
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	result := callRunInTerminal(t, c, "sh -c 'exit 42'")
	if !result.IsError {
		t.Error("IsError = false, want true (nonzero exit)")
	}
	out := getText(result)
	if !strings.Contains(out, "exit code: 42") {
		t.Errorf("output = %q, want exit code 42", out)
	}
}

func TestRunInTerminal_commandNotFound(t *testing.T) {
	resetTerminalState(t)
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	result := callRunInTerminal(t, c, "nonexistent_command_12345")
	if !result.IsError {
		t.Error("IsError = false, want true (command not found)")
	}
	out := getText(result)
	if !strings.Contains(out, "not found") && !strings.Contains(out, "No such file") {
		t.Errorf("output = %q, want error message", out)
	}
}

func TestRunInTerminal_toolDeniedByPolicy(t *testing.T) {
	resetTerminalState(t)
	f := false
	cfg := &config.Config{
		Tools: map[string]config.ToolPolicy{
			"execute_runInTerminal": {Allowed: &f},
		},
	}
	c := testutil.NewClient(t, cfg)
	result := callRunInTerminal(t, c, "echo should not run")
	if !result.IsError {
		t.Error("IsError = false, want true (tool denied)")
	}
}

func TestRunInTerminal_denyCommandsBlocks(t *testing.T) {
	resetTerminalState(t)
	f := true
	cfg := &config.Config{
		Tools: map[string]config.ToolPolicy{
			"execute_runInTerminal": {
				Allowed:      &f,
				DenyCommands: []string{"rm *"},
			},
		},
	}
	c := testutil.NewClient(t, cfg)
	result := callRunInTerminal(t, c, "rm -rf /tmp")
	if !result.IsError {
		t.Error("IsError = false, want true (denyCommands blocks)")
	}
}

func TestRunInTerminal_allowCommandsAllowsOnlyListed(t *testing.T) {
	resetTerminalState(t)
	f := true
	cfg := &config.Config{
		Tools: map[string]config.ToolPolicy{
			"execute_runInTerminal": {
				Allowed:       &f,
				AllowCommands: []string{"echo *"},
			},
		},
	}
	c := testutil.NewClient(t, cfg)
	result := callRunInTerminal(t, c, "echo allowed")
	if result.IsError {
		t.Errorf("IsError = true, want false (echo allowed)")
	}
	result2 := callRunInTerminal(t, c, "ls -l")
	if !result2.IsError {
		t.Error("IsError = false, want true (ls not in allowCommands)")
	}
}

func TestRunInTerminal_syncTimeoutReturnsTerminalID(t *testing.T) {
	resetTerminalState(t)
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	result := callExecuteTool(t, c, "execute_runInTerminal", map[string]any{
		"command": "printf start && sleep 5",
		"mode":    "sync",
		"timeout": 10,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	text := getText(result)
	if !strings.Contains(text, "status: running") {
		t.Fatalf("expected running status, got %q", text)
	}
	id := extractTerminalID(t, text)
	_ = callExecuteTool(t, c, "execute_killTerminal", map[string]any{"id": id})
}

func TestRunInTerminal_withWorkingDirectory(t *testing.T) {
	resetTerminalState(t)
	workDir := t.TempDir()
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	result := callExecuteTool(t, c, "execute_runInTerminal", map[string]any{
		"command": "pwd",
		"cwd":     workDir,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	if !strings.Contains(getText(result), workDir) {
		t.Fatalf("expected pwd output to include %q, got %q", workDir, getText(result))
	}
}

func TestRunInTerminal_withEnvironmentEntries(t *testing.T) {
	resetTerminalState(t)
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	result := callExecuteTool(t, c, "execute_runInTerminal", map[string]any{
		"command": "printf %s \"$NO_PILOT_TEST_VAR\"",
		"env":     "NO_PILOT_TEST_VAR=terminal-env-ok",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	if !strings.Contains(getText(result), "terminal-env-ok") {
		t.Fatalf("expected injected env value in output, got %q", getText(result))
	}
}

func TestRunInTerminal_invalidEnvEntry(t *testing.T) {
	resetTerminalState(t)
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	result := callExecuteTool(t, c, "execute_runInTerminal", map[string]any{
		"command": "echo never-runs",
		"env":     "BROKEN_ENV_ENTRY",
	})
	if !result.IsError {
		t.Fatalf("expected invalid env to fail, got %q", getText(result))
	}
}
