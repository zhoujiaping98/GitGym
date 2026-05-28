# Repo Snapshot Command Outcome UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a concise repo-state outcome line so the card can tell users what the last completed command changed in repository state.

**Architecture:** Keep backend contracts unchanged. Add a small web-only comparison helper that derives outcome copy from two successful snapshots, have `useRepoState` preserve the last successful outcome for command-driven refreshes, and keep `RepoPanel` presentational by rendering an already-computed string.

**Tech Stack:** React, TypeScript, Vitest, Testing Library

---

## File Map

- Create: `apps/web/src/lib/repoOutcome.ts`
- Create: `apps/web/src/lib/repoOutcome.test.ts`
- Modify: `apps/web/src/hooks/useRepoState.ts`
- Modify: `apps/web/src/components/RepoPanel.tsx`
- Modify: `apps/web/src/test/App.test.tsx`

## Task 1: Add repo outcome derivation under TDD

**Files:**
- Create: `apps/web/src/lib/repoOutcome.ts`
- Create: `apps/web/src/lib/repoOutcome.test.ts`

- [ ] **Step 1: Write the failing repo-outcome helper tests**

Create `apps/web/src/lib/repoOutcome.test.ts` with:

```ts
import { describe, expect, it } from "vitest";
import type { RepoStateSnapshot } from "../types";
import { repoOutcomeCopy } from "./repoOutcome";

function snapshot(
  overrides: Partial<RepoStateSnapshot> = {},
): RepoStateSnapshot {
  return {
    branch: "main",
    headCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    dirty: false,
    changedFiles: [],
    capturedAt: "2026-05-23T04:00:00.000Z",
    ...overrides,
  };
}

describe("repoOutcomeCopy", () => {
  it("reports when the working tree becomes dirty", () => {
    expect(
      repoOutcomeCopy(
        snapshot({ dirty: false, changedFiles: [] }),
        snapshot({ dirty: true, changedFiles: ["M notes.txt"] }),
      ),
    ).toBe("Working tree became dirty.");
  });

  it("reports when the working tree becomes clean", () => {
    expect(
      repoOutcomeCopy(
        snapshot({ dirty: true, changedFiles: ["M notes.txt"] }),
        snapshot({ dirty: false, changedFiles: [] }),
      ),
    ).toBe("Working tree is now clean.");
  });

  it("reports changed-file count deltas when dirty state does not flip", () => {
    expect(
      repoOutcomeCopy(
        snapshot({ dirty: true, changedFiles: ["M one.txt"] }),
        snapshot({ dirty: true, changedFiles: ["M one.txt", "M two.txt", "?? draft.md"] }),
      ),
    ).toBe("Changed files: 1 -> 3.");
  });

  it("reports branch changes ahead of count-only changes", () => {
    expect(
      repoOutcomeCopy(
        snapshot({ branch: "main", dirty: true, changedFiles: ["M one.txt"] }),
        snapshot({ branch: "feature/demo", dirty: true, changedFiles: ["M one.txt"] }),
      ),
    ).toBe("Branch changed: main -> feature/demo.");
  });

  it("reports head changes when branch and change count stay the same", () => {
    expect(
      repoOutcomeCopy(
        snapshot({ headCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", dirty: false }),
        snapshot({ headCommit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", dirty: false }),
      ),
    ).toBe("HEAD changed.");
  });

  it("returns null when the snapshots are meaningfully unchanged", () => {
    expect(repoOutcomeCopy(snapshot(), snapshot())).toBeNull();
  });
});
```

- [ ] **Step 2: Run the helper tests to verify they fail**

Run: `pnpm --dir apps/web test -- src/lib/repoOutcome.test.ts`

Expected: FAIL because `repoOutcome.ts` does not exist yet.

- [ ] **Step 3: Implement the minimal comparison helper**

Create `apps/web/src/lib/repoOutcome.ts` with:

