# Workspace Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add runner-owned workspace deletion with immediate expiry cleanup and delayed orphan cleanup, then trigger it from API lifecycle transitions.

**Architecture:** The runner owns filesystem deletion and terminal teardown through a small cleanup manager plus a new `DELETE /internal/workspaces/{workspaceID}` route. The API remains the lifecycle source of truth and only schedules cleanup by calling the runner client when sessions become `expired` or `orphaned`.

**Tech Stack:** Go, Chi HTTP handlers, `net/http`, `os.RemoveAll`, in-memory runner cleanup scheduling, Go test.

---

## File Map

- Modify: `services/runner/internal/http/router.go`
  - mount workspace delete route
- Create: `services/runner/internal/http/handlers/deletes.go`
  - delete handler and cleanup request payload decoding
- Modify: `services/runner/internal/http/handlers/workspace_paths_test.go`
  - if needed, share path-resolution expectations with delete route tests
- Create: `services/runner/internal/engine/workspace_cleanup.go`
  - cleanup manager, delayed scheduling, idempotent delete execution
- Modify: `services/runner/internal/engine/terminal_sessions.go`
  - expose the minimum release path the cleanup manager needs
- Create: `services/runner/internal/test/workspace_cleanup_test.go`
  - runner engine cleanup tests
- Create: `services/runner/internal/http/handlers/deletes_test.go`
  - runner HTTP delete route tests
- Modify: `services/api/internal/runner/client.go`
  - add `DeleteWorkspace`
- Modify: `services/api/internal/service/practice_service.go`
  - call runner cleanup from orphan/expiry transitions
- Modify: `services/api/internal/test/practice_service_test.go`
  - API-side cleanup scheduling tests
- Modify: `services/api/internal/test/practice_routes_test.go`
  - keep stubs implementing the expanded interfaces

### Task 1: Add the runner cleanup manager

**Files:**
- Create: `services/runner/internal/engine/workspace_cleanup.go`
- Modify: `services/runner/internal/engine/terminal_sessions.go`
- Test: `services/runner/internal/test/workspace_cleanup_test.go`

- [ ] **Step 1: Write the failing immediate-delete test**

```go
func TestCleanupManagerDeletesWorkspaceImmediately(t *testing.T) {
	t.Parallel()

	workRoot := t.TempDir()
	workspace, err := engine.CreateWorkspace(workRoot)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	manager := engine.NewTerminalManager()
	cleanup := engine.NewWorkspaceCleanupManager(manager, func() time.Time {
		return time.Date(2026, 5, 19, 16, 0, 0, 0, time.UTC)
	})

	if err := cleanup.Schedule(context.Background(), engine.WorkspaceCleanupRequest{
		WorkspaceID: workspace.ID,
		Path:        workspace.Path,
		Reason:      "expired",
		DeleteAfter: 0,
	}); err != nil {
		t.Fatalf("schedule cleanup: %v", err)
	}

	if _, err := os.Stat(workspace.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected workspace to be deleted, got %v", err)
	}
}
```

- [ ] **Step 2: Run the runner cleanup test to verify it fails**

Run: `go test ./internal/test -run "TestCleanupManagerDeletesWorkspaceImmediately" -v`

Expected: FAIL because `NewWorkspaceCleanupManager`, `WorkspaceCleanupRequest`, or `Schedule` do not exist yet.

- [ ] **Step 3: Write the failing delayed-delete test**

```go
func TestCleanupManagerDeletesWorkspaceAfterDelay(t *testing.T) {
	t.Parallel()

	workRoot := t.TempDir()
	workspace, err := engine.CreateWorkspace(workRoot)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	manager := engine.NewTerminalManager()
	cleanup := engine.NewWorkspaceCleanupManager(manager, time.Now)

	if err := cleanup.Schedule(context.Background(), engine.WorkspaceCleanupRequest{
		WorkspaceID: workspace.ID,
		Path:        workspace.Path,
		Reason:      "orphaned",
		DeleteAfter: 20 * time.Millisecond,
	}); err != nil {
		t.Fatalf("schedule delayed cleanup: %v", err)
	}

	if _, err := os.Stat(workspace.Path); err != nil {
		t.Fatalf("expected workspace to still exist before grace period, got %v", err)
	}

	time.Sleep(60 * time.Millisecond)

	if _, err := os.Stat(workspace.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected workspace to be deleted after grace period, got %v", err)
	}
}
```

