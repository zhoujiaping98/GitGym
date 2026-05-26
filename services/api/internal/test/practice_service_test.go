package test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitgym/services/api/internal/domain"
	"gitgym/services/api/internal/runner"
	"gitgym/services/api/internal/service"
	"github.com/coder/websocket"
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
		ScenarioID: 1,
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
	if store.lastSession.ScenarioID != 1 {
		t.Fatalf("expected scenario ID 1, got %d", store.lastSession.ScenarioID)
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

func TestPracticeServiceListsFallbackCatalog(t *testing.T) {
	t.Parallel()

	svc := service.NewPracticeService(
		service.NewInMemoryPracticeSessionStore(),
		&stubRunnerClient{},
		service.NewFallbackPracticeCatalog(),
		time.Now,
	)

	templates := svc.ListTemplates(context.Background())
	scenarios := svc.ListScenarios(context.Background())

	if len(templates) != 1 {
		t.Fatalf("expected one fallback template, got %d", len(templates))
	}
	if templates[0].Key != "standard" {
		t.Fatalf("expected fallback template key %q, got %q", "standard", templates[0].Key)
	}
	if len(scenarios) != 1 {
		t.Fatalf("expected one fallback scenario, got %d", len(scenarios))
	}
	if scenarios[0].Key != "sandbox-standard" {
		t.Fatalf("expected fallback scenario key %q, got %q", "sandbox-standard", scenarios[0].Key)
	}
	if scenarios[0].TemplateID != templates[0].ID {
		t.Fatalf("expected fallback scenario to reference template %d, got %d", templates[0].ID, scenarios[0].TemplateID)
	}
}

func TestPracticeServiceResolvesTemplateFromScenarioCatalog(t *testing.T) {
	t.Parallel()

	store := &stubPracticeSessionStore{}
	runnerClient := &stubRunnerClient{
		workspace: runner.Workspace{
			ID:       "ws-catalog-create",
			Path:     "/tmp/ws-catalog-create",
			Template: "standard",
		},
	}
	catalog := service.NewStaticPracticeCatalog(
		[]service.PracticeTemplate{
			{ID: 7, Key: "standard", Name: "Standard"},
		},
		[]service.PracticeScenario{
			{ID: 11, Key: "sandbox-standard", Name: "Standard Sandbox", TemplateID: 7},
		},
	)
	svc := service.NewPracticeService(store, runnerClient, catalog, time.Now)

	session, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     42,
		ScenarioID: 11,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	if runnerClient.lastTemplate != "standard" {
		t.Fatalf("expected runner template %q, got %q", "standard", runnerClient.lastTemplate)
	}
	if session.TemplateID != 7 {
		t.Fatalf("expected resolved template ID %d, got %d", 7, session.TemplateID)
	}
	if session.ScenarioID != 11 {
		t.Fatalf("expected scenario ID %d, got %d", 11, session.ScenarioID)
	}
}

func TestPracticeServiceAcceptsMatchingCompatibilityTemplateID(t *testing.T) {
	t.Parallel()

	store := &stubPracticeSessionStore{}
	runnerClient := &stubRunnerClient{
		workspace: runner.Workspace{
			ID:       "ws-compat-match",
			Path:     "/tmp/ws-compat-match",
			Template: "standard",
		},
	}
	catalog := service.NewStaticPracticeCatalog(
		[]service.PracticeTemplate{
			{ID: 7, Key: "standard", Name: "Standard"},
		},
		[]service.PracticeScenario{
			{ID: 11, Key: "sandbox-standard", Name: "Standard Sandbox", TemplateID: 7},
		},
	)
	svc := service.NewPracticeService(store, runnerClient, catalog, time.Now)

	session, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     42,
		ScenarioID: 11,
		TemplateID: 7,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}
	if session.TemplateID != 7 {
		t.Fatalf("expected template ID %d, got %d", 7, session.TemplateID)
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

	t.Run("reports misconfigured scenario template references", func(t *testing.T) {
		svc := service.NewPracticeService(
			&stubPracticeSessionStore{},
			&stubRunnerClient{},
			service.NewStaticPracticeCatalog(
				[]service.PracticeTemplate{
					{ID: 1, Key: "standard", Name: "Standard"},
				},
				[]service.PracticeScenario{
					{ID: 9, Key: "broken-scenario", Name: "Broken Scenario", TemplateID: 999},
				},
			),
			time.Now,
		)

		_, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
			UserID:     42,
			ScenarioID: 9,
		})

		if !errors.Is(err, service.ErrPracticeServiceConfiguration) {
			t.Fatalf("expected service configuration error, got %v", err)
		}
	})

	t.Run("rejects mismatched compatibility template", func(t *testing.T) {
		svc := service.NewPracticeService(
			&stubPracticeSessionStore{},
			&stubRunnerClient{},
			service.NewStaticPracticeCatalog(
				[]service.PracticeTemplate{
					{ID: 1, Key: "standard", Name: "Standard"},
				},
				[]service.PracticeScenario{
					{ID: 9, Key: "sandbox-standard", Name: "Standard Sandbox", TemplateID: 1},
				},
			),
			time.Now,
		)

		_, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
			UserID:     42,
			ScenarioID: 9,
			TemplateID: 999,
		})

		if !errors.Is(err, service.ErrUnknownPracticeTemplate) {
			t.Fatalf("expected unknown template error, got %v", err)
		}
	})

	t.Run("rejects unknown scenario", func(t *testing.T) {
		svc := service.NewPracticeService(&stubPracticeSessionStore{}, &stubRunnerClient{}, time.Now)

		_, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
			UserID:     42,
			ScenarioID: 9,
			TemplateID: 1,
		})

		if !errors.Is(err, service.ErrUnknownPracticeScenario) {
			t.Fatalf("expected unknown scenario error, got %v", err)
		}
	})

	t.Run("reports missing configuration", func(t *testing.T) {
		svc := service.NewPracticeService(nil, &stubRunnerClient{}, time.Now)

		_, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
			UserID:     42,
			ScenarioID: 1,
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
			ScenarioID: 1,
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
			ScenarioID: 1,
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
		ScenarioID: 1,
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

func TestPracticeServiceExpiresStaleCurrentSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	store := &stubPracticeSessionStore{
		savedSession: domain.PracticeSession{
			ID:               301,
			UserID:           42,
			ScenarioID:       1,
			TemplateID:       1,
			RunnerRef:        "ws-expired",
			WorkspacePathRef: "/tmp/ws-expired",
			Status:           "active",
			StartedAt:        now.Add(-3 * time.Hour),
			ExpiresAt:        now.Add(-5 * time.Minute),
			LastActivityAt:   now.Add(-10 * time.Minute),
		},
	}
	svc := service.NewPracticeService(store, &stubRunnerClient{}, func() time.Time { return now })

	_, err := svc.CurrentPracticeSession(context.Background(), 42)

	if !errors.Is(err, service.ErrPracticeSessionNotFound) {
		t.Fatalf("expected stale current session to disappear, got %v", err)
	}
	if store.updateCalls != 1 {
		t.Fatalf("expected one lifecycle update, got %d", store.updateCalls)
	}
	if store.lastUpdatedSession.Status != "expired" {
		t.Fatalf("expected expired status, got %q", store.lastUpdatedSession.Status)
	}
	if store.lastUpdatedSession.EndedAt == nil || !store.lastUpdatedSession.EndedAt.Equal(now) {
		t.Fatalf("expected ended at %v, got %v", now, store.lastUpdatedSession.EndedAt)
	}
}

func TestPracticeServiceReturnsOrphanedCurrentSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 19, 12, 30, 0, 0, time.UTC)
	store := &stubPracticeSessionStore{
		savedSession: domain.PracticeSession{
			ID:               302,
			UserID:           42,
			ScenarioID:       1,
			TemplateID:       1,
			RunnerRef:        "ws-orphaned",
			WorkspacePathRef: "/tmp/ws-orphaned",
			Status:           "orphaned",
			StartedAt:        now.Add(-30 * time.Minute),
			EndedAt:          timePtr(now.Add(-5 * time.Minute)),
			ExpiresAt:        now.Add(90 * time.Minute),
			LastActivityAt:   now.Add(-5 * time.Minute),
		},
	}
	svc := service.NewPracticeService(store, &stubRunnerClient{}, func() time.Time { return now })

	_, err := svc.CurrentPracticeSession(context.Background(), 42)

	if !errors.Is(err, service.ErrPracticeSessionOrphaned) {
		t.Fatalf("expected orphaned current session error, got %v", err)
	}
	if store.updateCalls != 0 {
		t.Fatalf("expected orphaned current session to avoid extra lifecycle updates, got %d", store.updateCalls)
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
		ScenarioID: 1,
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
		ScenarioID: 1,
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
			ScenarioID: 1,
			TemplateID: 1,
		})
		if err != nil {
			t.Fatalf("create practice session: %v", err)
		}

		if err := svc.ResetPracticeSession(context.Background(), 42, created.ID); !errors.Is(err, service.ErrRunnerWorkspaceReset) {
			t.Fatalf("expected runner reset error, got %v", err)
		}
	})

	t.Run("marks missing runner workspaces as orphaned", func(t *testing.T) {
		now := time.Date(2026, 5, 19, 13, 0, 0, 0, time.UTC)
		store := &stubPracticeSessionStore{}
		svc := service.NewPracticeService(store, &stubRunnerClient{
			workspace: runner.Workspace{
				ID:       "ws-reset-missing",
				Path:     "/tmp/ws-reset-missing",
				Template: "standard",
			},
			resetErr: runner.ErrWorkspaceNotFound,
		}, func() time.Time { return now })

		created, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
			UserID:     42,
			ScenarioID: 1,
			TemplateID: 1,
		})
		if err != nil {
			t.Fatalf("create practice session: %v", err)
		}

		err = svc.ResetPracticeSession(context.Background(), 42, created.ID)

		if !errors.Is(err, service.ErrPracticeSessionOrphaned) {
			t.Fatalf("expected orphaned session error, got %v", err)
		}
		if store.updateCalls != 1 {
			t.Fatalf("expected one lifecycle update, got %d", store.updateCalls)
		}
		if store.lastUpdatedSession.Status != "orphaned" {
			t.Fatalf("expected orphaned status, got %q", store.lastUpdatedSession.Status)
		}
	})
}

