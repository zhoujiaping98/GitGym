package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"gitgym/services/runner/internal/engine"
	httpx "gitgym/services/runner/internal/http"
	"gitgym/services/runner/internal/http/handlers"
	"github.com/go-chi/chi/v5"
)

func TestDeleteWorkspaceSchedulesCleanupWithPayload(t *testing.T) {
	t.Parallel()

	workRoot := t.TempDir()
	router := chi.NewRouter()
	workspace := createWorkspace(t, httpx.NewRouter(workRoot))

	requests := make(chan engine.WorkspaceCleanupRequest, 1)
	cleanup := engine.NewWorkspaceCleanupManagerWithHooks(
		engine.NewTerminalManager(),
		func(string) error { return nil },
		func(time.Duration) <-chan time.Time { return make(chan time.Time) },
		func(req engine.WorkspaceCleanupRequest, err error) {
			if err == nil {
				requests <- req
			}
		},
	)
	router.Delete("/internal/workspaces/{workspaceID}", handlers.DeleteWorkspace(workRoot, engine.NewTerminalManager(), cleanup))

	req := httptest.NewRequest(
		http.MethodDelete,
		"/internal/workspaces/"+workspace.ID,
		strings.NewReader(`{"reason":"expired","delete_after_seconds":0}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body for immediate delete, got %q", rec.Body.String())
	}

	select {
	case cleanupReq := <-requests:
		if cleanupReq.WorkspaceID != workspace.ID {
			t.Fatalf("expected cleanup workspace ID %q, got %q", workspace.ID, cleanupReq.WorkspaceID)
		}
		if cleanupReq.Path != workspace.Path {
			t.Fatalf("expected cleanup path %q, got %q", workspace.Path, cleanupReq.Path)
		}
		if cleanupReq.Reason != "expired" {
			t.Fatalf("expected cleanup reason %q, got %q", "expired", cleanupReq.Reason)
		}
		if cleanupReq.DeleteAfter != 0 {
			t.Fatalf("expected cleanup delay %s, got %s", 0*time.Second, cleanupReq.DeleteAfter)
		}
	default:
		t.Fatal("expected cleanup request to be scheduled")
	}
}

func TestDeleteWorkspaceSchedulesDelayedCleanup(t *testing.T) {
	t.Parallel()

	workRoot := t.TempDir()
	router := chi.NewRouter()
	workspace := createWorkspace(t, httpx.NewRouter(workRoot))

	afterCalls := make(chan time.Duration, 1)
	cleanup := engine.NewWorkspaceCleanupManagerWithHooks(
		engine.NewTerminalManager(),
		func(string) error { return nil },
		func(delay time.Duration) <-chan time.Time {
			afterCalls <- delay
			return make(chan time.Time)
		},
		nil,
	)
	router.Delete("/internal/workspaces/{workspaceID}", handlers.DeleteWorkspace(workRoot, engine.NewTerminalManager(), cleanup))

	req := httptest.NewRequest(
		http.MethodDelete,
		"/internal/workspaces/"+workspace.ID,
		strings.NewReader(`{"reason":"orphaned","delete_after_seconds":30}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusAccepted, rec.Code, rec.Body.String())
	}

	select {
	case delay := <-afterCalls:
		if delay != 30*time.Second {
			t.Fatalf("expected cleanup delay %s, got %s", 30*time.Second, delay)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected delayed cleanup to start timer")
	}

	if _, err := os.Stat(workspace.Path); err != nil {
		t.Fatalf("expected workspace to remain before delayed cleanup runs, stat err=%v", err)
	}
}

func TestDeleteWorkspaceTreatsMissingWorkspaceAsIdempotent(t *testing.T) {
	t.Parallel()

	workRoot := t.TempDir()
	missingWorkRoot := t.TempDir()
	router := chi.NewRouter()
	terminalManager := engine.NewTerminalManager()
	cleanup := engine.NewWorkspaceCleanupManager(terminalManager)
	workspace := createWorkspace(t, httpx.NewRouter(workRoot))
	session, err := terminalManager.Acquire(context.Background(), workspace.Path, workspace.ID)
	if err != nil {
		t.Fatalf("acquire terminal session: %v", err)
	}
	router.Delete("/internal/workspaces/{workspaceID}", handlers.DeleteWorkspace(missingWorkRoot, terminalManager, cleanup))

	req := httptest.NewRequest(
		http.MethodDelete,
		"/internal/workspaces/"+workspace.ID,
		strings.NewReader(`{"reason":"expired","delete_after_seconds":0}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal missing delete payload: %v", err)
	}
	if payload.Error != "workspace not found" {
		t.Fatalf("expected error %q, got %q", "workspace not found", payload.Error)
	}

	if err := session.WriteInput("pwd\n"); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("expected terminal session to be released on missing delete, got %v", err)
	}
}

func TestDeleteWorkspaceRejectsInvalidJSONBody(t *testing.T) {
	t.Parallel()

	workRoot := t.TempDir()
	router := chi.NewRouter()
	workspace := createWorkspace(t, httpx.NewRouter(workRoot))
	router.Delete(
		"/internal/workspaces/{workspaceID}",
		handlers.DeleteWorkspace(workRoot, engine.NewTerminalManager(), engine.NewWorkspaceCleanupManager(engine.NewTerminalManager())),
	)

	req := httptest.NewRequest(
		http.MethodDelete,
		"/internal/workspaces/"+workspace.ID,
		strings.NewReader(`{"reason":`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestDeleteWorkspaceRouteIsMountedInRunnerRouter(t *testing.T) {
	t.Parallel()

	workRoot := t.TempDir()
	router := httpx.NewRouter(workRoot)
	workspace := createWorkspace(t, router)

	req := httptest.NewRequest(http.MethodDelete, "/internal/workspaces/"+workspace.ID, http.NoBody)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(workspace.Path); !os.IsNotExist(err) {
		t.Fatalf("expected workspace to be deleted, stat err=%v", err)
	}
}
