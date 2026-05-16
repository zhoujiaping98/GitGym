package test

import (
	"context"
	"errors"
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

func TestPracticeServiceClassifiesCreateSessionErrors(t *testing.T) {
	t.Parallel()

	t.Run("rejects missing input", func(t *testing.T) {
		svc := service.NewPracticeService(&stubPracticeSessionStore{}, &stubRunnerClient{}, time.Now)

		_, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{})

		if !errors.Is(err, service.ErrInvalidPracticeSessionInput) {
			t.Fatalf("expected invalid input error, got %v", err)
		}
	})

	t.Run("rejects unknown template", func(t *testing.T) {
		svc := service.NewPracticeService(&stubPracticeSessionStore{}, &stubRunnerClient{}, time.Now)

		_, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
			UserID:     42,
			ScenarioID: 7,
			TemplateID: 999,
		})

		if !errors.Is(err, service.ErrUnknownPracticeTemplate) {
			t.Fatalf("expected unknown template error, got %v", err)
		}
	})

	t.Run("reports missing configuration", func(t *testing.T) {
		svc := service.NewPracticeService(nil, &stubRunnerClient{}, time.Now)

		_, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
			UserID:     42,
			ScenarioID: 7,
			TemplateID: 1,
		})

		if !errors.Is(err, service.ErrPracticeServiceConfiguration) {
			t.Fatalf("expected service configuration error, got %v", err)
		}
	})

	t.Run("reports runner client configuration errors as service configuration", func(t *testing.T) {
		svc := service.NewPracticeService(&stubPracticeSessionStore{}, runner.NewClient("", nil), time.Now)

		_, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
			UserID:     42,
			ScenarioID: 7,
			TemplateID: 1,
		})

		if !errors.Is(err, service.ErrPracticeServiceConfiguration) {
			t.Fatalf("expected service configuration error, got %v", err)
		}
	})

	t.Run("wraps runner creation failure", func(t *testing.T) {
		svc := service.NewPracticeService(&stubPracticeSessionStore{}, &stubRunnerClient{
			err: errors.New("runner unavailable"),
		}, time.Now)

		_, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
			UserID:     42,
			ScenarioID: 7,
			TemplateID: 1,
		})

		if !errors.Is(err, service.ErrRunnerWorkspaceCreation) {
			t.Fatalf("expected runner creation error, got %v", err)
		}
	})
}

func TestPracticeServiceReturnsCurrentSessionForUser(t *testing.T) {
	t.Parallel()

	store := service.NewInMemoryPracticeSessionStore()
	svc := service.NewPracticeService(store, &stubRunnerClient{
		workspace: runner.Workspace{
			ID:       "ws-current",
			Path:     "/tmp/ws-current",
			Template: "standard",
		},
	}, time.Now)

	created, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     42,
		ScenarioID: 7,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	current, err := svc.CurrentPracticeSession(context.Background(), 42)
	if err != nil {
		t.Fatalf("current practice session: %v", err)
	}
	if current.ID != created.ID {
		t.Fatalf("expected current session ID %d, got %d", created.ID, current.ID)
	}
}

func TestPracticeServiceFindsSessionByIDForOwningUser(t *testing.T) {
	t.Parallel()

	store := service.NewInMemoryPracticeSessionStore()
	svc := service.NewPracticeService(store, &stubRunnerClient{
		workspace: runner.Workspace{
			ID:       "ws-terminal",
			Path:     "/tmp/ws-terminal",
			Template: "standard",
		},
	}, time.Now)

	created, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     42,
		ScenarioID: 7,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	session, err := svc.PracticeSessionByID(context.Background(), 42, created.ID)
	if err != nil {
		t.Fatalf("practice session by ID: %v", err)
	}
	if session.ID != created.ID {
		t.Fatalf("expected session ID %d, got %d", created.ID, session.ID)
	}

	if _, err := svc.PracticeSessionByID(context.Background(), 99, created.ID); !errors.Is(err, service.ErrPracticeSessionNotFound) {
		t.Fatalf("expected not found for non-owning user, got %v", err)
	}
}

