# Repo Snapshot Data Chain Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add live repo snapshot data for practice sessions so the right-side workbench card can show branch, head commit, working-tree cleanliness, and changed files, including refresh after terminal commands complete.

**Architecture:** Add a dedicated runner repo-state endpoint and surface it through a small API route scoped to a practice session. In the web app, introduce a focused repo-state hook and keep `RepoPanel` presentational, with inline stale/unavailable degradation instead of page-shell changes.

**Tech Stack:** Go HTTP handlers and tests, React 19, TypeScript, Vitest, Playwright

---

## File Structure

### Existing files to modify

- `services/runner/internal/http/router.go`
  - mount the new internal repo-state endpoint
- `services/runner/internal/runner/client.go`
  - add repo-state transport support to the runner API client used by `services/api`
- `services/api/internal/http/router.go`
  - mount the authenticated practice-session repo-state route
- `services/api/internal/test/practice_routes_test.go`
  - add route-surface, success, auth, and failure coverage
- `apps/web/src/types.ts`
  - add repo-state and repo-state-view types
- `apps/web/src/lib/api.ts`
  - add a repo-state fetch helper and response mapping
- `apps/web/src/App.tsx`
  - wire the active session, terminal history, and repo-state hook into the workbench
- `apps/web/src/components/Workbench.tsx`
  - thread repo-state props into `RepoPanel`
- `apps/web/src/components/RepoPanel.tsx`
  - extend the operational card with live repo facts and inline unavailable/stale states
- `apps/web/src/styles.css`
  - style the repo snapshot section and dirty-file list
- `apps/web/src/test/App.test.tsx`
  - cover initial repo-state load, dirty display, unavailable fallback, and stale refresh behavior
- `apps/web/tests/e2e/smoke.spec.ts`
  - add one repo-snapshot smoke flow

### New files to create

- `services/runner/internal/http/handlers/repo_state.go`
  - expose workspace snapshot data from the runner
- `services/runner/internal/http/handlers/repo_state_test.go`
  - verify runner repo-state success and missing-workspace behavior
- `services/api/internal/http/handlers/repo_state.go`
  - resolve the authenticated session and map runner repo-state into public JSON
- `apps/web/src/hooks/useRepoState.ts`
  - own repo-state loading, stale fallback, and command-finished refresh triggers

---

### Task 1: Add a Runner Repo-State Endpoint and Client Support

**Files:**
- Create: `services/runner/internal/http/handlers/repo_state.go`
- Create: `services/runner/internal/http/handlers/repo_state_test.go`
- Modify: `services/runner/internal/http/router.go`
- Modify: `services/api/internal/runner/client.go`

- [ ] **Step 1: Write the failing runner handler tests**

Add handler tests that prove the runner can return a snapshot for a valid workspace and `404` for a missing one.

```go
func TestRepoStateReturnsWorkspaceSnapshot(t *testing.T) {
	t.Parallel()

	workRoot := t.TempDir()
	workspace, err := engine.CreateWorkspace(workRoot)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	if err := os.WriteFile(filepath.Join(workspace.Path, "notes.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	router := chi.NewRouter()
	router.Get("/internal/workspaces/{workspaceID}/repo-state", handlers.RepoState(workRoot))

	req := httptest.NewRequest(http.MethodGet, "/internal/workspaces/"+workspace.ID+"/repo-state", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		BranchName    string   `json:"branch_name"`
		HeadCommit    string   `json:"head_commit"`
		StatusSummary []string `json:"status_summary"`
		CapturedAt    string   `json:"captured_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal repo state: %v", err)
	}
	if payload.BranchName != "main" {
		t.Fatalf("expected branch main, got %q", payload.BranchName)
	}
	if payload.HeadCommit == "" {
		t.Fatalf("expected head commit to be populated")
	}
	if len(payload.StatusSummary) != 1 || !strings.Contains(payload.StatusSummary[0], "notes.txt") {
		t.Fatalf("expected dirty status summary, got %v", payload.StatusSummary)
	}
	if payload.CapturedAt == "" {
		t.Fatalf("expected captured_at to be populated")
	}
}

