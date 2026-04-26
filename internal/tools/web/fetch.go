package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	xhtml "golang.org/x/net/html"
)

const toolFetch = "web_fetch"

var fetchTool = mcp.NewTool(
	toolFetch,
	mcp.WithDescription("[WEB] Fetch a public URL and return extracted text content."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("url",
		mcp.Required(),
		mcp.Description("URL to fetch."),
	),
	mcp.WithString("query",
		mcp.Description("Optional case-insensitive filter; only matching text lines are returned."),
	),
	mcp.WithNumber("maxChars",
		mcp.Description("Maximum characters of extracted text to return. Defaults to 8000."),
	),
	mcp.WithNumber("timeoutMs",
		mcp.Description("HTTP timeout in milliseconds. Defaults to 15000."),
	),
	mcp.WithNumber("maxBytes",
		mcp.Description("Maximum response bytes to read. Defaults to 2097152 (2MiB)."),
	),
	mcp.WithNumber("maxRedirects",
		mcp.Description("Maximum redirects to follow. Defaults to 5. Set to 0 to disable redirects."),
	),
	mcp.WithString("allowedContentTypes",
		mcp.Description("Comma-separated list of allowed response content-type prefixes."),
	),
	mcp.WithString("includeSelector",
		mcp.Description("Optional simple selector to include only matching HTML nodes (#id, .class, or tag)."),
	),
	mcp.WithString("excludeSelector",
		mcp.Description("Optional simple selector to exclude matching HTML nodes (#id, .class, or tag)."),
	),
)

const (
	defaultMaxChars      = 8000
	defaultTimeoutMs     = 15000
	defaultMaxBytes      = 2 * 1024 * 1024
	defaultMaxRedirects  = 5
	maxAllowedReadBytes  = 10 * 1024 * 1024
	maxAllowedTimeoutMs  = 120000
	defaultAllowedCTypes = "text/html,text/plain,application/xhtml+xml,application/xml,text/xml"
)

type fetchCacheEntry struct {
	ETag         string
	LastModified string
	Status       int
	ContentType  string
	Title        string
	Text         string
	FetchedAt    time.Time
}

type fetchCacheStore struct {
	mu      sync.Mutex
	entries map[string]fetchCacheEntry
}

func (c *fetchCacheStore) get(rawURL string) (fetchCacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[rawURL]
	return e, ok
}

func (c *fetchCacheStore) put(rawURL string, e fetchCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[rawURL] = e
}

var fetchCache = &fetchCacheStore{entries: make(map[string]fetchCacheEntry)}

type fetchResult struct {
	URL           string   `json:"url"`
	FinalURL      string   `json:"finalUrl"`
	Status        int      `json:"status"`
	ContentType   string   `json:"contentType"`
	Title         string   `json:"title,omitempty"`
	Query         string   `json:"query,omitempty"`
	IncludeSel    string   `json:"includeSelector,omitempty"`
	ExcludeSel    string   `json:"excludeSelector,omitempty"`
	AllowedCTypes []string `json:"allowedContentTypes"`
	TimeoutMs     int      `json:"timeoutMs"`
	MaxBytes      int      `json:"maxBytes"`
	MaxChars      int      `json:"maxChars"`
	Truncated     bool     `json:"truncated"`
	Cached        bool     `json:"cached"`
	FetchedAt     string   `json:"fetchedAt"`
	Body          string   `json:"body"`
}

func registerFetch(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(fetchTool, policy.EnforceWithURL(cfg, toolFetch, "url")(handleFetch))
}

func handleFetch(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rawURL, err := req.RequireString("url")
	if err != nil || strings.TrimSpace(rawURL) == "" {
		return mcp.NewToolResultError("url is required and must be a non-empty string"), nil
	}
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid url %q: %v", rawURL, err)), nil
	}

	query := strings.TrimSpace(req.GetString("query", ""))
	maxChars := req.GetInt("maxChars", defaultMaxChars)
	if maxChars <= 0 {
		maxChars = defaultMaxChars
	}
	timeoutMs := req.GetInt("timeoutMs", defaultTimeoutMs)
	if timeoutMs <= 0 || timeoutMs > maxAllowedTimeoutMs {
		timeoutMs = defaultTimeoutMs
	}
	maxBytes := req.GetInt("maxBytes", defaultMaxBytes)
	if maxBytes <= 0 || maxBytes > maxAllowedReadBytes {
		maxBytes = defaultMaxBytes
	}
	maxRedirects := req.GetInt("maxRedirects", defaultMaxRedirects)
	if maxRedirects < 0 || maxRedirects > 20 {
		maxRedirects = defaultMaxRedirects
	}
	allowedContentTypes := parseAllowedContentTypes(req.GetString("allowedContentTypes", defaultAllowedCTypes))
	includeSelector := strings.TrimSpace(req.GetString("includeSelector", ""))
	excludeSelector := strings.TrimSpace(req.GetString("excludeSelector", ""))

	client := &http.Client{Timeout: time.Duration(timeoutMs) * time.Millisecond}
	if maxRedirects == 0 {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else {
		client.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return nil
		}
	}

	hreq, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("build request: %v", err)), nil
	}
	hreq.Header.Set("User-Agent", "no-pilot/1.0")
	if cached, ok := fetchCache.get(rawURL); ok {
		if cached.ETag != "" {
			hreq.Header.Set("If-None-Match", cached.ETag)
		}
		if cached.LastModified != "" {
			hreq.Header.Set("If-Modified-Since", cached.LastModified)
		}
	}

	resp, err := client.Do(hreq)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("fetch %s: %v", rawURL, err)), nil
	}
	defer resp.Body.Close()

	finalURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}

	if resp.StatusCode == http.StatusNotModified {
		if cached, ok := fetchCache.get(rawURL); ok {
			result := buildFetchResult(rawURL, finalURL, cached.Status, cached.ContentType, cached.Title, query, includeSelector, excludeSelector, allowedContentTypes, timeoutMs, maxBytes, maxChars, true, cached.Text)
			payload, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
			}
			return mcp.NewToolResultText(string(payload)), nil
		}
	}

	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if !isAllowedContentType(contentType, allowedContentTypes) {
		return mcp.NewToolResultError(fmt.Sprintf("content type %q is not allowed", contentType)), nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxBytes)))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("read response body: %v", err)), nil
	}

	text, title := extractText(string(body), contentType, includeSelector, excludeSelector)
	if query != "" {
		text = filterByQuery(text, query)
	}
	truncated := false
	if len(text) > maxChars {
		text = text[:maxChars]
		truncated = true
	}

	fetchCache.put(rawURL, fetchCacheEntry{
		ETag:         strings.TrimSpace(resp.Header.Get("ETag")),
		LastModified: strings.TrimSpace(resp.Header.Get("Last-Modified")),
		Status:       resp.StatusCode,
		ContentType:  contentType,
		Title:        title,
		Text:         text,
		FetchedAt:    time.Now().UTC(),
	})

	result := buildFetchResult(rawURL, finalURL, resp.StatusCode, contentType, title, query, includeSelector, excludeSelector, allowedContentTypes, timeoutMs, maxBytes, maxChars, false, text)
	result.Truncated = truncated
	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(payload)), nil
}

