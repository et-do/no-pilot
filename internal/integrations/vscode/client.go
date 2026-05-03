package vscode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

const VSCodeBridgeURLEnv = "NO_PILOT_VSCODE_BRIDGE_URL"

// BridgeURLEnv is kept as a compatibility alias.
const BridgeURLEnv = VSCodeBridgeURLEnv

var ErrNotConfigured = errors.New("vscode bridge is not configured")

type ToolResponse struct {
	Text    string `json:"text"`
	IsError bool   `json:"isError,omitempty"`
}

type Client struct {
	baseURL *url.URL
	http    *http.Client
}

func New(rawURL string) (*Client, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, ErrNotConfigured
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse bridge url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("bridge url must include scheme and host")
	}
	return &Client{
		baseURL: u,
		http:    &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func NewFromEnv() (*Client, error) {
	return New(os.Getenv(VSCodeBridgeURLEnv))
}

func (c *Client) ReadProblems(ctx context.Context, payload map[string]any) (ToolResponse, error) {
	var out ToolResponse
	if err := c.post(ctx, "/read/problems", payload, &out); err != nil {
		return ToolResponse{}, err
	}
	return out, nil
}

func (c *Client) TerminalRun(ctx context.Context, payload map[string]any) (ToolResponse, error) {
	var out ToolResponse
	if err := c.post(ctx, "/terminal/run", payload, &out); err != nil {
		return ToolResponse{}, err
	}
	return out, nil
}

func (c *Client) TerminalGetOutput(ctx context.Context, payload map[string]any) (ToolResponse, error) {
	var out ToolResponse
	if err := c.post(ctx, "/terminal/get_output", payload, &out); err != nil {
		return ToolResponse{}, err
	}
	return out, nil
}

func (c *Client) TerminalSend(ctx context.Context, payload map[string]any) (ToolResponse, error) {
	var out ToolResponse
	if err := c.post(ctx, "/terminal/send", payload, &out); err != nil {
		return ToolResponse{}, err
	}
	return out, nil
}

func (c *Client) TerminalKill(ctx context.Context, payload map[string]any) (ToolResponse, error) {
	var out ToolResponse
	if err := c.post(ctx, "/terminal/kill", payload, &out); err != nil {
		return ToolResponse{}, err
	}
	return out, nil
}

func (c *Client) TerminalList(ctx context.Context) (ToolResponse, error) {
	var out ToolResponse
	if err := c.post(ctx, "/terminal/list", map[string]any{}, &out); err != nil {
		return ToolResponse{}, err
	}
	return out, nil
}

func (c *Client) TerminalLastCommand(ctx context.Context) (ToolResponse, error) {
	var out ToolResponse
	if err := c.post(ctx, "/terminal/last_command", map[string]any{}, &out); err != nil {
		return ToolResponse{}, err
	}
	return out, nil
}

func (c *Client) post(ctx context.Context, endpoint string, payload any, out any) error {
	if c == nil || c.baseURL == nil {
		return ErrNotConfigured
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode bridge payload: %w", err)
	}
	u := *c.baseURL
	u.Path = path.Join(strings.TrimSuffix(c.baseURL.Path, "/"), endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create bridge request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("bridge request failed: %w", err)
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("read bridge response: %w", err)
	}
	if res.StatusCode >= 400 {
		msg := strings.TrimSpace(string(resBody))
		if msg == "" {
			msg = http.StatusText(res.StatusCode)
		}
		return fmt.Errorf("bridge error: %s", msg)
	}
	if len(resBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(resBody, out); err != nil {
		return fmt.Errorf("decode bridge response: %w", err)
	}
	return nil
}
