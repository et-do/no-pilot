package edit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/testutil"
)

func TestCreateDirectory_createsNested(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b", "c")
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := callEditTool(t, c, "edit_createDirectory", map[string]any{"dirPath": dir})
	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(res))
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("expected directory to exist, stat err=%v isDir=%v", err, err == nil && st.IsDir())
	}
}

func TestCreateDirectory_missingPath(t *testing.T) {
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := callEditTool(t, c, "edit_createDirectory", map[string]any{})
	if !res.IsError {
		t.Fatal("IsError = false, want true when dirPath missing")
	}
}

func TestCreateDirectory_denyPathBlocked(t *testing.T) {
	tf := true
	cfg := &config.Config{Tools: map[string]config.ToolPolicy{
		"edit_createDirectory": {Allowed: &tf, DenyPaths: []string{"**/blocked/**"}},
	}}
	dir := filepath.Join(t.TempDir(), "blocked", "x")
	c := testutil.NewClient(t, cfg)
	res := callEditTool(t, c, "edit_createDirectory", map[string]any{"dirPath": dir})
	if !res.IsError {
		t.Fatal("IsError = false, want true when deny_paths blocks")
	}
}
