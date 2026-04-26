package execute_test

import (
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/testutil"
)

func TestTestFailure_afterSuccessfulRunReportsNoFailures(t *testing.T) {
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := callExecuteTool(t, c, "execute_runTests", map[string]any{
		"files": []any{"./internal/tools/edit"},
	})
	if res.IsError {
		t.Fatalf("expected passing test run, got %q", getText(res))
	}

	failure := callExecuteTool(t, c, "execute_testFailure", map[string]any{})
	if failure.IsError {
		t.Fatalf("unexpected error: %q", getText(failure))
	}
	if !strings.Contains(getText(failure), "no failures") {
		t.Fatalf("expected no failures message, got %q", getText(failure))
	}
}
