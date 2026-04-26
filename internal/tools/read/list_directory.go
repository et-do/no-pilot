package read

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolListDirectory = "read_listDirectory"

var listDirectoryTool = mcp.NewTool(
	toolListDirectory,
	mcp.WithDescription("[READ] List the contents of a directory (files and subdirectories)."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("path",
		mcp.Required(),
		mcp.Description("Absolute path to the directory to list."),
	),
)

func registerListDirectory(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(listDirectoryTool, policy.EnforceWithPaths(cfg, toolListDirectory, "path")(handleListDirectory))
}

func handleListDirectory(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dirPath, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required and must be a string"), nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("readDir %s: %v", dirPath, err)), nil
	}

	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}
	sort.Strings(names)

	return mcp.NewToolResultText(strings.Join(names, "\n")), nil
}