func buildFetchResult(rawURL, finalURL string, status int, contentType, title, query, includeSelector, excludeSelector string, allowed []string, timeoutMs, maxBytes, maxChars int, cached bool, body string) fetchResult {
	return fetchResult{
		URL:           rawURL,
		FinalURL:      finalURL,
		Status:        status,
		ContentType:   contentType,
		Title:         title,
		Query:         query,
		IncludeSel:    includeSelector,
		ExcludeSel:    excludeSelector,
		AllowedCTypes: append([]string(nil), allowed...),
		TimeoutMs:     timeoutMs,
		MaxBytes:      maxBytes,
		MaxChars:      maxChars,
		Cached:        cached,
		FetchedAt:     time.Now().UTC().Format(time.RFC3339),
		Body:          strings.TrimSpace(body),
	}
}

func parseAllowedContentTypes(raw string) []string {
	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.ToLower(strings.TrimSpace(p))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	if len(out) == 0 {
		out = []string{"text/html", "text/plain", "application/xhtml+xml", "application/xml", "text/xml"}
	}
	sort.Strings(out)
	return out
}

func isAllowedContentType(contentType string, allowed []string) bool {
	if contentType == "" {
		return false
	}
	base := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	for _, prefix := range allowed {
		if strings.HasPrefix(base, prefix) {
			return true
		}
	}
	return false
}

