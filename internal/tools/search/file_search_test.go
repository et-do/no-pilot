package search_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func callFileSearch(t *testing.T, c testClient, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = "search_fileSearch"
	req.Params.Arguments = args
	result, err := c.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	return result
}

type testClient interface {
	CallTool(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
}

func extractText(res *mcp.CallToolResult) string {
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func TestFileSearch_globMatch(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "a", "one.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "b", "two.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "b", "skip.go"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := defaultConfig(t)
	c := newClient(t, cfg)
	res := callFileSearch(t, c, map[string]any{"query": filepath.Join(tmp, "**", "*.txt")})
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(res))
	}

	text := extractText(res)
	if !strings.Contains(text, "one.txt") || !strings.Contains(text, "two.txt") {
		t.Fatalf("expected txt files in result, got %q", text)
	}
	if strings.Contains(text, "skip.go") {
		t.Fatalf("unexpected non-matching file in result: %q", text)
	}
}

func TestFileSearch_maxResults(t *testing.T) {
	tmp := t.TempDir()
	for i := 0; i < 4; i++ {
		name := filepath.Join(tmp, "f"+string(rune('a'+i))+".txt")
		if err := os.WriteFile(name, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cfg := defaultConfig(t)
	c := newClient(t, cfg)
	res := callFileSearch(t, c, map[string]any{
		"query":      filepath.Join(tmp, "*.txt"),
		"maxResults": 2,
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(res))
	}

	text := strings.TrimSpace(extractText(res))
	if text == "" {
		t.Fatal("expected at least one result")
	}
	lines := strings.Split(text, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 results, got %d: %q", len(lines), text)
	}
}

func TestFileSearch_absoluteDirScope(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "nested", "a.go"), []byte("package main"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "nested", "b.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := defaultConfig(t)
	c := newClient(t, cfg)
	res := callFileSearch(t, c, map[string]any{"query": tmp})
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(res))
	}

	text := extractText(res)
	if !strings.Contains(text, "a.go") || !strings.Contains(text, "b.txt") {
		t.Fatalf("expected directory-scope results, got %q", text)
	}
}
