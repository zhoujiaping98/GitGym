# Repo Stale Recovery UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a direct in-card retry flow for repo-state errors and stale repo snapshots without leaving the live workbench.

**Architecture:** Keep backend contracts unchanged. Extend `useRepoState` with a manual refresh action and explicit in-flight state, then let `RepoPanel` render retry/loading affordances from those props while preserving the existing stale snapshot fallback.

**Tech Stack:** React, TypeScript, Vitest, Testing Library

---

## File Map

- Modify: `apps/web/src/hooks/useRepoState.ts`
- Modify: `apps/web/src/components/RepoPanel.tsx`
- Modify: `apps/web/src/components/Workbench.tsx`
- Modify: `apps/web/src/App.tsx`
- Modify: `apps/web/src/test/App.test.tsx`

## Task 1: Add manual repo refresh state to the hook under TDD

**Files:**
- Modify: `apps/web/src/hooks/useRepoState.ts`
- Modify: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Write the failing stale-retry RTL regressions**

Add these tests near the existing repo-state stale/error coverage in `apps/web/src/test/App.test.tsx`:

```ts
it("retries an unavailable repo snapshot from the card", async () => {
  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: activeSession,
    absenceReason: null,
    error: null,
    refresh: vi.fn().mockResolvedValue(activeSession),
  });

  mockUseTerminalSession.mockReturnValue(
    createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [],
    }),
  );

  let repoStateRequestCount = 0;
  let releaseRetry: (() => void) | null = null;
  mockFetch.mockImplementation((input: RequestInfo | URL) => {
    const url = String(input);

    if (url.endsWith("/api/v1/templates")) {
      return createCatalogResponse();
    }

    if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
      repoStateRequestCount += 1;
      if (repoStateRequestCount === 1) {
        return createErrorResponse(502, "Unable to load repository state.");
      }
      return new Promise<Response>((resolve) => {
        releaseRetry = () => resolve(createJsonResponse(defaultRepoStatePayload));
      });
    }

    throw new Error(`Unexpected fetch request: ${url}`);
  });

  render(<App />);

  expect(await screen.findByText("Repository state unavailable.")).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "Retry repository state" }));

  await waitFor(() => {
    expect(screen.getByText("Refreshing repository state...")).toBeInTheDocument();
  });
  expect(screen.getByRole("button", { name: "Retry repository state" })).toBeDisabled();

  releaseRetry?.();

  await waitFor(() => {
    expect(screen.getByText("main")).toBeInTheDocument();
  });
  expect(screen.queryByText("Repository state unavailable.")).not.toBeInTheDocument();
});

it("retries a stale repo snapshot without clearing the last visible snapshot", async () => {
  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: activeSession,
    absenceReason: null,
    error: null,
    refresh: vi.fn().mockResolvedValue(activeSession),
  });

  const initialTerminalState = createTerminalState({
    status: "ready",
    terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
    history: [],
  });
  const completedCommandTerminalState = createTerminalState({
    status: "ready",
    terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
    history: [
      { id: "cmd-2", command: "git status", phase: "stopped", exitCode: 0 },
    ],
  });

  mockUseTerminalSession.mockReturnValue(initialTerminalState);

  let repoStateRequestCount = 0;
  let releaseRetry: (() => void) | null = null;
  mockFetch.mockImplementation((input: RequestInfo | URL) => {
    const url = String(input);

    if (url.endsWith("/api/v1/templates")) {
      return createCatalogResponse();
    }

    if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
      repoStateRequestCount += 1;
      if (repoStateRequestCount === 1) {
        return createJsonResponse(defaultRepoStatePayload);
      }
      if (repoStateRequestCount === 2) {
        return createErrorResponse(502, "Unable to load repository state.");
      }
      return new Promise<Response>((resolve) => {
        releaseRetry = () =>
          resolve(
            createJsonResponse({
              data: {
                ...defaultRepoStatePayload.data,
                dirty: true,
                changed_files: ["?? notes.txt"],
                captured_at: "2026-05-23T04:04:00.000Z",
              },
            }),
          );
      });
    }

    throw new Error(`Unexpected fetch request: ${url}`);
  });

  const { rerender } = render(<App />);
  const sessionCard = await screen.findByLabelText("Operational session card");

  mockUseTerminalSession.mockReturnValue(completedCommandTerminalState);
  rerender(<App />);

  await waitFor(() => {
    expect(within(sessionCard).getByText("Repository state may be out of date.")).toBeInTheDocument();
  });
  expect(within(sessionCard).getByText("main")).toBeInTheDocument();

  fireEvent.click(within(sessionCard).getByRole("button", { name: "Retry repository state" }));

  await waitFor(() => {
    expect(within(sessionCard).getByText("Refreshing repository state...")).toBeInTheDocument();
  });
  expect(within(sessionCard).getByText("main")).toBeInTheDocument();

  releaseRetry?.();

  await waitFor(() => {
    expect(within(sessionCard).getByText("Dirty")).toBeInTheDocument();
  });
  expect(within(sessionCard).queryByText("Repository state may be out of date.")).not.toBeInTheDocument();
});

it("keeps the repo retry action available after a failed manual retry", async () => {
  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: activeSession,
    absenceReason: null,
    error: null,
    refresh: vi.fn().mockResolvedValue(activeSession),
  });

  mockUseTerminalSession.mockReturnValue(
    createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [],
    }),
  );

  let repoStateRequestCount = 0;
  mockFetch.mockImplementation((input: RequestInfo | URL) => {
    const url = String(input);

    if (url.endsWith("/api/v1/templates")) {
      return createCatalogResponse();
    }

    if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
      repoStateRequestCount += 1;
      return createErrorResponse(502, "Unable to load repository state.");
    }

    throw new Error(`Unexpected fetch request: ${url}`);
  });

  render(<App />);

  expect(await screen.findByText("Repository state unavailable.")).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "Retry repository state" }));

  await waitFor(() => {
    expect(screen.getByRole("button", { name: "Retry repository state" })).toBeEnabled();
  });
  expect(screen.getByText("Repository state unavailable.")).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the focused stale-retry regressions to verify they fail**

Run: `pnpm --dir apps/web test -- src/test/App.test.tsx -t "retries an unavailable repo snapshot from the card|retries a stale repo snapshot without clearing the last visible snapshot|keeps the repo retry action available after a failed manual retry"`

Expected: FAIL because the hook does not expose a retry action or refresh-in-flight state and the panel does not render a retry button.

- [ ] **Step 3: Add manual refresh state and retry action to `useRepoState`**

Update `apps/web/src/hooks/useRepoState.ts` so the hook result includes:

```ts
type UseRepoStateResult = {
  repoState: RepoStateView;
  repoAttribution: RepoAttribution | null;
  repoOutcome: string | null;
  retryRepoState: (() => void) | null;
  isRefreshingRepoState: boolean;
};
```

Add a fetch-in-flight state and a stable manual retry action:

```ts
const [isRefreshingRepoState, setIsRefreshingRepoState] = useState(false);

