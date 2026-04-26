package edit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/testutil"
)

func TestRenameSymbol_updatesMatches(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.go")
	f2 := filepath.Join(dir, "b.go")
	if err := os.WriteFile(f1, []byte("package p\nfunc OldName(){}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("package p\nfunc call(){ OldName() }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := callEditTool(t, c, "edit_renameSymbol", map[string]any{
		"symbol":   "OldName",
		"newName":  "NewName",
		"rootPath": dir,
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(res))
	}
	b1, _ := os.ReadFile(f1)
	b2, _ := os.ReadFile(f2)
	if !strings.Contains(string(b1), "NewName") || !strings.Contains(string(b2), "NewName") {
		t.Fatalf("expected files to be renamed, got %q and %q", string(b1), string(b2))
	}
}

func TestRenameSymbol_noMatches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	if err := os.WriteFile(path, []byte("package p\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := callEditTool(t, c, "edit_renameSymbol", map[string]any{
		"symbol":   "Missing",
		"newName":  "Renamed",
		"rootPath": dir,
	})
	if !res.IsError {
		t.Fatal("IsError = false, want true when no matches")
	}
}

func TestRenameSymbol_skipsDeniedPaths(t *testing.T) {
	dir := t.TempDir()
	allowed := filepath.Join(dir, "ok.go")
	blocked := filepath.Join(dir, "secret", "blocked.go")
	if err := os.WriteFile(allowed, []byte("package p\nfunc OldName(){}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(blocked), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blocked, []byte("package p\nfunc OldName(){}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tf := true
	cfg := &config.Config{Tools: map[string]config.ToolPolicy{
		"edit_renameSymbol": {Allowed: &tf, DenyPaths: []string{"**/secret/**"}},
	}}
	c := testutil.NewClient(t, cfg)
	res := callEditTool(t, c, "edit_renameSymbol", map[string]any{
		"symbol":   "OldName",
		"newName":  "NewName",
		"rootPath": dir,
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(res))
	}
	allowText, _ := os.ReadFile(allowed)
	blockedText, _ := os.ReadFile(blocked)
	if !strings.Contains(string(allowText), "NewName") {
		t.Fatalf("expected allowed file to change: %q", string(allowText))
	}
	if strings.Contains(string(blockedText), "NewName") {
		t.Fatalf("expected blocked file to remain unchanged: %q", string(blockedText))
	}
}