```ts
import type { RepoStateSnapshot } from "../types";

export function repoOutcomeCopy(
  previous: RepoStateSnapshot,
  current: RepoStateSnapshot,
) {
  if (!previous.dirty && current.dirty) {
    return "Working tree became dirty.";
  }

  if (previous.dirty && !current.dirty) {
    return "Working tree is now clean.";
  }

  if (previous.branch !== current.branch) {
    return `Branch changed: ${previous.branch} -> ${current.branch}.`;
  }

  if (previous.changedFiles.length !== current.changedFiles.length) {
    return `Changed files: ${previous.changedFiles.length} -> ${current.changedFiles.length}.`;
  }

  if (previous.headCommit !== current.headCommit) {
    return "HEAD changed.";
  }

  return null;
}
```

- [ ] **Step 4: Run the helper tests to verify they pass**

Run: `pnpm --dir apps/web test -- src/lib/repoOutcome.test.ts`

Expected: PASS

- [ ] **Step 5: Commit the outcome helper**

```bash
git add apps/web/src/lib/repoOutcome.ts apps/web/src/lib/repoOutcome.test.ts
git commit -m "feat: derive repo snapshot command outcomes"
```

## Task 2: Surface command outcomes through the hook and panel under TDD

**Files:**
- Modify: `apps/web/src/hooks/useRepoState.ts`
- Modify: `apps/web/src/components/RepoPanel.tsx`
- Modify: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Write the failing command-outcome RTL regressions**

Add these tests in `apps/web/src/test/App.test.tsx` near the command attribution and stale repo-state tests:

```ts
it("renders a command outcome when the working tree becomes dirty", async () => {
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
        id: "cmd-dirty",
        command: "touch notes.txt",
        phase: "stopped",
        exitCode: 0,
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
      return createJsonResponse(
        repoStateRequestCount === 1
          ? defaultRepoStatePayload
          : {
              data: {
                ...defaultRepoStatePayload.data,
                dirty: true,
                changed_files: ["?? notes.txt"],
                captured_at: "2026-05-23T04:01:00.000Z",
              },
            },
      );
    }

    throw new Error(`Unexpected fetch request: ${url}`);
  });

  const { rerender } = render(<App />);
  const sessionCard = await screen.findByLabelText("Operational session card");

  mockUseTerminalSession.mockReturnValue(completedCommandTerminalState);
  rerender(<App />);

  await waitFor(() => {
    expect(within(sessionCard).getByText("Working tree became dirty.")).toBeInTheDocument();
  });
});

it("renders a changed-files count delta when dirty state does not flip", async () => {
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
        id: "cmd-count",
        command: "touch two.txt",
        phase: "stopped",
        exitCode: 0,
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
      return createJsonResponse(
        repoStateRequestCount === 1
          ? {
              data: {
                ...defaultRepoStatePayload.data,
                dirty: true,
                changed_files: ["M one.txt"],
              },
            }
          : {
              data: {
                ...defaultRepoStatePayload.data,
                dirty: true,
                changed_files: ["M one.txt", "M two.txt", "?? draft.md"],
                captured_at: "2026-05-23T04:02:00.000Z",
              },
            },
      );
    }

    throw new Error(`Unexpected fetch request: ${url}`);
  });

  const { rerender } = render(<App />);
  const sessionCard = await screen.findByLabelText("Operational session card");

  mockUseTerminalSession.mockReturnValue(completedCommandTerminalState);
  rerender(<App />);

  await waitFor(() => {
    expect(within(sessionCard).getByText("Changed files: 1 -> 3.")).toBeInTheDocument();
  });
});

it("preserves the last successful command outcome when a later command refresh fails", async () => {
  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: activeSession,
    absenceReason: null,
    error: null,
    refresh: vi.fn().mockResolvedValue(activeSession),
  });

  const cleanState = createTerminalState({
    status: "ready",
    terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
    history: [],
  });
  const firstCommandState = createTerminalState({
    status: "ready",
    terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
    history: [
      { id: "cmd-1", command: "touch notes.txt", phase: "stopped", exitCode: 0 },
    ],
  });
  const secondCommandState = createTerminalState({
    status: "ready",
    terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
    history: [
      { id: "cmd-1", command: "touch notes.txt", phase: "stopped", exitCode: 0 },
      { id: "cmd-2", command: "git status", phase: "stopped", exitCode: 0 },
    ],
  });

  mockUseTerminalSession.mockReturnValue(cleanState);

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
      if (repoStateRequestCount === 2) {
        return createJsonResponse({
          data: {
            ...defaultRepoStatePayload.data,
            dirty: true,
            changed_files: ["?? notes.txt"],
            captured_at: "2026-05-23T04:01:00.000Z",
          },
        });
      }
      return createErrorResponse(502, "Unable to load repository state.");
    }

    throw new Error(`Unexpected fetch request: ${url}`);
  });

  const { rerender } = render(<App />);
  const sessionCard = await screen.findByLabelText("Operational session card");

  mockUseTerminalSession.mockReturnValue(firstCommandState);
  rerender(<App />);

  await waitFor(() => {
    expect(within(sessionCard).getByText("Working tree became dirty.")).toBeInTheDocument();
  });

  mockUseTerminalSession.mockReturnValue(secondCommandState);
  rerender(<App />);

  await waitFor(() => {
    expect(
      within(sessionCard).getByText("Repository state may be out of date."),
    ).toBeInTheDocument();
  });
  expect(within(sessionCard).getByText("Working tree became dirty.")).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the focused App tests to verify they fail**

Run: `pnpm --dir apps/web test -- src/test/App.test.tsx -t "renders a command outcome when the working tree becomes dirty|renders a changed-files count delta when dirty state does not flip|preserves the last successful command outcome when a later command refresh fails"`

Expected: FAIL because the hook and panel do not derive or render command outcomes yet.

- [ ] **Step 3: Thread repo outcomes through `useRepoState`**

Update `apps/web/src/hooks/useRepoState.ts`:

```ts
import { repoOutcomeCopy } from "../lib/repoOutcome";

