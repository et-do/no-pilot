package edit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolRenameSymbol = "edit_renameSymbol"

var renameSymbolTool = mcp.NewTool(
	toolRenameSymbol,
	mcp.WithDescription("[EDIT] Rename symbol text across files (lexical replacement with word boundaries, not language-server aware)."),
	mcp.WithString("symbol",
		mcp.Required(),
		mcp.Description("Current symbol name to rename."),
	),
	mcp.WithString("newName",
		mcp.Required(),
		mcp.Description("New symbol name."),
	),
	mcp.WithString("filePath",
		mcp.Description("Optional file or directory scope hint. If file is provided, its parent directory is scanned."),
	),
	mcp.WithString("rootPath",
		mcp.Description("Optional root directory to scan. Defaults to workspace root."),
	),
)

func registerRenameSymbol(s *server.MCPServer, cfg config.Provider) {
	h := policy.EnforceWithPaths(cfg, toolRenameSymbol, "filePath", "rootPath")(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleRenameSymbol(ctx, req, cfg)
	})
	s.AddTool(renameSymbolTool, h)
}

func handleRenameSymbol(_ context.Context, req mcp.CallToolRequest, cfg config.Provider) (*mcp.CallToolResult, error) {
	symbol, err := req.RequireString("symbol")
	if err != nil || strings.TrimSpace(symbol) == "" {
		return mcp.NewToolResultError("symbol is required and must be a non-empty string"), nil
	}
	newName, err := req.RequireString("newName")
	if err != nil || strings.TrimSpace(newName) == "" {
		return mcp.NewToolResultError("newName is required and must be a non-empty string"), nil
	}
	if symbol == newName {
		return mcp.NewToolResultError("symbol and newName must differ"), nil
	}

	root := strings.TrimSpace(req.GetString("rootPath", ""))
	if root == "" {
		root = "."
	}
	if hint := strings.TrimSpace(req.GetString("filePath", "")); hint != "" {
		if st, err := os.Stat(hint); err == nil {
			if st.IsDir() {
				root = hint
			} else {
				root = filepath.Dir(hint)
			}
		}
	}

	wordRE, err := regexp.Compile(`\b` + regexp.QuoteMeta(symbol) + `\b`)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid symbol: %v", err)), nil
	}

	changedFiles := 0
	replacementCount := 0
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !looksTextPath(path) {
			return nil
		}
		if denied, _ := pathDenied(cfg, toolRenameSymbol, path); denied {
			return nil
		}

		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(b)
		matches := wordRE.FindAllStringIndex(content, -1)
		if len(matches) == 0 {
			return nil
		}
		replacementCount += len(matches)
		updated := wordRE.ReplaceAllString(content, newName)
		if updated == content {
			return nil
		}
		if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
			return err
		}
		changedFiles++
		return nil
	})
	if walkErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rename failed: %v", walkErr)), nil
	}
	if changedFiles == 0 {
		return mcp.NewToolResultError("no symbol matches found"), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("renamed %q to %q in %d file(s), %d replacement(s)", symbol, newName, changedFiles, replacementCount)), nil
}
