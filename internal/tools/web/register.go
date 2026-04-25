// Package web registers the #web group of no-pilot tools.
//
// Tools in this group fetch content from the public internet. Each handler
// enforces the merged policy via policy.EnforceWithURL before executing,
// checking deny_urls.
package web

import (
	"github.com/et-do/no-pilot/internal/config"
	"github.com/mark3labs/mcp-go/server"
)

// Register adds all #web tools to s using cfg for policy enforcement.
func Register(s *server.MCPServer, cfg config.Provider) {
	// TODO: implement web/fetchWebpage.
	_ = s
	_ = cfg
}