func TestRepoStateReturnsNotFoundForMissingWorkspace(t *testing.T) {
	t.Parallel()

	router := chi.NewRouter()
	router.Get("/internal/workspaces/{workspaceID}/repo-state", handlers.RepoState(t.TempDir()))

	req := httptest.NewRequest(http.MethodGet, "/internal/workspaces/missing/repo-state", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run the runner handler tests to verify they fail**

Run: `go test ./internal/http/handlers -run RepoState` from `services/runner`

Expected: FAIL because `handlers.RepoState` and the route do not exist yet.

- [ ] **Step 3: Implement the runner repo-state handler and mount the route**

Add a dedicated handler that resolves the workspace path and returns the existing engine snapshot shape as JSON.

```go
func RepoState(workRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := chi.URLParam(r, "workspaceID")
		workspacePath, err := workspacePathFromID(workRoot, workspaceID)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		snapshot, err := engine.CaptureSnapshot(workspacePath)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			BranchName    string    `json:"branch_name"`
			HeadCommit    string    `json:"head_commit"`
			StatusSummary []string  `json:"status_summary"`
			CapturedAt    time.Time `json:"captured_at"`
		}{
			BranchName:    snapshot.BranchName,
			HeadCommit:    snapshot.HeadCommit,
			StatusSummary: snapshot.StatusSummary,
			CapturedAt:    snapshot.CapturedAt,
		})
	}
}
```

Mount it in the runner router:

```go
r.Get("/internal/workspaces/{workspaceID}/repo-state", handlers.RepoState(workRoot))
```

Add runner client support in `services/api/internal/runner/client.go`:

```go
type RepoState struct {
	BranchName    string    `json:"branch_name"`
	HeadCommit    string    `json:"head_commit"`
	StatusSummary []string  `json:"status_summary"`
	CapturedAt    time.Time `json:"captured_at"`
}

type Client interface {
	CreateWorkspace(ctx context.Context, template string) (Workspace, error)
	ResetWorkspace(ctx context.Context, workspaceID string) error
	ConnectTerminal(ctx context.Context, workspaceID string) (TerminalConnection, error)
	GetRepoState(ctx context.Context, workspaceID string) (RepoState, error)
	DeleteWorkspace(ctx context.Context, workspaceID string, reason string, deleteAfter time.Duration) error
}

func (c *HTTPClient) GetRepoState(ctx context.Context, workspaceID string) (RepoState, error) {
	if c.baseURL == "" {
		return RepoState{}, ErrClientNotConfigured
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/internal/workspaces/"+url.PathEscape(workspaceID)+"/repo-state",
		nil,
	)
	if err != nil {
		return RepoState{}, fmt.Errorf("build repo state request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return RepoState{}, fmt.Errorf("get runner repo state: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return RepoState{}, ErrWorkspaceNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return RepoState{}, fmt.Errorf("runner repo state returned status %d", resp.StatusCode)
	}

	var snapshot RepoState
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return RepoState{}, fmt.Errorf("decode repo state response: %w", err)
	}

	return snapshot, nil
}
```

- [ ] **Step 4: Re-run the runner handler tests to verify they pass**

Run: `go test ./internal/http/handlers -run RepoState` from `services/runner`

Expected: PASS

- [ ] **Step 5: Commit the runner repo-state surface**

```bash
git add services/runner/internal/http/router.go services/runner/internal/http/handlers/repo_state.go services/runner/internal/http/handlers/repo_state_test.go services/api/internal/runner/client.go
git commit -m "feat: add runner repo state endpoint"
```

---

### Task 2: Expose Repo State Through the Public Practice Session API

**Files:**
- Create: `services/api/internal/http/handlers/repo_state.go`
- Modify: `services/api/internal/http/router.go`
- Modify: `services/api/internal/test/practice_routes_test.go`

- [ ] **Step 1: Write the failing API route and handler tests**

Add route-surface and behavior coverage to `practice_routes_test.go`.

```go
func TestPracticeRoutesMountRepoStateRoute(t *testing.T) {
	authStore := authStoreWithSession("repo-state-route-token", 42)
	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: &stubPracticeService{},
		RunnerClient:    &stubRunnerClient{},
		AuthStore:       authStore,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/practice-sessions/123/repo-state", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "repo-state-route-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected mounted route to delegate and return handler result, got %d", rec.Code)
	}
}

func TestPracticeSessionRepoStateReturnsSnapshot(t *testing.T) {
	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: &stubPracticeService{
			practiceSessionByIDFunc: func(context.Context, uint64, uint64) (domain.PracticeSession, error) {
				return domain.PracticeSession{
					ID:        123,
					UserID:    42,
					RunnerRef: "ws-repo",
					Status:    service.PracticeSessionStatusActive,
				}, nil
			},
		},
		RunnerClient: &stubRunnerClient{
			repoState: runner.RepoState{
				BranchName:    "feature/repo-panel",
				HeadCommit:    "6f9bc9e2f9e3f4f24b88a1d8d76d8ef0f1b1c6a0",
				StatusSummary: []string{"M notes.txt", "?? scratch.md"},
				CapturedAt:    time.Date(2026, 5, 23, 4, 0, 0, 0, time.UTC),
			},
		},
		AuthStore: authStoreWithSession("repo-state-token", 42),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/practice-sessions/123/repo-state", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "repo-state-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Data struct {
			Branch      string   `json:"branch"`
			HeadCommit  string   `json:"head_commit"`
			Dirty       bool     `json:"dirty"`
			ChangedFiles []string `json:"changed_files"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal repo-state payload: %v", err)
	}
	if payload.Data.Branch != "feature/repo-panel" {
		t.Fatalf("expected branch to round-trip, got %q", payload.Data.Branch)
	}
	if !payload.Data.Dirty || len(payload.Data.ChangedFiles) != 2 {
		t.Fatalf("expected dirty repo-state payload, got %+v", payload.Data)
	}
}

