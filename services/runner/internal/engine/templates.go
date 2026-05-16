package engine

import (
	"os"
	"path/filepath"
)

func InitStandardTemplate(workspacePath string) error {
	files := map[string]string{
		"README.md":  "# Standard Template\n",
		".gitignore": ".git/\n",
	}

	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(workspacePath, name), []byte(contents), 0o644); err != nil {
			return err
		}
	}

	return nil
}
