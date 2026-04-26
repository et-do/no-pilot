package read_test

import (
	"context"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/terminalstate"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func callTerminalLastCommand(t *testing.T, c *client.Client) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = "read_terminalLastCommand"
	result, err := c.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	return result
}

func TestTerminalLastCommand_emptyBeforeAnyCommand(t *testing.T) {
	terminalstate.Reset()
	c := newClient(t, defaultConfig(t))
	result := callTerminalLastCommand(t, c)
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	if textContent(t, result) != "" {
		t.Fatalf("expected empty result, got %q", textContent(t, result))
	}
}

func TestTerminalLastCommand_returnsMostRecentRunInTerminal(t *testing.T) {
	terminalstate.Reset()
	terminalstate.Store("echo hello-last-command", "hello-last-command\n(exit code: 0, duration: 1ms)")
	c := newClient(t, defaultConfig(t))
	result := callTerminalLastCommand(t, c)
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	text := textContent(t, result)
	if !strings.Contains(text, "command: echo hello-last-command") {
		t.Fatalf("missing command in result: %q", text)
	}
	if !strings.Contains(text, "hello-last-command") {
		t.Fatalf("missing output in result: %q", text)
	}
}

func TestTerminalLastCommand_toolDeniedByPolicy(t *testing.T) {
	terminalstate.Reset()
	f := false
	cfg := &config.Config{Tools: map[string]config.ToolPolicy{
		"read_terminalLastCommand": {Allowed: &f},
	}}
	c := newClient(t, cfg)
	result := callTerminalLastCommand(t, c)
	if !result.IsError {
		t.Fatal("IsError = false, want true when tool denied")
	}
}

func TestTerminalLastCommand_stateIsIndependentOfReadPolicies(t *testing.T) {
	terminalstate.Reset()
	terminalstate.Store("printf terminal-state", "terminal-state\n(exit code: 0, duration: 1ms)")

	readAllowed := true
	cfg := &config.Config{Tools: map[string]config.ToolPolicy{
		"read_terminalLastCommand": {
			Allowed:   &readAllowed,
			DenyPaths: []string{"**/secret/**"},
		},
	}}
	c := newClient(t, cfg)
	result := callTerminalLastCommand(t, c)
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	if !strings.Contains(textContent(t, result), "terminal-state") {
		t.Fatalf("expected recorded output, got %q", textContent(t, result))
	}
}
