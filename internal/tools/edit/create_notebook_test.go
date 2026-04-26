package edit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/et-do/no-pilot/internal/testutil"
)

func TestCreateNotebook_createsNotebookFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "n.ipynb")
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := callEditTool(t, c, "edit_createNotebook", map[string]any{
		"filePath": path,
		"query":    "Seed content",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(res))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("invalid notebook JSON: %v", err)
	}
	cells, ok := doc["cells"].([]any)
	if !ok || len(cells) != 1 {
		t.Fatalf("expected one seeded cell, got %v", doc["cells"])
	}
}

func TestCreateNotebook_rejectsExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "n.ipynb")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := callEditTool(t, c, "edit_createNotebook", map[string]any{"filePath": path})
	if !res.IsError {
		t.Fatal("IsError = false, want true when notebook exists")
	}
}
