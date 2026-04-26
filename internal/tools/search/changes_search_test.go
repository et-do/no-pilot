package search_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/testutil"
)

func TestChangesSearch_filtersByState(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")

	staged := filepath.Join(repo, "staged.txt")
	if err := os.WriteFile(staged, []byte("staged"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "staged.txt")

	unstaged := filepath.Join(repo, "unstaged.txt")
	if err := os.WriteFile(unstaged, []byte("unstaged"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	all := testutil.CallTool(t, c, "search_changes", map[string]any{"repositoryPath": repo})
	allText := testutil.TextContent(all)
	if all.IsError {
		t.Fatalf("unexpected error: %q", allText)
	}
	if !strings.Contains(allText, "staged.txt") || !strings.Contains(allText, "unstaged.txt") {
		t.Fatalf("expected staged and unstaged entries, got %q", allText)
	}

	onlyStaged := testutil.CallTool(t, c, "search_changes", map[string]any{
		"repositoryPath":     repo,
		"sourceControlState": "staged",
	})
	stagedText := testutil.TextContent(onlyStaged)
	if onlyStaged.IsError {
		t.Fatalf("unexpected staged filter error: %q", stagedText)
	}
	if !strings.Contains(stagedText, "staged.txt") || strings.Contains(stagedText, "unstaged.txt") {
		t.Fatalf("unexpected staged filter result: %q", stagedText)
	}

	onlyUnstaged := testutil.CallTool(t, c, "search_changes", map[string]any{
		"repositoryPath":     repo,
		"sourceControlState": "unstaged",
	})
	unstagedText := testutil.TextContent(onlyUnstaged)
	if onlyUnstaged.IsError {
		t.Fatalf("unexpected unstaged filter error: %q", unstagedText)
	}
	if !strings.Contains(unstagedText, "unstaged.txt") {
		t.Fatalf("expected unstaged entry, got %q", unstagedText)
	}

	arrayFilter := testutil.CallTool(t, c, "search_changes", map[string]any{
		"repositoryPath":     repo,
		"sourceControlState": []any{"staged", "unstaged"},
	})
	arrayText := testutil.TextContent(arrayFilter)
	if arrayFilter.IsError {
		t.Fatalf("unexpected array state filter error: %q", arrayText)
	}
	if !strings.Contains(arrayText, "staged.txt") || !strings.Contains(arrayText, "unstaged.txt") {
		t.Fatalf("expected both states from array filter, got %q", arrayText)
	}
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}
