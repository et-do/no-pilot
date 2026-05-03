package execute_test

import (
	"encoding/json"
	"net/http"
	"strings"
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

func TestKillTerminal_vscodeTargetUsesBridge(t *testing.T) {
	withBridgeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/terminal/kill" {
			t.Fatalf("path = %q, want /terminal/kill", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"text": "bridge kill ok"})
	}))
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	result := callExecuteTool(t, c, "execute_killTerminal", map[string]any{"id": "bridge-id", "target": "vscode"})
	if result.IsError {
		t.Fatalf("unexpected bridge error: %q", getText(result))
	}
	if !strings.Contains(getText(result), "bridge kill ok") {
		t.Fatalf("unexpected text: %q", getText(result))
	}
}
