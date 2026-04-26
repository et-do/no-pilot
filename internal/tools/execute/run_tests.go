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
	mcp.WithDescription("Run Go unit tests and return output. Supports optional coverage mode."),
	mcp.WithString("files",
		mcp.Description("Optional test target(s). Accepts a string or array-style input. Defaults to ./..."),
	),
	mcp.WithString("testNames",
		mcp.Description("Optional test name(s) to run via -run. Accepts a string or array-style input."),
	),
	mcp.WithString("mode",
		mcp.Description("Execution mode: run (default) or coverage."),
	),
	mcp.WithString("coverageFiles",
		mcp.Description("Optional file filters for coverage output. Accepts a string or array-style input."),
	),
)

func registerRunTests(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(runTestsTool, policy.Enforce(cfg, toolRunTests)(handleRunTests))
}

func handleRunTests(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	files := parseStringListArg(req.GetArguments()["files"])
	if len(files) == 0 {
		files = []string{"./..."}
	}
	testNames := parseStringListArg(req.GetArguments()["testNames"])
	mode := strings.ToLower(strings.TrimSpace(req.GetString("mode", "run")))
	if mode == "" {
		mode = "run"
	}
	if mode != "run" && mode != "coverage" {
		return mcp.NewToolResultError("mode must be 'run' or 'coverage'"), nil
	}

	args := []string{"test", "-v"}
	if len(testNames) > 0 {
		args = append(args, "-run", joinTestNamesPattern(testNames))
	}
	coverageProfile := ""
	if mode == "coverage" {
		tmp, err := os.CreateTemp("", "no-pilot-cover-*.out")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("create coverage profile: %v", err)), nil
		}
		coverageProfile = tmp.Name()
		_ = tmp.Close()
		args = append(args, "-coverprofile", coverageProfile)
	}
	args = append(args, files...)

	cmd := exec.Command("go", args...)
	if root, ok := moduleRootDir(); ok {
		cmd.Dir = root
	}
	out, err := cmd.CombinedOutput()
	outputText := string(out)

	if coverageProfile != "" {
		defer os.Remove(coverageProfile)
		coverageSummary := readCoverageSummary(coverageProfile, parseStringListArg(req.GetArguments()["coverageFiles"]))
		if coverageSummary != "" {
			if strings.TrimSpace(outputText) != "" {
				outputText += "\n"
			}
			outputText += coverageSummary
		}
	}

	if strings.TrimSpace(outputText) == "" {
		outputText = "go test produced no output"
	}

	failureDetail := extractFailureDetail(outputText)
	state := lastTestRun{
		Ran:           true,
		Failed:        err != nil,
		Summary:       summarizeRun(mode, files, testNames, err),
		FailureDetail: failureDetail,
	}
	setLastTestRun(state)

	result := mcp.NewToolResultText(outputText)
	if err != nil {
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
		for _, part := range strings.FieldsFunc(v, func(r rune) bool { return r == '\n' || r == ',' }) {
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
