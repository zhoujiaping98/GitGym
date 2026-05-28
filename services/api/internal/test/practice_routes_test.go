package test

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitgym/services/api/internal/config"
	"gitgym/services/api/internal/domain"
	httpx "gitgym/services/api/internal/http"
	"gitgym/services/api/internal/runner"
	"gitgym/services/api/internal/service"
	"gitgym/services/api/internal/store"
)

func TestPracticeRoutesMatchPlanSurface(t *testing.T) {
	authStore := authStoreWithSession("persisted-route-token", 42)
	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: &stubPracticeService{},
		RunnerClient:    &stubRunnerClient{},
		AuthStore:       authStore,
	})

	t.Run("planned routes are mounted behind auth", func(t *testing.T) {
		cases := []struct {
			name   string
			method string
			target string
			body   []byte
			status int
		}{
			{name: "list templates", method: http.MethodGet, target: "/api/v1/templates", status: http.StatusOK},
			{name: "current session", method: http.MethodGet, target: "/api/v1/practice-sessions/current", status: http.StatusNotFound},
			{name: "create session", method: http.MethodPost, target: "/api/v1/practice-sessions", body: []byte(`{"scenario_id":1}`), status: http.StatusCreated},
			{name: "reset session", method: http.MethodPost, target: "/api/v1/practice-sessions/123/reset", status: http.StatusAccepted},
			{name: "repo state route", method: http.MethodGet, target: "/api/v1/practice-sessions/123/repo-state", status: http.StatusNotFound},
			{name: "terminal route", method: http.MethodGet, target: "/api/v1/practice-sessions/123/terminal", status: http.StatusNotFound},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(tc.method, tc.target, bytes.NewReader(tc.body))
				req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "persisted-route-token"})
				if len(tc.body) > 0 {
					req.Header.Set("Content-Type", "application/json")
				}
				rec := httptest.NewRecorder()

				router.ServeHTTP(rec, req)

				if rec.Code != tc.status {
					t.Fatalf("expected %d for %s %s, got %d", tc.status, tc.method, tc.target, rec.Code)
				}
			})
		}
	})

	t.Run("create session remains protected without cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/practice-sessions", strings.NewReader(`{"scenario_id":1}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("current session remains protected without cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/practice-sessions/current", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("dev auth bypass allows local requests without cookie", func(t *testing.T) {
		t.Setenv("DEV_AUTH_BYPASS", "true")

		createReq := httptest.NewRequest(http.MethodPost, "/api/v1/practice-sessions", strings.NewReader(`{"scenario_id":1}`))
		createReq.Header.Set("Content-Type", "application/json")
		createReq.RemoteAddr = "127.0.0.1:45678"
		createRec := httptest.NewRecorder()

		router.ServeHTTP(createRec, createReq)

		if createRec.Code != http.StatusCreated {
			t.Fatalf("expected 201 with dev auth bypass, got %d with body %s", createRec.Code, createRec.Body.String())
		}

		currentReq := httptest.NewRequest(http.MethodGet, "/api/v1/practice-sessions/current", nil)
		currentReq.RemoteAddr = "127.0.0.1:45678"
		currentRec := httptest.NewRecorder()

		router.ServeHTTP(currentRec, currentReq)

		if currentRec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 current session with dev auth bypass, got %d", currentRec.Code)
		}
	})

	t.Run("dev auth bypass does not apply to non-loopback requests", func(t *testing.T) {
		t.Setenv("DEV_AUTH_BYPASS", "true")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/practice-sessions/current", nil)
		req.RemoteAddr = "10.24.8.9:45678"
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for non-loopback bypass request, got %d", rec.Code)
		}
	})

	t.Run("reset session remains protected without cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/practice-sessions/123/reset", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("repo state remains protected without cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/practice-sessions/123/repo-state", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("legacy planned mismatches are not mounted", func(t *testing.T) {
		cases := []struct {
			method string
			target string
		}{
			{method: http.MethodGet, target: "/api/v1/practice/templates"},
			{method: http.MethodPost, target: "/api/v1/practice/sessions"},
			{method: http.MethodGet, target: "/api/v1/practice/sessions/123/terminal/ws"},
		}

		for _, tc := range cases {
			req := httptest.NewRequest(tc.method, tc.target, nil)
			req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "persisted-route-token"})
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("expected 404 for %s %s, got %d", tc.method, tc.target, rec.Code)
			}
		}
	})
}

func TestPracticeSessionRepoStateReturnsSnapshot(t *testing.T) {
	t.Parallel()

	capturedAt := time.Date(2026, 5, 23, 4, 0, 0, 0, time.UTC)
	store := service.NewInMemoryPracticeSessionStore()
	runnerClient := &stubRunnerClient{
		workspace: runner.Workspace{
			ID:       "ws-repo",
			Path:     "/tmp/ws-repo",
			Template: "standard",
		},
		repoState: runner.RepoState{
			BranchName:    "feature/repo-panel",
			HeadCommit:    "6f9bc9e2f9e3f4f24b88a1d8d76d8ef0f1b1c6a0",
			StatusSummary: []string{"M notes.txt", "?? scratch.md"},
			CapturedAt:    capturedAt,
		},
	}
	practiceService := service.NewPracticeService(store, runnerClient, func() time.Time { return capturedAt })
	created, err := practiceService.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     42,
		ScenarioID: 1,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: practiceService,
		RunnerClient:    runnerClient,
		AuthStore:       authStoreWithSession("repo-state-token", 42),
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/practice-sessions/%d/repo-state", created.ID), nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "repo-state-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Data struct {
			Branch       string   `json:"branch"`
			HeadCommit   string   `json:"head_commit"`
			Dirty        bool     `json:"dirty"`
			ChangedFiles []string `json:"changed_files"`
			CapturedAt   string   `json:"captured_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal repo-state payload: %v", err)
	}
	if payload.Data.Branch != "feature/repo-panel" {
		t.Fatalf("expected branch to round-trip, got %q", payload.Data.Branch)
	}
	if payload.Data.HeadCommit != "6f9bc9e2f9e3f4f24b88a1d8d76d8ef0f1b1c6a0" {
		t.Fatalf("expected head commit to round-trip, got %q", payload.Data.HeadCommit)
	}
	if !payload.Data.Dirty {
		t.Fatalf("expected dirty repo-state payload, got %+v", payload.Data)
	}
	if len(payload.Data.ChangedFiles) != 2 {
		t.Fatalf("expected changed files to round-trip, got %+v", payload.Data.ChangedFiles)
	}
	if payload.Data.CapturedAt != capturedAt.Format(time.RFC3339) {
		t.Fatalf("expected captured_at %q, got %q", capturedAt.Format(time.RFC3339), payload.Data.CapturedAt)
	}
	if runnerClient.repoStateCalls != 1 {
		t.Fatalf("expected one repo state lookup, got %d", runnerClient.repoStateCalls)
	}
	if runnerClient.lastRepoStateWorkspaceID != created.RunnerRef {
		t.Fatalf("expected repo state lookup for %q, got %q", created.RunnerRef, runnerClient.lastRepoStateWorkspaceID)
	}
}