func TestPracticeSessionRepoStateMapsMissingWorkspaceToGone(t *testing.T) {
	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: &stubPracticeService{
			practiceSessionByIDFunc: func(context.Context, uint64, uint64) (domain.PracticeSession, error) {
				return domain.PracticeSession{ID: 123, UserID: 42, RunnerRef: "ws-missing", Status: service.PracticeSessionStatusActive}, nil
			},
		},
		RunnerClient: &stubRunnerClient{repoStateErr: runner.ErrWorkspaceNotFound},
		AuthStore:    authStoreWithSession("repo-gone-token", 42),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/practice-sessions/123/repo-state", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "repo-gone-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d with body %s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run the API repo-state tests to verify they fail**

Run: `go test ./internal/test -run RepoState` from `services/api`

Expected: FAIL because the public route, handler, and runner stubs do not support repo-state yet.

- [ ] **Step 3: Implement the public repo-state handler and route**

Add a handler that resolves the authenticated practice session and maps runner repo-state to the public contract.

```go
func GetPracticeSessionRepoState(practiceService service.PracticeService, runnerClient runner.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.SessionUserID(r.Context())
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "Authentication required.")
			return
		}

		sessionID, err := parseUint64Param(r, "sessionId")
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "Invalid session ID.")
			return
		}

		session, err := practiceService.PracticeSessionByID(r.Context(), userID, sessionID)
		if err != nil {
			writePracticeSessionError(w, err)
			return
		}

		snapshot, err := runnerClient.GetRepoState(r.Context(), session.RunnerRef)
		if err != nil {
			switch {
			case errors.Is(err, runner.ErrWorkspaceNotFound):
				writeJSONError(w, http.StatusGone, "Current session workspace is unavailable.")
			case errors.Is(err, runner.ErrClientNotConfigured):
				writeJSONError(w, http.StatusInternalServerError, err.Error())
			default:
				writeJSONError(w, http.StatusBadGateway, "Unable to load repository state.")
			}
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"data": map[string]any{
				"branch":        snapshot.BranchName,
				"head_commit":   snapshot.HeadCommit,
				"dirty":         len(snapshot.StatusSummary) > 0,
				"changed_files": snapshot.StatusSummary,
				"captured_at":   snapshot.CapturedAt.UTC(),
			},
		})
	}
}
```

Mount it in `services/api/internal/http/router.go`:

```go
r.Get("/practice-sessions/{sessionId}/repo-state", handlers.GetPracticeSessionRepoState(dependencies.PracticeService, dependencies.RunnerClient))
```

Extend the API test stubs:

```go
type stubRunnerClient struct {
	workspace    runner.Workspace
	repoState    runner.RepoState
	repoStateErr error
}

func (s *stubRunnerClient) GetRepoState(context.Context, string) (runner.RepoState, error) {
	if s.repoStateErr != nil {
		return runner.RepoState{}, s.repoStateErr
	}
	return s.repoState, nil
}
```

- [ ] **Step 4: Re-run the API repo-state tests to verify they pass**

Run: `go test ./internal/test -run RepoState` from `services/api`

Expected: PASS

- [ ] **Step 5: Commit the public repo-state API**

```bash
git add services/api/internal/http/router.go services/api/internal/http/handlers/repo_state.go services/api/internal/test/practice_routes_test.go
git commit -m "feat: add practice session repo state api"
```

---

### Task 3: Load and Render Repo State in the Workbench

**Files:**
- Create: `apps/web/src/hooks/useRepoState.ts`
- Modify: `apps/web/src/types.ts`
- Modify: `apps/web/src/lib/api.ts`
- Modify: `apps/web/src/App.tsx`
- Modify: `apps/web/src/components/Workbench.tsx`
- Modify: `apps/web/src/components/RepoPanel.tsx`
- Modify: `apps/web/src/styles.css`
- Modify: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Write the failing frontend tests for initial repo-state render and unavailable fallback**

Add tests that prove the live card fetches repo state and renders both clean and unavailable states.

```tsx
it("renders live repo snapshot facts for the active session", async () => {
  mockFetch
    .mockResolvedValueOnce(jsonResponse(defaultCatalog))
    .mockResolvedValueOnce(jsonResponse({ session: defaultSession }))
    .mockResolvedValueOnce(
      jsonResponse({
        data: {
          branch: "main",
          head_commit: "6f9bc9e2f9e3f4f24b88a1d8d76d8ef0f1b1c6a0",
          dirty: false,
          changed_files: [],
          captured_at: "2026-05-23T04:00:00.000Z",
        },
      }),
    );

  render(<App />);

  const sessionCard = await screen.findByLabelText("Operational session card");
  expect(within(sessionCard).getByText("Branch")).toBeInTheDocument();
  expect(within(sessionCard).getByText("main")).toBeInTheDocument();
  expect(within(sessionCard).getByText("HEAD")).toBeInTheDocument();
  expect(within(sessionCard).getByText("6f9bc9e")).toBeInTheDocument();
  expect(within(sessionCard).getByText("Working tree")).toBeInTheDocument();
  expect(within(sessionCard).getByText("Clean")).toBeInTheDocument();
});

it("renders an inline unavailable repo state when the snapshot cannot be loaded", async () => {
  mockFetch
    .mockResolvedValueOnce(jsonResponse(defaultCatalog))
    .mockResolvedValueOnce(jsonResponse({ session: defaultSession }))
    .mockResolvedValueOnce(errorResponse(502, "Unable to load repository state."));

  render(<App />);

  const sessionCard = await screen.findByLabelText("Operational session card");
  expect(within(sessionCard).getByText("Repository state unavailable.")).toBeInTheDocument();
  expect(within(sessionCard).queryByText("Branch")).not.toBeInTheDocument();
});
```

- [ ] **Step 2: Run the focused frontend test to verify it fails**

Run: `pnpm --dir apps/web test -- src/test/App.test.tsx -t "renders live repo snapshot facts|renders an inline unavailable repo state"`

Expected: FAIL because repo-state types, fetches, and rendering do not exist yet.

- [ ] **Step 3: Add repo-state types, API helper, hook, and initial wiring**

Add web types in `apps/web/src/types.ts`:

```ts
export type RepoStateSnapshot = {
  branch: string;
  headCommit: string;
  dirty: boolean;
  changedFiles: string[];
  capturedAt: string;
};

export type RepoStateView =
  | { status: "idle"; snapshot: null; error: null }
  | { status: "loading"; snapshot: null; error: null }
  | { status: "ready"; snapshot: RepoStateSnapshot; error: null }
  | { status: "stale"; snapshot: RepoStateSnapshot; error: string }
  | { status: "error"; snapshot: null; error: string };
```

Add a repo-state fetcher in `apps/web/src/lib/api.ts`:

```ts
type RepoStateResponse = {
  data: {
    branch: string;
    head_commit: string;
    dirty: boolean;
    changed_files: string[];
    captured_at: string;
  };
};

function toRepoStateSnapshot(payload: RepoStateResponse["data"]): RepoStateSnapshot {
  return {
    branch: payload.branch,
    headCommit: payload.head_commit,
    dirty: payload.dirty,
    changedFiles: payload.changed_files,
    capturedAt: payload.captured_at,
  };
}

export async function fetchPracticeRepoState(sessionId: number, signal?: AbortSignal): Promise<RepoStateSnapshot> {
  const response = await fetch(`${API_BASE}/practice-sessions/${sessionId}/repo-state`, {
    credentials: "include",
    headers: { Accept: "application/json" },
    signal,
  });

  const payload = await readJson<RepoStateResponse | { error?: string }>(response);
  if (!response.ok) {
    const message =
      payload.data && "error" in payload.data && payload.data.error
        ? payload.data.error
        : "Unable to load repository state.";
    throw new ApiError(message, response.status);
  }
  if (!payload.data || !("data" in payload.data)) {
    throw new ApiError("Repository state response was malformed", response.status);
  }

  return toRepoStateSnapshot(payload.data.data);
}
```

Create `apps/web/src/hooks/useRepoState.ts` for initial session-driven loading:

```ts
export function useRepoState(session: PracticeSession | null): RepoStateView {
  const [state, setState] = useState<RepoStateView>({
    status: "idle",
    snapshot: null,
    error: null,
  });

  useEffect(() => {
    if (!session) {
      setState({ status: "idle", snapshot: null, error: null });
      return;
    }

    const controller = new AbortController();
    setState({ status: "loading", snapshot: null, error: null });

    void fetchPracticeRepoState(session.id, controller.signal)
      .then((snapshot) => {
        setState({ status: "ready", snapshot, error: null });
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) {
          return;
        }
        setState({
          status: "error",
          snapshot: null,
          error: error instanceof Error ? error.message : "Unable to load repository state.",
        });
      });

    return () => controller.abort();
  }, [session?.id]);

  return state;
}
```

Wire it through `App.tsx` and `Workbench.tsx`:

```tsx
const repoState = useRepoState(displayedSession);

<Workbench
  session={displayedSession}
  terminal={terminalSession}
  scenarioName={displayedScenario?.name ?? null}
  templateName={displayedTemplate?.name ?? null}
  repoState={repoState}
/>;
```

```tsx
type WorkbenchProps = {
  preview?: boolean;
  session?: PracticeSession | null;
  terminal?: TerminalSessionState;
  scenarioName?: string | null;
  templateName?: string | null;
  repoState?: RepoStateView;
};
```

Extend `RepoPanel.tsx` with the repo section:

```tsx
function shortHead(headCommit: string) {
  return headCommit.slice(0, 7);
}

{repoState.status === "ready" || repoState.status === "stale" ? (
  <>
    <dl className="repo-state-snapshot">
      <div>
        <dt>Branch</dt>
        <dd>{repoState.snapshot.branch}</dd>
      </div>
      <div>
        <dt>HEAD</dt>
        <dd title={repoState.snapshot.headCommit}>{shortHead(repoState.snapshot.headCommit)}</dd>
      </div>
      <div>
        <dt>Working tree</dt>
        <dd>{repoState.snapshot.dirty ? "Dirty" : "Clean"}</dd>
      </div>
    </dl>
    {repoState.snapshot.dirty ? (
      <ul className="repo-state-changes" aria-label="Changed files">
        {repoState.snapshot.changedFiles.map((entry) => (
          <li key={entry}>{entry}</li>
        ))}
      </ul>
    ) : null}
  </>
) : repoState.status === "loading" ? (
  <div className="repo-state-inline-note">Loading repository state...</div>
) : repoState.status === "error" ? (
  <div className="repo-state-inline-note">Repository state unavailable.</div>
) : null}
```

- [ ] **Step 4: Re-run the focused frontend test to verify it passes**

Run: `pnpm --dir apps/web test -- src/test/App.test.tsx -t "renders live repo snapshot facts|renders an inline unavailable repo state"`

Expected: PASS

- [ ] **Step 5: Commit initial frontend repo-state rendering**

```bash
git add apps/web/src/types.ts apps/web/src/lib/api.ts apps/web/src/hooks/useRepoState.ts apps/web/src/App.tsx apps/web/src/components/Workbench.tsx apps/web/src/components/RepoPanel.tsx apps/web/src/styles.css apps/web/src/test/App.test.tsx
git commit -m "feat: show repo snapshot in session card"
```

---

### Task 4: Refresh Repo State After Command Completion and Preserve Stale Data on Failure

**Files:**
- Modify: `apps/web/src/hooks/useRepoState.ts`
- Modify: `apps/web/src/App.tsx`
- Modify: `apps/web/src/components/RepoPanel.tsx`
- Modify: `apps/web/src/test/App.test.tsx`
- Modify: `apps/web/tests/e2e/smoke.spec.ts`

- [ ] **Step 1: Write the failing frontend tests for command-finished refresh and stale fallback**

Add tests that prove repo state refreshes after terminal command completion and preserves the last good snapshot when a refresh fails.

```tsx
it("refreshes repo state after a terminal command finishes", async () => {
  mockTerminalState.history = [{ id: "cmd-1", command: "git status", phase: "stopped", summary: "Command finished successfully" }];

  mockFetch
    .mockResolvedValueOnce(jsonResponse(defaultCatalog))
    .mockResolvedValueOnce(jsonResponse({ session: defaultSession }))
    .mockResolvedValueOnce(
      jsonResponse({
        data: {
          branch: "main",
          head_commit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          dirty: false,
          changed_files: [],
          captured_at: "2026-05-23T04:00:00.000Z",
        },
      }),
    )
    .mockResolvedValueOnce(
      jsonResponse({
        data: {
          branch: "feature/repo-panel",
          head_commit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
          dirty: true,
          changed_files: ["M notes.txt"],
          captured_at: "2026-05-23T04:01:00.000Z",
        },
      }),
    );

  render(<App />);

  expect(await screen.findByText("Clean")).toBeInTheDocument();
  await waitFor(() => expect(screen.getByText("Dirty")).toBeInTheDocument());
  expect(screen.getByText("M notes.txt")).toBeInTheDocument();
});

it("keeps the last snapshot visible and marks it stale when refresh fails", async () => {
  mockTerminalState.history = [{ id: "cmd-2", command: "git status", phase: "stopped", summary: "Command finished successfully" }];

  mockFetch
    .mockResolvedValueOnce(jsonResponse(defaultCatalog))
    .mockResolvedValueOnce(jsonResponse({ session: defaultSession }))
    .mockResolvedValueOnce(
      jsonResponse({
        data: {
          branch: "main",
          head_commit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          dirty: false,
          changed_files: [],
          captured_at: "2026-05-23T04:00:00.000Z",
        },
      }),
    )
    .mockResolvedValueOnce(errorResponse(502, "Unable to load repository state."));

  render(<App />);

  expect(await screen.findByText("main")).toBeInTheDocument();
  await waitFor(() =>
    expect(screen.getByText("Repository state may be out of date.")).toBeInTheDocument(),
  );
  expect(screen.getByText("Clean")).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the focused frontend tests to verify they fail**

Run: `pnpm --dir apps/web test -- src/test/App.test.tsx -t "refreshes repo state after a terminal command finishes|keeps the last snapshot visible and marks it stale"`

Expected: FAIL because the repo-state hook only loads once per session today.

- [ ] **Step 3: Implement command-finished refresh and stale-state preservation**

Extend `useRepoState.ts` so it can refresh on both session changes and terminal command completion.

```ts
type UseRepoStateOptions = {
  session: PracticeSession | null;
  commandHistory: CommandHistoryEntry[];
};

export function useRepoState({ session, commandHistory }: UseRepoStateOptions): RepoStateView {
  const [state, setState] = useState<RepoStateView>({ status: "idle", snapshot: null, error: null });
  const [refreshNonce, setRefreshNonce] = useState(0);
  const lastCompletedCommandIdRef = useRef<string | null>(null);

  const latestCompletedCommandId =
    [...commandHistory].reverse().find((entry) => entry.phase === "stopped")?.id ?? null;

  useEffect(() => {
    if (!session) {
      lastCompletedCommandIdRef.current = null;
      setState({ status: "idle", snapshot: null, error: null });
      return;
    }

    const controller = new AbortController();

    setState((current) =>
      current.snapshot
        ? { status: "stale", snapshot: current.snapshot, error: null }
        : { status: "loading", snapshot: null, error: null },
    );

    void fetchPracticeRepoState(session.id, controller.signal)
      .then((snapshot) => {
        setState({ status: "ready", snapshot, error: null });
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) {
          return;
        }

        const message =
          error instanceof Error ? error.message : "Unable to load repository state.";

        setState((current) =>
          current.snapshot
            ? { status: "stale", snapshot: current.snapshot, error: message }
            : { status: "error", snapshot: null, error: message },
        );
      });

    return () => controller.abort();
  }, [session?.id, refreshNonce]);

  useEffect(() => {
    if (!session || !latestCompletedCommandId) {
      return;
    }
    if (lastCompletedCommandIdRef.current === latestCompletedCommandId) {
      return;
    }

    lastCompletedCommandIdRef.current = latestCompletedCommandId;
    setRefreshNonce((value) => value + 1);
  }, [latestCompletedCommandId, session]);

  return state;
}
```

Wire the hook call in `App.tsx`:

```tsx
const repoState = useRepoState({
  session: displayedSession,
  commandHistory: terminalSession.history,
});
```

Render stale copy in `RepoPanel.tsx`:

```tsx
{repoState.status === "stale" && repoState.error ? (
  <div className="repo-state-inline-note">Repository state may be out of date.</div>
) : null}
```

- [ ] **Step 4: Add the repo-snapshot E2E smoke flow**

Add one focused smoke test in `apps/web/tests/e2e/smoke.spec.ts` that mutates the repo through the terminal and verifies the card updates.

```ts
test("updates the repo state card after a mutating git command completes", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByLabel("Operational session card")).toContainText("Working tree");
  await expect(page.getByLabel("Operational session card")).toContainText("Clean");

  const terminal = page.getByLabel("Practice terminal");
  await terminal.click();
  await page.keyboard.type("echo dirty >> notes.txt");
  await page.keyboard.press("Enter");

  await expect(page.getByLabel("Command history")).toContainText("Command finished");
  await expect(page.getByLabel("Operational session card")).toContainText("Dirty");
  await expect(page.getByLabel("Operational session card")).toContainText("notes.txt");
});
```

- [ ] **Step 5: Run the focused verification and full suites**

Run:

- `pnpm --dir apps/web test -- src/test/App.test.tsx -t "refreshes repo state after a terminal command finishes|keeps the last snapshot visible and marks it stale"`
- `pnpm --dir apps/web run test:e2e -- --grep "repo state card|repo snapshot|mutating git command"`
- `pnpm --dir apps/web test`
- `go test ./...` from `services/runner`
- `go test ./...` from `services/api`

Expected:

- focused Vitest assertions pass
- focused Playwright repo-state smoke passes
- full frontend and backend test suites pass

- [ ] **Step 6: Commit command-driven refresh and final coverage**

```bash
git add apps/web/src/hooks/useRepoState.ts apps/web/src/App.tsx apps/web/src/components/RepoPanel.tsx apps/web/src/test/App.test.tsx apps/web/tests/e2e/smoke.spec.ts
git commit -m "feat: refresh repo state after terminal commands"
```

---

## Final Verification

- [ ] Run `go test ./...` in `services/runner`
- [ ] Run `go test ./...` in `services/api`
- [ ] Run `pnpm --dir apps/web test`
- [ ] Run `pnpm --dir apps/web run test:e2e`
- [ ] Confirm the operational session card shows branch, head, working tree, and changed files for a live session
- [ ] Confirm a failed repo-state refresh leaves the workbench visible and marks the snapshot stale inline

## Notes For Implementers

- Keep repo-state additive to the existing operational card; do not redesign the workbench layout in this branch
- Keep page-level `Session unavailable` and `Workspace unavailable` flows untouched
- Prefer last-known-snapshot preservation over blanking the repo section on refresh failures
- Do not introduce polling or terminal websocket protocol changes in this slice
