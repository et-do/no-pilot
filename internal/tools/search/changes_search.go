package search

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolChangesSearch = "search_changes"

var changesSearchTool = mcp.NewTool(
	toolChangesSearch,
	mcp.WithDescription("[SEARCH] List current source control changes from git status."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("repositoryPath",
		mcp.Description("Absolute path to the git repository. Defaults to current working directory."),
	),
	mcp.WithString("sourceControlState",
		mcp.Description("Optional state filter. Accepts comma-separated states (staged,unstaged,merge-conflicts) and also accepts array-style input from MCP clients."),
	),
)

func registerChangesSearch(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(changesSearchTool, policy.EnforceWithPaths(cfg, toolChangesSearch, "repositoryPath")(handleChangesSearch))
}

func handleChangesSearch(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repo := req.GetString("repositoryPath", ".")
	cmd := exec.Command("git", "-C", repo, "status", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("git status failed: %v", err)), nil
	}

	filter := parseStateFilter(req.GetArguments()["sourceControlState"])
	lines := splitNonEmptyLines(string(out))
	if len(filter) == 0 {
		return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
	}

	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		x := line[0]
		y := line[1]
		if stateMatches(filter, x, y) {
			filtered = append(filtered, line)
		}
	}

	return mcp.NewToolResultText(strings.Join(filtered, "\n")), nil
}

func parseStateFilter(raw any) map[string]bool {
	states := extractStates(raw)
	if len(states) == 0 {
		return nil
	}
	out := make(map[string]bool, len(states))
	for _, p := range states {
		out[p] = true
	}
	return out
}

func extractStates(raw any) []string {
	normalize := func(v string) string {
		return strings.TrimSpace(strings.ToLower(v))
	}

	var states []string
	switch v := raw.(type) {
	case nil:
		return nil
	case string:
		for _, part := range strings.Split(v, ",") {
			p := normalize(part)
			if p != "" {
				states = append(states, p)
			}
		}
	case []string:
		for _, part := range v {
			p := normalize(part)
			if p != "" {
				states = append(states, p)
			}
		}
	case []any:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			p := normalize(s)
			if p != "" {
				states = append(states, p)
			}
		}
	}
	return states
}

func stateMatches(filter map[string]bool, x, y byte) bool {
	staged := x != ' ' && x != '?'
	unstaged := y != ' ' || x == '?' || y == '?'
	merge := x == 'U' || y == 'U' || (x == 'A' && y == 'A') || (x == 'D' && y == 'D')

	if filter["merge-conflicts"] && merge {
		return true
	}
	if filter["staged"] && staged {
		return true
	}
	if filter["unstaged"] && unstaged {
		return true
	}
	return false
}

func splitNonEmptyLines(s string) []string {
	parts := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
