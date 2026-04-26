package edit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/testutil"
)

func TestEditFiles_singleReplace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := callEditTool(t, c, "edit_editFiles", map[string]any{
		"filePath":  path,
		"oldString": "world",
		"newString": "there",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(res))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello there" {
		t.Fatalf("content = %q, want %q", string(b), "hello there")
	}
}

func TestEditFiles_multipleEditsArrayInput(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "a.txt")
	p2 := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(p1, []byte("x x x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p2, []byte("abc def"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := callEditTool(t, c, "edit_editFiles", map[string]any{
		"edits": []any{
			map[string]any{"filePath": p1, "oldString": "x", "newString": "z", "replaceAll": true},
			map[string]any{"filePath": p2, "oldString": "def", "newString": "ghi"},
		},
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(res))
	}
	b1, _ := os.ReadFile(p1)
	b2, _ := os.ReadFile(p2)
	if string(b1) != "z z z" {
		t.Fatalf("a.txt = %q", string(b1))
	}
	if string(b2) != "abc ghi" {
		t.Fatalf("b.txt = %q", string(b2))
	}
}

func TestEditFiles_oldStringNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := callEditTool(t, c, "edit_editFiles", map[string]any{
		"filePath":  path,
		"oldString": "missing",
		"newString": "x",
	})
	if !res.IsError {
		t.Fatal("IsError = false, want true when oldString missing")
	}
	if !strings.Contains(resultText(res), "oldString not found") {
		t.Fatalf("unexpected error text: %q", resultText(res))
	}
}

func TestEditFiles_denyPathBlocked(t *testing.T) {
	tf := true
	cfg := &config.Config{Tools: map[string]config.ToolPolicy{
		"edit_editFiles": {Allowed: &tf, DenyPaths: []string{"**/secret/**"}},
	}}
	path := filepath.Join(t.TempDir(), "secret", "f.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := testutil.NewClient(t, cfg)
	res := callEditTool(t, c, "edit_editFiles", map[string]any{
		"filePath":  path,
		"oldString": "a",
		"newString": "b",
	})
	if !res.IsError {
		t.Fatal("IsError = false, want true when deny_paths blocks")
	}
}