func TestPracticeServiceOrphanTransitionUpsertsDelayedCleanupJob(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 24, 16, 0, 0, 0, time.UTC)
	store := &stubPracticeSessionStore{}
	runnerClient := &stubRunnerClient{
		workspace: runner.Workspace{
			ID:       "ws-orphan-cleanup",
			Path:     "/tmp/ws-orphan-cleanup",
			Template: "standard",
		},
		connectErr: runner.ErrWorkspaceNotFound,
	}
	svc := service.NewPracticeService(store, runnerClient, func() time.Time { return now })

	created, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     42,
		ScenarioID: 1,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	_, err = svc.ConnectTerminal(context.Background(), 42, created.ID)

	if !errors.Is(err, service.ErrPracticeSessionOrphaned) {
		t.Fatalf("expected orphaned session error, got %v", err)
	}
	if store.updateCalls != 1 {
		t.Fatalf("expected one lifecycle update, got %d", store.updateCalls)
	}
	if store.lastUpdatedSession.Status != "orphaned" {
		t.Fatalf("expected orphaned status, got %q", store.lastUpdatedSession.Status)
	}
	if len(store.upsertCleanupJobCalls) != 1 {
		t.Fatalf("expected one cleanup job upsert, got %d", len(store.upsertCleanupJobCalls))
	}
	job := store.upsertCleanupJobCalls[0]
	if job.PracticeSessionID != created.ID {
		t.Fatalf("expected cleanup job for session %d, got %d", created.ID, job.PracticeSessionID)
	}
	if job.WorkspaceID != "ws-orphan-cleanup" {
		t.Fatalf("expected orphaned cleanup for ws-orphan-cleanup, got %q", job.WorkspaceID)
	}
	if job.Reason != service.PracticeSessionStatusOrphaned {
		t.Fatalf("expected orphaned cleanup reason, got %q", job.Reason)
	}
	if !job.ScheduledAt.Equal(now.Add(10 * time.Minute)) {
		t.Fatalf("expected 10 minute orphan cleanup delay, got %v", job.ScheduledAt)
	}
}

