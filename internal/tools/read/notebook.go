package read

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type notebookFile struct {
	Cells []notebookCell `json:"cells"`
}

type notebookCell struct {
	CellType       string           `json:"cell_type"`
	ID             string           `json:"id"`
	Source         json.RawMessage  `json:"source"`
	ExecutionCount *int             `json:"execution_count"`
	Outputs        []notebookOutput `json:"outputs"`
}

type notebookOutput struct {
	Name       string                     `json:"name"`
	Text       json.RawMessage            `json:"text"`
	Data       map[string]json.RawMessage `json:"data"`
	OutputType string                     `json:"output_type"`
}

func loadNotebook(path string) (*notebookFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read notebook %s: %w", path, err)
	}

	var nb notebookFile
	if err := json.Unmarshal(data, &nb); err != nil {
		return nil, fmt.Errorf("parse notebook %s: %w", path, err)
	}
	return &nb, nil
}

func decodeStringOrStringSlice(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return single
	}

	var parts []string
	if err := json.Unmarshal(raw, &parts); err == nil {
		return strings.Join(parts, "")
	}
	return ""
}

func formatNotebookSummary(nb *notebookFile) string {
	var lines []string
	for idx, cell := range nb.Cells {
		line := fmt.Sprintf("id=%s type=%s index=%d", notebookCellID(cell, idx), cell.CellType, idx)
		if cell.ExecutionCount != nil {
			line += fmt.Sprintf(" execution_count=%d", *cell.ExecutionCount)
		}
		if cell.CellType == "code" && len(cell.Outputs) > 0 {
			line += fmt.Sprintf(" outputs=%d", len(cell.Outputs))
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func notebookCellID(cell notebookCell, idx int) string {
	if cell.ID != "" {
		return cell.ID
	}
	return fmt.Sprintf("cell-%d", idx)
}

func findNotebookCell(nb *notebookFile, id string) (notebookCell, bool) {
	for idx, cell := range nb.Cells {
		if notebookCellID(cell, idx) == id {
			return cell, true
		}
	}
	return notebookCell{}, false
}

func formatNotebookOutputs(cell notebookCell) string {
	var parts []string
	for _, out := range cell.Outputs {
		if text := decodeStringOrStringSlice(out.Text); text != "" {
			parts = append(parts, text)
			continue
		}
		for _, key := range []string{"text/plain", "text/markdown", "text/html", "application/json"} {
			if raw, ok := out.Data[key]; ok {
				text := decodeStringOrStringSlice(raw)
				if text != "" {
					parts = append(parts, text)
					break
				}
			}
		}
	}
	return strings.Join(parts, "\n")
}
