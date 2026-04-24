package execute_test

import (
	"context"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	executeserver "github.com/et-do/no-pilot/internal/server"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func newClient(t *testing.T, cfg *config.Config) *client.Client {
	t.Helper()
	s := executeserver.Build(cfg, "test")
	c, err := client.NewInProcessClient(s)
	if err != nil {
		t.Fatalf("NewInProcessClient: %v", err)
	}
	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("client.Start: %v", err)
	}
	if _, err := c.Initialize(ctx, mcp.InitializeRequest{}); err != nil {
		t.Fatalf("client.Initialize: %v", err)
	}
	return c
}

func defaultConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func callRunInTerminal(t *testing.T, c *client.Client, cmd string) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = "execute_runInTerminal"
	req.Params.Arguments = map[string]any{"command": cmd}
	result, err := c.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	return result
}

func TestRunInTerminal_echo(t *testing.T) {
	c := newClient(t, defaultConfig(t))
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
	c := newClient(t, defaultConfig(t))
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
	c := newClient(t, defaultConfig(t))
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
	f := false
	cfg := &config.Config{
		Tools: map[string]config.ToolPolicy{
			"execute_runInTerminal": {Allowed: &f},
		},
	}
	c := newClient(t, cfg)
	result := callRunInTerminal(t, c, "echo should not run")
	if !result.IsError {
		t.Error("IsError = false, want true (tool denied)")
	}
}

func TestRunInTerminal_denyCommandsBlocks(t *testing.T) {
	f := true
	cfg := &config.Config{
		Tools: map[string]config.ToolPolicy{
			"execute_runInTerminal": {
				Allowed:      &f,
				DenyCommands: []string{"rm *"},
			},
		},
	}
	c := newClient(t, cfg)
	result := callRunInTerminal(t, c, "rm -rf /tmp")
	if !result.IsError {
		t.Error("IsError = false, want true (denyCommands blocks)")
	}
}

func TestRunInTerminal_allowCommandsAllowsOnlyListed(t *testing.T) {
	f := true
	cfg := &config.Config{
		Tools: map[string]config.ToolPolicy{
			"execute_runInTerminal": {
				Allowed:       &f,
				AllowCommands: []string{"echo *"},
			},
		},
	}
	c := newClient(t, cfg)
	result := callRunInTerminal(t, c, "echo allowed")
	if result.IsError {
		t.Errorf("IsError = true, want false (echo allowed)")
	}
	result2 := callRunInTerminal(t, c, "ls -l")
	if !result2.IsError {
		t.Error("IsError = false, want true (ls not in allowCommands)")
	}
}

func getText(result *mcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}
