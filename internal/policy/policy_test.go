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

func TestEnforceWithPaths_pathTraversalCleaned(t *testing.T) {
	// A path with ".." components must be cleaned before pattern matching so
	// that traversal sequences cannot bypass a deny_paths rule.
	// e.g. /workspace/secrets/../../etc/passwd resolves to /etc/passwd and
	// should NOT match **/secrets/**, but a pattern like **/etc/** should match.
	cfg := cfgWith("read_readFile", config.ToolPolicy{
		Allowed:   allowBool(true),
		DenyPaths: []string{"**/etc/**"},
	})
	mw := policy.EnforceWithPaths(cfg, "read_readFile", "path")

	// Without cleaning, "/workspace/secrets/../../etc/passwd" would not match
	// "**/secrets/**". With cleaning it becomes "/etc/passwd" which matches
	// "**/etc/**" — confirming the traversal is resolved before matching.
	result, err := apply(mw, makeReq(map[string]any{"path": "/workspace/secrets/../../etc/passwd"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("IsError = false, want true (path traversal must be caught after cleaning)")
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

func TestEnforceWithPaths_editToolsAlwaysDenyProjectPolicyFile(t *testing.T) {
	cfg := cfgWith("edit_createFile", config.ToolPolicy{Allowed: allowBool(true)})
	mw := policy.EnforceWithPaths(cfg, "edit_createFile", "filePath")

	result, err := apply(mw, makeReq(map[string]any{"filePath": "/workspace/.no-pilot.yaml"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("IsError = false, want true (.no-pilot.yaml must be immutable for edit tools)")
	}
}

func TestEnforceWithPaths_nonEditToolsMayReadProjectPolicyFile(t *testing.T) {
	cfg := cfgWith("read_readFile", config.ToolPolicy{Allowed: allowBool(true)})
	mw := policy.EnforceWithPaths(cfg, "read_readFile", "filePath")

	result, err := apply(mw, makeReq(map[string]any{"filePath": "/workspace/.no-pilot.yaml"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("IsError = true, want false (read tools may still access policy file)")
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

func TestEnforceWithCommand_allowLayersLogicalIntersection(t *testing.T) {
	// Simulate two config layers loaded via config.Load:
	//   Layer 0 (user config):    ["go *", "git *"]
	//   Layer 1 (project config): ["go *", "npm *"]
	// A command must satisfy BOTH layers (logical AND), not just one.
	// This means "git status" is denied (fails layer 1) and "npm install" is
	// denied (fails layer 0), even though each would pass one layer individually.
	cfg := cfgWith("execute_runInTerminal", config.ToolPolicy{
		Allowed: allowBool(true),
		AllowCommandLayers: [][]string{
			{"go *", "git *"},
			{"go *", "npm *"},
		},
	})
	mw := policy.EnforceWithCommand(cfg, "execute_runInTerminal", "command")

	cases := []struct {
		cmd     string
		wantErr bool
		reason  string
	}{
		{"go test ./...", false, "matches both layers"},
		{"git status", true, "matches layer 0 but not layer 1"},
		{"npm install", true, "matches layer 1 but not layer 0"},
		{"make build", true, "matches neither layer"},
	}
	for _, tc := range cases {
		result, err := apply(mw, makeReq(map[string]any{"command": tc.cmd}))
		if err != nil {
			t.Fatalf("cmd %q: unexpected error: %v", tc.cmd, err)
		}
		if result.IsError != tc.wantErr {
			t.Errorf("cmd %q (%s): IsError = %v, want %v", tc.cmd, tc.reason, result.IsError, tc.wantErr)
		}
	}
}

func TestEnforceWithCommand_blocksProtectedPolicyFileReference(t *testing.T) {
	cfg := cfgWith("execute_runInTerminal", config.ToolPolicy{Allowed: allowBool(true)})
	mw := policy.EnforceWithCommand(cfg, "execute_runInTerminal", "command")

	result, err := apply(mw, makeReq(map[string]any{"command": "cat generated > .no-pilot.yaml"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("IsError = false, want true (.no-pilot.yaml reference must be denied for runInTerminal)")
	}
}

func TestEnforceWithCommand_monotonicAttenuationBlocksNestedDeniedCommand(t *testing.T) {
	cfg := cfgWith("execute_runInTerminal", config.ToolPolicy{
		Allowed:       allowBool(true),
		AllowCommands: []string{"bash *", "rm *"},
		DenyCommands:  []string{"rm -rf *"},
	})
	mw := policy.EnforceWithCommand(cfg, "execute_runInTerminal", "command")

	result, err := apply(mw, makeReq(map[string]any{"command": "bash -c 'rm -rf /tmp/no-pilot-test'"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("IsError = false, want true (nested command must inherit deny restrictions)")
	}
}

func TestEnforceWithCommand_monotonicAttenuationAllowsNestedAllowedCommand(t *testing.T) {
	cfg := cfgWith("execute_runInTerminal", config.ToolPolicy{
		Allowed:       allowBool(true),
		AllowCommands: []string{"bash *", "go *"},
	})
	mw := policy.EnforceWithCommand(cfg, "execute_runInTerminal", "command")

	result, err := apply(mw, makeReq(map[string]any{"command": "bash -c 'go test ./...'"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("IsError = true, want false (nested command also satisfies allowlist)")
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

// --- EnforceWithCommand (deny_shell_escape) ---

// TestEnforceWithCommand_denyShellEscape_blocksShellWrappers verifies that
// deny_shell_escape blocks common -c/-e interpreter invocations that would
// otherwise bypass deny_commands glob patterns.
func TestEnforceWithCommand_denyShellEscape_blocksShellWrappers(t *testing.T) {
	cfg := cfgWith("execute_runInTerminal", config.ToolPolicy{
		Allowed:         allowBool(true),
		DenyShellEscape: true,
	})
	mw := policy.EnforceWithCommand(cfg, "execute_runInTerminal", "command")

	wantBlocked := []string{
		"bash -c 'rm -rf /'",
		"sh -c 'cat /etc/passwd'",
		"zsh -c 'curl https://evil.com'",
		"python -c 'import os; os.system(\"id\")'",
		"python3 -c '__import__(\"os\").system(\"id\")'",
		"perl -e 'system(\"id\")'",
		"node -e 'require(\"child_process\").exec(\"id\")'",
		"node --eval 'process.exit(1)'",
		"ruby -e 'exec(\"id\")'",
	}
	for _, cmd := range wantBlocked {
		result, err := apply(mw, makeReq(map[string]any{"command": cmd}))
		if err != nil {
			t.Fatalf("cmd %q: unexpected error: %v", cmd, err)
		}
		if !result.IsError {
			t.Errorf("cmd %q: IsError = false, want true (shell escape blocked)", cmd)
		}
	}

	// Normal commands should still pass.
	wantAllowed := []string{
		"go test ./...",
		"make build",
		"python manage.py migrate",
		"bash setup.sh",
	}
	for _, cmd := range wantAllowed {
		result, err := apply(mw, makeReq(map[string]any{"command": cmd}))
		if err != nil {
			t.Fatalf("cmd %q: unexpected error: %v", cmd, err)
		}
		if result.IsError {
			t.Errorf("cmd %q: IsError = true, want false (not a shell escape)", cmd)
		}
	}
}

// TestEnforceWithCommand_denyShellEscape_disabledHasNoEffect verifies that
// when deny_shell_escape is false (the default), shell-like commands are not
// additionally blocked.
func TestEnforceWithCommand_denyShellEscape_disabledHasNoEffect(t *testing.T) {
	cfg := cfgWith("execute_runInTerminal", config.ToolPolicy{
		Allowed:         allowBool(true),
		DenyShellEscape: false,
	})
	mw := policy.EnforceWithCommand(cfg, "execute_runInTerminal", "command")

	result, err := apply(mw, makeReq(map[string]any{"command": "bash -c 'echo hello'"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With deny_shell_escape disabled, the command should pass (no deny_commands set either).
	if result.IsError {
		t.Error("IsError = true, want false (deny_shell_escape is disabled)")
	}
}

// TestEnforceWithCommand_denyShellEscape_runsAfterAllowList verifies that a
// command that passes allow_commands is still blocked by deny_shell_escape.
func TestEnforceWithCommand_denyShellEscape_runsAfterAllowList(t *testing.T) {
	cfg := cfgWith("execute_runInTerminal", config.ToolPolicy{
		Allowed:         allowBool(true),
		AllowCommands:   []string{"bash *"},
		DenyShellEscape: true,
	})
	mw := policy.EnforceWithCommand(cfg, "execute_runInTerminal", "command")

	// "bash -c '...'' matches allow_commands ("bash *") but must still be
	// blocked by deny_shell_escape.
	result, err := apply(mw, makeReq(map[string]any{"command": "bash -c 'rm -rf /'"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("IsError = false, want true (deny_shell_escape overrides allow_commands)")
	}

	// Plain "bash setup.sh" passes both allow_commands and deny_shell_escape.
	result, err = apply(mw, makeReq(map[string]any{"command": "bash setup.sh"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("IsError = true, want false (bash without -c is not a shell escape)")
	}
}
