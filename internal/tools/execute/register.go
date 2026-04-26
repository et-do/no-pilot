// Package execute registers the #execute group of no-pilot tools.
//
// Tools in this group run commands in the workspace environment. Each handler
// enforces the merged policy via policy.EnforceWithCommand before executing,
// checking allow_commands and deny_commands.
package execute

import (
	"github.com/et-do/no-pilot/internal/config"
	"github.com/mark3labs/mcp-go/server"
)

// Register adds all #execute tools to s using cfg for policy enforcement.
func Register(s *server.MCPServer, cfg config.Provider) {
	registerRunInTerminal(s, cfg)
	registerListTerminals(s, cfg)
	registerGetTerminalOutput(s, cfg)
	registerSendToTerminal(s, cfg)
	registerKillTerminal(s, cfg)
}
