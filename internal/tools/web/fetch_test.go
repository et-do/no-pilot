package web_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/testutil"
)

type fetchResponse struct {
	URL             string   `json:"url"`
	FinalURL        string   `json:"finalUrl"`
	Status          int      `json:"status"`
	ContentType     string   `json:"contentType"`
	Title           string   `json:"title"`
	Body            string   `json:"body"`
	Query           string   `json:"query"`
	IncludeSelector string   `json:"includeSelector"`
	ExcludeSelector string   `json:"excludeSelector"`
	AllowedTypes    []string `json:"allowedContentTypes"`
	Cached          bool     `json:"cached"`
	Truncated       bool     `json:"truncated"`
	TimeoutMs       int      `json:"timeoutMs"`
	MaxBytes        int      `json:"maxBytes"`
	MaxChars        int      `json:"maxChars"`
}

func mustFetchJSON(t *testing.T, args map[string]any) fetchResponse {
	t.Helper()
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := testutil.CallTool(t, c, "web_fetch", args)
	if res.IsError {
		t.Fatalf("unexpected error: %q", testutil.TextContent(res))
	}
	var got fetchResponse
	if err := json.Unmarshal([]byte(testutil.TextContent(res)), &got); err != nil {
		t.Fatalf("unmarshal response JSON: %v\nraw=%q", err, testutil.TextContent(res))
	}
	return got
}

func TestFetch_basicHTMLExtraction(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Example</title></head><body><h1>Hello</h1><p>World text</p></body></html>`))
	}))
	defer ts.Close()

	got := mustFetchJSON(t, map[string]any{"url": ts.URL})
	if got.Status != http.StatusOK {
		t.Fatalf("status=%d, want 200", got.Status)
	}
	if got.Title != "Example" {
		t.Fatalf("title=%q, want Example", got.Title)
	}
	if !strings.Contains(got.Body, "Hello") || !strings.Contains(got.Body, "World text") {
		t.Fatalf("expected extracted text, got %q", got.Body)
	}
}

func TestFetch_queryFiltersLines(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<div>alpha line</div><div>beta line</div>`))
	}))
	defer ts.Close()

	got := mustFetchJSON(t, map[string]any{"url": ts.URL, "query": "beta"})
	if strings.Contains(strings.ToLower(got.Body), "alpha") {
		t.Fatalf("expected alpha to be filtered out, got %q", got.Body)
	}
	if !strings.Contains(strings.ToLower(got.Body), "beta") {
		t.Fatalf("expected beta line, got %q", got.Body)
	}
}

func TestFetch_selectorIncludeExclude(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><main id="article"><h1>Keep</h1><p class="ad">Drop me</p></main><footer>Footer</footer></body></html>`))
	}))
	defer ts.Close()

	got := mustFetchJSON(t, map[string]any{
		"url":             ts.URL,
		"includeSelector": "#article",
		"excludeSelector": ".ad",
	})
	if !strings.Contains(got.Body, "Keep") {
		t.Fatalf("expected included content, got %q", got.Body)
	}
	if strings.Contains(got.Body, "Drop me") {
		t.Fatalf("expected excluded content removed, got %q", got.Body)
	}
	if strings.Contains(got.Body, "Footer") {
		t.Fatalf("expected non-included content removed, got %q", got.Body)
	}
}

func TestFetch_disallowedContentType(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := testutil.CallTool(t, c, "web_fetch", map[string]any{"url": ts.URL})
	if !res.IsError {
		t.Fatal("IsError = false, want true for disallowed content type")
	}
}

func TestFetch_maxRedirectsZero(t *testing.T) {
	dst := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("target"))
	}))
	defer dst.Close()

	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, dst.URL, http.StatusFound)
	}))
	defer src.Close()

	got := mustFetchJSON(t, map[string]any{"url": src.URL, "maxRedirects": 0, "allowedContentTypes": "text/html,text/plain"})
	if got.Status != http.StatusFound {
		t.Fatalf("status=%d, want 302 when redirects disabled", got.Status)
	}
}

func TestFetch_usesConditionalCache(t *testing.T) {
	var seenIfNoneMatch string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenIfNoneMatch = r.Header.Get("If-None-Match")
		if seenIfNoneMatch == "v1" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("ETag", "v1")
		_, _ = w.Write([]byte(`<html><body><p>cached body</p></body></html>`))
	}))
	defer ts.Close()

	first := mustFetchJSON(t, map[string]any{"url": ts.URL})
	if first.Cached {
		t.Fatal("first response cached=true, want false")
	}
	second := mustFetchJSON(t, map[string]any{"url": ts.URL})
	if !second.Cached {
		t.Fatal("second response cached=false, want true")
	}
	if !strings.Contains(second.Body, "cached body") {
		t.Fatalf("expected cached body text, got %q", second.Body)
	}
	if seenIfNoneMatch != "v1" {
		t.Fatalf("If-None-Match=%q, want v1", seenIfNoneMatch)
	}
}

func TestFetch_denyURLBlocked(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	tf := true
	cfg := &config.Config{Tools: map[string]config.ToolPolicy{
		"web_fetch": {Allowed: &tf, DenyURLs: []string{"127.0.0.1"}},
	}}
	c := testutil.NewClient(t, cfg)
	res := testutil.CallTool(t, c, "web_fetch", map[string]any{"url": ts.URL})
	if !res.IsError {
		t.Fatal("IsError = false, want true when deny_urls blocks")
	}
}

func TestFetch_invalidURL(t *testing.T) {
	c := testutil.NewClient(t, testutil.DefaultConfig(t))
	res := testutil.CallTool(t, c, "web_fetch", map[string]any{"url": "://bad-url"})
	if !res.IsError {
		t.Fatal("IsError = false, want true for invalid url")
	}
}
