package config

import (
	"fmt"
	"os"
)

type Config struct {
	MySQLDSN            string
	GitHubClientID      string
	GitHubSecret        string
	SessionSecret       string
	RunnerBaseURL       string
	APIBaseURL          string
	FrontendRedirectURL string
}

func Load() (Config, error) {
	cfg := LoadRuntime()
	if cfg.MySQLDSN == "" || cfg.GitHubClientID == "" || cfg.GitHubSecret == "" || cfg.SessionSecret == "" {
		return Config{}, fmt.Errorf("missing required environment variables")
	}
	return cfg, nil
}

func LoadRuntime() Config {
	return Config{
		MySQLDSN:            os.Getenv("MYSQL_DSN"),
		GitHubClientID:      os.Getenv("GITHUB_CLIENT_ID"),
		GitHubSecret:        os.Getenv("GITHUB_CLIENT_SECRET"),
		SessionSecret:       os.Getenv("SESSION_COOKIE_SECRET"),
		RunnerBaseURL:       os.Getenv("RUNNER_BASE_URL"),
		APIBaseURL:          defaultIfBlank(os.Getenv("API_BASE_URL"), "http://127.0.0.1:8080"),
		FrontendRedirectURL: defaultIfBlank(os.Getenv("FRONTEND_REDIRECT_URL"), "http://127.0.0.1:5173"),
	}
}

func defaultIfBlank(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
