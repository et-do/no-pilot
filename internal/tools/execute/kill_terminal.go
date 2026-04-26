package execute

import (
	"context"

	"github.com/et-do/no-pilot/internal/config"
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
)

func registerKillTerminal(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(killTerminalTool, policy.Enforce(cfg, toolKillTerminal)(handleKillTerminal))
}

func handleKillTerminal(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required and must be a string"), nil
	}
	snapshot, err := terminalstate.Kill(id)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(formatRunningSnapshot(snapshot)), nil
}
