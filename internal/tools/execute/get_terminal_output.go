package execute

import (
	"context"
	"fmt"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/et-do/no-pilot/internal/terminalstate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolGetTerminalOutput = "execute_getTerminalOutput"

var getTerminalOutputTool = mcp.NewTool(
	toolGetTerminalOutput,
	mcp.WithDescription("[EXECUTE] Get the current output from a terminal session."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("id",
		mcp.Required(),
		mcp.Description("The terminal session id returned by execute_runInTerminal."),
	),
	mcp.WithNumber("startOffset",
		mcp.Description("Optional first output byte offset (0-based, inclusive)."),
	),
	mcp.WithNumber("endOffset",
		mcp.Description("Optional last output byte offset (0-based, exclusive)."),
	),
)

func registerGetTerminalOutput(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(getTerminalOutputTool, policy.Enforce(cfg, toolGetTerminalOutput)(handleGetTerminalOutput))
}

func handleGetTerminalOutput(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required and must be a string"), nil
	}
	snapshot, ok := terminalstate.GetOutput(id)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("terminal %q not found", id)), nil
	}
	hasRange, start, end := rangeFromRequest(req)
	if hasRange {
		sliced, err := sliceOutputBytes(snapshot.Output, start, end)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		result := mcp.NewToolResultText(formatRangeSnapshot(snapshot, sliced, start, end))
		if snapshot.HasExitCode && snapshot.ExitCode != 0 {
			result.IsError = true
		}
		return result, nil
	}
	if snapshot.Running {
		return mcp.NewToolResultText(formatRunningSnapshot(snapshot)), nil
	}
	result := mcp.NewToolResultText(formatCompletedSnapshot(snapshot))
	if snapshot.HasExitCode && snapshot.ExitCode != 0 {
		result.IsError = true
	}
	return result, nil
}

func rangeFromRequest(req mcp.CallToolRequest) (bool, int, int) {
	args := req.GetArguments()
	_, hasStart := args["startOffset"]
	_, hasEnd := args["endOffset"]
	if !hasStart && !hasEnd {
		return false, 0, 0
	}
	return true, req.GetInt("startOffset", 0), req.GetInt("endOffset", -1)
}

func sliceOutputBytes(text string, start, end int) (string, error) {
	b := []byte(text)
	if start < 0 {
		return "", fmt.Errorf("startOffset must be >= 0")
	}
	if end < -1 {
		return "", fmt.Errorf("endOffset must be >= -1")
	}
	if start > len(b) {
		return "", fmt.Errorf("startOffset %d exceeds output length %d", start, len(b))
	}
	if end == -1 || end > len(b) {
		end = len(b)
	}
	if end < start {
		return "", fmt.Errorf("endOffset %d is before startOffset %d", end, start)
	}
	return string(b[start:end]), nil
}

func formatRangeSnapshot(snapshot terminalstate.Snapshot, sliced string, start, end int) string {
	status := "completed"
	if snapshot.Running {
		status = "running"
	}
	text := fmt.Sprintf("terminal_id: %s\ncommand: %s\nstatus: %s\noutput_bytes: %d\nrange: [%d,%d)",
		snapshot.ID, snapshot.Command, status, snapshot.OutputBytes, start, end)
	if sliced != "" {
		text += "\n" + sliced
	}
	if !snapshot.Running && snapshot.HasExitCode {
		text += fmt.Sprintf("\n(exit code: %d, duration: %dms)", snapshot.ExitCode, snapshot.DurationMS)
	}
	return text
}