func TestPracticeServiceReturnsRepoStateForSession(t *testing.T) {
	t.Parallel()

	store := &stubPracticeSessionStore{}
	runnerClient := &stubRunnerClient{
		workspace: runner.Workspace{
			ID:       "ws-repo-state",
			Path:     "/tmp/ws-repo-state",
			Template: "standard",
		},
		repoState: runner.RepoState{
			BranchName:    "main",
			HeadCommit:    "abc123",
			StatusSummary: []string{"M notes.txt"},
			CapturedAt:    time.Date(2026, 5, 23, 4, 0, 0, 0, time.UTC),
		},
	}
	svc := service.NewPracticeService(store, runnerClient, time.Now)

	created, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     42,
		ScenarioID: 1,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	repoState, err := svc.PracticeSessionRepoState(context.Background(), 42, created.ID)
	if err != nil {
		t.Fatalf("expected repo state, got %v", err)
	}
	if repoState.BranchName != "main" {
		t.Fatalf("expected branch main, got %q", repoState.BranchName)
	}
	if runnerClient.repoStateCalls != 1 {
		t.Fatalf("expected one repo state lookup, got %d", runnerClient.repoStateCalls)
	}
	if runnerClient.lastRepoStateWorkspaceID != created.RunnerRef {
		t.Fatalf("expected repo state lookup for %q, got %q", created.RunnerRef, runnerClient.lastRepoStateWorkspaceID)
	}
}

func TestPracticeServiceMarksMissingRunnerWorkspaceOnRepoStateLookup(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 23, 4, 30, 0, 0, time.UTC)
	store := &stubPracticeSessionStore{}
	runnerClient := &stubRunnerClient{
		workspace: runner.Workspace{
			ID:       "ws-repo-missing",
			Path:     "/tmp/ws-repo-missing",
			Template: "standard",
		},
		repoStateErr: runner.ErrWorkspaceNotFound,
	}
	svc := service.NewPracticeService(store, runnerClient, func() time.Time { return now })

	created, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     42,
		ScenarioID: 1,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	_, err = svc.PracticeSessionRepoState(context.Background(), 42, created.ID)

	if !errors.Is(err, service.ErrPracticeSessionOrphaned) {
		t.Fatalf("expected orphaned session error, got %v", err)
	}
	if runnerClient.lastRepoStateWorkspaceID != created.RunnerRef {
		t.Fatalf("expected repo state lookup for %q, got %q", created.RunnerRef, runnerClient.lastRepoStateWorkspaceID)
	}
	if store.updateCalls != 1 {
		t.Fatalf("expected one lifecycle update, got %d", store.updateCalls)
	}
	if store.lastUpdatedSession.Status != service.PracticeSessionStatusOrphaned {
		t.Fatalf("expected orphaned status, got %q", store.lastUpdatedSession.Status)
	}
	if len(store.upsertCleanupJobCalls) != 1 {
		t.Fatalf("expected one cleanup job upsert, got %d", len(store.upsertCleanupJobCalls))
	}
	job := store.upsertCleanupJobCalls[0]
	if job.WorkspaceID != created.RunnerRef {
		t.Fatalf("expected orphaned cleanup for %q, got %q", created.RunnerRef, job.WorkspaceID)
	}
	if job.Reason != service.PracticeSessionStatusOrphaned {
		t.Fatalf("expected orphaned cleanup reason, got %q", job.Reason)
	}
}

