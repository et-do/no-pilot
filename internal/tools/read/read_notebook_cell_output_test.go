package read_test

import (
	"path/filepath"
	"strings"
	"testing"
)

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