const retryRepoState = session
  ? () => {
      setRefreshToken((value) => value + 1);
    }
  : null;
```

Set `isRefreshingRepoState` to `true` when a fetch begins and back to `false` on both success and failure. Keep the current preserved snapshot in place while retry is in flight:

```ts
setIsRefreshingRepoState(true);
setState((current) =>
  isSameSession && current.snapshot
    ? {
        status: "stale",
        snapshot: current.snapshot,
        error: current.error,
      }
    : {
        status: "loading",
        snapshot: null,
        error: null,
      },
);
```

On success:

```ts
setIsRefreshingRepoState(false);
```

On failure:

```ts
setIsRefreshingRepoState(false);
```

Return the new values from the hook.

- [ ] **Step 4: Re-run the focused hook-driven regressions to verify they still fail only on missing UI**

Run: `pnpm --dir apps/web test -- src/test/App.test.tsx -t "retries an unavailable repo snapshot from the card|retries a stale repo snapshot without clearing the last visible snapshot|keeps the repo retry action available after a failed manual retry"`

Expected: FAIL because the app still does not pass retry props through to the repo panel.

- [ ] **Step 5: Commit the hook-side retry state**

```bash
git add apps/web/src/hooks/useRepoState.ts apps/web/src/test/App.test.tsx
git commit -m "feat: add repo state retry controls"
```

## Task 2: Render repo retry affordances in the card under TDD

**Files:**
- Modify: `apps/web/src/components/RepoPanel.tsx`
- Modify: `apps/web/src/components/Workbench.tsx`
- Modify: `apps/web/src/App.tsx`
- Modify: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Extend the focused test with the loading-note expectation**

If not already present in Task 1, make sure the stale and unavailable retry tests assert:

```ts
expect(screen.getByText("Refreshing repository state...")).toBeInTheDocument();
expect(screen.getByRole("button", { name: "Retry repository state" })).toBeDisabled();
```

This should remain red until the panel renders explicit loading treatment.

- [ ] **Step 2: Wire retry props through `App` and `Workbench`**

Update `apps/web/src/App.tsx`:

```ts
const {
  repoState,
  repoAttribution,
  repoOutcome,
  retryRepoState,
  isRefreshingRepoState,
} = useRepoState({
  session: displayedSession,
  commandHistory: terminalSession.history,
  refreshContext: repoRefreshContext,
});
```

Pass both props into `Workbench`:

```tsx
<Workbench
  ...
  repoOutcome={repoOutcome}
  retryRepoState={retryRepoState}
  isRefreshingRepoState={isRefreshingRepoState}