func TestPracticeServiceCurrentSessionRemainsRecoverableAfterTerminalOrphansWorkspace(t *testing.T) {
	t.Parallel()

	store := service.NewInMemoryPracticeSessionStore()
	svc := service.NewPracticeService(store, &stubRunnerClient{
		workspace: runner.Workspace{
			ID:       "ws-terminal-current",
			Path:     "/tmp/ws-terminal-current",
			Template: "standard",
		},
		connectErr: runner.ErrWorkspaceNotFound,
	}, time.Now)

	created, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     42,
		ScenarioID: 1,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	if _, err := svc.ConnectTerminal(context.Background(), 42, created.ID); !errors.Is(err, service.ErrPracticeSessionOrphaned) {
		t.Fatalf("expected terminal connect to mark session orphaned, got %v", err)
	}

	_, err = svc.CurrentPracticeSession(context.Background(), 42)

	if !errors.Is(err, service.ErrPracticeSessionOrphaned) {
		t.Fatalf("expected current session to remain orphaned for recovery, got %v", err)
	}
}

func TestPracticeServiceExpireSweepUpsertsCleanupJob(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 24, 15, 0, 0, 0, time.UTC)
	store := &stubPracticeSessionStore{
		expirableSessions: []domain.PracticeSession{
			{
				ID:               41,
				UserID:           7,
				ScenarioID:       1,
				TemplateID:       1,
				RunnerRef:        "ws-sweep-cleanup",
				WorkspacePathRef: "/tmp/ws-sweep-cleanup",
				Status:           "active",
				StartedAt:        now.Add(-4 * time.Hour),
				ExpiresAt:        now.Add(-5 * time.Minute),
				LastActivityAt:   now.Add(-10 * time.Minute),
			},
			{
				ID:               402,
				UserID:           43,
				ScenarioID:       1,
				TemplateID:       1,
				RunnerRef:        "ws-sweep-fresh",
				WorkspacePathRef: "/tmp/ws-sweep-fresh",
				Status:           "active",
				StartedAt:        now.Add(-20 * time.Minute),
				ExpiresAt:        now.Add(30 * time.Minute),
				LastActivityAt:   now.Add(-2 * time.Minute),
			},
		},
	}
	svc := service.NewPracticeService(store, &stubRunnerClient{}, func() time.Time { return now })

	expiredCount, err := svc.ExpireStalePracticeSessions(context.Background())

	if err != nil {
		t.Fatalf("expire stale practice sessions: %v", err)
	}
	if expiredCount != 1 {
		t.Fatalf("expected one expired session, got %d", expiredCount)
	}
	if store.expireCalls != 1 {
		t.Fatalf("expected one expire call, got %d", store.expireCalls)
	}
	if !store.lastExpireBefore.Equal(now) {
		t.Fatalf("expected expire cutoff %v, got %v", now, store.lastExpireBefore)
	}
	if len(store.expireResults) != 1 {
		t.Fatalf("expected one transitioned session, got %d", len(store.expireResults))
	}
	if store.expireResults[0].Status != "expired" {
		t.Fatalf("expected expired status, got %q", store.expireResults[0].Status)
	}
	if store.expireResults[0].EndedAt == nil || !store.expireResults[0].EndedAt.Equal(now) {
		t.Fatalf("expected ended at %v, got %v", now, store.expireResults[0].EndedAt)
	}
	if store.expireResults[0].LastActivityAt != now {
		t.Fatalf("expected last activity at %v, got %v", now, store.expireResults[0].LastActivityAt)
	}
	if len(store.upsertCleanupJobCalls) != 1 {
		t.Fatalf("expected one cleanup job upsert, got %d", len(store.upsertCleanupJobCalls))
	}
	job := store.upsertCleanupJobCalls[0]
	if job.PracticeSessionID != 41 {
		t.Fatalf("expected cleanup job for session 41, got %d", job.PracticeSessionID)
	}
	if job.WorkspaceID != "ws-sweep-cleanup" {
		t.Fatalf("expected workspace id ws-sweep-cleanup, got %q", job.WorkspaceID)
	}
	if job.Reason != service.PracticeSessionStatusExpired {
		t.Fatalf("expected cleanup reason expired, got %q", job.Reason)
	}
	if !job.ScheduledAt.Equal(now) {
		t.Fatalf("expected immediate cleanup scheduling at %v, got %v", now, job.ScheduledAt)
	}
}

