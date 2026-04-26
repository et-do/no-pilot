package search_test

import (
	"os"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/testutil"
)

func TestGrepSearch_basicMatch(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/test.txt"
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString("hello world\nfoo bar\nhello again\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := testutil.CallTool(t, c, "search_grepSearch", map[string]any{"query": "hello", "includePattern": "*.txt", "workingDir": tmp})
	text := testutil.TextContent(res)
	if res.IsError {
		t.Fatalf("unexpected error: %v", text)
	}
	if got, want := text, "hello"; !strings.Contains(got, want) {
		t.Errorf("got %q, want substring %q", got, want)
	}
}

func TestGrepSearch_noMatch(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/test.txt"
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString("foo bar\nno match here\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := testutil.CallTool(t, c, "search_grepSearch", map[string]any{"query": "hello", "includePattern": "*.txt", "workingDir": tmp})
	text := testutil.TextContent(res)
	if res.IsError {
		t.Fatalf("unexpected error: %v", text)
	}
	if text != "" {
		t.Errorf("expected empty result, got %q", text)
	}
}
