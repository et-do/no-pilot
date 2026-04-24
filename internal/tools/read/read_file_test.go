package read_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// --- helpers ---

// writeFile creates file at path with content, making parent dirs as needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// callReadFile is a convenience wrapper around client.CallTool.
func callReadFile(t *testing.T, c *client.Client, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = "read_readFile"
	req.Params.Arguments = args
	result, err := c.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	return result
}

// textContent extracts all text from a non-error result.
func textContent(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	var sb strings.Builder
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

// --- registration ---

// TestReadFile_registeredAsTool verifies that read/readFile appears in the
// server's tool list.
func TestReadFile_registeredAsTool(t *testing.T) {
	c := newClient(t, defaultConfig(t))

	resp, err := c.ListTools(context.Background(), mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	for _, tool := range resp.Tools {
		if tool.Name == "read_readFile" {
			return // found
		}
	}
	t.Error("read/readFile not found in tool list")
}

// --- happy path ---

// TestReadFile_readsFullFile verifies that the full file content is returned
// when no line range is specified.
func TestReadFile_readsFullFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	writeFile(t, path, "line one\nline two\nline three\n")

	c := newClient(t, defaultConfig(t))
	result := callReadFile(t, c, map[string]any{"filePath": path})

	if result.IsError {
		t.Fatalf("unexpected error result: %v", textContent(t, result))
	}
	got := textContent(t, result)
	if !strings.Contains(got, "line one") || !strings.Contains(got, "line three") {
		t.Errorf("content = %q, want all three lines", got)
	}
}

// TestReadFile_startAndEndLine verifies that a sub-range is returned correctly.
func TestReadFile_startAndEndLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lines.txt")
	writeFile(t, path, "alpha\nbeta\ngamma\ndelta\nepsilon\n")

	c := newClient(t, defaultConfig(t))
	result := callReadFile(t, c, map[string]any{
		"filePath":  path,
		"startLine": 2,
		"endLine":   4,
	})

	if result.IsError {
		t.Fatalf("unexpected error: %v", textContent(t, result))
	}
	got := textContent(t, result)
	if strings.Contains(got, "alpha") {
		t.Errorf("got %q: line 1 (alpha) should be excluded", got)
	}
	if !strings.Contains(got, "beta") || !strings.Contains(got, "gamma") || !strings.Contains(got, "delta") {
		t.Errorf("got %q: lines 2-4 should all be present", got)
	}
	if strings.Contains(got, "epsilon") {
		t.Errorf("got %q: line 5 (epsilon) should be excluded", got)
	}
}

// TestReadFile_startLineOnly returns from startLine to end of file.
func TestReadFile_startLineOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lines.txt")
	writeFile(t, path, "one\ntwo\nthree\n")

	c := newClient(t, defaultConfig(t))
	result := callReadFile(t, c, map[string]any{"filePath": path, "startLine": 2})

	if result.IsError {
		t.Fatalf("unexpected error: %v", textContent(t, result))
	}
	got := textContent(t, result)
	if strings.Contains(got, "one") {
		t.Errorf("line 1 should be excluded, got %q", got)
	}
	if !strings.Contains(got, "two") || !strings.Contains(got, "three") {
		t.Errorf("lines 2-3 should be present, got %q", got)
	}
}

// TestReadFile_endLineOnly returns from the start of the file to endLine.
func TestReadFile_endLineOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lines.txt")
	writeFile(t, path, "one\ntwo\nthree\n")

	c := newClient(t, defaultConfig(t))
	result := callReadFile(t, c, map[string]any{"filePath": path, "endLine": 2})

	if result.IsError {
		t.Fatalf("unexpected error: %v", textContent(t, result))
	}
	got := textContent(t, result)
	if strings.Contains(got, "three") {
		t.Errorf("line 3 should be excluded, got %q", got)
	}
	if !strings.Contains(got, "one") || !strings.Contains(got, "two") {
		t.Errorf("lines 1-2 should be present, got %q", got)
	}
}