func TestPracticeServiceRunWorkspaceCleanupDueJobsMarksSuccess(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 24, 17, 0, 0, 0, time.UTC)
	store := &stubPracticeSessionStore{
		claimedCleanupJobs: []domain.WorkspaceCleanupJob{
			{
				ID:                7,
				PracticeSessionID: 41,
				WorkspaceID:       "ws-cleanup-run",
				Reason:            service.PracticeSessionStatusExpired,
				ScheduledAt:       now.Add(-time.Minute),
				Status:            "running",
			},
		},
	}
	runnerClient := &stubRunnerClient{}
	svc := service.NewPracticeService(store, runnerClient, func() time.Time { return now })

	if err := svc.RunWorkspaceCleanupDueJobs(context.Background(), 10); err != nil {
		t.Fatalf("run workspace cleanup jobs: %v", err)
	}
	if store.claimCleanupJobsCalls != 1 {
		t.Fatalf("expected one claim call, got %d", store.claimCleanupJobsCalls)
	}
	if !store.lastClaimCleanupNow.Equal(now) {
		t.Fatalf("expected claim time %v, got %v", now, store.lastClaimCleanupNow)
	}
	if store.lastClaimCleanupLimit != 10 {
		t.Fatalf("expected claim limit 10, got %d", store.lastClaimCleanupLimit)
	}
	if runnerClient.deleteWorkspaceCalls != 1 {
		t.Fatalf("expected one delete workspace call, got %d", runnerClient.deleteWorkspaceCalls)
	}
	if runnerClient.lastDeleteWorkspaceID != "ws-cleanup-run" {
		t.Fatalf("expected delete workspace id ws-cleanup-run, got %q", runnerClient.lastDeleteWorkspaceID)
	}
	if runnerClient.lastDeleteReason != service.PracticeSessionStatusExpired {
		t.Fatalf("expected delete reason %q, got %q", service.PracticeSessionStatusExpired, runnerClient.lastDeleteReason)
	}
	if runnerClient.lastDeleteDelay != 0 {
		t.Fatalf("expected worker delete delay 0, got %v", runnerClient.lastDeleteDelay)
	}
	if len(store.markCleanupSucceededCalls) != 1 || store.markCleanupSucceededCalls[0] != 7 {
		t.Fatalf("expected cleanup job 7 to be marked succeeded, got %v", store.markCleanupSucceededCalls)
	}
	if len(store.markCleanupFailedCalls) != 0 {
		t.Fatalf("expected no failed cleanup marks, got %d", len(store.markCleanupFailedCalls))
	}
}

func TestPracticeServiceRunWorkspaceCleanupDueJobsTreatsWorkspaceNotFoundAsSuccess(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 24, 17, 30, 0, 0, time.UTC)
	store := &stubPracticeSessionStore{
		claimedCleanupJobs: []domain.WorkspaceCleanupJob{
			{
				ID:                10,
				PracticeSessionID: 62,
				WorkspaceID:       "ws-cleanup-missing",
				Reason:            service.PracticeSessionStatusExpired,
				ScheduledAt:       now.Add(-time.Minute),
				Status:            "running",
			},
		},
	}
	runnerClient := &stubRunnerClient{
		deleteErr: runner.ErrWorkspaceNotFound,
	}
	svc := service.NewPracticeService(store, runnerClient, func() time.Time { return now })

	if err := svc.RunWorkspaceCleanupDueJobs(context.Background(), 10); err != nil {
		t.Fatalf("run workspace cleanup jobs: %v", err)
	}
	if runnerClient.deleteWorkspaceCalls != 1 {
		t.Fatalf("expected one delete workspace call, got %d", runnerClient.deleteWorkspaceCalls)
	}
	if len(store.markCleanupSucceededCalls) != 1 || store.markCleanupSucceededCalls[0] != 10 {
		t.Fatalf("expected cleanup job 10 to be marked succeeded, got %v", store.markCleanupSucceededCalls)
	}
	if len(store.markCleanupFailedCalls) != 0 {
		t.Fatalf("expected no failed cleanup marks, got %d", len(store.markCleanupFailedCalls))
	}
}

