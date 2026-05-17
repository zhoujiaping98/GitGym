package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	httpx "gitgym/services/runner/internal/http"
)

func TestRunCommandReturnsCommandResultJSON(t *testing.T) {
	router := httpx.NewRouter(t.TempDir())
	workspace := createWorkspace(t, router)

	req := httptest.NewRequest(
		http.MethodPost,
		"/internal/workspaces/"+workspace.ID+"/commands",
		strings.NewReader(`{"command":"git status --short"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		WorkspaceID string `json:"workspace_id"`
		Status      string `json:"status"`
		Command     string `json:"command"`
		Stdout      string `json:"stdout"`
		Stderr      string `json:"stderr"`
		ExitCode    int    `json:"exit_code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal run command payload: %v", err)
	}
	if payload.WorkspaceID != workspace.ID {
		t.Fatalf("expected workspace ID %q, got %q", workspace.ID, payload.WorkspaceID)
	}
	if payload.Status != "completed" {
		t.Fatalf("expected status %q, got %q", "completed", payload.Status)
	}
	if payload.Command != "git status --short" {
		t.Fatalf("expected command %q, got %q", "git status --short", payload.Command)
	}
	if payload.Stdout != "" || payload.Stderr != "" {
		t.Fatalf("expected empty command output for clean workspace, got stdout=%q stderr=%q", payload.Stdout, payload.Stderr)
	}
	if payload.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", payload.ExitCode)
	}
}

func TestResetWorkspaceReturnsResettingStatusAndRehydratesWorkspace(t *testing.T) {
	root := t.TempDir()
	router := httpx.NewRouter(root)
	workspace := createWorkspace(t, router)

	notesPath := filepath.Join(workspace.Path, "notes.txt")
	if err := os.WriteFile(notesPath, []byte("pending\n"), 0o644); err != nil {
		t.Fatalf("write dirty workspace file: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/workspaces/"+workspace.ID+"/reset", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusAccepted, rec.Code, rec.Body.String())
	}

	var payload struct {
		WorkspaceID string `json:"workspace_id"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal reset payload: %v", err)
	}
	if payload.WorkspaceID != workspace.ID {
		t.Fatalf("expected workspace ID %q, got %q", workspace.ID, payload.WorkspaceID)
	}
	if payload.Status != "resetting" {
		t.Fatalf("expected status %q, got %q", "resetting", payload.Status)
	}

	if _, err := os.Stat(notesPath); !os.IsNotExist(err) {
		t.Fatalf("expected dirty file to be removed during reset, stat err=%v", err)
	}

	readme, err := os.ReadFile(filepath.Join(workspace.Path, "README.md"))
	if err != nil {
		t.Fatalf("read reset README: %v", err)
	}
	if string(readme) != "# Standard Template\n" {
		t.Fatalf("unexpected README after reset: %q", string(readme))
	}

	gitDir := filepath.Join(workspace.Path, ".git")
	if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
		t.Fatalf("expected reset workspace git directory to exist, err=%v", err)
	}
}

func TestWorkspaceEndpointsRejectMalformedWorkspaceIDs(t *testing.T) {
	router := httpx.NewRouter(t.TempDir())

	for _, tc := range []struct {
		name   string
		method string
		target string
		body   string
	}{
		{
			name:   "commands rejects encoded current directory",
			method: http.MethodPost,
			target: "/internal/workspaces/%2e/commands",
			body:   `{"command":"git status --short"}`,
		},
		{
			name:   "commands rejects encoded dot space alias",
			method: http.MethodPost,
			target: "/internal/workspaces/.%20/commands",
			body:   `{"command":"git status --short"}`,
		},
		{
			name:   "reset rejects encoded parent directory",
			method: http.MethodPost,
			target: "/internal/workspaces/%2e%2e/reset",
		},
		{
			name:   "reset rejects trailing dot alias",
			method: http.MethodPost,
			target: "/internal/workspaces/ws-123./reset",
		},
		{
			name:   "commands rejects encoded trailing space",
			method: http.MethodPost,
			target: "/internal/workspaces/ws-123%20/commands",
			body:   `{"command":"git status --short"}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.target, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d with body %s", http.StatusBadRequest, rec.Code, rec.Body.String())
			}

			var payload map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("unmarshal error payload: %v", err)
			}
			if payload["error"] == "" {
				t.Fatalf("expected error message for malformed workspace ID, got %v", payload)
			}
		})
	}
}

type createdWorkspace struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

func createWorkspace(t *testing.T, router http.Handler) createdWorkspace {
	t.Helper()

	req := httptest.NewRequest(
		http.MethodPost,
		"/internal/workspaces",
		strings.NewReader(`{"template":"standard"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected workspace create status %d, got %d with body %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var workspace createdWorkspace
	if err := json.Unmarshal(rec.Body.Bytes(), &workspace); err != nil {
		t.Fatalf("unmarshal workspace payload: %v", err)
	}
	if workspace.ID == "" || workspace.Path == "" {
		t.Fatalf("expected populated workspace payload, got %+v", workspace)
	}
	return workspace
}
