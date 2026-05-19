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
		ScenarioID: 1,
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

	ctx, cancel := testTimeoutContext(t)
	defer cancel()

	_, resp, err := websocket.Dial(ctx, practiceTerminalURL(apiServer.URL, session.ID), &websocket.DialOptions{
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
		ScenarioID: 1,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	apiServer := httptest.NewServer(httpx.NewRouter(httpx.Dependencies{
		PracticeService: practiceService,
		AuthStore:       authStoreWithSession("runner-output-token", 42),
		RunnerClient:    runner.NewClient(runnerServer.URL, http.DefaultClient),
	}))
	defer apiServer.Close()

	conn := dialPracticeTerminal(t, apiServer.URL, session.ID, "runner-output-token")
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := testTimeoutContext(t)
	defer cancel()

	_, payload, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read bridged runner output: %v", err)
	}
	if string(payload) != "runner-output\r\n" {
		t.Fatalf("expected bridged runner output %q, got %q", "runner-output\r\n", string(payload))
	}
}

func TestPracticeTerminalWebSocketAllowsConfiguredFrontendOrigin(t *testing.T) {
	t.Parallel()

	runnerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/workspaces/ws-origin/terminal" {
			http.NotFound(w, r)
			return
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		if err := conn.Write(r.Context(), websocket.MessageText, []byte("origin-ok\r\n")); err != nil {
			t.Errorf("write runner output: %v", err)
		}
	}))
	defer runnerServer.Close()

	practiceService := service.NewPracticeService(
		service.NewInMemoryPracticeSessionStore(),
		&stubRunnerClient{
			workspace: runner.Workspace{
				ID:       "ws-origin",
				Path:     "/tmp/ws-origin",
				Template: "standard",
			},
		},
		func() time.Time {
			return time.Date(2026, 5, 19, 8, 20, 0, 0, time.UTC)
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

	apiServer := httptest.NewServer(httpx.NewRouter(httpx.Dependencies{
		PracticeService: practiceService,
		AuthStore:       authStoreWithSession("origin-token", 42),
		RunnerClient:    runner.NewClient(runnerServer.URL, http.DefaultClient),
		AuthConfig: config.Config{
			FrontendRedirectURL: "http://127.0.0.1:5173",
		},
	}))
	defer apiServer.Close()

	headers := practiceTerminalHeaders("origin-token")
	headers.Set("Origin", "http://127.0.0.1:5173")

	ctx, cancel := testTimeoutContext(t)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, practiceTerminalURL(apiServer.URL, session.ID), &websocket.DialOptions{
		HTTPHeader: headers,
	})
	if err != nil {
		t.Fatalf("dial websocket with configured frontend origin: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	_, payload, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read bridged runner output: %v", err)
	}
	if string(payload) != "origin-ok\r\n" {
		t.Fatalf("expected bridged runner output %q, got %q", "origin-ok\r\n", string(payload))
	}
}

func TestPracticeTerminalWebSocketBridgesRunnerCommandCompletionFrames(t *testing.T) {
	t.Parallel()

	runnerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/workspaces/ws-completion/terminal" {
			http.NotFound(w, r)
			return
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		for _, payload := range []string{
			`{"type":"status","phase":"running","detail":"git status"}`,
			`{"type":"exit","exitCode":0}`,
		} {
			if err := conn.Write(r.Context(), websocket.MessageText, []byte(payload)); err != nil {
				t.Errorf("write runner completion frame %q: %v", payload, err)
				return
			}
		}
	}))
	defer runnerServer.Close()

	practiceService := service.NewPracticeService(
		service.NewInMemoryPracticeSessionStore(),
		&stubRunnerClient{
			workspace: runner.Workspace{
				ID:       "ws-completion",
				Path:     "/tmp/ws-completion",
				Template: "standard",
			},
		},
		func() time.Time {
			return time.Date(2026, 5, 17, 8, 7, 0, 0, time.UTC)
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

	apiServer := httptest.NewServer(httpx.NewRouter(httpx.Dependencies{
		PracticeService: practiceService,
		AuthStore:       authStoreWithSession("runner-completion-token", 42),
		RunnerClient:    runner.NewClient(runnerServer.URL, http.DefaultClient),
	}))
	defer apiServer.Close()

	conn := dialPracticeTerminal(t, apiServer.URL, session.ID, "runner-completion-token")
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := testTimeoutContext(t)
	defer cancel()

	for _, want := range []string{
		`{"type":"status","phase":"running","detail":"git status"}`,
		`{"type":"exit","exitCode":0}`,
	} {
		_, payload, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read bridged runner completion frame %q: %v", want, err)
		}
		if string(payload) != want {
			t.Fatalf("expected bridged payload %q, got %q", want, string(payload))
		}
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
		ScenarioID: 1,
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	apiServer := httptest.NewServer(httpx.NewRouter(httpx.Dependencies{
		PracticeService: practiceService,
		AuthStore:       authStoreWithSession("browser-input-token", 42),
		RunnerClient:    runner.NewClient(runnerServer.URL, http.DefaultClient),
	}))
	defer apiServer.Close()

	conn := dialPracticeTerminal(t, apiServer.URL, session.ID, "browser-input-token")
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := testTimeoutContext(t)
	defer cancel()

	if err := conn.Write(ctx, websocket.MessageText, []byte("git status --short")); err != nil {
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

func TestPracticeTerminalWebSocketPreservesRunnerCloseStatus(t *testing.T) {
	t.Parallel()

	practiceService := service.NewPracticeService(
		service.NewInMemoryPracticeSessionStore(),
		&stubRunnerClient{
			workspace: runner.Workspace{
				ID:       "ws-close",
				Path:     "/tmp/ws-close",
				Template: "standard",
			},
		},
		func() time.Time {
			return time.Date(2026, 5, 17, 8, 15, 0, 0, time.UTC)
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

	apiServer := httptest.NewServer(httpx.NewRouter(httpx.Dependencies{
		PracticeService: practiceService,
		AuthStore:       authStoreWithSession("runner-close-token", 42),
		RunnerClient: &stubRunnerClient{
			connectTerminalFunc: func(context.Context, string) (runner.TerminalConnection, error) {
				return terminalConnectionStub{
					readErr: websocket.CloseError{
						Code:   websocket.StatusPolicyViolation,
						Reason: "runner refused",
					},
				}, nil
			},
		},
	}))
	defer apiServer.Close()

	conn := dialPracticeTerminal(t, apiServer.URL, session.ID, "runner-close-token")
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := testTimeoutContext(t)
	defer cancel()

	_, _, err = conn.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("expected browser close status %d, got err %v with status %d", websocket.StatusPolicyViolation, err, websocket.CloseStatus(err))
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

func dialPracticeTerminal(t *testing.T, serverURL string, sessionID uint64, token string) *websocket.Conn {
	t.Helper()

	ctx, cancel := testTimeoutContext(t)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, practiceTerminalURL(serverURL, sessionID), &websocket.DialOptions{
		HTTPHeader: practiceTerminalHeaders(token),
	})
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}

	return conn
}

func testTimeoutContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 2*time.Second)
}

type terminalConnectionStub struct {
	readType int
	readData []byte
	readErr  error
	writeErr error
	closeErr error
}

func (s terminalConnectionStub) Read(context.Context) (int, []byte, error) {
	return s.readType, s.readData, s.readErr
}

func (s terminalConnectionStub) Write(context.Context, int, []byte) error {
	return s.writeErr
}

func (s terminalConnectionStub) Close(websocket.StatusCode, string) error {
	return s.closeErr
}
