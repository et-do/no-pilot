package execute

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/et-do/no-pilot/internal/terminalstate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolRunInTerminal = "execute_runInTerminal"

var runInTerminalTool = mcp.NewTool(
	toolRunInTerminal,
	mcp.WithDescription("Run a shell command in the workspace. Supports sync and async terminal sessions."),
	mcp.WithString("command",
		mcp.Required(),
		mcp.Description("Shell command to execute (e.g. 'go build .')."),
	),
	mcp.WithString("mode",
		mcp.Description("Execution mode: 'sync' (default) or 'async'."),
	),
	mcp.WithNumber("timeout",
		mcp.Description("Timeout in milliseconds for sync execution. A timed out sync run returns a terminal id for follow-up reads."),
	),
	mcp.WithString("cwd",
		mcp.Description("Optional working directory for this terminal session."),
	),
	mcp.WithString("env",
		mcp.Description("Optional newline-separated KEY=VALUE entries to add to this terminal session environment."),
	),
)

func registerRunInTerminal(s *server.MCPServer, cfg config.Provider) {
	h := policy.EnforceWithCommand(cfg, toolRunInTerminal, "command")(handleRunInTerminal)
	h = policy.EnforceWithPaths(cfg, toolRunInTerminal, "cwd")(h)
	s.AddTool(runInTerminalTool, h)
}

func handleRunInTerminal(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cmdStr, err := req.RequireString("command")
	if err != nil {
		return mcp.NewToolResultError("command is required and must be a string"), nil
	}
	mode := req.GetString("mode", "sync")
	if mode != "sync" && mode != "async" {
		return mcp.NewToolResultError("mode must be 'sync' or 'async'"), nil
	}
	timeout := time.Duration(req.GetInt("timeout", 0)) * time.Millisecond
	cwd := strings.TrimSpace(req.GetString("cwd", ""))
	env, err := parseEnvEntries(req.GetString("env", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	snapshot, err := terminalstate.StartWithOptions(cmdStr, terminalstate.StartOptions{Cwd: cwd, Env: env})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("exec error: %v", err)), nil
	}

	if mode == "async" {
		return mcp.NewToolResultText(formatRunningSnapshot(snapshot)), nil
	}

	snapshot, completed, err := terminalstate.Wait(snapshot.ID, timeout)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if !completed {
		return mcp.NewToolResultText(formatRunningSnapshot(snapshot)), nil
	}

	result := mcp.NewToolResultText(formatCompletedSnapshot(snapshot))
	if snapshot.HasExitCode && snapshot.ExitCode != 0 {
		result.IsError = true
	}
	return result, nil
}

func formatRunningSnapshot(snapshot terminalstate.Snapshot) string {
	text := fmt.Sprintf("terminal_id: %s\ncommand: %s\nstatus: running", snapshot.ID, snapshot.Command)
	if snapshot.Output != "" {
		text += "\n" + snapshot.Output
	}
	return text
}

func formatCompletedSnapshot(snapshot terminalstate.Snapshot) string {
	text := snapshot.Output
	if text != "" {
		text += "\n"
	}
	text += fmt.Sprintf("(terminal_id: %s, exit code: %d, duration: %dms)", snapshot.ID, snapshot.ExitCode, snapshot.DurationMS)
	return text
}

func parseEnvEntries(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			return nil, fmt.Errorf("invalid env entry %q: expected KEY=VALUE", line)
		}
		out = append(out, line)
	}
	return out, nil
}
