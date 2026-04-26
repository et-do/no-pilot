package edit

import (
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/et-do/no-pilot/internal/config"
)

func pathDenied(cfg config.Provider, toolName, path string) (bool, string) {
	patterns := cfg.Policy(toolName).DenyPaths
	if len(patterns) == 0 {
		return false, ""
	}
	cleanPath := filepath.Clean(path)
	for _, p := range patterns {
		ok, err := doublestar.Match(p, cleanPath)
		if err != nil {
			continue
		}
		if ok {
			return true, p
		}
	}
	return false, ""
}

func looksTextPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".js", ".ts", ".tsx", ".jsx", ".py", ".java", ".rb", ".rs", ".c", ".h", ".cpp", ".hpp", ".json", ".yaml", ".yml", ".md", ".txt", ".sh", ".sql", ".ipynb":
		return true
	default:
		return ext == ""
	}
}
