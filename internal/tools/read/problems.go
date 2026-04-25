package read

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolProblems = "read_problems"

var problemsTool = mcp.NewTool(
	toolProblems,
	mcp.WithDescription("Check errors for a particular file. Only Go syntax errors are reported."),
	mcp.WithString("filePath",
		mcp.Required(),
		mcp.Description("Absolute path to the file to check for problems."),
	),
)

func registerProblems(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(problemsTool, policy.EnforceWithPaths(cfg, toolProblems, "filePath")(handleProblems))
}

func handleProblems(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, err := req.RequireString("filePath")
	if err != nil {
		return mcp.NewToolResultError("filePath is required and must be a string"), nil
	}
	if _, err := os.Stat(filePath); err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("%s: file not found", filePath)), nil
	}
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, filePath, nil, parser.AllErrors)
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("%s: %v", filePath, err)), nil
	}
	return mcp.NewToolResultText("no problems found"), nil
}
