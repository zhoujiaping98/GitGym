package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	WorkRoot string
}

func Load() Config {
	workRoot := os.Getenv("RUNNER_WORK_ROOT")
	if workRoot == "" {
		workRoot = "./var/workspaces"
	}

	if absWorkRoot, err := filepath.Abs(workRoot); err == nil {
		workRoot = absWorkRoot
	}

	return Config{WorkRoot: workRoot}
}