type UseRepoStateResult = {
  repoState: RepoStateView;
  repoAttribution: RepoAttribution | null;
  repoOutcome: string | null;
};

const [outcome, setOutcome] = useState<string | null>(null);

if (!session) {
  setOutcome(null);
}

void fetchPracticeRepoState(session.id, controller.signal)
  .then((snapshot) => {
    setState((current) => {
      const previousSnapshot = current.snapshot;
      setOutcome((previousOutcome) => {
        if (refreshContext.trigger !== "command_complete" || !previousSnapshot) {
          return previousOutcome;
        }

        return repoOutcomeCopy(previousSnapshot, snapshot);
      });

      return {
        status: "ready",
        snapshot,
        error: null,
      };
    });

    if (refreshContext.trigger !== "command_complete") {
      setOutcome(null);
    }

    setAttribution({
      trigger: refreshContext.trigger,
      capturedAt: snapshot.capturedAt,
      commandId: refreshContext.commandId,
      commandText: refreshContext.commandText,
    });
  })
```

Use the previous successful snapshot only. On fetch failure, do not overwrite `outcome`.

- [ ] **Step 4: Render the outcome line in `RepoPanel`**

Update `apps/web/src/components/RepoPanel.tsx`:

```tsx
type RepoPanelProps = {
  ...
  repoOutcome?: string | null;
};

export function RepoPanel({
  ...
  repoOutcome = null,
}: RepoPanelProps) {
  ...
  {freshnessCopy ? <span className="repo-state-inline-note">{freshnessCopy}</span> : null}
  {repoOutcome ? <span className="repo-state-inline-note">{repoOutcome}</span> : null}
}
```

Keep it as a separate line and do not merge it into attribution or freshness copy.

- [ ] **Step 5: Re-run the focused App tests to verify they pass**

Run: `pnpm --dir apps/web test -- src/test/App.test.tsx -t "renders a command outcome when the working tree becomes dirty|renders a changed-files count delta when dirty state does not flip|preserves the last successful command outcome when a later command refresh fails"`

Expected: PASS

- [ ] **Step 6: Run the full web suite**

Run: `pnpm --dir apps/web test`

Expected: PASS

- [ ] **Step 7: Commit the UI wiring and regressions**

```bash
git add apps/web/src/hooks/useRepoState.ts apps/web/src/components/RepoPanel.tsx apps/web/src/test/App.test.tsx
git commit -m "feat: show repo snapshot command outcomes"
```

## Final Verification

- [ ] Run `git status --short`
- [ ] Confirm the branch is clean
- [ ] Summarize changed behavior and test evidence before merging or presenting finish options
