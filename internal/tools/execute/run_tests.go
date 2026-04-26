package execute

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolRunTests = "execute_runTests"

var runTestsTool = mcp.NewTool(
	toolRunTests,
	mcp.WithDescription("[EXECUTE] Run tests and return output. Supports Go, Python (pytest), and Node test runners."),
	mcp.WithString("files",
		mcp.Description("Optional test target(s). Accepts a string or array-style input. Defaults vary by language (Go: ./..., Python/Node: current directory)."),
	),
	mcp.WithString("testNames",
		mcp.Description("Optional test name(s). Go maps to -run; Python maps to -k; Node maps to -t (framework-dependent)."),
	),
	mcp.WithString("mode",
		mcp.Description("Execution mode: run (default) or coverage. Coverage is currently supported for Go and Node."),
	),
	mcp.WithString("coverageFiles",
		mcp.Description("Optional file filters for Go coverage profile output. Accepts a string or array-style input."),
	),
	mcp.WithString("language",
		mcp.Description("Optional test language/runner: go (default), python, javascript, typescript, node, js, ts."),
	),
	mcp.WithString("cwd",
		mcp.Description("Optional working directory to run tests from."),
	),
)

func registerRunTests(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(runTestsTool, policy.EnforceWithPaths(cfg, toolRunTests, "cwd")(handleRunTests))
}

func handleRunTests(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	files := parseStringListArg(req.GetArguments()["files"])
	testNames := parseStringListArg(req.GetArguments()["testNames"])
	coverageFilters := parseStringListArg(req.GetArguments()["coverageFiles"])
	mode := strings.ToLower(strings.TrimSpace(req.GetString("mode", "run")))
	if mode == "" {
		mode = "run"
	}
	if mode != "run" && mode != "coverage" {
		return mcp.NewToolResultError("mode must be 'run' or 'coverage'"), nil
	}
	language := normalizeLanguage(req.GetString("language", "go"))
	if language == "" {
		return mcp.NewToolResultError("language must be one of: go, python, javascript, typescript, node, js, ts"), nil
	}

	cmdName, cmdArgs, coverageProfile, err := buildTestCommand(language, mode, files, testNames)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	cmd := exec.Command(cmdName, cmdArgs...)
	if cwd := strings.TrimSpace(req.GetString("cwd", "")); cwd != "" {
		cmd.Dir = filepath.Clean(cwd)
	} else if language == "go" {
		if root, ok := moduleRootDir(); ok {
			cmd.Dir = root
		}
	}

	out, runErr := cmd.CombinedOutput()
	outputText := string(out)
	if runErr != nil {
		if ee, ok := runErr.(*exec.Error); ok && ee.Err == exec.ErrNotFound {
			return mcp.NewToolResultError(fmt.Sprintf("test runner not found: %s", cmdName)), nil
		}
	}

	if coverageProfile != "" {
		defer os.Remove(coverageProfile)
		coverageSummary := readCoverageSummary(coverageProfile, coverageFilters)
		if coverageSummary != "" {
			if strings.TrimSpace(outputText) != "" {
				outputText += "\n"
			}
			outputText += coverageSummary
		}
	}

	if strings.TrimSpace(outputText) == "" {
		outputText = fmt.Sprintf("%s produced no output", cmdName)
	}

	failureDetail := extractFailureDetail(outputText)
	state := lastTestRun{
		Ran:           true,
		Failed:        runErr != nil,
		Summary:       summarizeRun(language+":"+mode, files, testNames, runErr),
		FailureDetail: failureDetail,
	}
	setLastTestRun(state)

	result := mcp.NewToolResultText(outputText)
	if runErr != nil {
		result.IsError = true
	}
	return result, nil
}

