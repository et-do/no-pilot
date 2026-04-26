package edit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolEditNotebook = "edit_editNotebook"

var editNotebookTool = mcp.NewTool(
	toolEditNotebook,
	mcp.WithDescription("[EDIT] Insert, delete, or modify notebook cells in a .ipynb file."),
	mcp.WithString("filePath",
		mcp.Required(),
		mcp.Description("Absolute path to the notebook file."),
	),
	mcp.WithString("editType",
		mcp.Required(),
		mcp.Description("Operation type: insert, edit, or delete."),
	),
	mcp.WithString("cellId",
		mcp.Description("Cell id target. For insert, use TOP, BOTTOM, or an existing cell id to insert after."),
	),
	mcp.WithString("newCode",
		mcp.Description("Cell source for insert/edit. Accepts a string or array-style input from MCP clients."),
	),
	mcp.WithString("language",
		mcp.Description("Optional language for inserted/edited cell metadata.language (e.g. markdown, python)."),
	),
)

func registerEditNotebook(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(editNotebookTool, policy.EnforceWithPaths(cfg, toolEditNotebook, "filePath")(handleEditNotebook))
}

func handleEditNotebook(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, err := req.RequireString("filePath")
	if err != nil {
		return mcp.NewToolResultError("filePath is required and must be a string"), nil
	}
	editType, err := req.RequireString("editType")
	if err != nil {
		return mcp.NewToolResultError("editType is required and must be a string"), nil
	}

	nb, err := loadNotebookDoc(filePath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("load notebook %s: %v", filePath, err)), nil
	}

	switch strings.ToLower(strings.TrimSpace(editType)) {
	case "insert":
		if err := insertNotebookCell(req, nb); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	case "edit":
		if err := editNotebookCell(req, nb); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	case "delete":
		if err := deleteNotebookCell(req, nb); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	default:
		return mcp.NewToolResultError("editType must be one of: insert, edit, delete"), nil
	}

	if err := writeNotebookDoc(filePath, nb); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("write notebook %s: %v", filePath, err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("updated notebook %s with %s", filePath, strings.ToLower(editType))), nil
}

func loadNotebookDoc(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if _, ok := doc["cells"]; !ok {
		doc["cells"] = []any{}
	}
	return doc, nil
}

func writeNotebookDoc(path string, doc map[string]any) error {
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o600)
}

func insertNotebookCell(req mcp.CallToolRequest, doc map[string]any) error {
	cells, err := notebookCells(doc)
	if err != nil {
		return err
	}
	target := strings.TrimSpace(req.GetString("cellId", "BOTTOM"))
	source, err := notebookSourceArg(req.GetArguments()["newCode"])
	if err != nil {
		return err
	}
	lang := strings.TrimSpace(req.GetString("language", ""))
	newCell := map[string]any{
		"cell_type": notebookCellType(lang),
		"metadata":  map[string]any{"language": notebookLanguage(lang)},
		"source":    source,
	}

	upper := strings.ToUpper(target)
	switch upper {
	case "", "BOTTOM":
		cells = append(cells, newCell)
	case "TOP":
		cells = append([]any{newCell}, cells...)
	default:
		idx := notebookCellIndex(cells, target)
		if idx < 0 {
			return fmt.Errorf("cellId %q not found for insert", target)
		}
		cells = append(cells[:idx+1], append([]any{newCell}, cells[idx+1:]...)...)
	}
	doc["cells"] = cells
	return nil
}

func editNotebookCell(req mcp.CallToolRequest, doc map[string]any) error {
	cells, err := notebookCells(doc)
	if err != nil {
		return err
	}
	cellID := strings.TrimSpace(req.GetString("cellId", ""))
	if cellID == "" {
		return fmt.Errorf("cellId is required for edit")
	}
	idx := notebookCellIndex(cells, cellID)
	if idx < 0 {
		return fmt.Errorf("cellId %q not found", cellID)
	}
	cell, ok := cells[idx].(map[string]any)
	if !ok {
		return fmt.Errorf("cell %q is malformed", cellID)
	}
	source, err := notebookSourceArg(req.GetArguments()["newCode"])
	if err != nil {
		return err
	}
	cell["source"] = source
	if lang := strings.TrimSpace(req.GetString("language", "")); lang != "" {
		cell["cell_type"] = notebookCellType(lang)
		metadata, _ := cell["metadata"].(map[string]any)
		if metadata == nil {
			metadata = map[string]any{}
		}
		metadata["language"] = notebookLanguage(lang)
		cell["metadata"] = metadata
	}
	cells[idx] = cell
	doc["cells"] = cells
	return nil
}

func deleteNotebookCell(req mcp.CallToolRequest, doc map[string]any) error {
	cells, err := notebookCells(doc)
	if err != nil {
		return err
	}
	cellID := strings.TrimSpace(req.GetString("cellId", ""))
	if cellID == "" {
		return fmt.Errorf("cellId is required for delete")
	}
	idx := notebookCellIndex(cells, cellID)
	if idx < 0 {
		return fmt.Errorf("cellId %q not found", cellID)
	}
	cells = append(cells[:idx], cells[idx+1:]...)
	doc["cells"] = cells
	return nil
}

func notebookCells(doc map[string]any) ([]any, error) {
	raw, ok := doc["cells"]
	if !ok {
		return []any{}, nil
	}
	cells, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("notebook cells is not an array")
	}
	return cells, nil
}

func notebookCellIndex(cells []any, cellID string) int {
	for i, raw := range cells {
		cell, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := cell["id"].(string); id != "" && id == cellID {
			return i
		}
		meta, _ := cell["metadata"].(map[string]any)
		if id, _ := meta["id"].(string); id != "" && id == cellID {
			return i
		}
	}
	return -1
}

func notebookSourceArg(raw any) ([]string, error) {
	switch v := raw.(type) {
	case nil:
		return nil, fmt.Errorf("newCode is required for insert/edit")
	case string:
		if strings.TrimSpace(v) == "" {
			return []string{""}, nil
		}
		return []string{v}, nil
	case []string:
		return append([]string(nil), v...), nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("newCode array must contain only strings")
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("newCode must be a string or string array")
	}
}

func notebookCellType(language string) string {
	if strings.EqualFold(language, "markdown") {
		return "markdown"
	}
	return "code"
}

func notebookLanguage(language string) string {
	if strings.TrimSpace(language) == "" {
		return "python"
	}
	return language
}