func TestPracticeSessionRepoStateMapsMissingWorkspaceToGone(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 23, 5, 0, 0, 0, time.UTC)
	store := service.NewInMemoryPracticeSessionStore()
	runnerClient := &stubRunnerClient{
		workspace: runner.Workspace{
			ID:       "ws-missing",
			Path:     "/tmp/ws-missing",
			Template: "standard",
		},
		repoStateErr: runner.ErrWorkspaceNotFound,
	}
	practiceService := service.NewPracticeService(store, runnerClient, func() time.Time { return now })
	created, err := practiceService.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     42,
		ScenarioID: 1,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: practiceService,
		RunnerClient:    runnerClient,
		AuthStore:       authStoreWithSession("repo-gone-token", 42),
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/practice-sessions/%d/repo-state", created.ID), nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "repo-gone-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d with body %s", rec.Code, rec.Body.String())
	}
	if runnerClient.lastRepoStateWorkspaceID != created.RunnerRef {
		t.Fatalf("expected repo state lookup for %q, got %q", created.RunnerRef, runnerClient.lastRepoStateWorkspaceID)
	}
	currentReq := httptest.NewRequest(http.MethodGet, "/api/v1/practice-sessions/current", nil)
	currentReq.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "repo-gone-token"})
	currentRec := httptest.NewRecorder()

	router.ServeHTTP(currentRec, currentReq)

	if currentRec.Code != http.StatusGone {
		t.Fatalf("expected current session to remain gone after orphan transition, got %d with body %s", currentRec.Code, currentRec.Body.String())
	}
}

func TestListPracticeCatalogReturnsTemplatesAndScenarios(t *testing.T) {
	t.Parallel()

	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: &stubPracticeService{
			listTemplatesResult: []service.PracticeTemplate{
				{ID: 2, Key: "recovery", Name: "Recovery"},
			},
			listScenariosResult: []service.PracticeScenario{
				{ID: 8, Key: "recover-branch", Name: "Recover Branch", TemplateID: 2},
			},
		},
		AuthStore: authStoreWithSession("catalog-store-token", 42),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "catalog-store-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Templates []struct {
			ID   uint64 `json:"id"`
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"templates"`
		Scenarios []struct {
			ID         uint64 `json:"id"`
			Key        string `json:"key"`
			Name       string `json:"name"`
			TemplateID uint64 `json:"template_id"`
		} `json:"scenarios"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal catalog payload: %v", err)
	}
	if len(payload.Templates) != 1 || payload.Templates[0].ID != 2 || payload.Templates[0].Key != "recovery" || payload.Templates[0].Name != "Recovery" {
		t.Fatalf("unexpected templates payload: %+v", payload.Templates)
	}
	if len(payload.Scenarios) != 1 || payload.Scenarios[0].ID != 8 || payload.Scenarios[0].Key != "recover-branch" || payload.Scenarios[0].Name != "Recover Branch" || payload.Scenarios[0].TemplateID != 2 {
		t.Fatalf("unexpected scenarios payload: %+v", payload.Scenarios)
	}
}

