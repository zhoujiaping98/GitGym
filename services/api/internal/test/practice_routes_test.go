package test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitgym/services/api/internal/domain"
	httpx "gitgym/services/api/internal/http"
	"gitgym/services/api/internal/service"
)

func TestPracticeRoutesMatchPlanSurface(t *testing.T) {
	t.Parallel()

	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: stubPracticeService{},
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
			{name: "create session", method: http.MethodPost, target: "/api/v1/practice-sessions", body: []byte(`{"user_id":1,"scenario_id":7,"template_id":1}`), status: http.StatusCreated},
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

type stubPracticeService struct{}

func (stubPracticeService) ListTemplates(context.Context) []service.PracticeTemplate {
	return []service.PracticeTemplate{{ID: 1, Key: "standard", Name: "Standard"}}
}

func (stubPracticeService) CreatePracticeSession(context.Context, service.CreatePracticeSessionInput) (domain.PracticeSession, error) {
	return domain.PracticeSession{ID: 1, Status: "active"}, nil
}
