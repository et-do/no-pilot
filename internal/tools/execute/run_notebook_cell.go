package execute

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/et-do/no-pilot/internal/config"
	"github.com/et-do/no-pilot/internal/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const toolRunNotebookCell = "execute_runNotebookCell"

var runNotebookCellTool = mcp.NewTool(
	toolRunNotebookCell,
	mcp.WithDescription("Execute notebook code cells up to a target cell and persist outputs to the .ipynb file."),
	mcp.WithString("filePath",
		mcp.Required(),
		mcp.Description("Absolute path to the notebook file."),
	),
	mcp.WithString("cellId",
		mcp.Required(),
		mcp.Description("ID of the target code cell to execute."),
	),
	mcp.WithString("continueOnError",
		mcp.Description("Whether execution should continue after a cell error."),
	),
)

func registerRunNotebookCell(s *server.MCPServer, cfg config.Provider) {
	s.AddTool(runNotebookCellTool, policy.EnforceWithPaths(cfg, toolRunNotebookCell, "filePath")(handleRunNotebookCell))
}

func handleRunNotebookCell(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, err := req.RequireString("filePath")
	if err != nil {
		return mcp.NewToolResultError("filePath is required and must be a string"), nil
	}
	cellID, err := req.RequireString("cellId")
	if err != nil {
		return mcp.NewToolResultError("cellId is required and must be a string"), nil
	}
	continueOnError := false
	if raw, ok := req.GetArguments()["continueOnError"]; ok {
		if b, ok := raw.(bool); ok {
			continueOnError = b
		}
	}

	doc, cells, err := loadNotebookCells(filePath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("load notebook: %v", err)), nil
	}

	targetIndex := -1
	for i, rawCell := range cells {
		cell, ok := rawCell.(map[string]any)
		if !ok {
			continue
		}
		if notebookCellID(cell) == cellID {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return mcp.NewToolResultError(fmt.Sprintf("cell %q not found", cellID)), nil
	}

	targetCell, ok := cells[targetIndex].(map[string]any)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("cell %q is malformed", cellID)), nil
	}
	if strings.TrimSpace(strings.ToLower(stringValue(targetCell["cell_type"]))) != "code" {
		return mcp.NewToolResultError("target cell is not a code cell"), nil
	}

	if _, err := exec.LookPath("python3"); err != nil {
		return mcp.NewToolResultError("python3 is required to execute notebook cells in standalone mode"), nil
	}

	codeCells := make([]codeCellRequest, 0)
	codeCellIndices := make([]int, 0)
	for i := 0; i <= targetIndex; i++ {
		cell, ok := cells[i].(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(strings.ToLower(stringValue(cell["cell_type"]))) != "code" {
			continue
		}
		code := joinNotebookSource(cell["source"])
		codeCells = append(codeCells, codeCellRequest{Code: code, CellID: notebookCellID(cell)})
		codeCellIndices = append(codeCellIndices, i)
	}
	if len(codeCells) == 0 {
		return mcp.NewToolResultError("no executable code cells found before target"), nil
	}

	execResult, err := executeCodeCellsPython(codeCells, continueOnError)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("execute notebook cells: %v", err)), nil
	}

	if len(execResult.Results) != len(codeCellIndices) {
		return mcp.NewToolResultError("unexpected execution result shape"), nil
	}

	for i, res := range execResult.Results {
		idx := codeCellIndices[i]
		cell, _ := cells[idx].(map[string]any)
		cell["execution_count"] = i + 1
		cell["outputs"] = res.Outputs
		cells[idx] = cell
	}
	doc["cells"] = cells

	if err := writeNotebookCells(filePath, doc); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("write notebook: %v", err)), nil
	}

	targetExec := execResult.Results[len(execResult.Results)-1]
	if !targetExec.OK {
		result := mcp.NewToolResultText(fmt.Sprintf("cell %q failed during execution", cellID))
		result.IsError = true
		return result, nil
	}

	outputText := flattenNotebookOutputs(targetExec.Outputs)
	if strings.TrimSpace(outputText) == "" {
		outputText = fmt.Sprintf("executed cell %q with no output", cellID)
	}
	return mcp.NewToolResultText(outputText), nil
}

type codeCellRequest struct {
	Code   string `json:"code"`
	CellID string `json:"cell_id"`
}

type pythonExecRequest struct {
	Cells           []codeCellRequest `json:"cells"`
	ContinueOnError bool              `json:"continue_on_error"`
}

type pythonExecCellResult struct {
	OK      bool             `json:"ok"`
	Outputs []map[string]any `json:"outputs"`
}

type pythonExecResponse struct {
	Results []pythonExecCellResult `json:"results"`
}

func executeCodeCellsPython(cells []codeCellRequest, continueOnError bool) (*pythonExecResponse, error) {
	req := pythonExecRequest{Cells: cells, ContinueOnError: continueOnError}
	in, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	py := `import contextlib
import io
import json
import traceback
import sys

payload = json.loads(sys.stdin.read())
globals_ns = {}
results = []

for cell in payload.get("cells", []):
    out_stdout = io.StringIO()
    out_stderr = io.StringIO()
    ok = True
    err_trace = ""
    with contextlib.redirect_stdout(out_stdout), contextlib.redirect_stderr(out_stderr):
        try:
            exec(cell.get("code", ""), globals_ns)
        except Exception:
            ok = False
            err_trace = traceback.format_exc()

    outputs = []
    stdout_text = out_stdout.getvalue()
    stderr_text = out_stderr.getvalue()
    if stdout_text:
        outputs.append({"output_type": "stream", "name": "stdout", "text": [stdout_text]})
    if stderr_text:
        outputs.append({"output_type": "stream", "name": "stderr", "text": [stderr_text]})
    if not ok:
        outputs.append({
            "output_type": "error",
            "ename": "ExecutionError",
            "evalue": "cell execution failed",
            "traceback": err_trace.splitlines(),
        })
    results.append({"ok": ok, "outputs": outputs})
    if (not ok) and (not payload.get("continue_on_error", False)):
        break

sys.stdout.write(json.dumps({"results": results}))
`

	cmd := exec.Command("python3", "-c", py)
	cmd.Stdin = strings.NewReader(string(in))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("python execution failed: %v: %s", err, string(out))
	}
	resp := &pythonExecResponse{}
	if err := json.Unmarshal(out, resp); err != nil {
		return nil, fmt.Errorf("parse python output: %w", err)
	}
	return resp, nil
}

func loadNotebookCells(path string) (map[string]any, []any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	doc := map[string]any{}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, nil, err
	}
	rawCells, ok := doc["cells"]
	if !ok {
		return doc, []any{}, nil
	}
	cells, ok := rawCells.([]any)
	if !ok {
		return nil, nil, fmt.Errorf("cells is not an array")
	}
	return doc, cells, nil
}

func writeNotebookCells(path string, doc map[string]any) error {
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o600)
}

func notebookCellID(cell map[string]any) string {
	if id := stringValue(cell["id"]); id != "" {
		return id
	}
	if meta, ok := cell["metadata"].(map[string]any); ok {
		if id := stringValue(meta["id"]); id != "" {
			return id
		}
	}
	return ""
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func joinNotebookSource(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "")
	case []string:
		return strings.Join(v, "")
	default:
		return ""
	}
}

func flattenNotebookOutputs(outputs []map[string]any) string {
	parts := make([]string, 0)
	for _, out := range outputs {
		if text, ok := out["text"]; ok {
			parts = append(parts, joinNotebookSource(text))
		}
		if tb, ok := out["traceback"]; ok {
			parts = append(parts, joinNotebookSource(tb))
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}
