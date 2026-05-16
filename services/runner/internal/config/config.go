package config

import "os"

type Config struct {
	WorkRoot string
}

func Load() Config {
	workRoot := os.Getenv("RUNNER_WORK_ROOT")
	if workRoot == "" {
		workRoot = "./var/workspaces"
	}
	return Config{WorkRoot: workRoot}
}