func TestWorkspaceCleanupOperatorRouteReturnsExhaustedJobs(t *testing.T) {
	t.Parallel()

	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: &stubPracticeService{
			listExhaustedWorkspaceCleanupJobsFunc: func(context.Context, int) ([]domain.WorkspaceCleanupJob, error) {
				return []domain.WorkspaceCleanupJob{
					{
						ID:                7,
						PracticeSessionID: 42,
						WorkspaceID:       "runner-42",
						Reason:            service.PracticeSessionStatusExpired,
						Status:            "failed",
						AttemptCount:      service.WorkspaceCleanupJobMaxAttempts,
						LastError:         "runner delete failed",
						ScheduledAt:       time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC),
						CreatedAt:         time.Date(2026, 5, 28, 11, 0, 0, 0, time.UTC),
						UpdatedAt:         time.Date(2026, 5, 28, 12, 5, 0, 0, time.UTC),
					},
				}, nil
			},
		},
		AuthConfig: config.Config{
			OperatorToken: "operator-secret",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/operator/workspace-cleanup-jobs/exhausted?limit=5", nil)
	req.Header.Set("Authorization", "Bearer operator-secret")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Data []struct {
			ID                uint64 `json:"id"`
			PracticeSessionID uint64 `json:"practice_session_id"`
			WorkspaceID       string `json:"workspace_id"`
			Reason            string `json:"reason"`
			Status            string `json:"status"`
			AttemptCount      uint32 `json:"attempt_count"`
			LastError         string `json:"last_error"`
			ScheduledAt       string `json:"scheduled_at"`
			CreatedAt         string `json:"created_at"`
			UpdatedAt         string `json:"updated_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal operator cleanup payload: %v", err)
	}
	if len(payload.Data) != 1 {
		t.Fatalf("expected one exhausted cleanup job, got %d", len(payload.Data))
	}
	if payload.Data[0].PracticeSessionID != 42 {
		t.Fatalf("expected practice session 42, got %d", payload.Data[0].PracticeSessionID)
	}
	if payload.Data[0].WorkspaceID != "runner-42" {
		t.Fatalf("expected workspace runner-42, got %q", payload.Data[0].WorkspaceID)
	}
	if payload.Data[0].AttemptCount != service.WorkspaceCleanupJobMaxAttempts {
		t.Fatalf("expected attempt count %d, got %d", service.WorkspaceCleanupJobMaxAttempts, payload.Data[0].AttemptCount)
	}
	if payload.Data[0].LastError != "runner delete failed" {
		t.Fatalf("expected last error to round-trip, got %q", payload.Data[0].LastError)
	}
}

func TestWorkspaceCleanupOperatorRouteRequiresBearerToken(t *testing.T) {
	t.Parallel()

	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: &stubPracticeService{},
		AuthConfig: config.Config{
			OperatorToken: "operator-secret",
		},
	})

	t.Run("missing bearer token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/operator/workspace-cleanup-jobs/exhausted", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("wrong bearer token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/operator/workspace-cleanup-jobs/exhausted", nil)
		req.Header.Set("Authorization", "Bearer wrong-secret")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}

func TestWorkspaceCleanupOperatorRouteIsNotMountedWithoutToken(t *testing.T) {
	t.Parallel()

	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: &stubPracticeService{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/operator/workspace-cleanup-jobs/exhausted", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when operator token is not configured, got %d", rec.Code)
	}
}

func TestRouterUsesFallbackCatalogWhenNoCatalogStoreIsAvailable(t *testing.T) {
	t.Parallel()

	router := httpx.NewRouter(httpx.Dependencies{
		AuthStore: authStoreWithSession("fallback-catalog-token", 42),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "fallback-catalog-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Templates []struct {
			Key string `json:"key"`
		} `json:"templates"`
		Scenarios []struct {
			Key string `json:"key"`
		} `json:"scenarios"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal fallback catalog payload: %v", err)
	}
	if len(payload.Templates) != 1 || payload.Templates[0].Key != "standard" {
		t.Fatalf("unexpected fallback templates: %+v", payload.Templates)
	}
	if len(payload.Scenarios) != 1 || payload.Scenarios[0].Key != "sandbox-standard" {
		t.Fatalf("unexpected fallback scenarios: %+v", payload.Scenarios)
	}
}

