package read

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolProblems = "read_problems"

var problemsTool = mcp.NewTool(
	toolProblems,
	mcp.WithDescription("[READ] Check Go syntax problems for a file, a set of paths, or a directory (defaults to workspace)."),
	mcp.WithString("filePath",
		mcp.Description("Optional single file path to check."),
	),
	mcp.WithString("paths",
		mcp.Description("Optional file/directory path set. Accepts a string or array-style input."),
	),
	mcp.WithString("path",
		mcp.Description("Optional file or directory path to check recursively for .go files."),
	),
)

func registerProblems(s *server.MCPServer, cfg config.Provider) {
	h := policy.EnforceWithPaths(cfg, toolProblems, "filePath", "path")(handleProblems(cfg))
	s.AddTool(problemsTool, h)
}

func handleProblems(cfg config.Provider) server.ToolHandlerFunc {
	return func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		targets, err := collectProblemTargets(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		targets = filterDeniedProblemTargets(cfg, targets)
		if len(targets) == 0 {
			return mcp.NewToolResultText("no problems found"), nil
		}

		files, missing := expandGoFiles(targets)
		problems := make([]string, 0, len(missing)+len(files))
		problems = append(problems, missing...)

		fset := token.NewFileSet()
		for _, filePath := range files {
			if _, parseErr := parser.ParseFile(fset, filePath, nil, parser.AllErrors); parseErr != nil {
				problems = append(problems, fmt.Sprintf("%s: %v", filePath, parseErr))
			}
		}

		problems = append(problems, compileProblemsForFiles(files)...)
		problems = uniqueStrings(problems)
		if len(problems) == 0 {
			return mcp.NewToolResultText("no problems found"), nil
		}
		sort.Strings(problems)
		return mcp.NewToolResultText(strings.Join(problems, "\n")), nil
	}
}

func collectProblemTargets(req mcp.CallToolRequest) ([]string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}

	add(req.GetString("filePath", ""))
	add(req.GetString("path", ""))
	for _, p := range parsePathListArg(req.GetArguments()["paths"]) {
		add(p)
	}

	if len(out) > 0 {
		return out, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve workspace path: %w", err)
	}
	return []string{wd}, nil
}

func expandGoFiles(targets []string) (files []string, missing []string) {
	fileSet := map[string]struct{}{}
	files = make([]string, 0)
	missing = make([]string, 0)

	for _, raw := range targets {
		target := filepath.Clean(strings.TrimSpace(raw))
		if target == "" {
			continue
		}
		info, statErr := os.Stat(target)
		if statErr != nil {
			missing = append(missing, fmt.Sprintf("%s: file not found", target))
			continue
		}
		if !info.IsDir() {
			if _, ok := fileSet[target]; !ok {
				fileSet[target] = struct{}{}
				files = append(files, target)
			}
			continue
		}

		_ = filepath.WalkDir(target, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if d.IsDir() {
				if d.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".go" {
				return nil
			}
			if _, ok := fileSet[path]; ok {
				return nil
			}
			fileSet[path] = struct{}{}
			files = append(files, path)
			return nil
		})
	}

	sort.Strings(files)
	return files, missing
}

func parsePathListArg(raw any) []string {
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

func compileProblemsForFiles(files []string) []string {
	if len(files) == 0 {
		return nil
	}
	moduleRoot, ok := moduleRootDir()
	if !ok {
		return nil
	}

	pkgSet := map[string]struct{}{}
	pkgs := make([]string, 0)
	for _, f := range files {
		clean := filepath.Clean(f)
		if filepath.Ext(clean) != ".go" {
			continue
		}
		abs, absErr := filepath.Abs(clean)
		if absErr != nil {
			continue
		}
		rel, err := filepath.Rel(moduleRoot, filepath.Dir(abs))
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		pkg := "./" + filepath.ToSlash(rel)
		if rel == "." {
			pkg = "./"
		}
		if _, exists := pkgSet[pkg]; exists {
			continue
		}
		pkgSet[pkg] = struct{}{}
		pkgs = append(pkgs, pkg)
	}
	if len(pkgs) == 0 {
		return nil
	}
	sort.Strings(pkgs)

	args := []string{"test", "-run", "^$", "-count=1"}
	args = append(args, pkgs...)
	cmd := exec.Command("go", args...)
	cmd.Dir = moduleRoot
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	problems := make([]string, 0)
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if strings.Contains(trim, ".go:") || strings.Contains(trim, "undefined:") {
			problems = append(problems, trim)
		}
	}
	return problems
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

func uniqueStrings(in []string) []string {
	if len(in) == 0 {
		return in
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func filterDeniedProblemTargets(cfg config.Provider, targets []string) []string {
	if cfg == nil || len(targets) == 0 {
		return targets
	}
	deny := cfg.Policy(toolProblems).DenyPaths
	if len(deny) == 0 {
		return targets
	}
	out := make([]string, 0, len(targets))
	for _, t := range targets {
		clean := filepath.Clean(t)
		blocked := false
		for _, pattern := range deny {
			ok, matchErr := doublestar.Match(pattern, clean)
			if matchErr != nil {
				continue
			}
			if ok {
				blocked = true
				break
			}
		}
		if !blocked {
			out = append(out, t)
		}
	}
	return out
}
