package edit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/testutil"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func callCreateFile(t *testing.T, c *client.Client, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	return testutil.CallTool(t, c, "edit_createFile", args)
}

func textContent(result *mcp.CallToolResult) string {
	return testutil.TextContent(result)
}

func TestCreateFile_createsNewFileAndParents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "folder", "new.txt")
	content := "hello\nworld\n"

	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	result := callCreateFile(t, c, map[string]any{
		"filePath": path,
		"content":  content,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", textContent(result))
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if string(got) != content {
		t.Fatalf("content = %q, want %q", string(got), content)
	}
}

func TestCreateFile_rejectsExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.txt")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	result := callCreateFile(t, c, map[string]any{
		"filePath": path,
		"content":  "new",
	})
	if !result.IsError {
		t.Fatal("IsError = false, want true when file exists")
	}
	if !strings.Contains(textContent(result), "file already exists") {
		t.Fatalf("error text = %q, want file already exists message", textContent(result))
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old" {
		t.Fatalf("existing file was modified: got %q, want %q", string(got), "old")
	}
}

func TestCreateFile_missingArgs(t *testing.T) {
	c := testutil.NewClient(t, testutil.DefaultConfig(t))

	missingPath := callCreateFile(t, c, map[string]any{"content": "x"})
	if !missingPath.IsError {
		t.Fatal("IsError = false, want true when filePath is missing")
	}

	missingContent := callCreateFile(t, c, map[string]any{"filePath": "/tmp/x"})
	if !missingContent.IsError {
		t.Fatal("IsError = false, want true when content is missing")
	}
}

func TestCreateFile_toolDeniedByPolicy(t *testing.T) {
	f := false
	cfg := &config.Config{Tools: map[string]config.ToolPolicy{
		"edit_createFile": {Allowed: &f},
	}}
	c := testutil.NewClient(t, cfg)
	result := callCreateFile(t, c, map[string]any{
		"filePath": filepath.Join(t.TempDir(), "x.txt"),
		"content":  "x",
	})
	if !result.IsError {
		t.Fatal("IsError = false, want true when tool is denied")
	}
}

func TestCreateFile_denyPathBlocked(t *testing.T) {
	tDir := t.TempDir()
	blocked := filepath.Join(tDir, "secrets", "k.txt")

	tf := true
	cfg := &config.Config{Tools: map[string]config.ToolPolicy{
		"edit_createFile": {
			Allowed:   &tf,
			DenyPaths: []string{"**/secrets/**"},
		},
	}}
	c := testutil.NewClient(t, cfg)
	result := callCreateFile(t, c, map[string]any{
		"filePath": blocked,
		"content":  "should not write",
	})
	if !result.IsError {
		t.Fatal("IsError = false, want true when deny_paths blocks")
	}
	if _, err := os.Stat(blocked); !os.IsNotExist(err) {
		t.Fatalf("blocked file exists or stat error: %v", err)
	}
}
