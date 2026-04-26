package execute_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/et-do/no-pilot/internal/terminalstate"
	"github.com/et-do/no-pilot/internal/testutil"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func callExecuteTool(t *testing.T, c *client.Client, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	return testutil.CallTool(t, c, name, args)
}

func getText(result *mcp.CallToolResult) string {
	return testutil.TextContent(result)
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

func resetTerminalState(t *testing.T) {
	t.Helper()
	terminalstate.Reset()
	t.Cleanup(terminalstate.Reset)
}
