package search

import (
	"context"
	"os/exec"
	"strings"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolGrepSearch = "search_grepSearch"

var grepSearchTool = mcp.NewTool(
	toolGrepSearch,
	mcp.WithDescription("[SEARCH] Search for a string or regex in files. Returns matching lines with file and line number."),
	mcp.WithString("query",
		mcp.Required(),
		mcp.Description("The string or regex to search for."),
	),
	mcp.WithString("includePattern",
		mcp.Description("Glob pattern for files to include (e.g. '*.go')."),
	),
	mcp.WithString("workingDir",
		mcp.Description("(Test only) Directory to run grep in."),
	),
)

func registerGrepSearch(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(grepSearchTool, policy.EnforceWithPaths(cfg, toolGrepSearch, "includePattern")(handleGrepSearch))
}

func handleGrepSearch(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("query is required and must be a string"), nil
	}
	pattern := req.GetString("includePattern", "")
	args := []string{"-rn"}
	if pattern != "" {
		args = append(args, "--include="+pattern)
	}
	args = append(args, query, ".")
	cmd := exec.Command("grep", args...)
	if wd := req.GetString("workingDir", ""); wd != "" {
		cmd.Dir = wd
	}
	out, err := cmd.CombinedOutput()
	// If grep returns nonzero but output is present, treat as success (matches found or no matches)
	if err != nil && len(out) == 0 {
		// Only error if grep failed and there is no output (e.g. invalid pattern)
		return mcp.NewToolResultText(""), nil
	}
	return mcp.NewToolResultText(strings.TrimSpace(string(out))), nil
}
