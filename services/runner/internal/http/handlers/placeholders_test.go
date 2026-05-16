package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunCommandReturnsNotImplemented(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/internal/workspaces/ws-1/commands", nil)
	rec := httptest.NewRecorder()

	RunCommand().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, rec.Code)
	}
}

func TestResetWorkspaceReturnsNotImplemented(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/internal/workspaces/ws-1/reset", nil)
	rec := httptest.NewRecorder()

	ResetWorkspace().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, rec.Code)
	}
}
