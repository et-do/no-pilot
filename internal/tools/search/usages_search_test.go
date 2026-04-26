package search_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/testutil"
)

func TestUsagesSearch_symbolMatchesAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.go")
	f2 := filepath.Join(dir, "b.go")
	if err := os.WriteFile(f1, []byte("package p\nfunc TargetSym() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("package p\nfunc call(){ TargetSym() }\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := testutil.CallTool(t, c, "search_usages", map[string]any{
		"symbol":     "TargetSym",
		"filePath":   f1,
		"maxResults": 10,
	})
	text := testutil.TextContent(res)
	if res.IsError {
		t.Fatalf("unexpected usages error: %q", text)
	}
	if !strings.Contains(text, "a.go") || !strings.Contains(text, "b.go") {
		t.Fatalf("expected matches in both files, got %q", text)
	}
}