func TestPracticeServiceRunWorkspaceCleanupDueJobsReschedulesFailure(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 24, 18, 0, 0, 0, time.UTC)
	store := &stubPracticeSessionStore{
		claimedCleanupJobs: []domain.WorkspaceCleanupJob{
			{
				ID:                8,
				PracticeSessionID: 52,
				WorkspaceID:       "ws-cleanup-fail",
				Reason:            service.PracticeSessionStatusOrphaned,
				ScheduledAt:       now.Add(-time.Minute),
				Status:            "running",
				AttemptCount:      1,
			},
		},
	}
	runnerClient := &stubRunnerClient{
		deleteErr: errors.New("runner unavailable"),
	}
	svc := service.NewPracticeService(store, runnerClient, func() time.Time { return now })

	err := svc.RunWorkspaceCleanupDueJobs(context.Background(), 10)

	if err == nil {
		t.Fatal("expected runner delete failure to be returned")
	}
	if runnerClient.deleteWorkspaceCalls != 1 {
		t.Fatalf("expected one delete workspace call, got %d", runnerClient.deleteWorkspaceCalls)
	}
	if runnerClient.lastDeleteDelay != 0 {
		t.Fatalf("expected worker delete delay 0, got %v", runnerClient.lastDeleteDelay)
	}
	if len(store.markCleanupSucceededCalls) != 0 {
		t.Fatalf("expected no succeeded cleanup marks, got %v", store.markCleanupSucceededCalls)
	}
	if len(store.markCleanupFailedCalls) != 1 {
		t.Fatalf("expected one failed cleanup mark, got %d", len(store.markCleanupFailedCalls))
	}
	failure := store.markCleanupFailedCalls[0]
	if failure.jobID != 8 {
		t.Fatalf("expected failed cleanup mark for job 8, got %d", failure.jobID)
	}
	if failure.lastError == "" {
		t.Fatal("expected failed cleanup to record last error")
	}
	if !failure.scheduledAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("expected first retry at %v, got %v", now.Add(time.Minute), failure.scheduledAt)
	}
	if !strings.Contains(err.Error(), "runner unavailable") {
		t.Fatalf("expected runner delete failure in error, got %v", err)
	}
}

func TestPracticeServiceRunWorkspaceCleanupDueJobsReturnsMarkSuccessError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 24, 19, 0, 0, 0, time.UTC)
	store := &stubPracticeSessionStore{
		claimedCleanupJobs: []domain.WorkspaceCleanupJob{
			{
				ID:                9,
				PracticeSessionID: 61,
				WorkspaceID:       "ws-cleanup-mark-success",
				Reason:            service.PracticeSessionStatusExpired,
				ScheduledAt:       now.Add(-time.Minute),
				Status:            "running",
			},
		},
		markCleanupSucceededErr: errors.New("write failed"),
	}
	runnerClient := &stubRunnerClient{}
	svc := service.NewPracticeService(store, runnerClient, func() time.Time { return now })

	err := svc.RunWorkspaceCleanupDueJobs(context.Background(), 10)

	if err == nil {
		t.Fatal("expected mark success error")
	}
	if runnerClient.deleteWorkspaceCalls != 1 {
		t.Fatalf("expected one delete workspace call, got %d", runnerClient.deleteWorkspaceCalls)
	}
	if len(store.markCleanupSucceededCalls) != 1 || store.markCleanupSucceededCalls[0] != 9 {
		t.Fatalf("expected cleanup job 9 to be marked succeeded, got %v", store.markCleanupSucceededCalls)
	}
	if len(store.upsertCleanupJobCalls) != 1 {
		t.Fatalf("expected one recovery cleanup job upsert, got %d", len(store.upsertCleanupJobCalls))
	}
	recovery := store.upsertCleanupJobCalls[0]
	if recovery.PracticeSessionID != 61 {
		t.Fatalf("expected recovery cleanup job for session 61, got %d", recovery.PracticeSessionID)
	}
	if recovery.WorkspaceID != "ws-cleanup-mark-success" {
		t.Fatalf("expected recovery cleanup workspace ws-cleanup-mark-success, got %q", recovery.WorkspaceID)
	}
	if recovery.Reason != service.PracticeSessionStatusExpired {
		t.Fatalf("expected recovery cleanup reason %q, got %q", service.PracticeSessionStatusExpired, recovery.Reason)
	}
	if recovery.Status != "pending" {
		t.Fatalf("expected recovery cleanup status pending, got %q", recovery.Status)
	}
	if !recovery.ScheduledAt.Equal(now) {
		t.Fatalf("expected immediate recovery scheduling at %v, got %v", now, recovery.ScheduledAt)
	}
	if !strings.Contains(err.Error(), "mark cleanup job 9 succeeded") {
		t.Fatalf("expected mark success context in error, got %v", err)
	}
}

func TestPracticeServiceRunWorkspaceCleanupDueJobsProcessesInMemoryCleanupJobs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 25, 9, 0, 0, 0, time.UTC)
	store := service.NewInMemoryPracticeSessionStore()
	runnerClient := &stubRunnerClient{
		workspace: runner.Workspace{
			ID:       "ws-in-memory-cleanup",
			Path:     "/tmp/ws-in-memory-cleanup",
			Template: "standard",
		},
	}
	svc := service.NewPracticeService(store, runnerClient, func() time.Time { return now })

	created, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     42,
		ScenarioID: 1,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	now = now.Add(3 * time.Hour)
	expiredCount, err := svc.ExpireStalePracticeSessions(context.Background())
	if err != nil {
		t.Fatalf("expire stale practice sessions: %v", err)
	}
	if expiredCount != 1 {
		t.Fatalf("expected one expired session, got %d", expiredCount)
	}

	if err := svc.RunWorkspaceCleanupDueJobs(context.Background(), 10); err != nil {
		t.Fatalf("run workspace cleanup jobs: %v", err)
	}
	if runnerClient.deleteWorkspaceCalls != 1 {
		t.Fatalf("expected one delete workspace call, got %d", runnerClient.deleteWorkspaceCalls)
	}
	if runnerClient.lastDeleteWorkspaceID != created.RunnerRef {
		t.Fatalf("expected delete workspace id %q, got %q", created.RunnerRef, runnerClient.lastDeleteWorkspaceID)
	}
	if runnerClient.lastDeleteReason != service.PracticeSessionStatusExpired {
		t.Fatalf("expected delete reason %q, got %q", service.PracticeSessionStatusExpired, runnerClient.lastDeleteReason)
	}

	if err := svc.RunWorkspaceCleanupDueJobs(context.Background(), 10); err != nil {
		t.Fatalf("rerun workspace cleanup jobs: %v", err)
	}
	if runnerClient.deleteWorkspaceCalls != 1 {
		t.Fatalf("expected cleanup job to be consumed once, got %d deletes", runnerClient.deleteWorkspaceCalls)
	}
}

