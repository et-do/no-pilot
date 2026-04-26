package read

import (
	"context"
	"fmt"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/et-do/no-pilot/internal/terminalstate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolTerminalLastCommand = "read_terminalLastCommand"

var terminalLastCommandTool = mcp.NewTool(
	toolTerminalLastCommand,
	mcp.WithDescription("Get the last terminal command run through no-pilot and its output."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
)

func registerTerminalLastCommand(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(terminalLastCommandTool, policy.Enforce(cfg, toolTerminalLastCommand)(handleTerminalLastCommand))
}

func handleTerminalLastCommand(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	last := terminalstate.Get()
	if last.Command == "" {
		return mcp.NewToolResultText(""), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("command: %s\n%s", last.Command, last.Output)), nil
}
