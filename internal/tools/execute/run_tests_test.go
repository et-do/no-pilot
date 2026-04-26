package execute_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/testutil"
)

func TestRunTests_passAndCoverage(t *testing.T) {
	dir, err := os.MkdirTemp(".", "run-tests-pass-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	src := filepath.Join(dir, "sample.go")
	testFile := filepath.Join(dir, "sample_test.go")
	if err := os.WriteFile(src, []byte("package runtestspass\nfunc Add(a,b int) int { return a+b }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, []byte("package runtestspass\nimport \"testing\"\nfunc TestAdd(t *testing.T){ if Add(1,2)!=3 { t.Fatal(\"bad\") } }\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := callExecuteTool(t, c, "execute_runTests", map[string]any{
		"files": []any{"./internal/tools/execute/" + filepath.Base(dir)},
		"mode":  "coverage",
	})
	if res.IsError {
		t.Fatalf("unexpected runTests error: %s", getText(res))
	}
	if !strings.Contains(getText(res), "PASS") {
		t.Fatalf("expected PASS output, got %q", getText(res))
	}
}

func TestRunTests_failureAndTestFailureTool(t *testing.T) {
	dir, err := os.MkdirTemp(".", "run-tests-fail-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	testFile := filepath.Join(dir, "fail_test.go")
	if err := os.WriteFile(testFile, []byte("package runtestsfail\nimport \"testing\"\nfunc TestBoom(t *testing.T){ t.Fatal(\"boom\") }\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := callExecuteTool(t, c, "execute_runTests", map[string]any{
		"files": []any{"./internal/tools/execute/" + filepath.Base(dir)},
	})
	if !res.IsError {
		t.Fatalf("expected failure from runTests, got %q", getText(res))
	}

	failure := callExecuteTool(t, c, "execute_testFailure", map[string]any{})
	if failure.IsError {
		t.Fatalf("unexpected testFailure error: %s", getText(failure))
	}
	if !strings.Contains(getText(failure), "FAIL") {
		t.Fatalf("expected failure detail, got %q", getText(failure))
	}
}