- [ ] **Step 4: Run the delayed cleanup test to verify it fails**

Run: `go test ./internal/test -run "TestCleanupManagerDeletesWorkspaceAfterDelay" -v`

Expected: FAIL because delayed scheduling is not implemented.

- [ ] **Step 5: Write the failing active-terminal cleanup test**

```go
func TestCleanupManagerReleasesTerminalBeforeDeletingWorkspace(t *testing.T) {
	t.Parallel()

	workspace := createGitWorkspace(t)
	terminalManager := engine.NewTerminalManager()

	session, err := terminalManager.Acquire(context.Background(), workspace.Path, workspace.ID)
	if err != nil {
		t.Fatalf("acquire terminal session: %v", err)
	}
	defer func() {
		_ = session.Wait()
	}()

	cleanup := engine.NewWorkspaceCleanupManager(terminalManager, time.Now)
	if err := cleanup.Schedule(context.Background(), engine.WorkspaceCleanupRequest{
		WorkspaceID: workspace.ID,
		Path:        workspace.Path,
		Reason:      "expired",
		DeleteAfter: 0,
	}); err != nil {
		t.Fatalf("schedule cleanup: %v", err)
	}

	if _, err := terminalManager.Acquire(context.Background(), workspace.Path, workspace.ID); err == nil {
		t.Fatal("expected acquire to fail because workspace should be gone")
	}
}
```

- [ ] **Step 6: Run the terminal-release cleanup test to verify it fails**

Run: `go test ./internal/test -run "TestCleanupManagerReleasesTerminalBeforeDeletingWorkspace" -v`

Expected: FAIL because cleanup does not yet coordinate with `TerminalManager.Release`.

- [ ] **Step 7: Implement the cleanup manager with the minimum surface**

```go
type WorkspaceCleanupRequest struct {
	WorkspaceID string
	Path        string
	Reason      string
	DeleteAfter time.Duration
}

type WorkspaceCleanupManager struct {
	mu       sync.Mutex
	terminal *TerminalManager
	now      func() time.Time
	pending  map[string]context.CancelFunc
}

func NewWorkspaceCleanupManager(terminal *TerminalManager, now func() time.Time) *WorkspaceCleanupManager {
	if now == nil {
		now = time.Now
	}
	return &WorkspaceCleanupManager{
		terminal: terminal,
		now:      now,
		pending:  make(map[string]context.CancelFunc),
	}
}
```

```go
func (m *WorkspaceCleanupManager) Schedule(ctx context.Context, req WorkspaceCleanupRequest) error {
	if strings.TrimSpace(req.WorkspaceID) == "" {
		return errors.New("workspace cleanup requires workspace ID")
	}
	if strings.TrimSpace(req.Path) == "" {
		return errors.New("workspace cleanup requires workspace path")
	}

	if req.DeleteAfter <= 0 {
		return m.deleteNow(req)
	}

	m.mu.Lock()
	if cancel, ok := m.pending[req.WorkspaceID]; ok {
		cancel()
	}
	timerCtx, cancel := context.WithCancel(context.Background())
	m.pending[req.WorkspaceID] = cancel
	m.mu.Unlock()

	go func() {
		timer := time.NewTimer(req.DeleteAfter)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timerCtx.Done():
			return
		case <-timer.C:
			_ = m.deleteNow(req)
			m.mu.Lock()
			delete(m.pending, req.WorkspaceID)
			m.mu.Unlock()
		}
	}()

	return nil
}
```

```go
func (m *WorkspaceCleanupManager) deleteNow(req WorkspaceCleanupRequest) error {
	if m.terminal != nil {
		if err := m.terminal.Release(req.WorkspaceID); err != nil {
			return err
		}
	}

	if err := os.RemoveAll(req.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
```

