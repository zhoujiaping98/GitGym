package test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitgym/services/api/internal/domain"
	httpx "gitgym/services/api/internal/http"
	"gitgym/services/api/internal/service"
)

func TestPracticeRoutesMatchPlanSurface(t *testing.T) {
	t.Parallel()

	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: &stubPracticeService{},
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
			{name: "create session", method: http.MethodPost, target: "/api/v1/practice-sessions", body: []byte(`{"scenario_id":7,"template_id":1}`), status: http.StatusCreated},
			{name: "terminal placeholder", method: http.MethodGet, target: "/api/v1/practice-sessions/123/terminal", status: http.StatusNotImplemented},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(tc.method, tc.target, bytes.NewReader(tc.body))
				req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "session-token"})
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
		req := httptest.NewRequest(http.MethodPost, "/api/v1/practice-sessions", strings.NewReader(`{"scenario_id":7,"template_id":1}`))
		req.Header.Set("Content-Type", "application/json")
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
			req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "session-token"})
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("expected 404 for %s %s, got %d", tc.method, tc.target, rec.Code)
			}
		}
	})
}

func TestCreatePracticeSessionUsesAuthenticatedUserAndReturnsStableJSON(t *testing.T) {
	t.Parallel()

	recordingService := &stubPracticeService{
		createPracticeSessionFunc: func(_ context.Context, input service.CreatePracticeSessionInput) (domain.PracticeSession, error) {
			return domain.PracticeSession{
				ID:               101,
				UserID:           input.UserID,
				ScenarioID:       input.ScenarioID,
				TemplateID:       input.TemplateID,
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
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/practice-sessions", strings.NewReader(`{"user_id":999,"scenario_id":7,"template_id":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "uid:42:session-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if recordingService.lastCreateInput.UserID != 42 {
		t.Fatalf("expected handler to use authenticated user ID 42, got %d", recordingService.lastCreateInput.UserID)
	}
	if recordingService.lastCreateInput.ScenarioID != 7 {
		t.Fatalf("expected scenario ID 7, got %d", recordingService.lastCreateInput.ScenarioID)
	}
	if recordingService.lastCreateInput.TemplateID != 1 {
		t.Fatalf("expected template ID 1, got %d", recordingService.lastCreateInput.TemplateID)
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
	if payload.Session.RunnerRef != "ws-123" || payload.Session.WorkspacePath != "/tmp/ws-123" {
		t.Fatalf("unexpected runner payload: %+v", payload.Session)
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
		{name: "unknown template", err: service.ErrUnknownPracticeTemplate, status: http.StatusBadRequest},
		{name: "service configuration", err: service.ErrPracticeServiceConfiguration, status: http.StatusInternalServerError},
		{name: "runner failure", err: service.ErrRunnerWorkspaceCreation, status: http.StatusBadGateway},
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
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/practice-sessions", strings.NewReader(`{"scenario_id":7,"template_id":1}`))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "uid:42:session-token"})
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Fatalf("expected %d, got %d with body %s", tc.status, rec.Code, rec.Body.String())
			}
		})
	}
}

type stubPracticeService struct {
	createPracticeSessionFunc func(context.Context, service.CreatePracticeSessionInput) (domain.PracticeSession, error)
	lastCreateInput           service.CreatePracticeSessionInput
}

func (s *stubPracticeService) ListTemplates(context.Context) []service.PracticeTemplate {
	return []service.PracticeTemplate{{ID: 1, Key: "standard", Name: "Standard"}}
}

func (s *stubPracticeService) CreatePracticeSession(ctx context.Context, input service.CreatePracticeSessionInput) (domain.PracticeSession, error) {
	s.lastCreateInput = input
	if s.createPracticeSessionFunc != nil {
		return s.createPracticeSessionFunc(ctx, input)
	}
	return domain.PracticeSession{ID: 1, Status: "active"}, nil
}
