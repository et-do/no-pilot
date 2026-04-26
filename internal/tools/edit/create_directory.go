package edit

import (
	"context"
	"fmt"
	"os"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolCreateDirectory = "edit_createDirectory"

var createDirectoryTool = mcp.NewTool(
	toolCreateDirectory,
	mcp.WithDescription("Create a directory and any missing parent directories."),
	mcp.WithString("dirPath",
		mcp.Required(),
		mcp.Description("Absolute path to the directory to create."),
	),
)

func registerCreateDirectory(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(createDirectoryTool, policy.EnforceWithPaths(cfg, toolCreateDirectory, "dirPath")(handleCreateDirectory))
}

func handleCreateDirectory(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dirPath, err := req.RequireString("dirPath")
	if err != nil {
		return mcp.NewToolResultError("dirPath is required and must be a string"), nil
	}

	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create directory %s: %v", dirPath, err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("created directory %s", dirPath)), nil
}