func TestRouterUsesCatalogStoreWhenAuthStoreSupportsCatalogReads(t *testing.T) {
	t.Parallel()

	router := httpx.NewRouter(httpx.Dependencies{
		AuthStore: &catalogPersistentStubStore{
			persistentStubStore: newPersistentStubStore("catalog-store-token", 42),
			templates: []service.PracticeTemplate{
				{ID: 2, Key: "recovery", Name: "Recovery"},
			},
			scenarios: []service.PracticeScenario{
				{ID: 8, Key: "recover-branch", Name: "Recover Branch", TemplateID: 2},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "catalog-store-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Templates []struct {
			ID   uint64 `json:"id"`
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"templates"`
		Scenarios []struct {
			ID         uint64 `json:"id"`
			Key        string `json:"key"`
			Name       string `json:"name"`
			TemplateID uint64 `json:"template_id"`
		} `json:"scenarios"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal catalog payload: %v", err)
	}
	if len(payload.Templates) != 1 || payload.Templates[0].ID != 2 || payload.Templates[0].Key != "recovery" {
		t.Fatalf("unexpected templates payload: %+v", payload.Templates)
	}
	if len(payload.Scenarios) != 1 || payload.Scenarios[0].ID != 8 || payload.Scenarios[0].Key != "recover-branch" || payload.Scenarios[0].TemplateID != 2 {
		t.Fatalf("unexpected scenarios payload: %+v", payload.Scenarios)
	}
}

func TestListPracticeCatalogReturnsServerErrorOnCatalogReadFailure(t *testing.T) {
	t.Parallel()

	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: &stubPracticeService{
			listTemplatesErr: errors.New("list practice templates: database offline"),
		},
		AuthStore: authStoreWithSession("catalog-error-token", 42),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "catalog-error-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d with body %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error payload: %v", err)
	}
	if payload.Error == "" {
		t.Fatalf("expected error payload, got %s", rec.Body.String())
	}
}

func TestListPracticeCatalogReturnsServerErrorWhenScenarioTemplateReferenceIsBroken(t *testing.T) {
	t.Parallel()

	router := httpx.NewRouter(httpx.Dependencies{
		AuthStore: &catalogPersistentStubStore{
			persistentStubStore: newPersistentStubStore("catalog-broken-token", 42),
			templates: []service.PracticeTemplate{
				{ID: 2, Key: "recovery", Name: "Recovery"},
			},
			scenarios: []service.PracticeScenario{
				{ID: 8, Key: "recover-branch", Name: "Recover Branch", TemplateID: 999},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "catalog-broken-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d with body %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error payload: %v", err)
	}
	if payload.Error == "" {
		t.Fatalf("expected error payload, got %s", rec.Body.String())
	}
}

func TestMySQLStoreUpdatePracticeSessionReturnsNotFoundWhenNoRowsUpdated(t *testing.T) {
	db := openExecOnlySQLDB(t, execOnlySQLDriver{
		rowsAffected: 0,
	})
	defer db.Close()

	mysqlStore := store.NewMySQLStore(db)

	_, err := mysqlStore.UpdatePracticeSession(context.Background(), domain.PracticeSession{
		ID:             999,
		Status:         service.PracticeSessionStatusExpired,
		LastActivityAt: time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC),
	})

	if !errors.Is(err, service.ErrPracticeSessionNotFound) {
		t.Fatalf("expected practice session not found error, got %v", err)
	}
}

func TestCreatePracticeSessionUsesAuthenticatedUserAndReturnsStableJSON(t *testing.T) {
	t.Parallel()

	recordingService := &stubPracticeService{
		createPracticeSessionFunc: func(_ context.Context, input service.CreatePracticeSessionInput) (domain.PracticeSession, error) {
			templateID := input.TemplateID
			if templateID == 0 {
				templateID = 1
			}
			return domain.PracticeSession{
				ID:               101,
				UserID:           input.UserID,
				ScenarioID:       input.ScenarioID,
				TemplateID:       templateID,
				RunnerRef:        "ws-123",
				WorkspacePathRef: "/tmp/ws-123",
				Status:           "active",
				StartedAt:        time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
				ExpiresAt:        time.Date(2026, 5, 16, 14, 0, 0, 0, time.UTC),
				LastActivityAt:   time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
			}, nil
		},
	}

	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: recordingService,
		AuthStore:       authStoreWithSession("user-42-session-token", 42),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/practice-sessions", strings.NewReader(`{"user_id":999,"scenario_id":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "user-42-session-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if recordingService.lastCreateInput.UserID != 42 {
		t.Fatalf("expected handler to use persisted authenticated user ID 42, got %d", recordingService.lastCreateInput.UserID)
	}
	if recordingService.lastCreateInput.ScenarioID != 1 {
		t.Fatalf("expected scenario ID 1, got %d", recordingService.lastCreateInput.ScenarioID)
	}
	if recordingService.lastCreateInput.TemplateID != 0 {
		t.Fatalf("expected handler to stop forwarding template ID, got %d", recordingService.lastCreateInput.TemplateID)
	}
	if strings.Contains(rec.Body.String(), `"UserID"`) {
		t.Fatalf("expected stable JSON field names, got body %s", rec.Body.String())
	}

	var payload struct {
		Session struct {
			ID             uint64 `json:"id"`
			UserID         uint64 `json:"user_id"`
			ScenarioID     uint64 `json:"scenario_id"`
			TemplateID     uint64 `json:"template_id"`
			RunnerRef      string `json:"runner_ref"`
			WorkspacePath  string `json:"workspace_path"`
			Status         string `json:"status"`
			StartedAt      string `json:"started_at"`
			ExpiresAt      string `json:"expires_at"`
			LastActivityAt string `json:"last_activity_at"`
		} `json:"session"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.Session.ID != 101 || payload.Session.UserID != 42 {
		t.Fatalf("unexpected session payload: %+v", payload.Session)
	}
	if payload.Session.ScenarioID != 1 || payload.Session.TemplateID != 1 {
		t.Fatalf("unexpected catalog identifiers in payload: %+v", payload.Session)
	}
	if payload.Session.RunnerRef != "ws-123" || payload.Session.WorkspacePath != "/tmp/ws-123" {
		t.Fatalf("unexpected runner payload: %+v", payload.Session)
	}
}

func TestCreatePracticeSessionAcceptsScenarioIDOnly(t *testing.T) {
	t.Parallel()

	recordingService := &stubPracticeService{}
	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: recordingService,
		AuthStore:       authStoreWithSession("scenario-only-token", 42),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/practice-sessions", strings.NewReader(`{"scenario_id":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "scenario-only-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", rec.Code, rec.Body.String())
	}
	if recordingService.lastCreateInput.ScenarioID != 1 {
		t.Fatalf("expected scenario ID 1, got %d", recordingService.lastCreateInput.ScenarioID)
	}
	if recordingService.lastCreateInput.TemplateID != 0 {
		t.Fatalf("expected template ID to be omitted, got %d", recordingService.lastCreateInput.TemplateID)
	}
}

func TestCreatePracticeSessionRejectsMissingScenarioID(t *testing.T) {
	t.Parallel()

	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: &stubPracticeService{},
		AuthStore:       authStoreWithSession("missing-scenario-token", 42),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/practice-sessions", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "missing-scenario-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error payload: %v", err)
	}
	if payload.Error != "scenario_id is required" {
		t.Fatalf("expected missing scenario error, got %q", payload.Error)
	}
}

func TestCurrentPracticeSessionReturnsStoredSession(t *testing.T) {
	t.Parallel()

	practiceService := service.NewPracticeService(
		service.NewInMemoryPracticeSessionStore(),
		&stubRunnerClient{
			workspace: runner.Workspace{
				ID:       "ws-current",
				Path:     "/tmp/ws-current",
				Template: "standard",
			},
		},
		func() time.Time {
			return time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
		},
	)

	session, err := practiceService.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     42,
		ScenarioID: 1,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: practiceService,
		AuthStore:       authStoreWithSession("current-session-token", 42),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/practice-sessions/current", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "current-session-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Session struct {
			ID        uint64 `json:"id"`
			RunnerRef string `json:"runner_ref"`
			Status    string `json:"status"`
		} `json:"session"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal current session payload: %v", err)
	}
	if payload.Session.ID != session.ID {
		t.Fatalf("expected session ID %d, got %d", session.ID, payload.Session.ID)
	}
	if payload.Session.RunnerRef != "ws-current" {
		t.Fatalf("expected runner ref ws-current, got %q", payload.Session.RunnerRef)
	}
	if payload.Session.Status != "active" {
		t.Fatalf("expected active status, got %q", payload.Session.Status)
	}
}

func TestCurrentPracticeSessionSurvivesRouterRebuildWhenStoreIsPersistent(t *testing.T) {
	t.Parallel()

	persistentStore := newPersistentStubStore("persistent-session-token", 42)
	runnerClient := &stubRunnerClient{
		workspace: runner.Workspace{
			ID:       "ws-persistent",
			Path:     "/tmp/ws-persistent",
			Template: "standard",
		},
	}

	routerA := httpx.NewRouter(httpx.Dependencies{
		AuthStore:    persistentStore,
		RunnerClient: runnerClient,
	})

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/practice-sessions", strings.NewReader(`{"scenario_id":1}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "persistent-session-token"})
	createRec := httptest.NewRecorder()

	routerA.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating session, got %d with body %s", createRec.Code, createRec.Body.String())
	}

	routerB := httpx.NewRouter(httpx.Dependencies{
		AuthStore:    persistentStore,
		RunnerClient: runnerClient,
	})

	currentReq := httptest.NewRequest(http.MethodGet, "/api/v1/practice-sessions/current", nil)
	currentReq.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "persistent-session-token"})
	currentRec := httptest.NewRecorder()

	routerB.ServeHTTP(currentRec, currentReq)

	if currentRec.Code != http.StatusOK {
		t.Fatalf("expected rebuilt router to recover current session, got %d with body %s", currentRec.Code, currentRec.Body.String())
	}
}

