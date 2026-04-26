package edit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolCreateNotebook = "edit_createNotebook"

var createNotebookTool = mcp.NewTool(
	toolCreateNotebook,
	mcp.WithDescription("Create a new Jupyter notebook file."),
	mcp.WithString("filePath",
		mcp.Required(),
		mcp.Description("Absolute path to the notebook file to create."),
	),
	mcp.WithString("query",
		mcp.Description("Optional prompt text to seed a markdown cell in the new notebook."),
	),
)

func registerCreateNotebook(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(createNotebookTool, policy.EnforceWithPaths(cfg, toolCreateNotebook, "filePath")(handleCreateNotebook))
}

func handleCreateNotebook(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, err := req.RequireString("filePath")
	if err != nil {
		return mcp.NewToolResultError("filePath is required and must be a string"), nil
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create parent directories for %s: %v", filePath, err)), nil
	}

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return mcp.NewToolResultError(fmt.Sprintf("file already exists: %s", filePath)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("create notebook %s: %v", filePath, err)), nil
	}
	defer f.Close()

	query := strings.TrimSpace(req.GetString("query", ""))
	cells := []any{}
	if query != "" {
		cells = append(cells, map[string]any{
			"cell_type": "markdown",
			"metadata":  map[string]any{"language": "markdown"},
			"source":    []string{query},
		})
	}

	nb := map[string]any{
		"cells": cells,
		"metadata": map[string]any{
			"language_info": map[string]any{"name": "python"},
		},
		"nbformat":       4,
		"nbformat_minor": 5,
	}
	encoded, err := json.MarshalIndent(nb, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("encode notebook %s: %v", filePath, err)), nil
	}
	encoded = append(encoded, '\n')
	if _, err := f.Write(encoded); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("write notebook %s: %v", filePath, err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("created notebook %s", filePath)), nil
}
