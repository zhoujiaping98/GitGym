package test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitgym/services/api/internal/domain"
	httpx "gitgym/services/api/internal/http"
	"gitgym/services/api/internal/runner"
	"gitgym/services/api/internal/service"
	"github.com/coder/websocket"
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
			{name: "current session", method: http.MethodGet, target: "/api/v1/practice-sessions/current", status: http.StatusNotFound},
			{name: "create session", method: http.MethodPost, target: "/api/v1/practice-sessions", body: []byte(`{"scenario_id":7,"template_id":1}`), status: http.StatusCreated},
			{name: "terminal route", method: http.MethodGet, target: "/api/v1/practice-sessions/123/terminal", status: http.StatusNotFound},
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

	t.Run("current session remains protected without cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/practice-sessions/current", nil)
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
	if recordingService.lastCreateInput.UserID != 1 {
		t.Fatalf("expected handler to use placeholder authenticated user ID 1, got %d", recordingService.lastCreateInput.UserID)
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
	if payload.Session.ID != 101 || payload.Session.UserID != 1 {
		t.Fatalf("unexpected session payload: %+v", payload.Session)
	}
	if payload.Session.RunnerRef != "ws-123" || payload.Session.WorkspacePath != "/tmp/ws-123" {
		t.Fatalf("unexpected runner payload: %+v", payload.Session)
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
		UserID:     1,
		ScenarioID: 7,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: practiceService,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/practice-sessions/current", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "session-token"})
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

func TestPracticeTerminalWebsocketSeedsAndEchoesSession(t *testing.T) {
	t.Parallel()

	practiceService := service.NewPracticeService(
		service.NewInMemoryPracticeSessionStore(),
		&stubRunnerClient{
			workspace: runner.Workspace{
				ID:       "ws-terminal",
				Path:     "/tmp/ws-terminal",
				Template: "standard",
			},
		},
		func() time.Time {
			return time.Date(2026, 5, 16, 13, 0, 0, 0, time.UTC)
		},
	)

	session, err := practiceService.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     1,
		ScenarioID: 11,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	server := httptest.NewServer(httpx.NewRouter(httpx.Dependencies{
		PracticeService: practiceService,
	}))
	defer server.Close()

	wsURL := fmt.Sprintf(
		"ws%s/api/v1/practice-sessions/%d/terminal",
		strings.TrimPrefix(server.URL, "http"),
		session.ID,
	)
	header := http.Header{}
	header.Add("Cookie", "gitgym_session=session-token")

	conn, _, err := websocket.Dial(context.Background(), wsURL, &websocket.DialOptions{
		HTTPHeader: header,
	})
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	_, seedLine, err := conn.Read(context.Background())
	if err != nil {
		t.Fatalf("read seed line: %v", err)
	}
	if !strings.Contains(string(seedLine), session.RunnerRef) {
		t.Fatalf("expected seed line to mention runner ref %q, got %q", session.RunnerRef, string(seedLine))
	}

	if err := conn.Write(context.Background(), websocket.MessageText, []byte("git status --short")); err != nil {
		t.Fatalf("write websocket payload: %v", err)
	}

	_, echoedLine, err := conn.Read(context.Background())
	if err != nil {
		t.Fatalf("read echoed line: %v", err)
	}
	if string(echoedLine) != "git status --short" {
		t.Fatalf("expected echoed line %q, got %q", "git status --short", string(echoedLine))
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
			req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "123"})
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Fatalf("expected %d, got %d with body %s", tc.status, rec.Code, rec.Body.String())
			}
		})
	}
}

type stubPracticeService struct {
	createPracticeSessionFunc  func(context.Context, service.CreatePracticeSessionInput) (domain.PracticeSession, error)
	currentPracticeSessionFunc func(context.Context, uint64) (domain.PracticeSession, error)
	practiceSessionByIDFunc    func(context.Context, uint64, uint64) (domain.PracticeSession, error)
	lastCreateInput            service.CreatePracticeSessionInput
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
