package execute_test

import (
	"testing"

	"github.com/et-do/no-pilot/internal/testutil"
)

func TestKillTerminal_stopsRunningSession(t *testing.T) {
	resetTerminalState(t)
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	start := callExecuteTool(t, c, "execute_runInTerminal", map[string]any{
		"command": "cat",
		"mode":    "async",
	})
	if start.IsError {
		t.Fatalf("unexpected async start error: %v", start.Content)
	}
	id := extractTerminalID(t, getText(start))

	kill := callExecuteTool(t, c, "execute_killTerminal", map[string]any{"id": id})
	if kill.IsError {
		t.Fatalf("unexpected kill error: %v", kill.Content)
	}
}
