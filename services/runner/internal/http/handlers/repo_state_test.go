package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	httpx "gitgym/services/runner/internal/http"
)

func TestRepoStateReturnsSnapshotJSON(t *testing.T) {
	t.Parallel()

	workRoot := t.TempDir()
	router := httpx.NewRouter(workRoot)
	workspace := createWorkspace(t, router)

	headCommit := gitOutput(t, workspace.Path, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(workspace.Path, "notes.txt"), []byte("pending\n"), 0o644); err != nil {
		t.Fatalf("write dirty workspace file: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/internal/workspaces/"+workspace.ID+"/repo-state", http.NoBody)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		BranchName    string    `json:"branch_name"`
		HeadCommit    string    `json:"head_commit"`
		StatusSummary []string  `json:"status_summary"`
		CapturedAt    time.Time `json:"captured_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal repo state response: %v", err)
	}

	if payload.BranchName != "main" {
		t.Fatalf("expected branch name %q, got %q", "main", payload.BranchName)
	}
	if payload.HeadCommit != headCommit {
		t.Fatalf("expected head commit %q, got %q", headCommit, payload.HeadCommit)
	}
	if len(payload.StatusSummary) != 1 || payload.StatusSummary[0] != "?? notes.txt" {
		t.Fatalf("expected status summary to round-trip, got %#v", payload.StatusSummary)
	}
	if payload.CapturedAt.IsZero() {
		t.Fatal("expected captured_at to be populated")
	}
	if time.Since(payload.CapturedAt) > time.Minute || payload.CapturedAt.After(time.Now().UTC().Add(time.Second)) {
		t.Fatalf("expected captured_at to be recent UTC time, got %s", payload.CapturedAt)
	}
}

func TestRepoStateReturnsNotFoundForMissingWorkspace(t *testing.T) {
	t.Parallel()

	router := httpx.NewRouter(t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/internal/workspaces/ws-missing/repo-state", http.NoBody)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}
}

func TestRepoStateReturnsInternalServerErrorWhenCaptureFails(t *testing.T) {
	t.Parallel()

	workRoot := t.TempDir()
	router := httpx.NewRouter(workRoot)
	workspace := createWorkspace(t, router)

	if err := os.RemoveAll(filepath.Join(workspace.Path, ".git")); err != nil {
		t.Fatalf("remove git directory: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/internal/workspaces/"+workspace.ID+"/repo-state", http.NoBody)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusInternalServerError, rec.Code, rec.Body.String())
	}
}

func TestRepoStateRouteIsMountedInRunnerRouter(t *testing.T) {
	t.Parallel()

	workRoot := t.TempDir()
	router := httpx.NewRouter(workRoot)
	workspace := createWorkspace(t, router)

	req := httptest.NewRequest(http.MethodGet, "/internal/workspaces/"+workspace.ID+"/repo-state", http.NoBody)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}

	return strings.TrimSpace(string(output))
}