- [ ] **Step 8: Run the focused runner cleanup tests and verify they pass**

Run: `go test ./internal/test -run "TestCleanupManagerDeletesWorkspaceImmediately|TestCleanupManagerDeletesWorkspaceAfterDelay|TestCleanupManagerReleasesTerminalBeforeDeletingWorkspace" -v`

Expected: PASS

- [ ] **Step 9: Commit the cleanup manager slice**

```bash
git add services/runner/internal/engine/workspace_cleanup.go services/runner/internal/engine/terminal_sessions.go services/runner/internal/test/workspace_cleanup_test.go
git commit -m "feat: add runner workspace cleanup manager"
```

### Task 2: Add the runner delete route

**Files:**
- Create: `services/runner/internal/http/handlers/deletes.go`
- Create: `services/runner/internal/http/handlers/deletes_test.go`
- Modify: `services/runner/internal/http/router.go`

- [ ] **Step 1: Write the failing delete-route test**

```go
func TestDeleteWorkspaceSchedulesImmediateCleanup(t *testing.T) {
	t.Parallel()

	workRoot := t.TempDir()
	workspace, err := engine.CreateWorkspace(workRoot)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	router := chi.NewRouter()
	router.Delete("/internal/workspaces/{workspaceID}", handlers.DeleteWorkspace(workRoot, engine.NewTerminalManager(), engine.NewWorkspaceCleanupManager(engine.NewTerminalManager(), time.Now)))

	req := httptest.NewRequest(http.MethodDelete, "/internal/workspaces/"+workspace.ID, strings.NewReader(`{"reason":"expired","delete_after_seconds":0}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted && rec.Code != http.StatusNoContent {
		t.Fatalf("expected 202 or 204, got %d with body %s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run the delete-route test to verify it fails**

Run: `go test ./internal/http/handlers -run "TestDeleteWorkspaceSchedulesImmediateCleanup" -v`

Expected: FAIL because `DeleteWorkspace` is not implemented or not mounted.

- [ ] **Step 3: Write the failing missing-workspace idempotency test**

```go
func TestDeleteWorkspaceTreatsMissingWorkspaceAsIdempotent(t *testing.T) {
	t.Parallel()

	router := chi.NewRouter()
	router.Delete("/internal/workspaces/{workspaceID}", handlers.DeleteWorkspace(t.TempDir(), engine.NewTerminalManager(), engine.NewWorkspaceCleanupManager(engine.NewTerminalManager(), time.Now)))

	req := httptest.NewRequest(http.MethodDelete, "/internal/workspaces/ws-missing", strings.NewReader(`{"reason":"expired","delete_after_seconds":0}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound && rec.Code != http.StatusAccepted {
		t.Fatalf("expected idempotent cleanup response, got %d with body %s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 4: Run the idempotency test to verify it fails**

Run: `go test ./internal/http/handlers -run "TestDeleteWorkspaceTreatsMissingWorkspaceAsIdempotent" -v`

Expected: FAIL because the route does not exist yet.

- [ ] **Step 5: Implement the delete handler**

```go
type deleteWorkspaceRequest struct {
	Reason             string `json:"reason"`
	DeleteAfterSeconds int    `json:"delete_after_seconds"`
}

func DeleteWorkspace(workRoot string, terminalManager *engine.TerminalManager, cleanup *engine.WorkspaceCleanupManager) http.HandlerFunc {
	if cleanup == nil {
		cleanup = engine.NewWorkspaceCleanupManager(terminalManager, time.Now)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := chi.URLParam(r, "workspaceID")
		workspacePath, err := resolveWorkspacePath(workRoot, workspaceID)
		if err != nil {
			writeWorkspaceError(w, err)
			return
		}

		var req deleteWorkspaceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeWorkspaceJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
			return
		}

		deleteAfter := time.Duration(req.DeleteAfterSeconds) * time.Second
		if err := cleanup.Schedule(r.Context(), engine.WorkspaceCleanupRequest{
			WorkspaceID: workspaceID,
			Path:        workspacePath,
			Reason:      req.Reason,
			DeleteAfter: deleteAfter,
		}); err != nil {
			writeWorkspaceJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}

		writeWorkspaceJSON(w, http.StatusAccepted, map[string]any{
			"workspace_id": workspaceID,
			"status":       "cleanup_scheduled",
		})
	}
}
```

- [ ] **Step 6: Mount the route in the runner router**

```go
func NewRouter(workRoot string) http.Handler {
	r := chi.NewRouter()
	terminalManager := engine.NewTerminalManager()
	cleanupManager := engine.NewWorkspaceCleanupManager(terminalManager, time.Now)
	r.Get("/healthz", handlers.Health())
	r.Post("/internal/workspaces", handlers.CreateWorkspace(workRoot))
	r.Post("/internal/workspaces/{workspaceID}/commands", handlers.RunCommand(workRoot))
	r.Post("/internal/workspaces/{workspaceID}/reset", handlers.ResetWorkspace(workRoot))
	r.Delete("/internal/workspaces/{workspaceID}", handlers.DeleteWorkspace(workRoot, terminalManager, cleanupManager))
	r.Get("/internal/workspaces/{workspaceID}/terminal", handlers.TerminalWebSocket(workRoot, terminalManager))
	return r
}
```

- [ ] **Step 7: Run the focused runner handler tests and verify they pass**

Run: `go test ./internal/http/handlers -run "TestDeleteWorkspaceSchedulesImmediateCleanup|TestDeleteWorkspaceTreatsMissingWorkspaceAsIdempotent" -v`

Expected: PASS

- [ ] **Step 8: Commit the delete-route slice**

```bash
git add services/runner/internal/http/router.go services/runner/internal/http/handlers/deletes.go services/runner/internal/http/handlers/deletes_test.go
git commit -m "feat: add runner workspace delete route"
```

### Task 3: Extend the API runner client with workspace delete support

**Files:**
- Modify: `services/api/internal/runner/client.go`
- Test: `services/api/internal/test/practice_service_test.go`

- [ ] **Step 1: Write the failing API runner-client cleanup test**

```go
func TestPracticeServiceSchedulesImmediateCleanupForExpiredSessions(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 19, 17, 0, 0, 0, time.UTC)
	store := &stubPracticeSessionStore{
		expirableSessions: []domain.PracticeSession{
			{
				ID:               501,
				UserID:           42,
				ScenarioID:       1,
				TemplateID:       1,
				RunnerRef:        "ws-cleanup-expired",
				WorkspacePathRef: "/tmp/ws-cleanup-expired",
				Status:           "active",
				StartedAt:        now.Add(-3 * time.Hour),
				ExpiresAt:        now.Add(-time.Minute),
				LastActivityAt:   now.Add(-2 * time.Minute),
			},
		},
	}
	runnerClient := &stubRunnerClient{}
	svc := service.NewPracticeService(store, runnerClient, func() time.Time { return now })

	expiredCount, err := svc.ExpireStalePracticeSessions(context.Background())
	if err != nil {
		t.Fatalf("expire stale practice sessions: %v", err)
	}

	if expiredCount != 1 {
		t.Fatalf("expected one expired session, got %d", expiredCount)
	}
	if runnerClient.deleteWorkspaceCalls != 1 {
		t.Fatalf("expected one cleanup request, got %d", runnerClient.deleteWorkspaceCalls)
	}
	if runnerClient.lastDeleteWorkspaceID != "ws-cleanup-expired" {
		t.Fatalf("expected cleanup for ws-cleanup-expired, got %q", runnerClient.lastDeleteWorkspaceID)
	}
	if runnerClient.lastDeleteReason != "expired" {
		t.Fatalf("expected expired cleanup reason, got %q", runnerClient.lastDeleteReason)
	}
	if runnerClient.lastDeleteDelay != 0 {
		t.Fatalf("expected immediate cleanup delay, got %v", runnerClient.lastDeleteDelay)
	}
}
```

- [ ] **Step 2: Run the focused service test to verify it fails**

Run: `go test ./internal/test -run "TestPracticeServiceSchedulesImmediateCleanupForExpiredSessions" -v`

Expected: FAIL because the runner client interface has no delete method and the service does not schedule cleanup yet.

- [ ] **Step 3: Extend the runner client interface and HTTP implementation**

```go
type Client interface {
	CreateWorkspace(ctx context.Context, template string) (Workspace, error)
	ResetWorkspace(ctx context.Context, workspaceID string) error
	ConnectTerminal(ctx context.Context, workspaceID string) (TerminalConnection, error)
	DeleteWorkspace(ctx context.Context, workspaceID string, reason string, deleteAfter time.Duration) error
}
```

```go
func (c *HTTPClient) DeleteWorkspace(ctx context.Context, workspaceID string, reason string, deleteAfter time.Duration) error {
	if c.baseURL == "" {
		return ErrClientNotConfigured
	}
	if strings.TrimSpace(workspaceID) == "" {
		return fmt.Errorf("workspace ID is required")
	}

	payload := struct {
		Reason             string `json:"reason"`
		DeleteAfterSeconds int    `json:"delete_after_seconds"`
	}{
		Reason:             reason,
		DeleteAfterSeconds: int(deleteAfter / time.Second),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal delete workspace request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/internal/workspaces/"+url.PathEscape(workspaceID), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build delete workspace request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete runner workspace: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrWorkspaceNotFound
	}
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("runner delete workspace returned status %d", resp.StatusCode)
	}
	return nil
}
```

- [ ] **Step 4: Run the focused service test again and verify it still fails for the right reason**

Run: `go test ./internal/test -run "TestPracticeServiceSchedulesImmediateCleanupForExpiredSessions" -v`

Expected: FAIL because `PracticeService` still does not call `DeleteWorkspace`.

- [ ] **Step 5: Commit the API runner-client slice**

```bash
git add services/api/internal/runner/client.go services/api/internal/test/practice_service_test.go
git commit -m "feat: add api runner workspace delete client"
```

### Task 4: Trigger cleanup from API lifecycle transitions

**Files:**
- Modify: `services/api/internal/service/practice_service.go`
- Modify: `services/api/internal/test/practice_service_test.go`
- Modify: `services/api/internal/test/practice_routes_test.go`

- [ ] **Step 1: Write the failing orphan-cleanup scheduling test**

```go
func TestPracticeServiceSchedulesDelayedCleanupForOrphanedSessions(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 19, 17, 15, 0, 0, time.UTC)
	store := &stubPracticeSessionStore{}
	runnerClient := &stubRunnerClient{
		workspace: runner.Workspace{
			ID:       "ws-cleanup-orphaned",
			Path:     "/tmp/ws-cleanup-orphaned",
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

	if _, err := svc.ConnectTerminal(context.Background(), 42, created.ID); !errors.Is(err, service.ErrPracticeSessionOrphaned) {
		t.Fatalf("expected orphaned session error, got %v", err)
	}

	if runnerClient.deleteWorkspaceCalls != 1 {
		t.Fatalf("expected one delayed cleanup request, got %d", runnerClient.deleteWorkspaceCalls)
	}
	if runnerClient.lastDeleteWorkspaceID != "ws-cleanup-orphaned" {
		t.Fatalf("expected orphaned cleanup for ws-cleanup-orphaned, got %q", runnerClient.lastDeleteWorkspaceID)
	}
	if runnerClient.lastDeleteReason != "orphaned" {
		t.Fatalf("expected orphaned cleanup reason, got %q", runnerClient.lastDeleteReason)
	}
	if runnerClient.lastDeleteDelay != 10*time.Minute {
		t.Fatalf("expected 10 minute orphan cleanup delay, got %v", runnerClient.lastDeleteDelay)
	}
}
```

- [ ] **Step 2: Run the orphan scheduling test to verify it fails**

Run: `go test ./internal/test -run "TestPracticeServiceSchedulesDelayedCleanupForOrphanedSessions" -v`

Expected: FAIL because the orphan transition does not yet request runner cleanup.

- [ ] **Step 3: Implement cleanup scheduling constants and helper**

```go
const (
	practiceSessionTTL                 = 2 * time.Hour
	practiceSessionOrphanCleanupGrace = 10 * time.Minute
)

func (s *practiceService) scheduleWorkspaceCleanup(ctx context.Context, session domain.PracticeSession, reason string, deleteAfter time.Duration) error {
	if s.runner == nil {
		return fmt.Errorf("%w: runner client is not configured", ErrPracticeServiceConfiguration)
	}

	if err := s.runner.DeleteWorkspace(ctx, session.RunnerRef, reason, deleteAfter); err != nil && !errors.Is(err, runner.ErrWorkspaceNotFound) {
		return err
	}
	return nil
}
```

- [ ] **Step 4: Call cleanup after expiry sweep and orphan transitions**

```go
func (s *practiceService) ExpireStalePracticeSessions(ctx context.Context) (int, error) {
	now := s.now().UTC()
	expiredSessions, err := s.store.ExpirePracticeSessions(ctx, now, now)
	if err != nil {
		return 0, fmt.Errorf("expire stale practice sessions: %w", err)
	}

	for _, session := range expiredSessions {
		if err := s.scheduleWorkspaceCleanup(ctx, session, PracticeSessionStatusExpired, 0); err != nil {
			// log-like best effort: keep lifecycle transition, skip rollback
		}
	}

	return len(expiredSessions), nil
}
```

```go
if errors.Is(err, runner.ErrWorkspaceNotFound) {
	if _, transitionErr := s.transitionSession(ctx, session, PracticeSessionStatusOrphaned); transitionErr != nil {
		return nil, transitionErr
	}
	_ = s.scheduleWorkspaceCleanup(ctx, session, PracticeSessionStatusOrphaned, practiceSessionOrphanCleanupGrace)
	return nil, fmt.Errorf("%w", ErrPracticeSessionOrphaned)
}
```

- [ ] **Step 5: Extend the service test stub runner**

```go
type stubRunnerClient struct {
	createWorkspaceCalls int
	lastTemplate         string
	resetWorkspaceCalls  int
	lastResetWorkspaceID string
	deleteWorkspaceCalls int
	lastDeleteWorkspaceID string
	lastDeleteReason      string
	lastDeleteDelay       time.Duration
	workspace            runner.Workspace
	connectTerminalFunc  func(context.Context, string) (runner.TerminalConnection, error)
	err                  error
	resetErr             error
	connectErr           error
	deleteErr            error
}

func (s *stubRunnerClient) DeleteWorkspace(_ context.Context, workspaceID string, reason string, deleteAfter time.Duration) error {
	s.deleteWorkspaceCalls++
	s.lastDeleteWorkspaceID = workspaceID
	s.lastDeleteReason = reason
	s.lastDeleteDelay = deleteAfter
	return s.deleteErr
}
```

- [ ] **Step 6: Run the focused service tests and verify they pass**

Run: `go test ./internal/test -run "TestPracticeServiceSchedulesImmediateCleanupForExpiredSessions|TestPracticeServiceSchedulesDelayedCleanupForOrphanedSessions|TestPracticeServiceMarksMissingRunnerWorkspaceOnTerminalConnect|TestPracticeServiceExpiresStaleSessionsInSweep" -v`

Expected: PASS

- [ ] **Step 7: Commit the lifecycle-trigger slice**

```bash
git add services/api/internal/service/practice_service.go services/api/internal/test/practice_service_test.go services/api/internal/test/practice_routes_test.go
git commit -m "feat: schedule workspace cleanup from session lifecycle transitions"
```

### Task 5: Verify runner and API end to end

**Files:**
- None

- [ ] **Step 1: Run the runner test suite**

Run: `go test ./...`

Workdir: `services/runner`

Expected: PASS

- [ ] **Step 2: Run the API test suite**

Run: `go test ./...`

Workdir: `services/api`

Expected: PASS

- [ ] **Step 3: Report remaining gaps explicitly**

Use this close-out checklist:

```text
- delayed cleanup is in-memory only
- cleanup retries are best-effort only
- no cleanup audit table exists yet
```

- [ ] **Step 4: Commit any final verification-only doc touchups if needed**

```bash
git status --short
```

Expected: clean worktree or only intentional files from the plan.
