package read

import (
	"context"
	"fmt"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolGetNotebookSummary = "read_getNotebookSummary"

var getNotebookSummaryTool = mcp.NewTool(
	toolGetNotebookSummary,
	mcp.WithDescription("[READ] List notebook cells and their metadata."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("filePath",
		mcp.Required(),
		mcp.Description("Absolute path to the notebook file."),
	),
)

func registerGetNotebookSummary(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(getNotebookSummaryTool, policy.EnforceWithPaths(cfg, toolGetNotebookSummary, "filePath")(handleGetNotebookSummary))
}

func handleGetNotebookSummary(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, err := req.RequireString("filePath")
	if err != nil {
		return mcp.NewToolResultError("filePath is required and must be a string"), nil
	}

	nb, err := loadNotebook(filePath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("load notebook summary: %v", err)), nil
	}

	return mcp.NewToolResultText(formatNotebookSummary(nb)), nil
}
