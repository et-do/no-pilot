// Package edit registers the #edit group of no-pilot tools.
//
// Tools in this group mutate files in the workspace. Each handler enforces the
// merged policy via policy.EnforceWithPaths before executing.
package edit

import (
	"github.com/et-do/no-pilot/internal/config"
	"github.com/mark3labs/mcp-go/server"
)

// Register adds all #edit tools to s using cfg for policy enforcement.
func Register(s *server.MCPServer, cfg config.Provider) {
	registerCreateFile(s, cfg)
	registerCreateDirectory(s, cfg)
	registerEditFiles(s, cfg)
	registerRenameSymbol(s, cfg)
	registerEditNotebook(s, cfg)
	registerCreateNotebook(s, cfg)
}
