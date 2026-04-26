// no-pilot is a zero-trust MCP server that mirrors GitHub Copilot's built-in
// VSCode tools with configurable system-level restrictions.
//
// Each tool (file reads, shell commands, search, etc.) proxied through no-pilot
// can be restricted at the user level (~/.config/no-pilot/config.yaml) or at
// the project level (.no-pilot.yaml in the workspace root). Restrictions can
// only tighten as config layers accumulate — never loosen.
//
// Distribution: teams point their VSCode MCP server config at the no-pilot
// binary; no daemon or sidecar required.
//
// Usage:
//
//	no-pilot     # start MCP server on stdio (default)
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/logging"
	"github.com/et-do/no-pilot/internal/policy"
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
	logger := log.New(os.Stderr, "[no-pilot] ", log.LstdFlags)
	started := time.Now()
	logLevel := logging.ParseLevel(os.Getenv("NO_PILOT_LOG_LEVEL"))

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	if logging.Enabled(logLevel, logging.LevelInfo) {
		logger.Printf("starting version=%s pid=%d cwd=%s log_level=%s", version, os.Getpid(), wd, logging.String(logLevel))
	}

	policy.SetLogger(logger, logLevel)

	watcher, err := config.NewWatcher(wd, logger)
	if err != nil {
		return fmt.Errorf("init config watcher: %w", err)
	}
	defer func() {
		if err := watcher.Close(); err != nil {
			logger.Printf("config watcher close error: %v", err)
		}
	}()

	s := nopilotserver.Build(watcher, version)
	if logging.Enabled(logLevel, logging.LevelInfo) {
		logger.Printf("server built, listening on stdio")
	}
	err = server.ServeStdio(s, server.WithErrorLogger(logger))
	if err != nil {
		logger.Printf("server exited with error=%v uptime=%s", err, time.Since(started).Round(time.Millisecond))
		return err
	}
	if logging.Enabled(logLevel, logging.LevelInfo) {
		logger.Printf("server exited cleanly uptime=%s", time.Since(started).Round(time.Millisecond))
	}
	return nil
}
