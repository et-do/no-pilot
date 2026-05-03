package execute_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/testutil"
)

func TestGetTerminalOutput_withByteRange(t *testing.T) {
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
		"command": "abcdef",
	})
	if send.IsError {
		t.Fatalf("unexpected send error: %v", send.Content)
	}
	if !waitForTerminalOutput(t, c, id, "abcdef") {
		t.Fatalf("expected full output before slicing")
	}

	ranged := callExecuteTool(t, c, "execute_getTerminalOutput", map[string]any{
		"id":          id,
		"startOffset": 2,
		"endOffset":   5,
	})
	if ranged.IsError {
		t.Fatalf("unexpected range read error: %v", ranged.Content)
	}
	text := getText(ranged)
	if !strings.Contains(text, "range: [2,5)") {
		t.Fatalf("expected range metadata in %q", text)
	}
	if !strings.Contains(text, "cde") {
		t.Fatalf("expected sliced output cde in %q", text)
	}

	_ = callExecuteTool(t, c, "execute_killTerminal", map[string]any{"id": id})
}

func TestGetTerminalOutput_afterKilledSessionShowsCompletion(t *testing.T) {
	resetTerminalState(t)
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	start := callExecuteTool(t, c, "execute_runInTerminal", map[string]any{
		"command": "cat",
		"mode":    "async",
	})
	id := extractTerminalID(t, getText(start))
	_ = callExecuteTool(t, c, "execute_killTerminal", map[string]any{"id": id})

	output := callExecuteTool(t, c, "execute_getTerminalOutput", map[string]any{"id": id})
	if !strings.Contains(getText(output), "exit code:") {
		t.Fatalf("expected completed terminal output, got %q", getText(output))
	}
	if !output.IsError {
		t.Fatalf("expected killed process to report error exit, got %q", getText(output))
	}
}

func TestGetTerminalOutput_vscodeTargetUsesBridge(t *testing.T) {
	withBridgeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/terminal/get_output" {
			t.Fatalf("path = %q, want /terminal/get_output", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"text": "bridge output text"})
	}))
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	result := callExecuteTool(t, c, "execute_getTerminalOutput", map[string]any{
		"id":     "bridge-id",
		"target": "vscode",
	})
	if result.IsError {
		t.Fatalf("unexpected bridge error: %q", getText(result))
	}
	if !strings.Contains(getText(result), "bridge output text") {
		t.Fatalf("unexpected text: %q", getText(result))
	}
}
