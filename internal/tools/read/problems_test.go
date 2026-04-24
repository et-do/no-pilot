package read_test

import (
	"context"
	"os"
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
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			text = tc.Text
		}
	}
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
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			text = tc.Text
		}
	}
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
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			text = tc.Text
		}
	}
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