func extractText(src, contentType, includeSelector, excludeSelector string) (string, string) {
	if strings.HasPrefix(contentType, "text/plain") {
		return normalizeWhitespace(src), ""
	}
	doc, err := xhtml.Parse(strings.NewReader(src))
	if err != nil {
		return normalizeWhitespace(src), ""
	}

	title := findTitle(doc)
	includeFn := compileSelector(includeSelector)
	excludeFn := compileSelector(excludeSelector)
	includeNodes := []*xhtml.Node{}
	if includeFn != nil {
		collectMatchingNodes(doc, includeFn, &includeNodes)
	}

	var lines []string
	if len(includeNodes) == 0 {
		collectText(doc, excludeFn, &lines)
	} else {
		for _, n := range includeNodes {
			collectText(n, excludeFn, &lines)
		}
	}
	return strings.Join(lines, "\n"), title
}

func compileSelector(sel string) func(*xhtml.Node) bool {
	sel = strings.TrimSpace(sel)
	if sel == "" {
		return nil
	}
	if strings.HasPrefix(sel, "#") {
		id := strings.TrimSpace(strings.TrimPrefix(sel, "#"))
		if id == "" {
			return nil
		}
		return func(n *xhtml.Node) bool {
			return n.Type == xhtml.ElementNode && attrValue(n, "id") == id
		}
	}
	if strings.HasPrefix(sel, ".") {
		className := strings.TrimSpace(strings.TrimPrefix(sel, "."))
		if className == "" {
			return nil
		}
		return func(n *xhtml.Node) bool {
			if n.Type != xhtml.ElementNode {
				return false
			}
			for _, c := range strings.Fields(attrValue(n, "class")) {
				if c == className {
					return true
				}
			}
			return false
		}
	}
	tag := strings.ToLower(sel)
	return func(n *xhtml.Node) bool {
		return n.Type == xhtml.ElementNode && strings.EqualFold(n.Data, tag)
	}
}

func collectMatchingNodes(n *xhtml.Node, match func(*xhtml.Node) bool, out *[]*xhtml.Node) {
	if n == nil {
		return
	}
	if match(n) {
		*out = append(*out, n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectMatchingNodes(c, match, out)
	}
}

func collectText(n *xhtml.Node, exclude func(*xhtml.Node) bool, lines *[]string) {
	if n == nil {
		return
	}
	if exclude != nil && exclude(n) {
		return
	}
	if n.Type == xhtml.ElementNode {
		tag := strings.ToLower(n.Data)
		if tag == "script" || tag == "style" || tag == "noscript" || tag == "template" {
			return
		}
	}
	if n.Type == xhtml.TextNode {
		v := normalizeWhitespace(n.Data)
		if v != "" {
			*lines = append(*lines, v)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectText(c, exclude, lines)
	}
}

func normalizeWhitespace(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = html.UnescapeString(s)
	parts := strings.Fields(s)
	return strings.TrimSpace(strings.Join(parts, " "))
}

func findTitle(n *xhtml.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == xhtml.ElementNode && strings.EqualFold(n.Data, "title") {
		return normalizeWhitespace(nodeText(n))
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if t := findTitle(c); t != "" {
			return t
		}
	}
	return ""
}

func nodeText(n *xhtml.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == xhtml.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(nodeText(c))
		b.WriteString(" ")
	}
	return b.String()
}

func attrValue(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func filterByQuery(text, query string) string {
	ql := strings.ToLower(query)
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), ql) {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
