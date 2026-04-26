package read_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

const sampleNotebook = `{
  "cells": [
    {
      "cell_type": "markdown",
      "id": "md-1",
      "source": ["# Title\n", "Some text\n"],
      "metadata": {}
    },
    {
      "cell_type": "code",
      "execution_count": 3,
      "id": "code-1",
      "source": ["print('hello')\n"],
      "outputs": [
        {
          "output_type": "stream",
          "name": "stdout",
          "text": ["hello\\n"]
        }
      ],
      "metadata": {}
    }
  ],
  "metadata": {},
  "nbformat": 4,
  "nbformat_minor": 5
}`

func callNotebookSummary(t *testing.T, c *client.Client, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = "read_getNotebookSummary"
	req.Params.Arguments = args
	result, err := c.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	return result
}

func callNotebookOutput(t *testing.T, c *client.Client, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = "read_readNotebookCellOutput"
	req.Params.Arguments = args
	result, err := c.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	return result
}

func TestGetNotebookSummary_listsCells(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.ipynb")
	writeFile(t, path, sampleNotebook)

	c := newClient(t, defaultConfig(t))
	result := callNotebookSummary(t, c, map[string]any{"filePath": path})
	if result.IsError {
		t.Fatalf("unexpected error: %v", textContent(t, result))
	}
	text := textContent(t, result)
	if !strings.Contains(text, "id=md-1 type=markdown index=0") {
		t.Fatalf("missing markdown cell summary: %q", text)
	}
	if !strings.Contains(text, "id=code-1 type=code index=1 execution_count=3 outputs=1") {
		t.Fatalf("missing code cell summary: %q", text)
	}
}

func TestGetNotebookSummary_denyPathBlocked(t *testing.T) {
	f := true
	cfg := &config.Config{Tools: map[string]config.ToolPolicy{
		"read_getNotebookSummary": {Allowed: &f, DenyPaths: []string{"**/secret/**"}},
	}}
	dir := t.TempDir()
	path := filepath.Join(dir, "secret", "sample.ipynb")
	writeFile(t, path, sampleNotebook)
	c := newClient(t, cfg)
	result := callNotebookSummary(t, c, map[string]any{"filePath": path})
	if !result.IsError {
		t.Fatal("IsError = false, want true when deny_paths blocks")
	}
}

func TestReadNotebookCellOutput_returnsText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.ipynb")
	writeFile(t, path, sampleNotebook)

	c := newClient(t, defaultConfig(t))
	result := callNotebookOutput(t, c, map[string]any{"filePath": path, "cellId": "code-1"})
	if result.IsError {
		t.Fatalf("unexpected error: %v", textContent(t, result))
	}
	if !strings.Contains(textContent(t, result), "hello") {
		t.Fatalf("expected cell output, got %q", textContent(t, result))
	}
}

func TestReadNotebookCellOutput_missingCellReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.ipynb")
	writeFile(t, path, sampleNotebook)

	c := newClient(t, defaultConfig(t))
	result := callNotebookOutput(t, c, map[string]any{"filePath": path, "cellId": "missing"})
	if !result.IsError {
		t.Fatal("IsError = false, want true for missing cell")
	}
}
