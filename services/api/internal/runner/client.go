package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

var ErrClientNotConfigured = errors.New("runner client is not configured")

type Workspace struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Template string `json:"template"`
}

type Client interface {
	CreateWorkspace(ctx context.Context, template string) (Workspace, error)
	ResetWorkspace(ctx context.Context, workspaceID string) error
}

type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, httpClient *http.Client) *HTTPClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &HTTPClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

func (c *HTTPClient) CreateWorkspace(ctx context.Context, template string) (Workspace, error) {
	if c.baseURL == "" {
		return Workspace{}, ErrClientNotConfigured
	}

	payload := struct {
		Template string `json:"template"`
	}{
		Template: template,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Workspace{}, fmt.Errorf("marshal workspace request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/workspaces", bytes.NewReader(body))
	if err != nil {
		return Workspace{}, fmt.Errorf("build workspace request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Workspace{}, fmt.Errorf("create runner workspace: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return Workspace{}, fmt.Errorf("runner create workspace returned status %d", resp.StatusCode)
	}

	var workspace Workspace
	if err := json.NewDecoder(resp.Body).Decode(&workspace); err != nil {
		return Workspace{}, fmt.Errorf("decode workspace response: %w", err)
	}

	return workspace, nil
}

func (c *HTTPClient) ResetWorkspace(ctx context.Context, workspaceID string) error {
	if c.baseURL == "" {
		return ErrClientNotConfigured
	}
	if strings.TrimSpace(workspaceID) == "" {
		return fmt.Errorf("workspace ID is required")
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/internal/workspaces/"+url.PathEscape(workspaceID)+"/reset",
		nil,
	)
	if err != nil {
		return fmt.Errorf("build reset workspace request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("reset runner workspace: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("runner reset workspace returned status %d", resp.StatusCode)
	}

	return nil
}
