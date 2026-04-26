package execute

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/et-do/no-pilot/internal/terminalstate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolListTerminals = "execute_listTerminals"

var listTerminalsTool = mcp.NewTool(
	toolListTerminals,
	mcp.WithDescription("[EXECUTE] List tracked terminal sessions."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
)

func registerListTerminals(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(listTerminalsTool, policy.Enforce(cfg, toolListTerminals)(handleListTerminals))
}

func handleListTerminals(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	snapshots := terminalstate.ListSnapshots()
	if len(snapshots) == 0 {
		return mcp.NewToolResultText("no terminal sessions"), nil
	}

	lines := make([]string, 0, len(snapshots))
	for _, s := range snapshots {
		status := "running"
		if !s.Running {
			status = fmt.Sprintf("completed(exit=%d)", s.ExitCode)
		}
		line := fmt.Sprintf("id=%s status=%s command=%s output_bytes=%d", s.ID, status, strconv.Quote(s.Command), s.OutputBytes)
		if s.Cwd != "" {
			line += " cwd=" + strconv.Quote(s.Cwd)
		}
		if len(s.Env) > 0 {
			line += fmt.Sprintf(" env=%d", len(s.Env))
		}
		lines = append(lines, line)
	}

	return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
}
