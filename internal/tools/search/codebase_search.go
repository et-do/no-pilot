package search

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolCodebaseSearch = "search_codebase"

var codebaseSearchTool = mcp.NewTool(
	toolCodebaseSearch,
	mcp.WithDescription("[SEARCH] Run a relevance-ranked lexical search across source files."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("query",
		mcp.Required(),
		mcp.Description("Query terms to rank relevant code snippets."),
	),
	mcp.WithNumber("maxResults",
		mcp.Description("Maximum number of snippets to return. Default 20."),
	),
)

func registerCodebaseSearch(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(codebaseSearchTool, policy.Enforce(cfg, toolCodebaseSearch)(handleCodebaseSearch))
}

type codeHit struct {
	path  string
	line  int
	text  string
	score int
}

func handleCodebaseSearch(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil || strings.TrimSpace(query) == "" {
		return mcp.NewToolResultError("query is required and must be a non-empty string"), nil
	}

	terms := strings.Fields(strings.ToLower(query))
	maxResults := req.GetInt("maxResults", 20)
	if maxResults <= 0 {
		maxResults = 20
	}

	hits := make([]codeHit, 0, maxResults*2)
	_ = filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
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
			lineLower := strings.ToLower(line)
			score := scoreLine(path, lineLower, terms)
			if score == 0 {
				continue
			}
			hits = append(hits, codeHit{path: path, line: lineNo, text: strings.TrimSpace(line), score: score})
		}
		return nil
	})

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		if hits[i].path != hits[j].path {
			return hits[i].path < hits[j].path
		}
		return hits[i].line < hits[j].line
	})

	if len(hits) > maxResults {
		hits = hits[:maxResults]
	}

	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, fmt.Sprintf("%s:%d: %s (score=%d)", h.path, h.line, h.text, h.score))
	}
	return mcp.NewToolResultText(strings.Join(out, "\n")), nil
}

func scoreLine(path, line string, terms []string) int {
	if len(terms) == 0 {
		return 0
	}
	pathLower := strings.ToLower(path)
	score := 0
	matchedTerms := 0
	for _, term := range terms {
		if term == "" {
			continue
		}
		c := strings.Count(line, term)
		if c > 0 {
			score += c * 2
			matchedTerms++
		}
		if strings.Contains(pathLower, term) {
			score += 1
		}
	}
	if matchedTerms == len(terms) {
		score += 3
	}
	return score
}
