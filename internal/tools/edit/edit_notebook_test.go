package edit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/testutil"
)

const notebookFixture = `{
  "cells": [
    {
      "cell_type": "markdown",
      "metadata": {"id": "m1", "language": "markdown"},
      "source": ["hello"]
    },
    {
      "cell_type": "code",
      "metadata": {"id": "c1", "language": "python"},
      "source": ["print('x')"]
    }
  ],
  "metadata": {},
  "nbformat": 4,
  "nbformat_minor": 5
}`

func TestEditNotebook_insertEditDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "n.ipynb")
	if err := os.WriteFile(path, []byte(notebookFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	c := testutil.NewClient(t, testutil.DefaultConfig(t))

	insert := callEditTool(t, c, "edit_editNotebook", map[string]any{
		"filePath": path,
		"editType": "insert",
		"cellId":   "c1",
		"newCode":  []any{"print('inserted')"},
		"language": "python",
	})
	if insert.IsError {
		t.Fatalf("insert failed: %s", resultText(insert))
	}

	edit := callEditTool(t, c, "edit_editNotebook", map[string]any{
		"filePath": path,
		"editType": "edit",
		"cellId":   "c1",
		"newCode":  "print('edited')",
	})
	if edit.IsError {
		t.Fatalf("edit failed: %s", resultText(edit))
	}

	deleteRes := callEditTool(t, c, "edit_editNotebook", map[string]any{
		"filePath": path,
		"editType": "delete",
		"cellId":   "m1",
	})
	if deleteRes.IsError {
		t.Fatalf("delete failed: %s", resultText(deleteRes))
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	cells, _ := doc["cells"].([]any)
	if len(cells) != 2 {
		t.Fatalf("expected 2 cells after delete, got %d", len(cells))
	}
}

func TestEditNotebook_denyPathBlocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret", "n.ipynb")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(notebookFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	tf := true
	cfg := &config.Config{Tools: map[string]config.ToolPolicy{
		"edit_editNotebook": {Allowed: &tf, DenyPaths: []string{"**/secret/**"}},
	}}
	c := testutil.NewClient(t, cfg)
	res := callEditTool(t, c, "edit_editNotebook", map[string]any{
		"filePath": path,
		"editType": "delete",
		"cellId":   "m1",
	})
	if !res.IsError {
		t.Fatal("IsError = false, want true when deny_paths blocks")
	}
}
