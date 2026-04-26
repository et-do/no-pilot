package execute_test

import (
	"testing"

	"github.com/et-do/no-pilot/internal/testutil"
)

func TestSendToTerminal_asyncSessionEchoesInput(t *testing.T) {
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

	_ = callExecuteTool(t, c, "execute_killTerminal", map[string]any{"id": id})
}
