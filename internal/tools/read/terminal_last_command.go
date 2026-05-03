package read

import (
	"context"
	"fmt"
	"strings"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/integrations/vscode"
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
	mcp.WithString("target",
		mcp.Description("Terminal source: managed (default) or vscode (requires bridge)."),
	),
)

func registerTerminalLastCommand(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(terminalLastCommandTool, policy.Enforce(cfg, toolTerminalLastCommand)(handleTerminalLastCommand))
}

func handleTerminalLastCommand(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	target := strings.ToLower(strings.TrimSpace(req.GetString("target", "managed")))
	if target == "vscode" {
		bridge, bridgeErr := vscode.NewFromEnv()
		if bridgeErr != nil {
			return mcp.NewToolResultError(bridgeErr.Error()), nil
		}
		resp, bridgeErr := bridge.TerminalLastCommand(ctx)
		if bridgeErr != nil {
			return mcp.NewToolResultError(bridgeErr.Error()), nil
		}
		result := mcp.NewToolResultText(resp.Text)
		result.IsError = resp.IsError
		return result, nil
	}

	last := terminalstate.Get()
	if last.Command == "" {
		return mcp.NewToolResultText(""), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("command: %s\n%s", last.Command, last.Output)), nil
}
