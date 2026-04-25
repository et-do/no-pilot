// Package read registers the #read group of no-pilot tools.
//
// Tools in this group provide read-only access to files, directories, and
// notebook content. Each handler enforces the merged policy via
// policy.EnforceWithPaths before executing.
package read

import (
	"github.com/et-do/no-pilot/internal/config"
	"github.com/mark3labs/mcp-go/server"
)

// Register adds all #read tools to s using cfg for policy enforcement.
func Register(s *server.MCPServer, cfg config.Provider) {
	registerReadFile(s, cfg)
	registerListDirectory(s, cfg)
	registerProblems(s, cfg)
}