func TestPracticeServiceResetsOwnedSessionWorkspace(t *testing.T) {
	t.Parallel()

	store := service.NewInMemoryPracticeSessionStore()
	runnerClient := &stubRunnerClient{
		workspace: runner.Workspace{
			ID:       "ws-reset",
			Path:     "/tmp/ws-reset",
			Template: "standard",
		},
	}
	svc := service.NewPracticeService(store, runnerClient, time.Now)

	created, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     42,
		ScenarioID: 7,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	if err := svc.ResetPracticeSession(context.Background(), 42, created.ID); err != nil {
		t.Fatalf("reset practice session: %v", err)
	}
	if runnerClient.resetWorkspaceCalls != 1 {
		t.Fatalf("expected runner reset workspace to be called once, got %d", runnerClient.resetWorkspaceCalls)
	}
	if runnerClient.lastResetWorkspaceID != "ws-reset" {
		t.Fatalf("expected reset workspace ID %q, got %q", "ws-reset", runnerClient.lastResetWorkspaceID)
	}
}

func TestPracticeServiceClassifiesResetSessionErrors(t *testing.T) {
	t.Parallel()

	t.Run("rejects missing input", func(t *testing.T) {
		svc := service.NewPracticeService(service.NewInMemoryPracticeSessionStore(), &stubRunnerClient{}, time.Now)

		if err := svc.ResetPracticeSession(context.Background(), 0, 0); !errors.Is(err, service.ErrInvalidPracticeSessionInput) {
			t.Fatalf("expected invalid input error, got %v", err)
		}
	})

	t.Run("reports missing configuration", func(t *testing.T) {
		svc := service.NewPracticeService(service.NewInMemoryPracticeSessionStore(), nil, time.Now)

		if err := svc.ResetPracticeSession(context.Background(), 42, 99); !errors.Is(err, service.ErrPracticeServiceConfiguration) {
			t.Fatalf("expected service configuration error, got %v", err)
		}
	})

	t.Run("reports runner reset errors", func(t *testing.T) {
		store := service.NewInMemoryPracticeSessionStore()
		svc := service.NewPracticeService(store, &stubRunnerClient{
			workspace: runner.Workspace{
				ID:       "ws-reset-error",
				Path:     "/tmp/ws-reset-error",
				Template: "standard",
			},
			resetErr: errors.New("runner unavailable"),
		}, time.Now)

		created, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
			UserID:     42,
			ScenarioID: 7,
			TemplateID: 1,
		})
		if err != nil {
			t.Fatalf("create practice session: %v", err)
		}

		if err := svc.ResetPracticeSession(context.Background(), 42, created.ID); !errors.Is(err, service.ErrRunnerWorkspaceReset) {
			t.Fatalf("expected runner reset error, got %v", err)
		}
	})
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

func (s *stubPracticeSessionStore) CurrentPracticeSession(_ context.Context, userID uint64) (domain.PracticeSession, error) {
	if s.savedSession.ID == 0 || s.savedSession.UserID != userID {
		return domain.PracticeSession{}, service.ErrPracticeSessionNotFound
	}
	return s.savedSession, nil
}

func (s *stubPracticeSessionStore) PracticeSessionByID(_ context.Context, sessionID uint64) (domain.PracticeSession, error) {
	if s.savedSession.ID == sessionID {
		return s.savedSession, nil
	}
	return domain.PracticeSession{}, service.ErrPracticeSessionNotFound
}

type stubRunnerClient struct {
	createWorkspaceCalls int
	lastTemplate         string
	resetWorkspaceCalls  int
	lastResetWorkspaceID string
	workspace            runner.Workspace
	err                  error
	resetErr             error
}

func (s *stubRunnerClient) CreateWorkspace(_ context.Context, template string) (runner.Workspace, error) {
	s.createWorkspaceCalls++
	s.lastTemplate = template
	if s.err != nil {
		return runner.Workspace{}, s.err
	}
	return s.workspace, nil
}

func (s *stubRunnerClient) ResetWorkspace(_ context.Context, workspaceID string) error {
	s.resetWorkspaceCalls++
	s.lastResetWorkspaceID = workspaceID
	if s.resetErr != nil {
		return s.resetErr
	}
	return nil
}
