package policy_test

import (
	"context"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// allowBool returns a pointer to b, for constructing ToolPolicy.Allowed.
func allowBool(b bool) *bool { return &b }

// cfgWith builds a *config.Config with a single tool policy.
func cfgWith(tool string, pol config.ToolPolicy) *config.Config {
	return &config.Config{
		Tools: map[string]config.ToolPolicy{tool: pol},
	}
}

// okHandler is a no-op tool handler that always returns success.
func okHandler(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("ok"), nil
}

// apply runs middleware → handler and returns the result.
func apply(mw server.ToolHandlerMiddleware, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mw(okHandler)(context.Background(), req)
}

// makeReq builds a CallToolRequest with the given string arguments.
func makeReq(args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

// --- Enforce (tool-level only) ---

func TestEnforce_allowedByDefault(t *testing.T) {
	cfg := &config.Config{} // no policy → default allow
	mw := policy.Enforce(cfg, "read_readFile")

	result, err := apply(mw, makeReq(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("IsError = true, want false (tool allowed by default)")
	}
}

func TestEnforce_explicitlyAllowed(t *testing.T) {
	cfg := cfgWith("read_readFile", config.ToolPolicy{Allowed: allowBool(true)})
	mw := policy.Enforce(cfg, "read_readFile")

	result, err := apply(mw, makeReq(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("IsError = true, want false")
	}
}

func TestEnforce_denied(t *testing.T) {
	cfg := cfgWith("execute_runInTerminal", config.ToolPolicy{Allowed: allowBool(false)})
	mw := policy.Enforce(cfg, "execute_runInTerminal")

	result, err := apply(mw, makeReq(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("IsError = false, want true (tool denied)")
	}
}

func TestEnforce_deniedToolDoesNotCallNext(t *testing.T) {
	cfg := cfgWith("execute_runInTerminal", config.ToolPolicy{Allowed: allowBool(false)})
	mw := policy.Enforce(cfg, "execute_runInTerminal")

	called := false
	sentinel := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return mcp.NewToolResultText("should not reach"), nil
	}

	mw(sentinel)(context.Background(), makeReq(nil)) //nolint:errcheck
	if called {
		t.Error("inner handler was called despite tool being denied")
	}
}

// --- EnforceWithPaths ---

func TestEnforceWithPaths_noDenyPaths_passes(t *testing.T) {
	cfg := cfgWith("read_readFile", config.ToolPolicy{Allowed: allowBool(true)})
	mw := policy.EnforceWithPaths(cfg, "read_readFile", "path")

	result, err := apply(mw, makeReq(map[string]any{"path": "/home/user/code/main.go"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("IsError = true, want false (no deny paths)")
	}
}

func TestEnforceWithPaths_denyPathBlocks(t *testing.T) {
	cfg := cfgWith("read_readFile", config.ToolPolicy{
		Allowed:   allowBool(true),
		DenyPaths: []string{"**/.env", "**/secrets/**"},
	})
	mw := policy.EnforceWithPaths(cfg, "read_readFile", "path")

	cases := []struct {
		path    string
		wantErr bool
	}{
		{"/workspace/.env", true},
		{"/workspace/secrets/db.yaml", true},
		{"/workspace/internal/main.go", false},
		{"/workspace/.environment", false}, // partial match should not trigger
	}

	for _, tc := range cases {
		result, err := apply(mw, makeReq(map[string]any{"path": tc.path}))
		if err != nil {
			t.Fatalf("path %q: unexpected error: %v", tc.path, err)
		}
		if result.IsError != tc.wantErr {
			t.Errorf("path %q: IsError = %v, want %v", tc.path, result.IsError, tc.wantErr)
		}
	}
}

func TestEnforceWithPaths_toolDeniedIgnoresPaths(t *testing.T) {
	// Even if the path would be fine, a denied tool stops immediately.
	cfg := cfgWith("read_readFile", config.ToolPolicy{
		Allowed:   allowBool(false),
		DenyPaths: []string{},
	})
	mw := policy.EnforceWithPaths(cfg, "read_readFile", "path")

	result, err := apply(mw, makeReq(map[string]any{"path": "/workspace/ok.go"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("IsError = false, want true (tool itself denied)")
	}
}

func TestEnforceWithPaths_missingPathArgSkipped(t *testing.T) {
	// If the path argument key is absent, enforcement is a no-op for that key.
	cfg := cfgWith("read_readFile", config.ToolPolicy{
		Allowed:   allowBool(true),
		DenyPaths: []string{"**/.env"},
	})
	mw := policy.EnforceWithPaths(cfg, "read_readFile", "path")

	// Pass args without "path" — should pass through.
	result, err := apply(mw, makeReq(map[string]any{"other": "value"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("IsError = true, want false (path arg absent → skip)")
	}
}

func TestEnforceWithPaths_nonStringPathArgSkipped(t *testing.T) {
	cfg := cfgWith("read_readFile", config.ToolPolicy{
		Allowed:   allowBool(true),
		DenyPaths: []string{"**/.env"},
	})
	mw := policy.EnforceWithPaths(cfg, "read_readFile", "path")

	// Non-string value for "path" should be skipped gracefully.
	result, err := apply(mw, makeReq(map[string]any{"path": 42}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("IsError = true, want false (non-string path → skip)")
	}
}

func TestEnforceWithPaths_invalidPatternSkipped(t *testing.T) {
	// A syntactically invalid glob pattern must not crash; it is silently skipped.
	cfg := cfgWith("read_readFile", config.ToolPolicy{
		Allowed:   allowBool(true),
		DenyPaths: []string{"[invalid"}, // unclosed bracket → bad pattern
	})
	mw := policy.EnforceWithPaths(cfg, "read_readFile", "path")

	result, err := apply(mw, makeReq(map[string]any{"path": "/workspace/main.go"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("IsError = true, want false (invalid pattern skipped)")
	}
}

func TestEnforceWithPaths_multiplePathArgs(t *testing.T) {
	// Both "src" and "dst" are checked; denying either blocks the call.
	cfg := cfgWith("edit/editFiles", config.ToolPolicy{
		Allowed:   allowBool(true),
		DenyPaths: []string{"**/.env"},
	})
	mw := policy.EnforceWithPaths(cfg, "edit/editFiles", "src", "dst")

	// src is safe, dst is denied.
	result, err := apply(mw, makeReq(map[string]any{
		"src": "/workspace/main.go",
		"dst": "/workspace/.env",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("IsError = false, want true (dst matches deny pattern)")
	}
}

// --- EnforceWithCommand ---

func TestEnforceWithCommand_noRestrictions_passes(t *testing.T) {
	cfg := cfgWith("execute_runInTerminal", config.ToolPolicy{Allowed: allowBool(true)})
	mw := policy.EnforceWithCommand(cfg, "execute_runInTerminal", "command")

	result, err := apply(mw, makeReq(map[string]any{"command": "go build ./..."}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("IsError = true, want false (no command restrictions)")
	}
}

func TestEnforceWithCommand_allowListPermits(t *testing.T) {
	cfg := cfgWith("execute_runInTerminal", config.ToolPolicy{
		Allowed:       allowBool(true),
		AllowCommands: []string{"go *", "git *", "make *"},
	})
	mw := policy.EnforceWithCommand(cfg, "execute_runInTerminal", "command")

	result, err := apply(mw, makeReq(map[string]any{"command": "go test ./..."}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("IsError = true, want false (command matches allow_commands)")
	}
}

func TestEnforceWithCommand_allowListBlocks(t *testing.T) {
	cfg := cfgWith("execute_runInTerminal", config.ToolPolicy{
		Allowed:       allowBool(true),
		AllowCommands: []string{"go *", "git *"},
	})
	mw := policy.EnforceWithCommand(cfg, "execute_runInTerminal", "command")

	result, err := apply(mw, makeReq(map[string]any{"command": "npm install"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("IsError = false, want true (command not in allow_commands)")
	}
}

func TestEnforceWithCommand_denyListBlocks(t *testing.T) {
	cfg := cfgWith("execute_runInTerminal", config.ToolPolicy{
		Allowed:      allowBool(true),
		DenyCommands: []string{"rm -rf *", "sudo *"},
	})
	mw := policy.EnforceWithCommand(cfg, "execute_runInTerminal", "command")

	result, err := apply(mw, makeReq(map[string]any{"command": "sudo apt install curl"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("IsError = false, want true (command matches deny_commands)")
	}
}

func TestEnforceWithCommand_denyEvaluatedAfterAllow(t *testing.T) {
	// Command matches allow_commands but also matches deny_commands → denied.
	cfg := cfgWith("execute_runInTerminal", config.ToolPolicy{
		Allowed:       allowBool(true),
		AllowCommands: []string{"go *"},
		DenyCommands:  []string{"go generate *"},
	})
	mw := policy.EnforceWithCommand(cfg, "execute_runInTerminal", "command")

	result, err := apply(mw, makeReq(map[string]any{"command": "go generate ./..."}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("IsError = false, want true (deny evaluated after allow)")
	}
}

func TestEnforceWithCommand_missingArgSkipped(t *testing.T) {
	cfg := cfgWith("execute_runInTerminal", config.ToolPolicy{
		Allowed:      allowBool(true),
		DenyCommands: []string{"rm *"},
	})
	mw := policy.EnforceWithCommand(cfg, "execute_runInTerminal", "command")

	result, err := apply(mw, makeReq(map[string]any{"other": "value"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("IsError = true, want false (missing command arg → skip)")
	}
}

func TestEnforceWithCommand_toolDenied(t *testing.T) {
	cfg := cfgWith("execute_runInTerminal", config.ToolPolicy{Allowed: allowBool(false)})
	mw := policy.EnforceWithCommand(cfg, "execute_runInTerminal", "command")

	result, err := apply(mw, makeReq(map[string]any{"command": "go build ./..."}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("IsError = false, want true (tool denied)")
	}
}

// --- EnforceWithURL ---

func TestEnforceWithURL_noDenyURLs_passes(t *testing.T) {
	cfg := cfgWith("browser/navigate", config.ToolPolicy{Allowed: allowBool(true)})
	mw := policy.EnforceWithURL(cfg, "browser/navigate", "url")

	result, err := apply(mw, makeReq(map[string]any{"url": "https://example.com"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("IsError = true, want false (no deny_urls)")
	}
}

func TestEnforceWithURL_denyURLBlocks(t *testing.T) {
	cfg := cfgWith("browser/navigate", config.ToolPolicy{
		Allowed:  allowBool(true),
		DenyURLs: []string{"*.internal", "169.254.*"},
	})
	mw := policy.EnforceWithURL(cfg, "browser/navigate", "url")

	cases := []struct {
		url     string
		wantErr bool
	}{
		{"http://internal.corp.internal", true},
		{"http://169.254.169.254/latest/meta-data", true},
		{"https://example.com/page", false},
	}
	for _, tc := range cases {
		result, err := apply(mw, makeReq(map[string]any{"url": tc.url}))
		if err != nil {
			t.Fatalf("url %q: unexpected error: %v", tc.url, err)
		}
		if result.IsError != tc.wantErr {
			t.Errorf("url %q: IsError = %v, want %v", tc.url, result.IsError, tc.wantErr)
		}
	}
}

func TestEnforceWithURL_missingArgSkipped(t *testing.T) {
	cfg := cfgWith("browser/navigate", config.ToolPolicy{
		Allowed:  allowBool(true),
		DenyURLs: []string{"*.internal"},
	})
	mw := policy.EnforceWithURL(cfg, "browser/navigate", "url")

	result, err := apply(mw, makeReq(map[string]any{"other": "value"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("IsError = true, want false (missing url arg → skip)")
	}
}

func TestEnforceWithURL_toolDenied(t *testing.T) {
	cfg := cfgWith("browser/navigate", config.ToolPolicy{Allowed: allowBool(false)})
	mw := policy.EnforceWithURL(cfg, "browser/navigate", "url")

	result, err := apply(mw, makeReq(map[string]any{"url": "https://example.com"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("IsError = false, want true (tool denied)")
	}
}
