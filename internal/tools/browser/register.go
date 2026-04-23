// Package browser registers the #browser group of no-pilot tools.
//
// Tools in this group control an embedded browser session. Each handler
// enforces the merged policy via policy.EnforceWithURL before executing,
// checking deny_urls.
package browser

import (
	"github.com/et-do/no-pilot/internal/config"
	"github.com/mark3labs/mcp-go/server"
)

// Register adds all #browser tools to s using cfg for policy enforcement.
func Register(s *server.MCPServer, cfg *config.Config) {
	// TODO: implement browser/navigate, browser/click, browser/screenshot,
	// browser/fillInput, browser/select, browser/hover, browser/scroll,
	// browser/networkRequests, browser/console.
	_ = s
	_ = cfg
}
