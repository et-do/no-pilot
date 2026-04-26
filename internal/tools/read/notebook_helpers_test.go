package read_test

import (
	"context"
	"testing"

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
