package execute

import "testing"

func TestParseStringListArgSplitsCommaAndNewline(t *testing.T) {
	got := parseStringListArg("./pkg/a, ./pkg/b\n./pkg/c")
	want := []string{"./pkg/a", "./pkg/b", "./pkg/c"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseStringListArgFromAnySlice(t *testing.T) {
	got := parseStringListArg([]any{" ./a ", 42, "./b"})
	want := []string{"./a", "./b"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestJoinTestNamesPatternEscapesMetaChars(t *testing.T) {
	pattern := joinTestNamesPattern([]string{"Test[A]", "Case(1)"})
	want := "(Test\\[A\\]|Case\\(1\\))"
	if pattern != want {
		t.Fatalf("pattern = %q, want %q", pattern, want)
	}
	if pattern == "(Test[A]|Case(1))" {
		t.Fatal("regex metacharacters were not escaped")
	}
}

func TestCoverageLineMatchesFilters(t *testing.T) {
	line := "github.com/et-do/no-pilot/internal/tools/execute/run_tests.go:10.1,15.2 1 1"
	if !coverageLineMatchesFilters(line, []string{"internal/tools/execute/run_tests.go"}) {
		t.Fatal("expected filter match by substring")
	}
	if coverageLineMatchesFilters(line, []string{"internal/tools/read/problems.go"}) {
		t.Fatal("expected non-matching filter")
	}
}