func TestCurrentPracticeSessionMapsOrphanedStateToGone(t *testing.T) {
	t.Parallel()

	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: &stubPracticeService{
			currentPracticeSessionFunc: func(context.Context, uint64) (domain.PracticeSession, error) {
				return domain.PracticeSession{}, service.ErrPracticeSessionOrphaned
			},
		},
		AuthStore: authStoreWithSession("current-orphaned-token", 42),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/practice-sessions/current", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "current-orphaned-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func TestCreatePracticeSessionMapsErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		err    error
		status int
	}{
		{name: "bad input", err: service.ErrInvalidPracticeSessionInput, status: http.StatusBadRequest},
		{name: "unknown scenario", err: service.ErrUnknownPracticeScenario, status: http.StatusBadRequest},
		{name: "unknown template", err: service.ErrUnknownPracticeTemplate, status: http.StatusBadRequest},
		{name: "service configuration", err: service.ErrPracticeServiceConfiguration, status: http.StatusInternalServerError},
		{name: "runner creation failure", err: service.ErrRunnerWorkspaceCreation, status: http.StatusBadGateway},
		{name: "wrapped runner failure", err: errors.Join(service.ErrRunnerWorkspaceCreation, errors.New("dial tcp timeout")), status: http.StatusBadGateway},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := httpx.NewRouter(httpx.Dependencies{
				PracticeService: &stubPracticeService{
					createPracticeSessionFunc: func(context.Context, service.CreatePracticeSessionInput) (domain.PracticeSession, error) {
						return domain.PracticeSession{}, tc.err
					},
				},
				AuthStore: authStoreWithSession("create-error-token", 42),
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/practice-sessions", strings.NewReader(`{"scenario_id":1}`))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "create-error-token"})
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Fatalf("expected %d, got %d with body %s", tc.status, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestResetPracticeSessionUsesAuthenticatedUser(t *testing.T) {
	t.Parallel()

	recordingService := &stubPracticeService{}
	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: recordingService,
		AuthStore:       authStoreWithSession("reset-session-token", 42),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/practice-sessions/321/reset", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "reset-session-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d with body %s", rec.Code, rec.Body.String())
	}
	if recordingService.lastResetUserID != 42 {
		t.Fatalf("expected handler to use persisted authenticated user ID 42, got %d", recordingService.lastResetUserID)
	}
	if recordingService.lastResetSessionID != 321 {
		t.Fatalf("expected session ID 321, got %d", recordingService.lastResetSessionID)
	}
}

func TestResetPracticeSessionMapsErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		err    error
		status int
	}{
		{name: "bad input", err: service.ErrInvalidPracticeSessionInput, status: http.StatusBadRequest},
		{name: "not found", err: service.ErrPracticeSessionNotFound, status: http.StatusNotFound},
		{name: "expired", err: service.ErrPracticeSessionExpired, status: http.StatusGone},
		{name: "orphaned", err: service.ErrPracticeSessionOrphaned, status: http.StatusGone},
		{name: "service configuration", err: service.ErrPracticeServiceConfiguration, status: http.StatusInternalServerError},
		{name: "runner reset failure", err: service.ErrRunnerWorkspaceReset, status: http.StatusBadGateway},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := httpx.NewRouter(httpx.Dependencies{
				PracticeService: &stubPracticeService{
					resetPracticeSessionFunc: func(context.Context, uint64, uint64) error {
						return tc.err
					},
				},
				AuthStore: authStoreWithSession("reset-error-token", 42),
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/practice-sessions/123/reset", nil)
			req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "reset-error-token"})
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Fatalf("expected %d, got %d with body %s", tc.status, rec.Code, rec.Body.String())
			}
		})
	}
}