/>
```

Update `apps/web/src/components/Workbench.tsx` to accept and forward:

```ts
type WorkbenchProps = {
  ...
  repoOutcome?: string | null;
  retryRepoState?: (() => void) | null;
  isRefreshingRepoState?: boolean;
};
```

- [ ] **Step 3: Render retry/loading behavior in `RepoPanel`**

Update `apps/web/src/components/RepoPanel.tsx`:

```tsx
type RepoPanelProps = {
  ...
  retryRepoState?: (() => void) | null;
  isRefreshingRepoState?: boolean;
};
```

Use these defaults:

```tsx
repoOutcome = null,
retryRepoState = null,
isRefreshingRepoState = false,
```

Render the loading note in the snapshot header:

```tsx
{isRefreshingRepoState ? (
  <span className="repo-state-inline-note">Refreshing repository state...</span>
) : repoState.status === "loading" ? (
  <span className="repo-state-inline-note">Loading repository state...</span>
) : null}
```

Render a retry button only for degraded repo-state:

```tsx
const canRetryRepoState =
  retryRepoState !== null &&
  (repoState.status === "error" || (repoState.status === "stale" && repoState.error));
```

Render it near the degraded copy:

```tsx
{canRetryRepoState ? (
  <button
    className="top-bar-button"
    onClick={retryRepoState}
    type="button"
    disabled={isRefreshingRepoState}
    aria-label="Retry repository state"
  >
    Retry
  </button>
) : null}
```

Keep the stale warning and the preserved snapshot behavior unchanged.

- [ ] **Step 4: Re-run the focused stale-retry regressions to verify they pass**

Run: `pnpm --dir apps/web test -- src/test/App.test.tsx -t "retries an unavailable repo snapshot from the card|retries a stale repo snapshot without clearing the last visible snapshot|keeps the repo retry action available after a failed manual retry"`

Expected: PASS

- [ ] **Step 5: Run the full web suite**

Run: `pnpm --dir apps/web test`

Expected: PASS

- [ ] **Step 6: Commit the repo-card retry UX**

```bash
git add apps/web/src/App.tsx apps/web/src/components/Workbench.tsx apps/web/src/components/RepoPanel.tsx apps/web/src/test/App.test.tsx
git commit -m "feat: add repo stale recovery ux"
```

## Final Verification

- [ ] Run `git status --short`
- [ ] Confirm the branch is clean
- [ ] Summarize changed behavior and test evidence before merging or presenting finish options
