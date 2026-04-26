package search_test

import (
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/testutil"
	"github.com/mark3labs/mcp-go/client"
)

func newClient(t *testing.T, cfg *config.Config) *client.Client {
	t.Helper()
	return testutil.NewClient(t, cfg)
}

func defaultConfig(t *testing.T) *config.Config {
	t.Helper()
	return testutil.DefaultConfig(t)
}
