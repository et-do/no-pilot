package testutil

import (
	"context"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	nopilotserver "github.com/et-do/no-pilot/internal/server"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// NewClient creates and initializes an in-process MCP client against no-pilot.
func NewClient(t *testing.T, cfg *config.Config) *client.Client {
	t.Helper()
	s := nopilotserver.Build(cfg, "test")
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

// DefaultConfig loads a default config for tests with isolated user config dir.
func DefaultConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

// CallTool invokes an MCP tool by name with arguments.
func CallTool(t *testing.T, c *client.Client, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	result, err := c.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	return result
}

// TextContent returns concatenated text from text contents.
func TextContent(result *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}
