// Package server wires together the MCP server and the no-pilot policy engine.
package server

import (
	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/tools/browser"
	"github.com/et-do/no-pilot/internal/tools/edit"
	"github.com/et-do/no-pilot/internal/tools/execute"
	"github.com/et-do/no-pilot/internal/tools/read"
	"github.com/et-do/no-pilot/internal/tools/search"
	"github.com/et-do/no-pilot/internal/tools/web"
	"github.com/mark3labs/mcp-go/server"
)

const (
	serverName = "no-pilot"
)

// Build creates and returns a configured MCPServer.
// cfg is the merged no-pilot policy; version is injected at build time.
// Tools are registered here; each tool handler checks the policy before
// executing.
func Build(cfg *config.Config, version string) *server.MCPServer {
	s := server.NewMCPServer(
		serverName,
		version,
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	read.Register(s, cfg)
	search.Register(s, cfg)
	edit.Register(s, cfg)
	execute.Register(s, cfg)
	browser.Register(s, cfg)
	web.Register(s, cfg)

	return s
}
