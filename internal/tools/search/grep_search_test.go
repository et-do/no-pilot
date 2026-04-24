package search_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	nopilotserver "github.com/et-do/no-pilot/internal/server"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func newClient(t *testing.T, cfg *config.Config) *client.Client {
	t.Helper()
	s := buildTestServer(cfg)
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


func buildTestServer(cfg *config.Config) *server.MCPServer {
	return nopilotserver.Build(cfg, "test")
}

func callGrepSearch(t *testing.T, c *client.Client, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = "search_grepSearch"
	req.Params.Arguments = args
	result, err := c.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	return result
}

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
	cfg := defaultConfig(t)
	c := newClient(t, cfg)
	res := callGrepSearch(t, c, map[string]any{"query": "hello", "includePattern": "*.txt", "workingDir": tmp})
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			text = tc.Text
		}
	}
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
		t.Logf("Test file: %s in dir: %s", path, tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString("foo bar\nno match here\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	cfg := defaultConfig(t)
	c := newClient(t, cfg)
	res := callGrepSearch(t, c, map[string]any{"query": "hello", "includePattern": "*.txt", "workingDir": tmp})
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			text = tc.Text
		}
	}
	if res.IsError {
		t.Fatalf("unexpected error: %v", text)
	}
	if text != "" {
		t.Errorf("expected empty result, got %q", text)
	}
}