type stubPracticeService struct {
	createPracticeSessionFunc             func(context.Context, service.CreatePracticeSessionInput) (domain.PracticeSession, error)
	resetPracticeSessionFunc              func(context.Context, uint64, uint64) error
	currentPracticeSessionFunc            func(context.Context, uint64) (domain.PracticeSession, error)
	practiceSessionByIDFunc               func(context.Context, uint64, uint64) (domain.PracticeSession, error)
	practiceSessionRepoStateFunc          func(context.Context, uint64, uint64) (runner.RepoState, error)
	connectTerminalFunc                   func(context.Context, uint64, uint64) (runner.TerminalConnection, error)
	expireStaleSessionsFunc               func(context.Context) (int, error)
	reconcileCleanupJobsFunc              func(context.Context, int) (service.WorkspaceCleanupReconciliationSummary, error)
	listExhaustedWorkspaceCleanupJobsFunc func(context.Context, int) ([]domain.WorkspaceCleanupJob, error)
	listTemplatesResult                   []service.PracticeTemplate
	listScenariosResult                   []service.PracticeScenario
	listTemplatesErr                      error
	listScenariosErr                      error
	lastCreateInput                       service.CreatePracticeSessionInput
	lastResetUserID                       uint64
	lastResetSessionID                    uint64
}