func TestRunnerClientDeleteWorkspaceSendsCleanupRequest(t *testing.T) {
	t.Parallel()

	type deleteWorkspaceRequest struct {
		Reason             string `json:"reason"`
		DeleteAfterSeconds int    `json:"delete_after_seconds"`
	}

	requests := make(chan deleteWorkspaceRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected method %s, got %s", http.MethodDelete, r.Method)
		}
		if r.URL.Path != "/internal/workspaces/ws-delete" {
			t.Fatalf("expected path %q, got %q", "/internal/workspaces/ws-delete", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("expected content type %q, got %q", "application/json", got)
		}

		var req deleteWorkspaceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode delete workspace request: %v", err)
		}

		requests <- req
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := runner.NewClient(server.URL, http.DefaultClient)

	err := client.DeleteWorkspace(context.Background(), "ws-delete", "expired", 2*time.Minute)

	if err != nil {
		t.Fatalf("delete workspace: %v", err)
	}

	select {
	case req := <-requests:
		if req.Reason != "expired" {
			t.Fatalf("expected reason %q, got %q", "expired", req.Reason)
		}
		if req.DeleteAfterSeconds != 120 {
			t.Fatalf("expected delete_after_seconds %d, got %d", 120, req.DeleteAfterSeconds)
		}
	default:
		t.Fatal("expected delete workspace request to be sent")
	}
}

func TestRunnerClientDeleteWorkspaceMapsNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := runner.NewClient(server.URL, http.DefaultClient)

	err := client.DeleteWorkspace(context.Background(), "ws-missing", "expired", 0)

	if !errors.Is(err, runner.ErrWorkspaceNotFound) {
		t.Fatalf("expected workspace not found error, got %v", err)
	}
}

