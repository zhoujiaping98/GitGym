package test

import (
	"context"
	"testing"
	"time"

	"gitgym/services/api/internal/domain"
	"gitgym/services/api/internal/runner"
	"gitgym/services/api/internal/service"
)

func TestPracticeServiceCreatesSessionFromRunnerWorkspace(t *testing.T) {
	t.Parallel()

	store := &stubPracticeSessionStore{}
	runner := &stubRunnerClient{
		workspace: runner.Workspace{
			ID:       "ws-123",
			Path:     "/tmp/ws-123",
			Template: "standard",
		},
	}
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	svc := service.NewPracticeService(store, runner, func() time.Time { return now })

	session, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     42,
		ScenarioID: 7,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	if runner.createWorkspaceCalls != 1 {
		t.Fatalf("expected runner create workspace to be called once, got %d", runner.createWorkspaceCalls)
	}
	if runner.lastTemplate != "standard" {
		t.Fatalf("expected runner template %q, got %q", "standard", runner.lastTemplate)
	}
	if store.createCalls != 1 {
		t.Fatalf("expected store create session to be called once, got %d", store.createCalls)
	}
	if store.lastSession.UserID != 42 {
		t.Fatalf("expected user ID 42, got %d", store.lastSession.UserID)
	}
	if store.lastSession.ScenarioID != 7 {
		t.Fatalf("expected scenario ID 7, got %d", store.lastSession.ScenarioID)
	}
	if store.lastSession.TemplateID != 1 {
		t.Fatalf("expected template ID 1, got %d", store.lastSession.TemplateID)
	}
	if store.lastSession.RunnerRef != "ws-123" {
		t.Fatalf("expected runner ref %q, got %q", "ws-123", store.lastSession.RunnerRef)
	}
	if store.lastSession.WorkspacePathRef != "/tmp/ws-123" {
		t.Fatalf("expected workspace path %q, got %q", "/tmp/ws-123", store.lastSession.WorkspacePathRef)
	}
	if store.lastSession.Status != "active" {
		t.Fatalf("expected status %q, got %q", "active", store.lastSession.Status)
	}
	if !store.lastSession.StartedAt.Equal(now) {
		t.Fatalf("expected started at %v, got %v", now, store.lastSession.StartedAt)
	}
	expectedExpiry := now.Add(2 * time.Hour)
	if !store.lastSession.ExpiresAt.Equal(expectedExpiry) {
		t.Fatalf("expected expires at %v, got %v", expectedExpiry, store.lastSession.ExpiresAt)
	}
	if !store.lastSession.LastActivityAt.Equal(now) {
		t.Fatalf("expected last activity at %v, got %v", now, store.lastSession.LastActivityAt)
	}
	if session != store.savedSession {
		t.Fatalf("expected returned session to match stored session")
	}
}

type stubPracticeSessionStore struct {
	createCalls  int
	lastSession  domain.PracticeSession
	savedSession domain.PracticeSession
}

func (s *stubPracticeSessionStore) CreatePracticeSession(_ context.Context, session domain.PracticeSession) (domain.PracticeSession, error) {
	s.createCalls++
	s.lastSession = session
	session.ID = 101
	s.savedSession = session
	return session, nil
}

type stubRunnerClient struct {
	createWorkspaceCalls int
	lastTemplate         string
	workspace            runner.Workspace
}

func (s *stubRunnerClient) CreateWorkspace(_ context.Context, template string) (runner.Workspace, error) {
	s.createWorkspaceCalls++
	s.lastTemplate = template
	return s.workspace, nil
}
