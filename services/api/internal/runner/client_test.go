package runner

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPClientGetRepoStateDecodesSuccessResponse(t *testing.T) {
	t.Parallel()

	capturedAt := time.Date(2026, 5, 23, 10, 11, 12, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected method %s, got %s", http.MethodGet, r.Method)
		}
		if r.URL.Path != "/internal/workspaces/ws-123/repo-state" {
			t.Fatalf("expected path %q, got %q", "/internal/workspaces/ws-123/repo-state", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"branch_name":"main","head_commit":"abc123","status_summary":["M README.md"],"captured_at":"` + capturedAt.Format(time.RFC3339) + `"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, http.DefaultClient)

	repoState, err := client.GetRepoState(context.Background(), "ws-123")

	if err != nil {
		t.Fatalf("get repo state: %v", err)
	}
	if repoState.BranchName != "main" {
		t.Fatalf("expected branch name %q, got %q", "main", repoState.BranchName)
	}
	if repoState.HeadCommit != "abc123" {
		t.Fatalf("expected head commit %q, got %q", "abc123", repoState.HeadCommit)
	}
	if len(repoState.StatusSummary) != 1 || repoState.StatusSummary[0] != "M README.md" {
		t.Fatalf("expected status summary to decode, got %#v", repoState.StatusSummary)
	}
	if !repoState.CapturedAt.Equal(capturedAt) {
		t.Fatalf("expected captured at %s, got %s", capturedAt, repoState.CapturedAt)
	}
}

func TestHTTPClientGetRepoStateMapsNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, http.DefaultClient)

	_, err := client.GetRepoState(context.Background(), "ws-missing")

	if !errors.Is(err, ErrWorkspaceNotFound) {
		t.Fatalf("expected workspace not found error, got %v", err)
	}
}

func TestHTTPClientGetRepoStateReturnsStatusErrorForUnexpectedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewClient(server.URL, http.DefaultClient)

	_, err := client.GetRepoState(context.Background(), "ws-error")

	if err == nil {
		t.Fatal("expected non-200 response to return error")
	}
	if !strings.Contains(err.Error(), "runner get repo state returned status 502") {
		t.Fatalf("expected descriptive status error, got %v", err)
	}
}
