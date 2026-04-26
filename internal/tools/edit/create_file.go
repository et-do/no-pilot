package edit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolCreateFile = "edit_createFile"

var createFileTool = mcp.NewTool(
	toolCreateFile,
	mcp.WithDescription("[EDIT] Create a new file in the workspace. The file is created with the provided content, and parent directories are created when needed."),
	mcp.WithString("filePath",
		mcp.Required(),
		mcp.Description("Absolute path to the file to create."),
	),
	mcp.WithString("content",
		mcp.Required(),
		mcp.Description("Content to write into the new file."),
	),
)

func registerCreateFile(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(createFileTool, policy.EnforceWithPaths(cfg, toolCreateFile, "filePath")(handleCreateFile))
}

func handleCreateFile(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, err := req.RequireString("filePath")
	if err != nil {
		return mcp.NewToolResultError("filePath is required and must be a string"), nil
	}

	content, err := req.RequireString("content")
	if err != nil {
		return mcp.NewToolResultError("content is required and must be a string"), nil
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create parent directories for %s: %v", filePath, err)), nil
	}

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return mcp.NewToolResultError(fmt.Sprintf("file already exists: %s", filePath)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("create file %s: %v", filePath, err)), nil
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("write file %s: %v", filePath, err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("created %s", filePath)), nil
}
