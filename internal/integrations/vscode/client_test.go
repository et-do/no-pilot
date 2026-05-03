package vscode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientReadProblems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/read/problems" {
			t.Fatalf("path = %q, want /read/problems", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(ToolResponse{Text: "x.go: undefined: foo", IsError: false})
	}))
	defer ts.Close()

	c, err := New(ts.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res, err := c.ReadProblems(context.Background(), map[string]any{"filePath": "x.go"})
	if err != nil {
		t.Fatalf("ReadProblems: %v", err)
	}
	if res.Text == "" {
		t.Fatal("expected non-empty response text")
	}
}

func TestClientNotConfigured(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("expected not configured error")
	}
}
