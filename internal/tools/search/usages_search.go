package search

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolUsagesSearch = "search_usages"

var usagesSearchTool = mcp.NewTool(
	toolUsagesSearch,
	mcp.WithDescription("[SEARCH] Find textual symbol usages across files (definitions/references are not language-server aware)."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("symbol",
		mcp.Required(),
		mcp.Description("Exact symbol name to search for."),
	),
	mcp.WithString("filePath",
		mcp.Description("Optional path hint used to scope the search to its directory."),
	),
	mcp.WithNumber("maxResults",
		mcp.Description("Maximum matches to return. Default 100."),
	),
)

func registerUsagesSearch(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(usagesSearchTool, policy.EnforceWithPaths(cfg, toolUsagesSearch, "filePath")(handleUsagesSearch))
}

func handleUsagesSearch(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	symbol, err := req.RequireString("symbol")
	if err != nil || strings.TrimSpace(symbol) == "" {
		return mcp.NewToolResultError("symbol is required and must be a non-empty string"), nil
	}

	root := "."
	if fp := strings.TrimSpace(req.GetString("filePath", "")); fp != "" {
		root = filepath.Dir(fp)
	}

	maxResults := req.GetInt("maxResults", 100)
	if maxResults <= 0 {
		maxResults = 100
	}

	wordRE, err := regexp.Compile(`\b` + regexp.QuoteMeta(symbol) + `\b`)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid symbol: %v", err)), nil
	}

	matches := make([]string, 0, maxResults)
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if len(matches) >= maxResults {
			return fmt.Errorf("done")
		}
		if !looksTextPath(path) {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		s := bufio.NewScanner(f)
		lineNo := 0
		for s.Scan() {
			lineNo++
			line := s.Text()
			if wordRE.MatchString(line) {
				matches = append(matches, fmt.Sprintf("%s:%d: %s", path, lineNo, strings.TrimSpace(line)))
				if len(matches) >= maxResults {
					return fmt.Errorf("done")
				}
			}
		}
		return nil
	})

	sort.Strings(matches)
	return mcp.NewToolResultText(strings.Join(matches, "\n")), nil
}

func looksTextPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".js", ".ts", ".tsx", ".jsx", ".py", ".java", ".rb", ".rs", ".c", ".h", ".cpp", ".hpp", ".json", ".yaml", ".yml", ".md", ".txt", ".sh", ".sql":
		return true
	default:
		return ext == ""
	}
}
