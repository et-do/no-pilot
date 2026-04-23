// Package search registers the #search group of no-pilot tools.
//
// Tools in this group provide code and text search across the workspace.
// Each handler enforces the merged policy via policy.EnforceWithPaths before
// executing.
package search

import (
	"github.com/et-do/no-pilot/internal/config"
	"github.com/mark3labs/mcp-go/server"
)

// Register adds all #search tools to s using cfg for policy enforcement.
func Register(s *server.MCPServer, cfg *config.Config) {
	registerGrepSearch(s, cfg)
}
