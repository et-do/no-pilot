package read_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// helpers: newClient, defaultConfig are in helpers_test.go
func callProblems(t *testing.T, c *client.Client, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = "read_problems"
	req.Params.Arguments = args
	result, err := c.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	return result
}

func TestProblems_fileNotFound(t *testing.T) {
	cfg := defaultConfig(t)
	c := newClient(t, cfg)
	res := callProblems(t, c, map[string]any{"filePath": "doesnotexist.go"})
	text := textContent(t, res)
	if res.IsError {
		t.Fatalf("unexpected error: %v", text)
	}
	if got, want := text, "file not found"; !strings.Contains(got, want) {
		t.Errorf("got %q, want substring %q", got, want)
	}
}

func TestProblems_validGoFile_noProblems(t *testing.T) {
	f, err := os.CreateTemp("", "ok.go")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString("package main\nfunc main() {}\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	cfg := defaultConfig(t)
	c := newClient(t, cfg)
	res := callProblems(t, c, map[string]any{"filePath": f.Name()})
	text := textContent(t, res)
	if res.IsError {
		t.Fatalf("unexpected error: %v", text)
	}
	if got, want := text, "no problems found"; !strings.Contains(got, want) {
		t.Errorf("got %q, want substring %q", got, want)
	}
}

func TestProblems_invalidGoFile_reportsError(t *testing.T) {
	f, err := os.CreateTemp("", "bad.go")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString("package main\nfunc main( {\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	cfg := defaultConfig(t)
	c := newClient(t, cfg)
	res := callProblems(t, c, map[string]any{"filePath": f.Name()})
	text := textContent(t, res)
	if res.IsError {
		t.Fatalf("unexpected error: %v", text)
	}
	if got, want := text, f.Name(); !strings.Contains(got, want) {
		t.Errorf("got %q, want substring %q", got, want)
	}
	if got, want := text, "expected ')'"; !strings.Contains(got, want) {
		t.Errorf("got %q, want substring %q", got, want)
	}
}

func TestProblems_directoryScan_reportsNestedErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.go"), []byte("package p\nfunc OK() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	badPath := filepath.Join(dir, "bad.go")
	if err := os.WriteFile(badPath, []byte("package p\nfunc Bad( {\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := defaultConfig(t)
	c := newClient(t, cfg)
	res := callProblems(t, c, map[string]any{"path": dir})
	text := textContent(t, res)
	if res.IsError {
		t.Fatalf("unexpected error: %v", text)
	}
	if got, want := text, badPath; !strings.Contains(got, want) {
		t.Errorf("got %q, want substring %q", got, want)
	}
}

func TestProblems_pathsSet_supportsArrayStyle(t *testing.T) {
	dir := t.TempDir()
	okPath := filepath.Join(dir, "ok.go")
	if err := os.WriteFile(okPath, []byte("package p\nfunc OK() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	badPath := filepath.Join(dir, "bad.go")
	if err := os.WriteFile(badPath, []byte("package p\nfunc Bad( {\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := defaultConfig(t)
	c := newClient(t, cfg)
	res := callProblems(t, c, map[string]any{"paths": []any{okPath, badPath}})
	text := textContent(t, res)
	if res.IsError {
		t.Fatalf("unexpected error: %v", text)
	}
	if got, want := text, badPath; !strings.Contains(got, want) {
		t.Errorf("got %q, want substring %q", got, want)
	}
}

func TestProblems_reportsCompilerUndefinedName(t *testing.T) {
	dir, err := os.MkdirTemp(".", "problems-compile-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	badPath := filepath.Join(dir, "bad_test.go")
	if err := os.WriteFile(badPath, []byte("package compilebad\nimport \"testing\"\nfunc TestBad(t *testing.T){ _ = missingSymbol }\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := defaultConfig(t)
	c := newClient(t, cfg)
	res := callProblems(t, c, map[string]any{"path": dir})
	text := textContent(t, res)
	if res.IsError {
		t.Fatalf("unexpected error: %v", text)
	}
	if got, want := text, "undefined"; !strings.Contains(got, want) {
		t.Errorf("got %q, want substring %q", got, want)
	}
}
