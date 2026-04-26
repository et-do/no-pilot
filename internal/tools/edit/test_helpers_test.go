package edit_test

import (
	"testing"

	"github.com/et-do/no-pilot/internal/testutil"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func callEditTool(t *testing.T, c *client.Client, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	return testutil.CallTool(t, c, name, args)
}

func resultText(result *mcp.CallToolResult) string {
	return testutil.TextContent(result)
}
