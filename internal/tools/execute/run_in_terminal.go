package execute

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolRunInTerminal = "execute/runInTerminal"

var runInTerminalTool = mcp.NewTool(
	toolRunInTerminal,
	mcp.WithDescription("Run a shell command in the workspace. Returns stdout and stderr output, exit code, and duration."),
	mcp.WithString("command",
		mcp.Required(),
		mcp.Description("Shell command to execute (e.g. 'go build .')."),
	),
)

func registerRunInTerminal(s *server.MCPServer, cfg *config.Config) {
	s.AddTool(runInTerminalTool, policy.EnforceWithCommand(cfg, toolRunInTerminal, "command")(handleRunInTerminal))
}

func handleRunInTerminal(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cmdStr, err := req.RequireString("command")
	if err != nil {
		return mcp.NewToolResultError("command is required and must be a string"), nil
	}

	start := time.Now()
	cmd := exec.Command("sh", "-c", cmdStr)
	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return mcp.NewToolResultError(fmt.Sprintf("exec error: %v", err)), nil
		}
	}

	resultText := string(output) + fmt.Sprintf("\n(exit code: %d, duration: %dms)", exitCode, duration.Milliseconds())
	result := mcp.NewToolResultText(resultText)
	if exitCode != 0 {
		result.IsError = true
	}
	return result, nil
}
