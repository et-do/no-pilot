package execute

import "testing"

func TestNormalizeLanguage(t *testing.T) {
	cases := map[string]string{
		"go":         "go",
		"golang":     "go",
		"python":     "python",
		"py":         "python",
		"javascript": "node",
		"typescript": "node",
		"node":       "node",
		"unknown":    "",
	}
	for in, want := range cases {
		if got := normalizeLanguage(in); got != want {
			t.Fatalf("normalizeLanguage(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseStringListArgSplitsWhitespace(t *testing.T) {
	got := parseStringListArg("./pkg/a, ./pkg/b\n./pkg/c\t./pkg/d")
	want := []string{"./pkg/a", "./pkg/b", "./pkg/c", "./pkg/d"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNormalizeGoTestTargetsFileToPackageAndDedup(t *testing.T) {
	got := normalizeGoTestTargets([]string{
		"internal/tools/execute/run_tests.go",
		"./internal/tools/execute/run_tests_test.go",
		"internal/tools/execute",
	})
	want := []string{"./internal/tools/execute"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	if got[0] != want[0] {
		t.Fatalf("got[0] = %q, want %q", got[0], want[0])
	}
}

func TestDetectLanguageFromTargets(t *testing.T) {
	if got := detectLanguageFromTargets([]string{"tests/test_api.py"}); got != "python" {
		t.Fatalf("python detection = %q, want python", got)
	}
	if got := detectLanguageFromTargets([]string{"src/foo.spec.ts"}); got != "node" {
		t.Fatalf("node detection = %q, want node", got)
	}
	if got := detectLanguageFromTargets([]string{"internal/config/config_test.go"}); got != "go" {
		t.Fatalf("go fallback = %q, want go", got)
	}
}
