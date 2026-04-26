package execute_test

import (
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
