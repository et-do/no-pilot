package search

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolFileSearch = "search_fileSearch"

var fileSearchTool = mcp.NewTool(
	toolFileSearch,
	mcp.WithDescription("Search for files in the workspace by glob pattern."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("query",
		mcp.Required(),
		mcp.Description("Glob pattern to match files. Can also be an absolute path to scope the search."),
	),
	mcp.WithNumber("maxResults",
		mcp.Description("Maximum number of results to return. Omit or use 0 for no limit."),
	),
)

func registerFileSearch(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(fileSearchTool, policy.EnforceWithPaths(cfg, toolFileSearch, "query")(handleFileSearch))
}

func handleFileSearch(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("query is required and must be a string"), nil
	}

	pattern := query
	if !hasGlobMeta(pattern) {
		pattern = filepath.Join(pattern, "**")
	}

	matches, err := doublestar.FilepathGlob(pattern, doublestar.WithFilesOnly())
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("file search failed for pattern %q: %v", query, err)), nil
	}

	sort.Strings(matches)

	maxResults := req.GetInt("maxResults", 0)
	if maxResults > 0 && len(matches) > maxResults {
		matches = matches[:maxResults]
	}

	return mcp.NewToolResultText(strings.Join(matches, "\n")), nil
}

func hasGlobMeta(s string) bool {
	for _, ch := range s {
		switch ch {
		case '*', '?', '[', '{':
			return true
		}
	}
	return false
}
