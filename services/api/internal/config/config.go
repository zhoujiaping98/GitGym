package config

import (
	"fmt"
	"os"
)

type Config struct {
	MySQLDSN       string
	GitHubClientID string
	GitHubSecret   string
	SessionSecret  string
	RunnerBaseURL  string
	APIBaseURL     string
}

func Load() (Config, error) {
	cfg := Config{
		MySQLDSN:       os.Getenv("MYSQL_DSN"),
		GitHubClientID: os.Getenv("GITHUB_CLIENT_ID"),
		GitHubSecret:   os.Getenv("GITHUB_CLIENT_SECRET"),
		SessionSecret:  os.Getenv("SESSION_COOKIE_SECRET"),
		RunnerBaseURL:  os.Getenv("RUNNER_BASE_URL"),
		APIBaseURL:     os.Getenv("API_BASE_URL"),
	}
	if cfg.MySQLDSN == "" || cfg.GitHubClientID == "" || cfg.GitHubSecret == "" || cfg.SessionSecret == "" {
		return Config{}, fmt.Errorf("missing required environment variables")
	}
	return cfg, nil
}
