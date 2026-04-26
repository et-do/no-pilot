package read_test

import (
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/testutil"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func newClient(t *testing.T, cfg *config.Config) *client.Client {
	return testutil.NewClient(t, cfg)
}

func defaultConfig(t *testing.T) *config.Config {
	return testutil.DefaultConfig(t)
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
