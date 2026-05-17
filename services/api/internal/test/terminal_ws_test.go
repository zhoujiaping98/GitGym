package test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitgym/services/api/internal/config"
	httpx "gitgym/services/api/internal/http"
	"gitgym/services/api/internal/runner"
	"gitgym/services/api/internal/service"
	"github.com/coder/websocket"
)

func TestPracticeTerminalWebSocketRejectsForeignSession(t *testing.T) {
	t.Parallel()

	practiceService := service.NewPracticeService(
		service.NewInMemoryPracticeSessionStore(),
		&stubRunnerClient{
			workspace: runner.Workspace{
				ID:       "ws-foreign",
				Path:     "/tmp/ws-foreign",
				Template: "standard",
			},
		},
		func() time.Time {
			return time.Date(2026, 5, 17, 8, 0, 0, 0, time.UTC)
		},
	)

	session, err := practiceService.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     99,
		ScenarioID: 7,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	apiServer := httptest.NewServer(httpx.NewRouter(httpx.Dependencies{
		PracticeService: practiceService,
		AuthStore:       authStoreWithSession("foreign-session-token", 42),
	}))
	defer apiServer.Close()

	_, resp, err := websocket.Dial(context.Background(), practiceTerminalURL(apiServer.URL, session.ID), &websocket.DialOptions{
		HTTPHeader: practiceTerminalHeaders("foreign-session-token"),
	})
	if err == nil {
		t.Fatal("expected websocket dial to fail for foreign session")
	}
	if resp == nil {
		t.Fatal("expected HTTP response for rejected websocket dial")
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for foreign session, got %d", resp.StatusCode)
	}
}

func TestPracticeTerminalWebSocketBridgesRunnerOutput(t *testing.T) {
	t.Parallel()

	runnerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/workspaces/ws-output/terminal" {
			http.NotFound(w, r)
			return
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		if err := conn.Write(r.Context(), websocket.MessageText, []byte("runner-output\r\n")); err != nil {
			t.Errorf("write runner output: %v", err)
		}
	}))
	defer runnerServer.Close()

	practiceService := service.NewPracticeService(
		service.NewInMemoryPracticeSessionStore(),
		&stubRunnerClient{
			workspace: runner.Workspace{
				ID:       "ws-output",
				Path:     "/tmp/ws-output",
				Template: "standard",
			},
		},
		func() time.Time {
			return time.Date(2026, 5, 17, 8, 5, 0, 0, time.UTC)
		},
	)

	session, err := practiceService.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     42,
		ScenarioID: 7,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	apiServer := httptest.NewServer(httpx.NewRouter(httpx.Dependencies{
		PracticeService: practiceService,
		AuthStore:       authStoreWithSession("runner-output-token", 42),
		AuthConfig: config.Config{
			RunnerBaseURL: runnerServer.URL,
		},
	}))
	defer apiServer.Close()

	conn, _, err := websocket.Dial(context.Background(), practiceTerminalURL(apiServer.URL, session.ID), &websocket.DialOptions{
		HTTPHeader: practiceTerminalHeaders("runner-output-token"),
	})
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	_, payload, err := conn.Read(context.Background())
	if err != nil {
		t.Fatalf("read bridged runner output: %v", err)
	}
	if string(payload) != "runner-output\r\n" {
		t.Fatalf("expected bridged runner output %q, got %q", "runner-output\r\n", string(payload))
	}
}

func TestPracticeTerminalWebSocketForwardsBrowserInput(t *testing.T) {
	t.Parallel()

	forwarded := make(chan string, 1)
	runnerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/workspaces/ws-input/terminal" {
			http.NotFound(w, r)
			return
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		_, payload, err := conn.Read(r.Context())
		if err != nil {
			t.Errorf("read forwarded browser input: %v", err)
			return
		}

		select {
		case forwarded <- string(payload):
		default:
		}
	}))
	defer runnerServer.Close()

	practiceService := service.NewPracticeService(
		service.NewInMemoryPracticeSessionStore(),
		&stubRunnerClient{
			workspace: runner.Workspace{
				ID:       "ws-input",
				Path:     "/tmp/ws-input",
				Template: "standard",
			},
		},
		func() time.Time {
			return time.Date(2026, 5, 17, 8, 10, 0, 0, time.UTC)
		},
	)

	session, err := practiceService.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     42,
		ScenarioID: 7,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	apiServer := httptest.NewServer(httpx.NewRouter(httpx.Dependencies{
		PracticeService: practiceService,
		AuthStore:       authStoreWithSession("browser-input-token", 42),
		AuthConfig: config.Config{
			RunnerBaseURL: runnerServer.URL,
		},
	}))
	defer apiServer.Close()

	conn, _, err := websocket.Dial(context.Background(), practiceTerminalURL(apiServer.URL, session.ID), &websocket.DialOptions{
		HTTPHeader: practiceTerminalHeaders("browser-input-token"),
	})
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	if err := conn.Write(context.Background(), websocket.MessageText, []byte("git status --short")); err != nil {
		t.Fatalf("write browser input: %v", err)
	}

	select {
	case payload := <-forwarded:
		if payload != "git status --short" {
			t.Fatalf("expected forwarded browser input %q, got %q", "git status --short", payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for runner to receive browser input")
	}
}

func practiceTerminalURL(serverURL string, sessionID uint64) string {
	return fmt.Sprintf("ws%s/api/v1/practice-sessions/%d/terminal", strings.TrimPrefix(serverURL, "http"), sessionID)
}

func practiceTerminalHeaders(token string) http.Header {
	header := http.Header{}
	header.Add("Cookie", "gitgym_session="+token)
	return header
}
