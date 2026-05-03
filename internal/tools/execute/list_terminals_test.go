package execute_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/testutil"
)

func TestListTerminals_includesSessionMetadata(t *testing.T) {
	resetTerminalState(t)
	workDir := t.TempDir()
	c := testutil.NewClient(t, testutil.DefaultConfig(t))

	result := callExecuteTool(t, c, "execute_runInTerminal", map[string]any{
		"command": "sleep 5",
		"mode":    "async",
		"cwd":     workDir,
		"env":     "FOO=bar",
	})
	if result.IsError {
		t.Fatalf("unexpected async start error: %v", result.Content)
	}
	id := extractTerminalID(t, getText(result))

	list := callExecuteTool(t, c, "execute_listTerminals", map[string]any{})
	if list.IsError {
		t.Fatalf("unexpected list error: %v", list.Content)
	}
	text := getText(list)
	if !strings.Contains(text, "id="+id) {
		t.Fatalf("expected terminal id in list, got %q", text)
	}
	if !strings.Contains(text, "cwd=\""+workDir+"\"") {
		t.Fatalf("expected cwd metadata in list, got %q", text)
	}
	if !strings.Contains(text, "env=1") {
		t.Fatalf("expected env count in list, got %q", text)
	}

	_ = callExecuteTool(t, c, "execute_killTerminal", map[string]any{"id": id})
}

func TestListTerminals_vscodeTargetUsesBridge(t *testing.T) {
	withBridgeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/terminal/list" {
			t.Fatalf("path = %q, want /terminal/list", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"text": "bridge list output"})
	}))

	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	result := callExecuteTool(t, c, "execute_listTerminals", map[string]any{"target": "vscode"})
	if result.IsError {
		t.Fatalf("unexpected bridge error: %q", getText(result))
	}
	if !strings.Contains(getText(result), "bridge list output") {
		t.Fatalf("unexpected text: %q", getText(result))
	}
}
