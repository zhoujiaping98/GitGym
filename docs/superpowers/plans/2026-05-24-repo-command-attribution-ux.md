# Repo Command Attribution UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add lightweight attribution for the currently displayed repository snapshot so the right-side card can tell users whether the latest visible repo state came from session lifecycle refresh or the last completed terminal command.

**Architecture:** Keep the existing repo-state API and stale fallback behavior intact. Extend the web app with a thin attribution model that is stamped onto successful repo-state refreshes, then render that attribution in `RepoPanel` without turning the card into a command-history feature.

**Tech Stack:** React 19, TypeScript, Vitest, React Testing Library, Playwright

---

## File Structure

### Existing files to modify

- `apps/web/src/types.ts`
  - add repo refresh trigger and repo-attribution types alongside the existing repo-state view
- `apps/web/src/hooks/useRepoState.ts`
  - accept refresh trigger context and stamp successful snapshots with the trigger that produced them
- `apps/web/src/App.tsx`
  - derive repo refresh trigger context from lifecycle events and completed terminal commands, then pass it into `useRepoState`
- `apps/web/src/components/Workbench.tsx`
  - thread attribution props into `RepoPanel`
- `apps/web/src/components/RepoPanel.tsx`
  - render one compact attribution/freshness line while preserving current stale/unavailable behavior
- `apps/web/src/test/App.test.tsx`
  - add focused RTL coverage for initial attribution, command attribution, lifecycle attribution, and stale-preservation behavior
- `apps/web/tests/e2e/smoke.spec.ts`
  - assert that a mutating terminal command updates both repo facts and attribution text

### No backend changes

- `services/api/**`
  - no changes; reuse existing `captured_at` and repo-state route
- `services/runner/**`
  - no changes; command attribution remains client-side

---

### Task 1: Introduce repo-attribution types and initial render behavior under TDD

**Files:**
- Modify: `apps/web/src/types.ts`
- Modify: `apps/web/src/hooks/useRepoState.ts`
- Modify: `apps/web/src/components/RepoPanel.tsx`
- Modify: `apps/web/src/components/Workbench.tsx`
- Modify: `apps/web/src/App.tsx`
- Modify: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Write the failing test for neutral initial attribution**

Add a focused regression near the existing repo snapshot tests proving that the first successful live-session snapshot renders neutral attribution text instead of only raw repo facts.

```tsx
it("renders neutral attribution for the initial repo snapshot load", async () => {
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

  mockFetch.mockImplementation((input: RequestInfo | URL) => {
    const url = String(input);

    if (url.endsWith("/api/v1/templates")) {
      return createCatalogResponse();
    }

    if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
      return createJsonResponse(defaultRepoStatePayload);
    }

    throw new Error(`Unexpected fetch request: ${url}`);
  });

  render(<App />);

  const sessionCard = await screen.findByLabelText("Operational session card");
  expect(await within(sessionCard).findByText("Snapshot loaded")).toBeInTheDocument();
  expect(within(sessionCard).getByText("main")).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
pnpm --dir apps/web test -- src/test/App.test.tsx -t "renders neutral attribution for the initial repo snapshot load"
```

Expected:

```text
FAIL
Unable to find an element with the text: Snapshot loaded
```

- [ ] **Step 3: Add the minimal attribution types**

In `apps/web/src/types.ts`, add explicit trigger and attribution types next to `RepoStateSnapshot` and `RepoStateView`.

```ts
export type RepoRefreshTrigger =
  | "session_load"
  | "session_create"
  | "session_reset"
  | "session_sync"
  | "command_complete";

export type RepoAttribution = {
  trigger: RepoRefreshTrigger;
  capturedAt: string;
  commandId?: string;
  commandText?: string;
};
```

Notes for the engineer:
- Do not fold attribution into `RepoStateSnapshot`; keep snapshot facts and UI attribution separate.
- Do not add backend-facing types here.

- [ ] **Step 4: Thread minimal attribution through the hook and panel**

Update the hook signature in `apps/web/src/hooks/useRepoState.ts` so the caller can pass a pending trigger context, then expose attribution alongside the existing `RepoStateView`.

```ts
type RepoRefreshContext = {
  trigger: RepoRefreshTrigger;
  commandId?: string;
  commandText?: string;
};

type UseRepoStateOptions = {
  session: PracticeSession | null;
  commandHistory: CommandHistoryEntry[];
  refreshContext: RepoRefreshContext;
};
```

When a fetch succeeds, stamp the current snapshot with attribution:

