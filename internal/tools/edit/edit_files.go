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

const toolEditFiles = "edit_editFiles"

type fileEdit struct {
	FilePath   string `json:"filePath"`
	OldString  string `json:"oldString"`
	NewString  string `json:"newString"`
	ReplaceAll bool   `json:"replaceAll"`
}

var editFilesTool = mcp.NewTool(
	toolEditFiles,
	mcp.WithDescription("Apply targeted string replacements in one or more files."),
	mcp.WithString("filePath",
		mcp.Description("Single-file edit path. Used with oldString/newString when edits is not provided."),
	),
	mcp.WithString("oldString",
		mcp.Description("Old string to find in filePath for single-file mode."),
	),
	mcp.WithString("newString",
		mcp.Description("Replacement string for single-file mode."),
	),
	mcp.WithString("edits",
		mcp.Description("Optional JSON array of edits: [{\"filePath\":\"...\",\"oldString\":\"...\",\"newString\":\"...\",\"replaceAll\":true}]."),
	),
)

func registerEditFiles(s *server.MCPServer, cfg config.Provider) {
	h := policy.Enforce(cfg, toolEditFiles)(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleEditFiles(ctx, req, cfg)
	})
	s.AddTool(editFilesTool, h)
}

func handleEditFiles(_ context.Context, req mcp.CallToolRequest, cfg config.Provider) (*mcp.CallToolResult, error) {
	edits, err := parseEdits(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	type pendingWrite struct {
		path    string
		content string
	}
	writes := make([]pendingWrite, 0, len(edits))
	for _, e := range edits {
		if strings.TrimSpace(e.FilePath) == "" {
			return mcp.NewToolResultError("filePath is required for every edit"), nil
		}
		if e.OldString == "" {
			return mcp.NewToolResultError("oldString must be non-empty for every edit"), nil
		}
		if denied, pattern := pathDenied(cfg, toolEditFiles, e.FilePath); denied {
			return mcp.NewToolResultError(fmt.Sprintf("path %q is denied by policy pattern %q", e.FilePath, pattern)), nil
		}

		b, err := os.ReadFile(e.FilePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read file %s: %v", e.FilePath, err)), nil
		}
		content := string(b)
		var updated string
		if e.ReplaceAll {
			updated = strings.ReplaceAll(content, e.OldString, e.NewString)
		} else {
			updated = strings.Replace(content, e.OldString, e.NewString, 1)
		}
		if updated == content {
			return mcp.NewToolResultError(fmt.Sprintf("oldString not found in %s", e.FilePath)), nil
		}
		writes = append(writes, pendingWrite{path: e.FilePath, content: updated})
	}

	for _, w := range writes {
		if err := os.WriteFile(w.path, []byte(w.content), 0o600); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("write file %s: %v", w.path, err)), nil
		}
	}

	return mcp.NewToolResultText(fmt.Sprintf("applied %d edit(s)", len(writes))), nil
}

func parseEdits(req mcp.CallToolRequest) ([]fileEdit, error) {
	if raw, ok := req.GetArguments()["edits"]; ok {
		switch v := raw.(type) {
		case string:
			var edits []fileEdit
			if err := json.Unmarshal([]byte(v), &edits); err != nil {
				return nil, fmt.Errorf("invalid edits JSON: %w", err)
			}
			if len(edits) == 0 {
				return nil, fmt.Errorf("edits must contain at least one edit")
			}
			return edits, nil
		case []any:
			b, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("invalid edits input: %w", err)
			}
			var edits []fileEdit
			if err := json.Unmarshal(b, &edits); err != nil {
				return nil, fmt.Errorf("invalid edits input: %w", err)
			}
			if len(edits) == 0 {
				return nil, fmt.Errorf("edits must contain at least one edit")
			}
			return edits, nil
		default:
			return nil, fmt.Errorf("edits must be a JSON string or array input")
		}
	}

	filePath, err := req.RequireString("filePath")
	if err != nil {
		return nil, fmt.Errorf("filePath is required when edits is not provided")
	}
	oldString, err := req.RequireString("oldString")
	if err != nil {
		return nil, fmt.Errorf("oldString is required when edits is not provided")
	}
	newString, _ := req.RequireString("newString")
	replaceAll := false
	if raw, ok := req.GetArguments()["replaceAll"]; ok {
		if b, ok := raw.(bool); ok {
			replaceAll = b
		}
	}
	return []fileEdit{{
		FilePath:   filePath,
		OldString:  oldString,
		NewString:  newString,
		ReplaceAll: replaceAll,
	}}, nil
}
