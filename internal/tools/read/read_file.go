package read

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolReadFile = "read_readFile"

var readFileTool = mcp.NewTool(
	toolReadFile,
	mcp.WithDescription("Read the contents of a file in the workspace. Optionally restrict to a line range (1-based, inclusive)."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("filePath",
		mcp.Required(),
		mcp.Description("Absolute path to the file to read."),
	),
	mcp.WithNumber("startLine",
		mcp.Description("First line to return (1-based). Omit to start from the beginning."),
	),
	mcp.WithNumber("endLine",
		mcp.Description("Last line to return (1-based, inclusive). Omit to read to the end of the file."),
	),
)

// registerReadFile adds the read/readFile tool to the server.
func registerReadFile(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(readFileTool, policy.EnforceWithPaths(cfg, toolReadFile, "filePath")(handleReadFile))
}

func handleReadFile(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, err := req.RequireString("filePath")
	if err != nil {
		return mcp.NewToolResultError("filePath is required and must be a string"), nil
	}

	data, err := os.ReadFile(filePath) //nolint:gosec // path is policy-checked before this handler runs
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("read %s: %v", filePath, err)), nil
	}

	content := string(data)

	startLine := req.GetInt("startLine", 0)
	endLine := req.GetInt("endLine", 0)

	if startLine > 0 || endLine > 0 {
		content, err = sliceLines(content, startLine, endLine)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}

	return mcp.NewToolResultText(content), nil
}

// sliceLines extracts a line range from content. Both startLine and endLine
// are 1-based and inclusive. A value of 0 means "unbounded" for that side.
func sliceLines(content string, startLine, endLine int) (string, error) {
	lines := strings.Split(content, "\n")
	total := len(lines)

	start := 1
	end := total

	if startLine > 0 {
		start = startLine
	}
	if endLine > 0 {
		end = endLine
	}

	if start < 1 {
		start = 1
	}
	if start > total {
		return "", fmt.Errorf("startLine %d exceeds file length %d", start, total)
	}
	if end > total {
		end = total
	}
	if end < start {
		return "", fmt.Errorf("endLine %d is before startLine %d", end, start)
	}

	return strings.Join(lines[start-1:end], "\n"), nil
}
