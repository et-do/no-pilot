package execute

import (
	"context"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolTestFailure = "execute_testFailure"

var testFailureTool = mcp.NewTool(
	toolTestFailure,
	mcp.WithDescription("Return failure details from the most recent execute_runTests invocation."),
)

func registerTestFailure(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(testFailureTool, policy.Enforce(cfg, toolTestFailure)(handleTestFailure))
}

func handleTestFailure(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	last := getLastTestRun()
	if !last.Ran {
		return mcp.NewToolResultText("no test run recorded yet"), nil
	}
	if !last.Failed {
		return mcp.NewToolResultText("last test run had no failures"), nil
	}
	if last.FailureDetail == "" {
		return mcp.NewToolResultText("last test run failed but no failure detail was captured"), nil
	}
	return mcp.NewToolResultText(last.FailureDetail), nil
}
