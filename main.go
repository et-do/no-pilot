// no-pilot is a zero-trust MCP server that mirrors GitHub Copilot's built-in
// VSCode tools with configurable system-level restrictions.
//
// Each tool (file reads, shell commands, search, etc.) proxied through no-pilot
// can be restricted at the user level (~/.config/no-pilot/config.yaml) or at
// the project level (.no-pilot.yaml in the workspace root). Project config
// always takes precedence.
//
// Distribution: teams point their VSCode MCP server config at the no-pilot
// binary; no daemon or sidecar required.
//
// Usage:
//
//	no-pilot                        # start MCP server on stdio (default)
//	no-pilot --config ./no-pilot.yaml
package main

import (
	"fmt"
	"os"

	"github.com/et-do/no-pilot/internal/config"
	nopilotserver "github.com/et-do/no-pilot/internal/server"
	"github.com/mark3labs/mcp-go/server"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "no-pilot: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, err := config.Load(wd)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	s := nopilotserver.Build(cfg, version)
	return server.ServeStdio(s)
}