type stubPracticeSessionStore struct {
	createCalls               int
	updateCalls               int
	expireCalls               int
	claimCleanupJobsCalls     int
	lastSession               domain.PracticeSession
	savedSession              domain.PracticeSession
	lastUpdatedSession        domain.PracticeSession
	lastExpireBefore          time.Time
	lastClaimCleanupNow       time.Time
	lastClaimCleanupLimit     int
	expirableSessions         []domain.PracticeSession
	expireResults             []domain.PracticeSession
	claimedCleanupJobs        []domain.WorkspaceCleanupJob
	upsertCleanupJobCalls     []domain.WorkspaceCleanupJob
	markCleanupSucceededCalls []uint64
	markCleanupFailedCalls    []cleanupFailureMark
	upsertCleanupJobErr       error
	claimCleanupJobsErr       error
	markCleanupSucceededErr   error
	markCleanupFailedErr      error
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

func (s *stubPracticeSessionStore) UpdatePracticeSession(_ context.Context, session domain.PracticeSession) (domain.PracticeSession, error) {
	s.updateCalls++
	s.lastUpdatedSession = session
	s.savedSession = session
	return session, nil
}

func (s *stubPracticeSessionStore) ExpirePracticeSessions(_ context.Context, before time.Time, endedAt time.Time) ([]domain.PracticeSession, error) {
	s.expireCalls++
	s.lastExpireBefore = before
	s.expireResults = nil

	for index, session := range s.expirableSessions {
		if session.Status != service.PracticeSessionStatusActive || session.ExpiresAt.After(before) {
			continue
		}

		session.Status = service.PracticeSessionStatusExpired
		session.LastActivityAt = endedAt
		if session.EndedAt == nil {
			session.EndedAt = timePtr(endedAt)
		}
		s.expirableSessions[index] = session
		s.expireResults = append(s.expireResults, session)
	}

	return append([]domain.PracticeSession(nil), s.expireResults...), nil
}

func (s *stubPracticeSessionStore) UpsertWorkspaceCleanupJob(_ context.Context, job domain.WorkspaceCleanupJob) error {
	if s.upsertCleanupJobErr != nil {
		return s.upsertCleanupJobErr
	}

	s.upsertCleanupJobCalls = append(s.upsertCleanupJobCalls, job)

	return nil
}

func (s *stubPracticeSessionStore) ClaimDueWorkspaceCleanupJobs(_ context.Context, now time.Time, limit int) ([]domain.WorkspaceCleanupJob, error) {
	s.claimCleanupJobsCalls++
	s.lastClaimCleanupNow = now
	s.lastClaimCleanupLimit = limit
	if s.claimCleanupJobsErr != nil {
		return nil, s.claimCleanupJobsErr
	}
	return append([]domain.WorkspaceCleanupJob(nil), s.claimedCleanupJobs...), nil
}

func (s *stubPracticeSessionStore) MarkWorkspaceCleanupJobSucceeded(_ context.Context, jobID uint64) error {
	s.markCleanupSucceededCalls = append(s.markCleanupSucceededCalls, jobID)
	return s.markCleanupSucceededErr
}

func (s *stubPracticeSessionStore) MarkWorkspaceCleanupJobFailed(_ context.Context, jobID uint64, scheduledAt time.Time, lastErr string) error {
	s.markCleanupFailedCalls = append(s.markCleanupFailedCalls, cleanupFailureMark{
		jobID:       jobID,
		scheduledAt: scheduledAt,
		lastError:   lastErr,
	})
	return s.markCleanupFailedErr
}

type cleanupFailureMark struct {
	jobID       uint64
	scheduledAt time.Time
	lastError   string
}

func (s *persistentStubStore) ClaimDueWorkspaceCleanupJobs(ctx context.Context, now time.Time, limit int) ([]domain.WorkspaceCleanupJob, error) {
	return s.practiceSessions.ClaimDueWorkspaceCleanupJobs(ctx, now, limit)
}

func (s *persistentStubStore) MarkWorkspaceCleanupJobSucceeded(ctx context.Context, jobID uint64) error {
	return s.practiceSessions.MarkWorkspaceCleanupJobSucceeded(ctx, jobID)
}

func (s *persistentStubStore) MarkWorkspaceCleanupJobFailed(ctx context.Context, jobID uint64, scheduledAt time.Time, lastErr string) error {
	return s.practiceSessions.MarkWorkspaceCleanupJobFailed(ctx, jobID, scheduledAt, lastErr)
}

func (s *stubPracticeService) RunWorkspaceCleanupDueJobs(context.Context, int) error {
	return nil
}

type stubRunnerClient struct {
	createWorkspaceCalls     int
	lastTemplate             string
	repoStateCalls           int
	lastRepoStateWorkspaceID string
	repoState                runner.RepoState
	resetWorkspaceCalls      int
	lastResetWorkspaceID     string
	deleteWorkspaceCalls     int
	lastDeleteWorkspaceID    string
	lastDeleteReason         string
	lastDeleteDelay          time.Duration
	workspace                runner.Workspace
	connectTerminalFunc      func(context.Context, string) (runner.TerminalConnection, error)
	deleteWorkspaceFunc      func(context.Context, int, string, string, time.Duration) error
	err                      error
	repoStateErr             error
	resetErr                 error
	connectErr               error
	deleteErr                error
}

func (s *stubRunnerClient) CreateWorkspace(_ context.Context, template string) (runner.Workspace, error) {
	s.createWorkspaceCalls++
	s.lastTemplate = template
	if s.err != nil {
		return runner.Workspace{}, s.err
	}
	return s.workspace, nil
}

func (s *stubRunnerClient) GetRepoState(_ context.Context, workspaceID string) (runner.RepoState, error) {
	s.repoStateCalls++
	s.lastRepoStateWorkspaceID = workspaceID
	if s.repoStateErr != nil {
		return runner.RepoState{}, s.repoStateErr
	}
	return s.repoState, nil
}

func (s *stubRunnerClient) ResetWorkspace(_ context.Context, workspaceID string) error {
	s.resetWorkspaceCalls++
	s.lastResetWorkspaceID = workspaceID
	if s.resetErr != nil {
		return s.resetErr
	}
	return nil
}

func (s *stubRunnerClient) ConnectTerminal(ctx context.Context, workspaceID string) (runner.TerminalConnection, error) {
	if s.connectTerminalFunc != nil {
		return s.connectTerminalFunc(ctx, workspaceID)
	}
	if s.connectErr != nil {
		return nil, s.connectErr
	}
	return &stubTerminalConnection{}, nil
}

func (s *stubRunnerClient) DeleteWorkspace(ctx context.Context, workspaceID string, reason string, deleteAfter time.Duration) error {
	s.deleteWorkspaceCalls++
	s.lastDeleteWorkspaceID = workspaceID
	s.lastDeleteReason = reason
	s.lastDeleteDelay = deleteAfter
	if s.deleteWorkspaceFunc != nil {
		return s.deleteWorkspaceFunc(ctx, s.deleteWorkspaceCalls, workspaceID, reason, deleteAfter)
	}
	return s.deleteErr
}

type stubTerminalConnection struct{}

func timePtr(value time.Time) *time.Time {
	return &value
}

func (s *stubTerminalConnection) Read(context.Context) (int, []byte, error) {
	return 0, nil, context.Canceled
}

func (s *stubTerminalConnection) Write(context.Context, int, []byte) error {
	return nil
}

func (s *stubTerminalConnection) Close(websocket.StatusCode, string) error {
	return nil
}