func parseStringListArg(raw any) []string {
	out := []string{}
	switch v := raw.(type) {
	case nil:
		return out
	case string:
		normalized := strings.NewReplacer(",", " ", "\n", " ", "\r", " ", "\t", " ").Replace(v)
		for _, part := range strings.Fields(normalized) {
			p := strings.TrimSpace(part)
			if p != "" {
				out = append(out, p)
			}
		}
	case []string:
		for _, item := range v {
			p := strings.TrimSpace(item)
			if p != "" {
				out = append(out, p)
			}
		}
	case []any:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			p := strings.TrimSpace(s)
			if p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func normalizeLanguage(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "go", "golang":
		return "go"
	case "python", "py":
		return "python"
	case "javascript", "typescript", "node", "js", "ts":
		return "node"
	default:
		return ""
	}
}

func detectLanguageFromTargets(files []string) string {
	for _, f := range files {
		v := strings.ToLower(strings.TrimSpace(f))
		switch {
		case strings.HasSuffix(v, ".py"):
			return "python"
		case strings.HasSuffix(v, ".js"), strings.HasSuffix(v, ".jsx"), strings.HasSuffix(v, ".ts"), strings.HasSuffix(v, ".tsx"):
			return "node"
		}
	}
	return "go"
}

func buildTestCommand(language, mode string, files, testNames []string) (string, []string, string, error) {
	switch language {
	case "go":
		goFiles := normalizeGoTestTargets(files)
		if len(goFiles) == 0 {
			goFiles = []string{"./..."}
		}
		args := []string{"test", "-v"}
		if len(testNames) > 0 {
			args = append(args, "-run", joinTestNamesPattern(testNames))
		}
		coverageProfile := ""
		if mode == "coverage" {
			tmp, err := os.CreateTemp("", "no-pilot-cover-*.out")
			if err != nil {
				return "", nil, "", fmt.Errorf("create coverage profile: %v", err)
			}
			coverageProfile = tmp.Name()
			_ = tmp.Close()
			args = append(args, "-coverprofile", coverageProfile)
		}
		args = append(args, goFiles...)
		return "go", args, coverageProfile, nil
	case "python":
		args := []string{"-m", "pytest", "-q"}
		if mode == "coverage" {
			args = append(args, "--cov=.", "--cov-report=term-missing:skip-covered")
		}
		if len(testNames) > 0 {
			args = append(args, "-k", strings.Join(testNames, " or "))
		}
		if len(files) == 0 {
			files = []string{"."}
		}
		args = append(args, files...)
		return "python3", args, "", nil
	case "node":
		args := []string{"test"}
		if mode == "coverage" {
			args = append(args, "--", "--coverage")
			if len(testNames) > 0 {
				args = append(args, "-t", strings.Join(testNames, "|"))
			}
			if len(files) > 0 {
				args = append(args, files...)
			}
			return "npm", args, "", nil
		}
		if len(testNames) > 0 || len(files) > 0 {
			args = append(args, "--")
			if len(testNames) > 0 {
				args = append(args, "-t", strings.Join(testNames, "|"))
			}
			args = append(args, files...)
		}
		return "npm", args, "", nil
	default:
		return "", nil, "", fmt.Errorf("unsupported language %q", language)
	}
}

func normalizeGoTestTargets(targets []string) []string {
	if len(targets) == 0 {
		return nil
	}
	out := make([]string, 0, len(targets))
	seen := map[string]struct{}{}
	for _, target := range targets {
		t := strings.TrimSpace(target)
		if t == "" {
			continue
		}
		if strings.HasSuffix(t, ".go") {
			t = filepath.Dir(t)
			if t == "." {
				t = "./"
			}
		}
		if !filepath.IsAbs(t) && !strings.HasPrefix(t, "./") && !strings.HasPrefix(t, "../") {
			t = "./" + t
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func joinTestNamesPattern(testNames []string) string {
	escaped := make([]string, 0, len(testNames))
	for _, name := range testNames {
		escaped = append(escaped, regexp.QuoteMeta(name))
	}
	return "(" + strings.Join(escaped, "|") + ")"
}

func readCoverageSummary(profilePath string, filters []string) string {
	if profilePath == "" {
		return ""
	}
	f, err := os.Open(profilePath)
	if err != nil {
		return ""
	}
	defer f.Close()

	filtered := make([]string, 0)
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "mode:") {
			continue
		}
		if len(filters) > 0 && !coverageLineMatchesFilters(line, filters) {
			continue
		}
		filtered = append(filtered, line)
	}
	if len(filtered) == 0 {
		return ""
	}
	return fmt.Sprintf("coverage profile entries: %d", len(filtered))
}

func coverageLineMatchesFilters(line string, filters []string) bool {
	for _, f := range filters {
		base := strings.TrimSpace(f)
		if base == "" {
			continue
		}
		if strings.Contains(line, base) || strings.Contains(line, filepath.Clean(base)) {
			return true
		}
	}
	return false
}

func summarizeRun(mode string, files, testNames []string, runErr error) string {
	status := "passed"
	if runErr != nil {
		status = "failed"
	}
	return fmt.Sprintf("mode=%s status=%s files=%d testNames=%d", mode, status, len(files), len(testNames))
}

func extractFailureDetail(out string) string {
	lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	picked := make([]string, 0)
	for _, l := range lines {
		trim := strings.TrimSpace(l)
		if strings.HasPrefix(trim, "--- FAIL:") || strings.HasPrefix(trim, "FAIL") {
			picked = append(picked, l)
		}
	}
	return strings.Join(picked, "\n")
}

func moduleRootDir() (string, bool) {
	cmd := exec.Command("go", "env", "GOMOD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", false
	}
	gomod := strings.TrimSpace(string(out))
	if gomod == "" || gomod == os.DevNull {
		return "", false
	}
	return filepath.Dir(gomod), true
}
