package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func InitWorkspaceRepository(workspacePath string) error {
	commands := [][]string{
		{"init", "-b", "main"},
		{"config", "user.name", "GitGym Test"},
		{"config", "user.email", "test@gitgym.dev"},
		{"add", "."},
		{"commit", "-m", "Initial commit"},
	}

	for _, args := range commands {
		cmd := exec.Command("git", args...)
		cmd.Dir = workspacePath
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
		}
	}

	return nil
}
