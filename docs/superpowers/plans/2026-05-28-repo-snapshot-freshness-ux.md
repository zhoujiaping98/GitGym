# Repo Snapshot Freshness UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show when the visible repo snapshot was captured so the right-side repository card communicates both cause and freshness.

**Architecture:** Keep backend data and refresh behavior unchanged. Add a small presentation helper for `capturedAt`, render one `Captured ...` line for successful snapshots, and preserve that line when the card falls back to stale state after a failed refresh.

**Tech Stack:** React, TypeScript, Vitest, Testing Library

---

## File Map

- Modify: `apps/web/src/components/RepoPanel.tsx`
- Modify: `apps/web/src/test/App.test.tsx`

## Task 1: Add freshness rendering for successful snapshots under TDD

**Files:**
- Modify: `apps/web/src/components/RepoPanel.tsx`
- Modify: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Write the failing ready-state freshness regressions**

Add these tests in `apps/web/src/test/App.test.tsx` near the existing repo snapshot attribution tests:

```ts
it("renders freshness copy for the initial repo snapshot load", async () => {
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
  expect(within(sessionCard).getByText("Captured May 23, 12:00 PM")).toBeInTheDocument();
});

it("updates freshness copy when a command-triggered refresh returns a newer snapshot", async () => {
  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: activeSession,
    absenceReason: null,
    error: null,
    refresh: vi.fn().mockResolvedValue(activeSession),
  });

  const terminalState = createTerminalState({
    status: "ready",
    terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
    history: [],
  });
  mockUseTerminalSession.mockReturnValue(terminalState);

  let repoStateRequestCount = 0;
  mockFetch.mockImplementation((input: RequestInfo | URL) => {
    const url = String(input);

    if (url.endsWith("/api/v1/templates")) {
      return createCatalogResponse();
    }

    if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
      repoStateRequestCount += 1;
      return createJsonResponse(
        repoStateRequestCount === 1
          ? defaultRepoStatePayload
          : {
              data: {
                ...defaultRepoStatePayload.data,
                dirty: true,
                changed_files: ["M notes.txt"],
                captured_at: "2026-05-23T04:02:00.000Z",
              },
            },
      );
    }

    throw new Error(`Unexpected fetch request: ${url}`);
  });

  const { rerender } = render(<App />);
  const sessionCard = await screen.findByLabelText("Operational session card");
  expect(await within(sessionCard).findByText("Captured May 23, 12:00 PM")).toBeInTheDocument();

  mockUseTerminalSession.mockReturnValue({
    ...terminalState,
    history: [
      {
        id: "cmd-1",
        command: "git add .",
        phase: "stopped",
        executedAt: "2026-05-23T04:01:30.000Z",
        exitCode: 0,
      },
    ],
  });

  rerender(<App />);

  await waitFor(() => {
    expect(within(sessionCard).getByText("Updated after git add .")).toBeInTheDocument();
  });
  expect(within(sessionCard).getByText("Captured May 23, 12:02 PM")).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the focused freshness tests to verify they fail**

Run: `pnpm --dir apps/web test -- src/test/App.test.tsx -t "renders freshness copy for the initial repo snapshot load|updates freshness copy when a command-triggered refresh returns a newer snapshot"`

Expected: FAIL because `RepoPanel` does not render freshness text yet.

- [ ] **Step 3: Add minimal freshness formatting and rendering**

Update `apps/web/src/components/RepoPanel.tsx`:

```tsx
function formatDate(value: string) {
  return new Intl.DateTimeFormat("en", {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  }).format(new Date(value));
}

function repoFreshnessCopy(capturedAt: string | null) {
  if (!capturedAt) {
    return null;
  }

  return `Captured ${formatDate(capturedAt)}`;
}

const freshnessCopy =
  repoState.status === "ready" || repoState.status === "stale"
    ? repoFreshnessCopy(repoState.snapshot.capturedAt)
    : null;

{attributionCopy ? <span className="repo-state-inline-note">{attributionCopy}</span> : null}
{freshnessCopy ? <span className="repo-state-inline-note">{freshnessCopy}</span> : null}
```

Keep the freshness line separate from attribution copy.

- [ ] **Step 4: Re-run the focused freshness tests to verify they pass**

Run: `pnpm --dir apps/web test -- src/test/App.test.tsx -t "renders freshness copy for the initial repo snapshot load|updates freshness copy when a command-triggered refresh returns a newer snapshot"`

Expected: PASS

- [ ] **Step 5: Commit the ready-state freshness behavior**

```bash
git add apps/web/src/components/RepoPanel.tsx apps/web/src/test/App.test.tsx
git commit -m "feat: show repo snapshot freshness"
```

## Task 2: Preserve freshness through stale fallback and verify the full suite

**Files:**
- Modify: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Write the failing stale-state freshness regression**

Add this test near the existing stale snapshot coverage:

```ts
it("keeps rendering the last captured timestamp when the repo snapshot becomes stale", async () => {
  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: activeSession,
    absenceReason: null,
    error: null,
    refresh: vi.fn().mockResolvedValue(activeSession),
  });

  const terminalState = createTerminalState({
    status: "ready",
    terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
    history: [],
  });
  mockUseTerminalSession.mockReturnValue(terminalState);

  let repoStateRequestCount = 0;
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

      return Promise.reject(new Error("repo refresh failed"));
    }

    throw new Error(`Unexpected fetch request: ${url}`);
  });

  const { rerender } = render(<App />);
  const sessionCard = await screen.findByLabelText("Operational session card");
  expect(await within(sessionCard).findByText("Captured May 23, 12:00 PM")).toBeInTheDocument();

  mockUseTerminalSession.mockReturnValue({
    ...terminalState,
    history: [
      {
        id: "cmd-2",
        command: "git status",
        phase: "stopped",
        executedAt: "2026-05-23T04:03:00.000Z",
        exitCode: 0,
      },
    ],
  });

  rerender(<App />);

  await waitFor(() => {
    expect(
      within(sessionCard).getByText("Repository state may be out of date."),
    ).toBeInTheDocument();
  });
  expect(within(sessionCard).getByText("Captured May 23, 12:00 PM")).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the stale freshness regression to verify it fails if needed**

Run: `pnpm --dir apps/web test -- src/test/App.test.tsx -t "keeps rendering the last captured timestamp when the repo snapshot becomes stale"`

Expected: If Task 1 did not already cover stale rendering correctly, FAIL until the panel renders freshness during stale state.

- [ ] **Step 3: Adjust the panel only if stale rendering still fails**

If the stale freshness regression fails, keep `freshnessCopy` derived from `repoState.snapshot.capturedAt` for both `ready` and `stale` states and ensure it renders alongside the stale warning.

- [ ] **Step 4: Run the focused freshness regression set**

Run:

```bash
pnpm --dir apps/web test -- src/test/App.test.tsx -t "renders freshness copy for the initial repo snapshot load|updates freshness copy when a command-triggered refresh returns a newer snapshot|keeps rendering the last captured timestamp when the repo snapshot becomes stale"
```

Expected: PASS

- [ ] **Step 5: Run the full web test suite**

Run: `pnpm --dir apps/web test`

Expected: PASS

- [ ] **Step 6: Commit the stale freshness coverage**

```bash
git add apps/web/src/test/App.test.tsx
git commit -m "test: cover repo snapshot freshness ux"
```

## Final Verification

- [ ] Run `git status --short`
- [ ] Confirm the branch is clean
- [ ] Summarize changed behavior and test evidence before presenting finish options
