package read

import (
	"context"
	"fmt"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolReadNotebookCellOutput = "read_readNotebookCellOutput"

var readNotebookCellOutputTool = mcp.NewTool(
	toolReadNotebookCellOutput,
	mcp.WithDescription("[READ] Read the most recent output of a notebook cell."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("filePath",
		mcp.Required(),
		mcp.Description("Absolute path to the notebook file."),
	),
	mcp.WithString("cellId",
		mcp.Required(),
		mcp.Description("ID of the notebook cell whose output should be returned."),
	),
)

func registerReadNotebookCellOutput(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(readNotebookCellOutputTool, policy.EnforceWithPaths(cfg, toolReadNotebookCellOutput, "filePath")(handleReadNotebookCellOutput))
}

func handleReadNotebookCellOutput(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, err := req.RequireString("filePath")
	if err != nil {
		return mcp.NewToolResultError("filePath is required and must be a string"), nil
	}
	cellID, err := req.RequireString("cellId")
	if err != nil {
		return mcp.NewToolResultError("cellId is required and must be a string"), nil
	}

	nb, err := loadNotebook(filePath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("load notebook output: %v", err)), nil
	}

	cell, ok := findNotebookCell(nb, cellID)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("cell %q not found", cellID)), nil
	}
	return mcp.NewToolResultText(formatNotebookOutputs(cell)), nil
}