```ts
const [attribution, setAttribution] = useState<RepoAttribution | null>(null);

void fetchPracticeRepoState(session.id, controller.signal)
  .then((snapshot) => {
    setState({
      status: "ready",
      snapshot,
      error: null,
    });
    setAttribution({
      trigger: refreshContext.trigger,
      capturedAt: snapshot.capturedAt,
      commandId: refreshContext.commandId,
      commandText: refreshContext.commandText,
    });
  })
```

Render the neutral copy in `apps/web/src/components/RepoPanel.tsx`:

```tsx
function repoAttributionCopy(attribution: RepoAttribution | null) {
  if (!attribution) {
    return null;
  }

  if (attribution.trigger === "session_load") {
    return "Snapshot loaded";
  }

  return null;
}
```

```tsx
const attributionCopy = repoAttributionCopy(repoAttribution);

{attributionCopy ? <span className="repo-state-inline-note">{attributionCopy}</span> : null}
```

Pass the new prop from `Workbench` and `App.tsx` with the initial caller context set to `session_load`.

In `apps/web/src/App.tsx`, initialize explicit trigger state up front so later tasks can update it from lifecycle actions and completed commands:

```tsx
const [repoRefreshContext, setRepoRefreshContext] = useState<RepoRefreshContext>({
  trigger: "session_load",
});
```

Notes for the engineer:
- Preserve existing stale/unavailable copy exactly in this task.
- Keep `RepoPanel` presentational; the panel should receive already-computed attribution metadata or copy inputs, not inspect terminal history.

- [ ] **Step 5: Re-run the initial attribution test to verify it passes**

Run:

```bash
pnpm --dir apps/web test -- src/test/App.test.tsx -t "renders neutral attribution for the initial repo snapshot load"
```

Expected:

```text
PASS
```

- [ ] **Step 6: Commit the attribution scaffolding**

```bash
git add apps/web/src/types.ts apps/web/src/hooks/useRepoState.ts apps/web/src/App.tsx apps/web/src/components/Workbench.tsx apps/web/src/components/RepoPanel.tsx apps/web/src/test/App.test.tsx
git commit -m "feat: add repo snapshot attribution state"
```

---

### Task 2: Attribute successful command-driven refreshes and preserve prior attribution on failure

**Files:**
- Modify: `apps/web/src/hooks/useRepoState.ts`
- Modify: `apps/web/src/components/RepoPanel.tsx`
- Modify: `apps/web/src/App.tsx`
- Modify: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Write the failing test for command-based attribution**

Extend the existing command-refresh coverage with an assertion that the post-command snapshot is explicitly attributed to the completed command.

```tsx
it("renders command attribution after repo state refreshes for a completed command", async () => {
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
      {
        id: "cmd-1",
        command: "git add .",
        phase: "stopped",
        summary: "Command finished successfully",
      },
    ],
  });

  mockUseTerminalSession.mockReturnValue(initialTerminalState);

  mockFetch.mockImplementation((input: RequestInfo | URL) => {
    const url = String(input);

    if (url.endsWith("/api/v1/templates")) {
      return createCatalogResponse();
    }

    if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
      return createJsonResponse(
        mockFetch.mock.calls.filter(
          ([request]) => String(request).endsWith("/api/v1/practice-sessions/42/repo-state"),
        ).length === 1
          ? defaultRepoStatePayload
          : {
              data: {
                branch: "feature/repo-panel",
                head_commit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                dirty: true,
                changed_files: ["M notes.txt"],
                captured_at: "2026-05-24T04:01:00.000Z",
              },
            },
      );
    }

    throw new Error(`Unexpected fetch request: ${url}`);
  });

  const { rerender } = render(<App />);

  expect(await screen.findByText("Snapshot loaded")).toBeInTheDocument();

  mockUseTerminalSession.mockReturnValue(completedCommandTerminalState);
  rerender(<App />);

  await waitFor(() =>
    expect(screen.getByText("Updated after git add .")).toBeInTheDocument(),
  );
  expect(screen.getByText("M notes.txt")).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
pnpm --dir apps/web test -- src/test/App.test.tsx -t "renders command attribution after repo state refreshes for a completed command"
```

Expected:

```text
FAIL
Unable to find an element with the text: Updated after git add .
```

- [ ] **Step 3: Implement command-trigger stamping and rendering**

In `apps/web/src/App.tsx`, watch terminal history and promote the latest completed command into the refresh context state:

