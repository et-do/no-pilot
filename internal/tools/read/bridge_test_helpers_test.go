package read_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/et-do/no-pilot/internal/integrations/vscode"
)

func withBridgeServer(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	t.Setenv(vscode.VSCodeBridgeURLEnv, ts.URL)
	return ts.URL
}
