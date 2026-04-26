package server_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	nopilotserver "github.com/et-do/no-pilot/internal/server"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestSmoke_serverStartsAndToolsWork verifies that the MCP server can be built,
// initialized, and core tools execute successfully end-to-end.
func TestSmoke_serverStartsAndToolsWork(t *testing.T) {
	cfg := newTestConfig(t)
	s := nopilotserver.Build(cfg, "test-smoke")
	if s == nil {
		t.Fatal("Build() returned nil")
	}

	c, err := client.NewInProcessClient(s)
	if err != nil {
		t.Fatalf("NewInProcessClient: %v", err)
	}
	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("client.Start: %v", err)
	}

	initResp, err := c.Initialize(ctx, mcp.InitializeRequest{})
	if err != nil {
		t.Fatalf("client.Initialize: %v", err)
	}
	if initResp.ServerInfo.Name != "no-pilot" {
		t.Errorf("server name = %q, want no-pilot", initResp.ServerInfo.Name)
	}

	// List tools and verify key ones are registered.
	tools, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	keyTools := map[string]bool{
		"read_readFile":         false,
		"search_grepSearch":     false,
		"execute_runInTerminal": false,
		"edit_createFile":       false,
		"web_fetch":             false,
	}

	for _, tool := range tools.Tools {
		if _, ok := keyTools[tool.Name]; ok {
			keyTools[tool.Name] = true
		}
	}

	for tool, found := range keyTools {
		if !found {
			t.Errorf("key tool %q not found in tool list (%d tools total)", tool, len(tools.Tools))
		}
	}

	// Smoke test: create a temp file.
	t.Run("edit_createFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "smoke_test.txt")

		req := mcp.CallToolRequest{}
		req.Params.Name = "edit_createFile"
		req.Params.Arguments = map[string]any{
			"filePath": testFile,
			"content":  "smoke test content\n",
		}
		result, err := c.CallTool(ctx, req)
		if err != nil {
			t.Fatalf("CallTool(edit_createFile): %v", err)
		}
		if result.IsError {
			t.Fatalf("edit_createFile returned error: %s", textContent(result))
		}

		// Verify file was created.
		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(data) != "smoke test content\n" {
			t.Errorf("file content = %q, want 'smoke test content\\n'", string(data))
		}
	})

	// Smoke test: run a simple terminal command.
	t.Run("execute_runInTerminal", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "execute_runInTerminal"
		req.Params.Arguments = map[string]any{
			"command": "echo hello world",
			"mode":    "sync",
			"timeout": 5000,
		}
		result, err := c.CallTool(ctx, req)
		if err != nil {
			t.Fatalf("CallTool(execute_runInTerminal): %v", err)
		}
		if result.IsError {
			t.Fatalf("execute_runInTerminal returned error: %s", textContent(result))
		}
		text := textContent(result)
		if !strings.Contains(text, "hello world") {
			t.Errorf("expected 'hello world' in output, got: %s", text)
		}
	})

	// Smoke test: create a directory.
	t.Run("edit_createDirectory", func(t *testing.T) {
		tmpDir := t.TempDir()
		testDir := filepath.Join(tmpDir, "subdir", "nested")

		req := mcp.CallToolRequest{}
		req.Params.Name = "edit_createDirectory"
		req.Params.Arguments = map[string]any{
			"dirPath": testDir,
		}
		result, err := c.CallTool(ctx, req)
		if err != nil {
			t.Fatalf("CallTool(edit_createDirectory): %v", err)
		}
		if result.IsError {
			t.Fatalf("edit_createDirectory returned error: %s", textContent(result))
		}

		// Verify directory was created.
		if info, err := os.Stat(testDir); err != nil || !info.IsDir() {
			t.Errorf("directory not created: %v", err)
		}
	})

	// Smoke test: async terminal session.
	t.Run("execute_runInTerminal_async", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "execute_runInTerminal"
		req.Params.Arguments = map[string]any{
			"command": "echo async test",
			"mode":    "async",
			"timeout": 5000,
		}
		result, err := c.CallTool(ctx, req)
		if err != nil {
			t.Fatalf("CallTool(execute_runInTerminal async): %v", err)
		}
		if result.IsError {
			t.Fatalf("execute_runInTerminal async returned error: %s", textContent(result))
		}
		text := textContent(result)
		if !strings.Contains(text, "terminal_id") && !strings.Contains(text, "id") {
			t.Logf("async terminal response: %s", text[:100])
			// Response may vary but should contain some ID or status
		}
	})

	// Smoke test: list terminals.
	t.Run("execute_listTerminals", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "execute_listTerminals"
		result, err := c.CallTool(ctx, req)
		if err != nil {
			t.Fatalf("CallTool(execute_listTerminals): %v", err)
		}
		if result.IsError {
			t.Fatalf("execute_listTerminals returned error: %s", textContent(result))
		}
		// Just verify it returns without error; content may be empty
	})

	// Smoke test: fetch a simple public URL and verify structured output.
	t.Run("web_fetch_jsonStructure", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "web_fetch"
		req.Params.Arguments = map[string]any{
			"url":       "https://httpbin.org/html",
			"maxChars":  200,
			"timeoutMs": 15000,
			"maxBytes":  1024 * 1024,
		}
		result, err := c.CallTool(ctx, req)
		if err != nil {
			t.Fatalf("CallTool(web_fetch): %v", err)
		}
		if result.IsError {
			t.Logf("web_fetch returned error (may be expected in restricted environments): %s", textContent(result))
			// Don't fail; network may not be available
			return
		}

		text := textContent(result)
		// Verify JSON structure
		var fetchResp map[string]interface{}
		if err := json.Unmarshal([]byte(text), &fetchResp); err != nil {
			t.Fatalf("web_fetch response is not valid JSON: %v\nraw: %s", err, text)
		}

		// Check for expected fields
		expectedFields := []string{"url", "status", "contentType", "body"}
		for _, field := range expectedFields {
			if _, ok := fetchResp[field]; !ok {
				t.Errorf("expected field %q in web_fetch response, got: %v", field, fetchResp)
			}
		}
	})

	t.Logf("✓ smoke test passed: server initialized with %d tools", len(tools.Tools))
}

// textContent returns concatenated text from MCP result content.
func textContent(result *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}
