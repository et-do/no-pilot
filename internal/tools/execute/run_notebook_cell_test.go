package execute_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/testutil"
)

const notebookRunFixture = `{
  "cells": [
    {
      "cell_type": "markdown",
      "metadata": {"id": "m1", "language": "markdown"},
      "source": ["# title"]
    },
    {
      "cell_type": "code",
      "metadata": {"id": "c1", "language": "python"},
      "source": ["x = 1\n"]
    },
    {
      "cell_type": "code",
      "metadata": {"id": "c2", "language": "python"},
      "source": ["print(x + 1)\n"]
    }
  ],
  "metadata": {},
  "nbformat": 4,
  "nbformat_minor": 5
}`

func TestRunNotebookCell_executesCodeAndPersistsOutput(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}

	path := filepath.Join(t.TempDir(), "sample.ipynb")
	if err := os.WriteFile(path, []byte(notebookRunFixture), 0o600); err != nil {
		t.Fatal(err)
	}

	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := callExecuteTool(t, c, "execute_runNotebookCell", map[string]any{
		"filePath": path,
		"cellId":   "c2",
	})
	if res.IsError {
		t.Fatalf("unexpected run notebook error: %s", getText(res))
	}
	if !strings.Contains(getText(res), "2") {
		t.Fatalf("expected output containing 2, got %q", getText(res))
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
	if len(cells) < 3 {
		t.Fatalf("expected notebook cells, got %v", doc["cells"])
	}
	cell, _ := cells[2].(map[string]any)
	if _, ok := cell["outputs"]; !ok {
		t.Fatalf("expected outputs on target cell")
	}
}

func TestRunNotebookCell_rejectsMarkdownTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.ipynb")
	if err := os.WriteFile(path, []byte(notebookRunFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := callExecuteTool(t, c, "execute_runNotebookCell", map[string]any{
		"filePath": path,
		"cellId":   "m1",
	})
	if !res.IsError {
		t.Fatal("IsError = false, want true for markdown cell target")
	}
}

func TestRunNotebookCell_missingCell(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.ipynb")
	if err := os.WriteFile(path, []byte(notebookRunFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := callExecuteTool(t, c, "execute_runNotebookCell", map[string]any{
		"filePath": path,
		"cellId":   "missing",
	})
	if !res.IsError {
		t.Fatal("IsError = false, want true for missing cell")
	}
}

func TestRunNotebookCell_denyPathBlocked(t *testing.T) {
	tf := true
	cfg := &config.Config{Tools: map[string]config.ToolPolicy{
		"execute_runNotebookCell": {Allowed: &tf, DenyPaths: []string{"**/secret/**"}},
	}}
	path := filepath.Join(t.TempDir(), "secret", "sample.ipynb")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(notebookRunFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	c := testutil.NewClient(t, cfg)
	res := callExecuteTool(t, c, "execute_runNotebookCell", map[string]any{
		"filePath": path,
		"cellId":   "c2",
	})
	if !res.IsError {
		t.Fatal("IsError = false, want true when deny_paths blocks")
	}
}
