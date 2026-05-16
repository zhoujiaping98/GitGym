package engine

import (
	"os"
	"os/exec"
)

func InitStandardTemplate(workspacePath string) error {
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = workspacePath
	if err := cmd.Run(); err != nil {
		return err
	}
	return os.WriteFile(workspacePath+"/README.md", []byte("# Standard Template\n"), 0o644)
}
