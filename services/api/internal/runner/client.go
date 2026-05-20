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
	"time"

	"github.com/coder/websocket"
)

var ErrClientNotConfigured = errors.New("runner client is not configured")
var ErrWorkspaceNotFound = errors.New("runner workspace not found")

type Workspace struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Template string `json:"template"`
}

type TerminalConnection interface {
	Read(ctx context.Context) (int, []byte, error)
	Write(ctx context.Context, messageType int, payload []byte) error
	Close(status websocket.StatusCode, reason string) error
}

type Client interface {
	CreateWorkspace(ctx context.Context, template string) (Workspace, error)
	ResetWorkspace(ctx context.Context, workspaceID string) error
	ConnectTerminal(ctx context.Context, workspaceID string) (TerminalConnection, error)
	DeleteWorkspace(ctx context.Context, workspaceID string, reason string, deleteAfter time.Duration) error
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
		if resp.StatusCode == http.StatusNotFound {
			return ErrWorkspaceNotFound
		}
		return fmt.Errorf("runner reset workspace returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *HTTPClient) ConnectTerminal(ctx context.Context, workspaceID string) (TerminalConnection, error) {
	terminalURL, err := c.terminalURL(workspaceID)
	if err != nil {
		return nil, err
	}

	conn, resp, err := websocket.Dial(ctx, terminalURL, nil)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, ErrWorkspaceNotFound
		}
		return nil, fmt.Errorf("connect runner terminal: %w", err)
	}

	return websocketTerminalConnection{conn: conn}, nil
}

func (c *HTTPClient) DeleteWorkspace(ctx context.Context, workspaceID string, reason string, deleteAfter time.Duration) error {
	if c.baseURL == "" {
		return ErrClientNotConfigured
	}
	if strings.TrimSpace(workspaceID) == "" {
		return fmt.Errorf("workspace ID is required")
	}

	payload := struct {
		Reason             string `json:"reason"`
		DeleteAfterSeconds int    `json:"delete_after_seconds"`
	}{
		Reason:             reason,
		DeleteAfterSeconds: int(deleteAfter / time.Second),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal delete workspace request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodDelete,
		c.baseURL+"/internal/workspaces/"+url.PathEscape(workspaceID),
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("build delete workspace request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete runner workspace: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrWorkspaceNotFound
	}
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("runner delete workspace returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *HTTPClient) terminalURL(workspaceID string) (string, error) {
	if c.baseURL == "" {
		return "", ErrClientNotConfigured
	}
	if strings.TrimSpace(workspaceID) == "" {
		return "", fmt.Errorf("workspace ID is required")
	}

	baseURL, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse runner base URL: %w", err)
	}

	switch baseURL.Scheme {
	case "http":
		baseURL.Scheme = "ws"
	case "https":
		baseURL.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported runner base URL scheme %q", baseURL.Scheme)
	}

	baseURL.Path = strings.TrimRight(baseURL.Path, "/") + "/internal/workspaces/" + url.PathEscape(workspaceID) + "/terminal"
	baseURL.RawQuery = ""
	baseURL.Fragment = ""

	return baseURL.String(), nil
}

type websocketTerminalConnection struct {
	conn *websocket.Conn
}

func (c websocketTerminalConnection) Read(ctx context.Context) (int, []byte, error) {
	messageType, payload, err := c.conn.Read(ctx)
	return int(messageType), payload, err
}

func (c websocketTerminalConnection) Write(ctx context.Context, messageType int, payload []byte) error {
	return c.conn.Write(ctx, websocket.MessageType(messageType), payload)
}

func (c websocketTerminalConnection) Close(status websocket.StatusCode, reason string) error {
	return c.conn.Close(status, reason)
}