```tsx
useEffect(() => {
  const latestCompletedCommand = [...terminalSession.history]
    .reverse()
    .find((entry) => entry.phase === "stopped");

  if (!displayedSession || !latestCompletedCommand) {
    return;
  }

  setRepoRefreshContext((current) =>
    current.trigger === "command_complete" &&
    current.commandId === latestCompletedCommand.id &&
    current.commandText === latestCompletedCommand.command
      ? current
      : {
          trigger: "command_complete",
          commandId: latestCompletedCommand.id,
          commandText: latestCompletedCommand.command,
        },
  );
}, [displayedSession, terminalSession.history]);
```

Pass it into `useRepoState`:

```tsx
const repoState = useRepoState({
  session: displayedSession,
  commandHistory: terminalSession.history,
  refreshContext: repoRefreshContext,
});
```

In `apps/web/src/hooks/useRepoState.ts`, preserve the previous attribution when a refresh fails:

```ts
.catch((error: unknown) => {
  if (controller.signal.aborted) {
    return;
  }

  const message = getRepoStateError(error);
  setState((current) =>
    current.snapshot
      ? {
          status: "stale",
          snapshot: current.snapshot,
          error: message,
        }
      : {
          status: "error",
          snapshot: null,
          error: message,
        },
  );
})
```

Do not call `setAttribution` in the failure path.

In `apps/web/src/components/RepoPanel.tsx`, extend attribution copy:

```tsx
if (attribution.trigger === "command_complete" && attribution.commandText) {
  return `Updated after ${attribution.commandText}`;
}
```

Notes for the engineer:
- Keep the existing completed-command detection for refresh triggering; this task adds attribution, not a new refresh trigger.
- Do not overwrite prior attribution when the follow-up fetch fails.

- [ ] **Step 4: Add the failing stale-preservation regression**

Add a second regression that proves a failed command-triggered refresh keeps the previous attribution instead of stamping the failed command onto the card.

```tsx
it("preserves the last successful attribution when a command-triggered refresh fails", async () => {
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
  const failedRefreshState = createTerminalState({
    status: "ready",
    terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
    history: [
      {
        id: "cmd-2",
        command: "git status",
        phase: "stopped",
        summary: "Command finished successfully",
      },
    ],
  });

  mockUseTerminalSession.mockReturnValue(initialTerminalState);

  let repoStateRequestCount = 0;
  mockFetch.mockImplementation((input: RequestInfo | URL) => {
    const url = String(input);

    if (url.endsWith("/api/v1/templates")) {
      return createCatalogResponse();
    }

    if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
      repoStateRequestCount += 1;
      return repoStateRequestCount === 1
        ? createJsonResponse(defaultRepoStatePayload)
        : createErrorResponse(502, "Unable to load repository state.");
    }

    throw new Error(`Unexpected fetch request: ${url}`);
  });

  const { rerender } = render(<App />);

  expect(await screen.findByText("Snapshot loaded")).toBeInTheDocument();

  mockUseTerminalSession.mockReturnValue(failedRefreshState);
  rerender(<App />);

  await waitFor(() =>
    expect(screen.getByText("Repository state may be out of date.")).toBeInTheDocument(),
  );
  expect(screen.getByText("Snapshot loaded")).toBeInTheDocument();
  expect(screen.queryByText("Updated after git status")).not.toBeInTheDocument();
});
```

- [ ] **Step 5: Run the focused command-attribution regression set**

Run:

```bash
pnpm --dir apps/web test -- src/test/App.test.tsx -t "renders command attribution after repo state refreshes for a completed command|preserves the last successful attribution when a command-triggered refresh fails|does not refresh repo state for a running command entry but refreshes when that command completes"
```

Expected:

```text
3 passed
```

- [ ] **Step 6: Commit the command attribution behavior**

```bash
git add apps/web/src/hooks/useRepoState.ts apps/web/src/App.tsx apps/web/src/components/RepoPanel.tsx apps/web/src/test/App.test.tsx
git commit -m "feat: attribute repo snapshots to completed commands"
```

---

### Task 3: Cover lifecycle attribution copy and end-to-end command visibility

**Files:**
- Modify: `apps/web/src/App.tsx`
- Modify: `apps/web/src/components/RepoPanel.tsx`
- Modify: `apps/web/src/test/App.test.tsx`
- Modify: `apps/web/tests/e2e/smoke.spec.ts`

- [ ] **Step 1: Write the failing test for lifecycle attribution after reset**

Add a focused regression proving that a same-session reset refresh uses lifecycle wording rather than command wording.

