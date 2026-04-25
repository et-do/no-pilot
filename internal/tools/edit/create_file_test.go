package edit_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	nopilotserver "github.com/et-do/no-pilot/internal/server"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func newClient(t *testing.T, cfg *config.Config) *client.Client {
	t.Helper()
	s := nopilotserver.Build(cfg, "test")
	c, err := client.NewInProcessClient(s)
	if err != nil {
		t.Fatalf("NewInProcessClient: %v", err)
	}
	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("client.Start: %v", err)
	}
	if _, err := c.Initialize(ctx, mcp.InitializeRequest{}); err != nil {
		t.Fatalf("client.Initialize: %v", err)
	}
	return c
}

func defaultConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func callCreateFile(t *testing.T, c *client.Client, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = "edit_createFile"
	req.Params.Arguments = args
	result, err := c.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	return result
}

func textContent(result *mcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func TestCreateFile_createsNewFileAndParents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "folder", "new.txt")
	content := "hello\nworld\n"

	c := newClient(t, defaultConfig(t))
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

	c := newClient(t, defaultConfig(t))
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
	c := newClient(t, defaultConfig(t))

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
	c := newClient(t, cfg)
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
	c := newClient(t, cfg)
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
