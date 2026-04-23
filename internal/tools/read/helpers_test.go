package read_test

import (
	"context"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	nopilotserver "github.com/et-do/no-pilot/internal/server"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)
func newClient(t *testing.T, cfg *config.Config) *client.Client {
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

func defaultConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

// textSlice splits a single TextContent result into lines (for listDirectory).
func textSlice(result *mcp.CallToolResult) []string {
	var out []string
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			if tc.Text == "" {
				continue
			}
			out = append(out, splitLines(tc.Text)...)
		}
	}
	return out
}

func splitLines(s string) []string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}
