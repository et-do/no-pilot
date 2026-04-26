package read_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func writeDir(t *testing.T, path string, files ...string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		p := filepath.Join(path, f)
		if f[len(f)-1] == '/' {
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.WriteFile(p, []byte("test"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func callListDirectory(t *testing.T, c *client.Client, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = "read_listDirectory"
	req.Params.Arguments = args
	result, err := c.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	return result
}

func TestListDirectory_listsFilesAndDirs(t *testing.T) {
	dir := t.TempDir()
	writeDir(t, dir, "a.txt", "b.txt", "subdir/")

	c := newClient(t, defaultConfig(t))
	result := callListDirectory(t, c, map[string]any{"path": dir})

	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	got := textSlice(result)
	sort.Strings(got)
	want := []string{"a.txt", "b.txt", "subdir/"}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("got[%d]=%q, want %q", i, got[i], w)
		}
	}
}

func TestListDirectory_emptyDir(t *testing.T) {
	dir := t.TempDir()
	c := newClient(t, defaultConfig(t))
	result := callListDirectory(t, c, map[string]any{"path": dir})
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}
	if len(textSlice(result)) != 0 {
		t.Errorf("expected empty result, got %v", textSlice(result))
	}
}

func TestListDirectory_nonExistentDir(t *testing.T) {
	c := newClient(t, defaultConfig(t))
	result := callListDirectory(t, c, map[string]any{"path": "/no/such/dir"})
	if !result.IsError {
		t.Error("IsError = false, want true (dir does not exist)")
	}
}

func TestListDirectory_pathIsFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	c := newClient(t, defaultConfig(t))
	result := callListDirectory(t, c, map[string]any{"path": file})
	if !result.IsError {
		t.Error("IsError = false, want true (path is file, not dir)")
	}
}

func TestListDirectory_missingPathArg(t *testing.T) {
	c := newClient(t, defaultConfig(t))
	result := callListDirectory(t, c, map[string]any{})
	if !result.IsError {
		t.Error("IsError = false, want true (missing path)")
	}
}

func TestListDirectory_toolDeniedByPolicy(t *testing.T) {
	f := false
	cfg := &config.Config{
		Tools: map[string]config.ToolPolicy{
			"read_listDirectory": {Allowed: &f},
		},
	}
	dir := t.TempDir()
	writeDir(t, dir, "a.txt")
	c := newClient(t, cfg)
	result := callListDirectory(t, c, map[string]any{"path": dir})
	if !result.IsError {
		t.Error("IsError = false, want true (tool denied)")
	}
}

func TestListDirectory_denyPathBlocked(t *testing.T) {
	f := true
	cfg := &config.Config{
		Tools: map[string]config.ToolPolicy{
			"read_listDirectory": {
				Allowed:   &f,
				DenyPaths: []string{"**/private/**"},
			},
		},
	}
	dir := t.TempDir()
	private := filepath.Join(dir, "private")
	if err := os.MkdirAll(private, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	c := newClient(t, cfg)
	result := callListDirectory(t, c, map[string]any{"path": private})
	if !result.IsError {
		t.Error("IsError = false, want true (denyPaths blocks)")
	}
}

func TestListDirectory_denyPathAllowsOtherDirs(t *testing.T) {
	f := true
	cfg := &config.Config{
		Tools: map[string]config.ToolPolicy{
			"read_listDirectory": {
				Allowed:   &f,
				DenyPaths: []string{"**/private/**"},
			},
		},
	}
	dir := t.TempDir()
	public := filepath.Join(dir, "public")
	if err := os.MkdirAll(public, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	c := newClient(t, cfg)
	result := callListDirectory(t, c, map[string]any{"path": public})
	if result.IsError {
		t.Errorf("IsError = true, want false (public dir allowed)")
	}
}