func (s *stubPracticeService) ListTemplates(context.Context) []service.PracticeTemplate {
	if s.listTemplatesResult != nil {
		return append([]service.PracticeTemplate(nil), s.listTemplatesResult...)
	}
	return []service.PracticeTemplate{{ID: 1, Key: "standard", Name: "Standard"}}
}

func (s *stubPracticeService) ListScenarios(context.Context) []service.PracticeScenario {
	if s.listScenariosResult != nil {
		return append([]service.PracticeScenario(nil), s.listScenariosResult...)
	}
	return []service.PracticeScenario{{ID: 1, Key: "sandbox-standard", Name: "Standard Sandbox", TemplateID: 1}}
}

func (s *stubPracticeService) ListTemplatesWithError(_ context.Context) ([]service.PracticeTemplate, error) {
	if s.listTemplatesErr != nil {
		return nil, s.listTemplatesErr
	}
	return s.ListTemplates(context.Background()), nil
}

func (s *stubPracticeService) ListScenariosWithError(_ context.Context) ([]service.PracticeScenario, error) {
	if s.listScenariosErr != nil {
		return nil, s.listScenariosErr
	}
	return s.ListScenarios(context.Background()), nil
}

func (s *stubPracticeService) CreatePracticeSession(ctx context.Context, input service.CreatePracticeSessionInput) (domain.PracticeSession, error) {
	s.lastCreateInput = input
	if s.createPracticeSessionFunc != nil {
		return s.createPracticeSessionFunc(ctx, input)
	}
	templateID := input.TemplateID
	if templateID == 0 {
		templateID = 1
	}
	return domain.PracticeSession{
		ID:         1,
		UserID:     input.UserID,
		ScenarioID: input.ScenarioID,
		TemplateID: templateID,
		Status:     "active",
	}, nil
}

func (s *stubPracticeService) ResetPracticeSession(ctx context.Context, userID uint64, sessionID uint64) error {
	s.lastResetUserID = userID
	s.lastResetSessionID = sessionID
	if s.resetPracticeSessionFunc != nil {
		return s.resetPracticeSessionFunc(ctx, userID, sessionID)
	}
	return nil
}

func (s *stubPracticeService) CurrentPracticeSession(ctx context.Context, userID uint64) (domain.PracticeSession, error) {
	if s.currentPracticeSessionFunc != nil {
		return s.currentPracticeSessionFunc(ctx, userID)
	}
	return domain.PracticeSession{}, service.ErrPracticeSessionNotFound
}

func (s *stubPracticeService) PracticeSessionByID(ctx context.Context, userID uint64, sessionID uint64) (domain.PracticeSession, error) {
	if s.practiceSessionByIDFunc != nil {
		return s.practiceSessionByIDFunc(ctx, userID, sessionID)
	}
	return domain.PracticeSession{}, service.ErrPracticeSessionNotFound
}

func (s *stubPracticeService) PracticeSessionRepoState(ctx context.Context, userID uint64, sessionID uint64) (runner.RepoState, error) {
	if s.practiceSessionRepoStateFunc != nil {
		return s.practiceSessionRepoStateFunc(ctx, userID, sessionID)
	}
	return runner.RepoState{}, service.ErrPracticeSessionNotFound
}

func (s *stubPracticeService) ConnectTerminal(ctx context.Context, userID uint64, sessionID uint64) (runner.TerminalConnection, error) {
	if s.connectTerminalFunc != nil {
		return s.connectTerminalFunc(ctx, userID, sessionID)
	}
	return nil, service.ErrPracticeSessionNotFound
}

func (s *stubPracticeService) ExpireStalePracticeSessions(ctx context.Context) (int, error) {
	if s.expireStaleSessionsFunc != nil {
		return s.expireStaleSessionsFunc(ctx)
	}
	return 0, nil
}

func (s *stubPracticeService) ReconcileWorkspaceCleanupJobs(ctx context.Context, limit int) (service.WorkspaceCleanupReconciliationSummary, error) {
	if s.reconcileCleanupJobsFunc != nil {
		return s.reconcileCleanupJobsFunc(ctx, limit)
	}
	return service.WorkspaceCleanupReconciliationSummary{}, nil
}

