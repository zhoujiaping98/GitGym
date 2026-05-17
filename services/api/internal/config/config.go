package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
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
	apiBaseURL := defaultIfBlank(os.Getenv("API_BASE_URL"), "http://127.0.0.1:8080")

	return Config{
		MySQLDSN:            os.Getenv("MYSQL_DSN"),
		GitHubClientID:      os.Getenv("GITHUB_CLIENT_ID"),
		GitHubSecret:        os.Getenv("GITHUB_CLIENT_SECRET"),
		SessionSecret:       os.Getenv("SESSION_COOKIE_SECRET"),
		RunnerBaseURL:       os.Getenv("RUNNER_BASE_URL"),
		APIBaseURL:          apiBaseURL,
		FrontendRedirectURL: frontendRedirectURL(os.Getenv("FRONTEND_REDIRECT_URL"), apiBaseURL),
	}
}

func defaultIfBlank(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func frontendRedirectURL(explicit string, apiBaseURL string) string {
	if explicit != "" {
		return explicit
	}
	if isLocalBaseURL(apiBaseURL) {
		return "http://127.0.0.1:5173"
	}
	return ""
}

func isLocalBaseURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}

	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
