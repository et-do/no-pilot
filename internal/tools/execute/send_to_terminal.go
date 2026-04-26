package execute

import (
	"context"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/et-do/no-pilot/internal/terminalstate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolSendToTerminal = "execute_sendToTerminal"

var sendToTerminalTool = mcp.NewTool(
	toolSendToTerminal,
	mcp.WithDescription("Send a line of input to a running terminal session."),
	mcp.WithString("id",
		mcp.Required(),
		mcp.Description("The terminal session id returned by execute_runInTerminal."),
	),
	mcp.WithString("command",
		mcp.Required(),
		mcp.Description("The input line to send. Empty or whitespace sends just Enter."),
	),
)

func registerSendToTerminal(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(sendToTerminalTool, policy.Enforce(cfg, toolSendToTerminal)(handleSendToTerminal))
}

func handleSendToTerminal(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required and must be a string"), nil
	}
	input, err := req.RequireString("command")
	if err != nil {
		return mcp.NewToolResultError("command is required and must be a string"), nil
	}
	snapshot, err := terminalstate.Send(id, input)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(formatRunningSnapshot(snapshot)), nil
}