// TestReadFile_endLineBeyondFileLengthClamped verifies that requesting an
// endLine beyond the file is silently clamped to the last line.
func TestReadFile_endLineBeyondFileLengthClamped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "short.txt")
	writeFile(t, path, "only line\n")

	c := newClient(t, defaultConfig(t))
	result := callReadFile(t, c, map[string]any{"filePath": path, "startLine": 1, "endLine": 999})

	if result.IsError {
		t.Fatalf("unexpected error: %v", textContent(t, result))
	}
	got := textContent(t, result)
	if !strings.Contains(got, "only line") {
		t.Errorf("expected file content, got %q", got)
	}
}

// --- error cases ---

// TestReadFile_missingFilePathArg verifies that omitting filePath returns an
// error result (not a Go error).
func TestReadFile_missingFilePathArg(t *testing.T) {
	c := newClient(t, defaultConfig(t))
	result := callReadFile(t, c, map[string]any{})

	if !result.IsError {
		t.Error("IsError = false, want true (filePath missing)")
	}
}

// TestReadFile_nonExistentFile verifies that a missing file produces an error
// result with a helpful message.
func TestReadFile_nonExistentFile(t *testing.T) {
	c := newClient(t, defaultConfig(t))
	result := callReadFile(t, c, map[string]any{"filePath": "/nonexistent/path/file.txt"})

	if !result.IsError {
		t.Error("IsError = false, want true (file not found)")
	}
}

// TestReadFile_startLineBeyondFileLength verifies an out-of-range startLine
// returns an error result.
func TestReadFile_startLineBeyondFileLength(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiny.txt")
	writeFile(t, path, "one line\n")

	c := newClient(t, defaultConfig(t))
	result := callReadFile(t, c, map[string]any{"filePath": path, "startLine": 100})

	if !result.IsError {
		t.Error("IsError = false, want true (startLine beyond file)")
	}
}

// TestReadFile_endLineBeforeStartLine verifies that endLine < startLine
// returns an error result.
func TestReadFile_endLineBeforeStartLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.txt")
	writeFile(t, path, "a\nb\nc\nd\n")

	c := newClient(t, defaultConfig(t))
	result := callReadFile(t, c, map[string]any{"filePath": path, "startLine": 3, "endLine": 1})

	if !result.IsError {
		t.Error("IsError = false, want true (endLine before startLine)")
	}
}

// --- policy enforcement ---

// TestReadFile_toolDeniedByPolicy verifies that a disabled tool returns an
// error result without reading any file.
func TestReadFile_toolDeniedByPolicy(t *testing.T) {
	f := false
	cfg := &config.Config{
		Tools: map[string]config.ToolPolicy{
			"read_readFile": {Allowed: &f},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "secret.txt")
	writeFile(t, path, "top secret\n")

	c := newClient(t, cfg)
	result := callReadFile(t, c, map[string]any{"filePath": path})

	if !result.IsError {
		t.Error("IsError = false, want true (tool disabled by policy)")
	}
}

// TestReadFile_denyPathBlocked verifies that a file matching a deny_paths
// pattern is refused even though the tool is allowed.
func TestReadFile_denyPathBlocked(t *testing.T) {
	f := true
	cfg := &config.Config{
		Tools: map[string]config.ToolPolicy{
			"read_readFile": {
				Allowed:   &f,
				DenyPaths: []string{"**/.env", "**/secrets/**"},
			},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	writeFile(t, path, "SECRET=hunter2\n")

	c := newClient(t, cfg)
	result := callReadFile(t, c, map[string]any{"filePath": path})

	if !result.IsError {
		t.Error("IsError = false, want true (.env matches deny_paths)")
	}
}

// TestReadFile_denyPathAllowsOtherFiles verifies that only the matching path
// is blocked — other files in the same directory pass through.
func TestReadFile_denyPathAllowsOtherFiles(t *testing.T) {
	f := true
	cfg := &config.Config{
		Tools: map[string]config.ToolPolicy{
			"read_readFile": {
				Allowed:   &f,
				DenyPaths: []string{"**/.env"},
			},
		},
	}

	dir := t.TempDir()
	safeFile := filepath.Join(dir, "main.go")
	writeFile(t, safeFile, "package main\n")

	c := newClient(t, cfg)
	result := callReadFile(t, c, map[string]any{"filePath": safeFile})

	if result.IsError {
		t.Errorf("IsError = true, want false (main.go is not denied): %v", textContent(t, result))
	}
}
