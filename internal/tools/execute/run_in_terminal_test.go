package execute_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/terminalstate"
	"github.com/et-do/no-pilot/internal/testutil"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func callRunInTerminal(t *testing.T, c *client.Client, cmd string) *mcp.CallToolResult {
	t.Helper()
	return callExecuteTool(t, c, "execute_runInTerminal", map[string]any{"command": cmd})
}

func callExecuteTool(t *testing.T, c *client.Client, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	return testutil.CallTool(t, c, name, args)
}

func TestRunInTerminal_echo(t *testing.T) {
	terminalstate.Reset()
	t.Cleanup(terminalstate.Reset)
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
	terminalstate.Reset()
	t.Cleanup(terminalstate.Reset)
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	result := callRunInTerminal(t, c, "sh -c 'exit 42'")
	if !result.IsError {
		t.Error("IsError = false, want true (nonzero exit)")
	}
	// Parse exit code from output
	out := getText(result)
	if !strings.Contains(out, "exit code: 42") {
		t.Errorf("output = %q, want exit code 42", out)
	}
}

func TestRunInTerminal_commandNotFound(t *testing.T) {
	terminalstate.Reset()
	t.Cleanup(terminalstate.Reset)
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
	terminalstate.Reset()
	t.Cleanup(terminalstate.Reset)
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
	terminalstate.Reset()
	t.Cleanup(terminalstate.Reset)
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
	terminalstate.Reset()
	t.Cleanup(terminalstate.Reset)
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
	terminalstate.Reset()
	t.Cleanup(terminalstate.Reset)
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
	terminalstate.Reset()
	t.Cleanup(terminalstate.Reset)
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
	terminalstate.Reset()
	t.Cleanup(terminalstate.Reset)
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
	terminalstate.Reset()
	t.Cleanup(terminalstate.Reset)
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	result := callExecuteTool(t, c, "execute_runInTerminal", map[string]any{
		"command": "echo never-runs",
		"env":     "BROKEN_ENV_ENTRY",
	})
	if !result.IsError {
		t.Fatalf("expected invalid env to fail, got %q", getText(result))
	}
}

func TestTerminalSession_asyncSendOutputAndKill(t *testing.T) {
	terminalstate.Reset()
	t.Cleanup(terminalstate.Reset)
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	start := callExecuteTool(t, c, "execute_runInTerminal", map[string]any{
		"command": "cat",
		"mode":    "async",
	})
	if start.IsError {
		t.Fatalf("unexpected async start error: %v", start.Content)
	}
	id := extractTerminalID(t, getText(start))

	send := callExecuteTool(t, c, "execute_sendToTerminal", map[string]any{
		"id":      id,
		"command": "hello session",
	})
	if send.IsError {
		t.Fatalf("unexpected send error: %v", send.Content)
	}

	if !waitForTerminalOutput(t, c, id, "hello session") {
		output := callExecuteTool(t, c, "execute_getTerminalOutput", map[string]any{"id": id})
		t.Fatalf("expected echoed input in terminal output, got %q", getText(output))
	}

	kill := callExecuteTool(t, c, "execute_killTerminal", map[string]any{"id": id})
	if kill.IsError {
		t.Fatalf("unexpected kill error: %v", kill.Content)
	}

	output := callExecuteTool(t, c, "execute_getTerminalOutput", map[string]any{"id": id})
	if !strings.Contains(getText(output), "exit code:") {
		t.Fatalf("expected completed terminal output, got %q", getText(output))
	}
	if !output.IsError {
		t.Fatalf("expected killed process to report error exit, got %q", getText(output))
	}
}

func TestGetTerminalOutput_withByteRange(t *testing.T) {
	terminalstate.Reset()
	t.Cleanup(terminalstate.Reset)
	c := testutil.NewClient(t, testutil.DefaultConfig(t))

	start := callExecuteTool(t, c, "execute_runInTerminal", map[string]any{
		"command": "cat",
		"mode":    "async",
	})
	if start.IsError {
		t.Fatalf("unexpected async start error: %v", start.Content)
	}
	id := extractTerminalID(t, getText(start))

	send := callExecuteTool(t, c, "execute_sendToTerminal", map[string]any{
		"id":      id,
		"command": "abcdef",
	})
	if send.IsError {
		t.Fatalf("unexpected send error: %v", send.Content)
	}
	if !waitForTerminalOutput(t, c, id, "abcdef") {
		t.Fatalf("expected full output before slicing")
	}

	ranged := callExecuteTool(t, c, "execute_getTerminalOutput", map[string]any{
		"id":          id,
		"startOffset": 2,
		"endOffset":   5,
	})
	if ranged.IsError {
		t.Fatalf("unexpected range read error: %v", ranged.Content)
	}
	text := getText(ranged)
	if !strings.Contains(text, "range: [2,5)") {
		t.Fatalf("expected range metadata in %q", text)
	}
	if !strings.Contains(text, "cde") {
		t.Fatalf("expected sliced output cde in %q", text)
	}

	_ = callExecuteTool(t, c, "execute_killTerminal", map[string]any{"id": id})
}

func TestListTerminals_includesSessionMetadata(t *testing.T) {
	terminalstate.Reset()
	t.Cleanup(terminalstate.Reset)
	workDir := t.TempDir()
	c := testutil.NewClient(t, testutil.DefaultConfig(t))

	result := callExecuteTool(t, c, "execute_runInTerminal", map[string]any{
		"command": "sleep 5",
		"mode":    "async",
		"cwd":     workDir,
		"env":     "FOO=bar",
	})
	if result.IsError {
		t.Fatalf("unexpected async start error: %v", result.Content)
	}
	id := extractTerminalID(t, getText(result))

	list := callExecuteTool(t, c, "execute_listTerminals", map[string]any{})
	if list.IsError {
		t.Fatalf("unexpected list error: %v", list.Content)
	}
	text := getText(list)
	if !strings.Contains(text, "id="+id) {
		t.Fatalf("expected terminal id in list, got %q", text)
	}
	if !strings.Contains(text, "cwd=\""+workDir+"\"") {
		t.Fatalf("expected cwd metadata in list, got %q", text)
	}
	if !strings.Contains(text, "env=1") {
		t.Fatalf("expected env count in list, got %q", text)
	}

	_ = callExecuteTool(t, c, "execute_killTerminal", map[string]any{"id": id})
}

func extractTerminalID(t *testing.T, text string) string {
	t.Helper()
	match := regexp.MustCompile(`terminal_id: ([a-zA-Z0-9-]+)`).FindStringSubmatch(text)
	if len(match) != 2 {
		t.Fatalf("failed to extract terminal id from %q", text)
	}
	return match[1]
}

func waitForTerminalOutput(t *testing.T, c *client.Client, id, want string) bool {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		result := callExecuteTool(t, c, "execute_getTerminalOutput", map[string]any{"id": id})
		if strings.Contains(getText(result), want) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func getText(result *mcp.CallToolResult) string {
	return testutil.TextContent(result)
}