func (s *stubPracticeService) ListExhaustedWorkspaceCleanupJobs(ctx context.Context, limit int) ([]domain.WorkspaceCleanupJob, error) {
	if s.listExhaustedWorkspaceCleanupJobsFunc != nil {
		return s.listExhaustedWorkspaceCleanupJobsFunc(ctx, limit)
	}
	return nil, nil
}

func authStoreWithSession(rawToken string, userID uint64) *stubUserStore {
	return &stubUserStore{
		sessionByTokenHash: map[string]domain.BrowserSession{
			service.HashSessionToken(rawToken): {
				ID:               1,
				UserID:           userID,
				SessionTokenHash: service.HashSessionToken(rawToken),
				ExpiresAt:        time.Now().Add(24 * time.Hour).UTC(),
			},
		},
	}
}

type persistentStubStore struct {
	*stubUserStore
	practiceSessions *service.InMemoryPracticeSessionStore
}

type catalogPersistentStubStore struct {
	*persistentStubStore
	templates    []service.PracticeTemplate
	scenarios    []service.PracticeScenario
	templatesErr error
	scenariosErr error
}

func newPersistentStubStore(rawToken string, userID uint64) *persistentStubStore {
	return &persistentStubStore{
		stubUserStore:    authStoreWithSession(rawToken, userID),
		practiceSessions: service.NewInMemoryPracticeSessionStore(),
	}
}

func (s *persistentStubStore) CreatePracticeSession(ctx context.Context, session domain.PracticeSession) (domain.PracticeSession, error) {
	return s.practiceSessions.CreatePracticeSession(ctx, session)
}

func (s *persistentStubStore) CurrentPracticeSession(ctx context.Context, userID uint64) (domain.PracticeSession, error) {
	return s.practiceSessions.CurrentPracticeSession(ctx, userID)
}

func (s *persistentStubStore) PracticeSessionByID(ctx context.Context, sessionID uint64) (domain.PracticeSession, error) {
	return s.practiceSessions.PracticeSessionByID(ctx, sessionID)
}

func (s *persistentStubStore) UpdatePracticeSession(ctx context.Context, session domain.PracticeSession) (domain.PracticeSession, error) {
	return s.practiceSessions.UpdatePracticeSession(ctx, session)
}

func (s *persistentStubStore) ExpirePracticeSessions(ctx context.Context, before time.Time, endedAt time.Time) ([]domain.PracticeSession, error) {
	return s.practiceSessions.ExpirePracticeSessions(ctx, before, endedAt)
}

func (s *persistentStubStore) UpsertWorkspaceCleanupJob(ctx context.Context, job domain.WorkspaceCleanupJob) error {
	return s.practiceSessions.UpsertWorkspaceCleanupJob(ctx, job)
}

func (s *persistentStubStore) ListPracticeSessionsMissingWorkspaceCleanupJob(ctx context.Context, limit int) ([]domain.PracticeSession, error) {
	return s.practiceSessions.ListPracticeSessionsMissingWorkspaceCleanupJob(ctx, limit)
}

func (s *persistentStubStore) ListExhaustedWorkspaceCleanupJobs(ctx context.Context, limit int) ([]domain.WorkspaceCleanupJob, error) {
	return s.practiceSessions.ListExhaustedWorkspaceCleanupJobs(ctx, limit)
}

func (s *catalogPersistentStubStore) ListPracticeTemplates(context.Context) ([]service.PracticeTemplate, error) {
	if s.templatesErr != nil {
		return nil, s.templatesErr
	}
	return append([]service.PracticeTemplate(nil), s.templates...), nil
}

func (s *catalogPersistentStubStore) ListPracticeScenarios(context.Context) ([]service.PracticeScenario, error) {
	if s.scenariosErr != nil {
		return nil, s.scenariosErr
	}
	return append([]service.PracticeScenario(nil), s.scenarios...), nil
}

type execOnlySQLDriver struct {
	rowsAffected int64
	execErr      error
}

func openExecOnlySQLDB(t *testing.T, driverImpl execOnlySQLDriver) *sql.DB {
	t.Helper()

	driverName := fmt.Sprintf("exec-only-%d", time.Now().UnixNano())
	sql.Register(driverName, driverImpl)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open test sql db: %v", err)
	}

	return db
}

func (d execOnlySQLDriver) Open(string) (driver.Conn, error) {
	return execOnlySQLConn{driver: d}, nil
}

type execOnlySQLConn struct {
	driver execOnlySQLDriver
}

func (c execOnlySQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not implemented")
}

func (c execOnlySQLConn) Close() error {
	return nil
}

func (c execOnlySQLConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions not implemented")
}

func (c execOnlySQLConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	if c.driver.execErr != nil {
		return nil, c.driver.execErr
	}

	return driver.RowsAffected(c.driver.rowsAffected), nil
}