```tsx
it("renders lifecycle attribution after reset refreshes the current session snapshot", async () => {
  const refresh = vi.fn().mockResolvedValue(reconciledSession);

  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: activeSession,
    absenceReason: null,
    error: null,
    refresh,
  });

  mockUseTerminalSession.mockReturnValue(
    createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [],
    }),
  );

  let repoStateRequestCount = 0;
  mockFetch.mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);

    if (url.endsWith("/api/v1/templates")) {
      return createCatalogResponse();
    }

    if (url.endsWith("/api/v1/practice-sessions/42/reset") && init?.method === "POST") {
      return createJsonResponse({ ok: true });
    }

    if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
      repoStateRequestCount += 1;
      return createJsonResponse({
        data: {
          branch: repoStateRequestCount === 1 ? "main" : "reset/main",
          head_commit:
            repoStateRequestCount === 1
              ? "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
              : "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
          dirty: repoStateRequestCount > 1,
          changed_files: repoStateRequestCount > 1 ? ["M notes.txt"] : [],
          captured_at: `2026-05-24T04:0${repoStateRequestCount}:00.000Z`,
        },
      });
    }

    throw new Error(`Unexpected fetch request: ${url}`);
  });

  render(<App />);

  await screen.findByText("Snapshot loaded");

  fireEvent.click(screen.getByRole("button", { name: "Reset" }));

  await waitFor(() => expect(screen.getByText("Snapshot refreshed after reset")).toBeInTheDocument());
  expect(screen.getByText("reset/main")).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the lifecycle test to verify it fails**

Run:

```bash
pnpm --dir apps/web test -- src/test/App.test.tsx -t "renders lifecycle attribution after reset refreshes the current session snapshot"
```

Expected:

```text
FAIL
Unable to find an element with the text: Snapshot refreshed after reset
```

- [ ] **Step 3: Implement lifecycle attribution mapping**

In `apps/web/src/App.tsx`, update the repo refresh context before calling `currentSession.refresh()` from lifecycle paths.

For same-session reconciliation paths, set lifecycle triggers explicitly:

```tsx
const [repoRefreshContext, setRepoRefreshContext] = useState<RepoRefreshContext>({
  trigger: "session_load",
});
```

Before `reconcileSessionAction("reset", ...)` refreshes:

```tsx
setRepoRefreshContext({ trigger: "session_reset" });
```

Before retry-sync refreshes:

```tsx
setRepoRefreshContext({ trigger: "session_sync" });
```

Before creating a new session and reconciling it:

```tsx
setRepoRefreshContext({ trigger: "session_create" });
```

In `apps/web/src/components/RepoPanel.tsx`, extend the copy mapper:

```tsx
if (attribution.trigger === "session_create") {
  return "Snapshot refreshed after new session";
}
if (attribution.trigger === "session_reset") {
  return "Snapshot refreshed after reset";
}
if (attribution.trigger === "session_sync") {
  return "Snapshot refreshed after sync";
}
```

Notes for the engineer:
- Only stamp these lifecycle triggers onto successful repo-state fetches.
- When a command completion later refreshes repo state successfully, it should replace the prior lifecycle attribution as the current cause.

- [ ] **Step 4: Add the e2e assertion for command attribution**

In `apps/web/tests/e2e/smoke.spec.ts`, extend the existing repo-snapshot smoke so it verifies attribution text after a mutating command.

```ts
test("updates the repo state card after a mutating terminal command completes", async ({
  page,
}) => {
  await page.route("**/api/v1/practice-sessions/current", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(activeSessionPayload),
    });
  });

  await page.goto("/");

  await expect(page.getByText("Snapshot loaded")).toBeVisible();

  repoStateDirty = true;
  await page.getByLabel("Interactive terminal").pressSequentially("git add notes.txt");
  await page.getByLabel("Interactive terminal").press("Enter");

  await expect(page.getByText("M notes.txt")).toBeVisible();
  await expect(page.getByText("Updated after git add notes.txt")).toBeVisible();
});
```

Notes for the engineer:
- Reuse the existing terminal stub and route counters.
- Do not add a second e2e scenario; extend the current smoke coverage only.

- [ ] **Step 5: Run the full web verification**

Run:

```bash
pnpm --dir apps/web test
pnpm --dir apps/web run test:e2e -- --grep "updates the repo state card after a mutating terminal command completes"
```

Expected:

```text
PASS
PASS
```

- [ ] **Step 6: Commit the lifecycle copy and e2e coverage**

```bash
git add apps/web/src/App.tsx apps/web/src/components/RepoPanel.tsx apps/web/src/test/App.test.tsx apps/web/tests/e2e/smoke.spec.ts
git commit -m "test: cover repo snapshot attribution ux"
```
