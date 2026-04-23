package server_test

import (
	"context"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	nopilotserver "github.com/et-do/no-pilot/internal/server"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// newTestConfig returns a Config with all tools allowed — the zero-value state.
func newTestConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	return cfg
}

// newClient creates an in-process MCP client connected to the given server and
// performs the initialization handshake, failing the test on any error.
func newClient(t *testing.T, cfg *config.Config, version string) *client.Client {
	t.Helper()
	s := nopilotserver.Build(cfg, version)
	c, err := client.NewInProcessClient(s)
	if err != nil {
		t.Fatalf("NewInProcessClient() error = %v", err)
	}
	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("client.Start() error = %v", err)
	}
	if _, err := c.Initialize(ctx, mcp.InitializeRequest{}); err != nil {
		t.Fatalf("client.Initialize() error = %v", err)
	}
	return c
}
// TestBuild_returnsServer verifies that Build produces a non-nil MCPServer.
func TestBuild_returnsServer(t *testing.T) {
	s := nopilotserver.Build(newTestConfig(t), "test")
	if s == nil {
		t.Fatal("Build() returned nil")
	}
}

// TestBuild_serverNameAndVersion verifies the server advertises the correct
// name and version through the MCP initialize handshake.
func TestBuild_serverNameAndVersion(t *testing.T) {
	const wantVersion = "v0.1.0-test"
	s := nopilotserver.Build(newTestConfig(t), wantVersion)

	c, err := client.NewInProcessClient(s)
	if err != nil {
		t.Fatalf("NewInProcessClient() error = %v", err)
	}
	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("client.Start() error = %v", err)
	}

	result, err := c.Initialize(ctx, mcp.InitializeRequest{})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if result.ServerInfo.Name != "no-pilot" {
		t.Errorf("ServerInfo.Name = %q, want %q", result.ServerInfo.Name, "no-pilot")
	}
	if result.ServerInfo.Version != wantVersion {
		t.Errorf("ServerInfo.Version = %q, want %q", result.ServerInfo.Version, wantVersion)
	}
}

// TestBuild_noToolsRegisteredYet verifies the server starts with an empty
// TestBuild_toolListContainsReadFile verifies that read/readFile is registered
// in the server's tool list.
func TestBuild_toolListContainsReadFile(t *testing.T) {
	c := newClient(t, newTestConfig(t), "test")

	tools, err := c.ListTools(context.Background(), mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "read/readFile" {
			return
		}
	}
	t.Errorf("read/readFile not found in tool list; got %d tools", len(tools.Tools))
}
