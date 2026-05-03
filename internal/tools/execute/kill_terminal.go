package execute

import (
	"context"
	"strings"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/integrations/vscode"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/et-do/no-pilot/internal/terminalstate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolKillTerminal = "execute_killTerminal"

var killTerminalTool = mcp.NewTool(
	toolKillTerminal,
	mcp.WithDescription("Terminate a running terminal session."),
	mcp.WithString("id",
		mcp.Required(),
		mcp.Description("The terminal session id returned by execute_runInTerminal."),
	),
	mcp.WithString("target",
		mcp.Description("Terminal target: managed (default) or vscode (requires bridge)."),
	),
)

func registerKillTerminal(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(killTerminalTool, policy.Enforce(cfg, toolKillTerminal)(handleKillTerminal))
}

func handleKillTerminal(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required and must be a string"), nil
	}
	target := strings.ToLower(strings.TrimSpace(req.GetString("target", "managed")))
	if target == "vscode" {
		bridge, bridgeErr := vscode.NewFromEnv()
		if bridgeErr != nil {
			return mcp.NewToolResultError(bridgeErr.Error()), nil
		}
		resp, bridgeErr := bridge.TerminalKill(ctx, map[string]any{"id": id})
		if bridgeErr != nil {
			return mcp.NewToolResultError(bridgeErr.Error()), nil
		}
		result := mcp.NewToolResultText(resp.Text)
		result.IsError = resp.IsError
		return result, nil
	}

	snapshot, err := terminalstate.Kill(id)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(formatRunningSnapshot(snapshot)), nil
}
