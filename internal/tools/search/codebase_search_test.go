package search_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/testutil"
)

func TestCodebaseSearch_returnsRankedSnippets(t *testing.T) {
	dir, err := os.MkdirTemp(".", "codebase-search-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	file1 := filepath.Join(dir, "alpha.go")
	file2 := filepath.Join(dir, "beta.go")
	const token = "UNIQUE_NO_PILOT_CODEBASE_TOKEN"
	if err := os.WriteFile(file1, []byte("package p\n// UNIQUE_NO_PILOT_CODEBASE_TOKEN alpha beta\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("package p\n// UNIQUE_NO_PILOT_CODEBASE_TOKEN alpha\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := testutil.CallTool(t, c, "search_codebase", map[string]any{
		"query":      token + " alpha beta",
		"maxResults": 2,
	})
	text := testutil.TextContent(res)
	if res.IsError {
		t.Fatalf("unexpected codebase error: %q", text)
	}
	if !strings.Contains(text, token) {
		t.Fatalf("expected unique token in result, got %q", text)
	}
	if len(strings.Split(strings.TrimSpace(text), "\n")) > 2 {
		t.Fatalf("expected maxResults limit, got %q", text)
	}
}
