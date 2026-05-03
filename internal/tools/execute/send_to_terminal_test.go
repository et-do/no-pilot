package execute_test

import (
	"encoding/json"
	"net/http"
	"strings"
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

func TestSendToTerminal_vscodeTargetUsesBridge(t *testing.T) {
	withBridgeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/terminal/send" {
			t.Fatalf("path = %q, want /terminal/send", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"text": "bridge send ok"})
	}))
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	result := callExecuteTool(t, c, "execute_sendToTerminal", map[string]any{
		"id":      "bridge-id",
		"command": "hello",
		"target":  "vscode",
	})
	if result.IsError {
		t.Fatalf("unexpected bridge error: %q", getText(result))
	}
	if !strings.Contains(getText(result), "bridge send ok") {
		t.Fatalf("unexpected text: %q", getText(result))
	}
}
