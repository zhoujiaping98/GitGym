# Changed Files Expand/Collapse UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users expand long changed-file sections in the repo card and collapse them again without changing the current summary-first default.

**Architecture:** Keep backend contracts unchanged. Extend the repo change summary helper so the panel can render either truncated or full rows from the same grouped data, then add local expand/collapse state in `RepoPanel` that resets when a newer repo snapshot arrives.

**Tech Stack:** React, TypeScript, Vitest, Testing Library

---

## File Map

- Modify: `apps/web/src/lib/repoChangeSummary.ts`
- Modify: `apps/web/src/lib/repoChangeSummary.test.ts`
- Modify: `apps/web/src/components/RepoPanel.tsx`
- Modify: `apps/web/src/styles.css`
- Modify: `apps/web/src/test/App.test.tsx`

## Task 1: Expose full repo-change rows alongside summary metadata under TDD

**Files:**
- Modify: `apps/web/src/lib/repoChangeSummary.ts`
- Modify: `apps/web/src/lib/repoChangeSummary.test.ts`

- [ ] **Step 1: Write the failing helper regressions for collapsed vs full rows**

Update `apps/web/src/lib/repoChangeSummary.test.ts` with two new assertions:

```ts
it("keeps the full group entries available alongside the collapsed visible rows", () => {
  const summary = summarizeRepoChanges(
    groups({
      unstaged: [
        { key: "1", bucket: "unstaged", label: "Modified", path: "a.txt" },
        { key: "2", bucket: "unstaged", label: "Modified", path: "b.txt" },
        { key: "3", bucket: "unstaged", label: "Modified", path: "c.txt" },
        { key: "4", bucket: "unstaged", label: "Modified", path: "d.txt" },
      ],
    }),
  );

  expect(summary.groups[0].visible.map((entry) => entry.path)).toEqual([
    "a.txt",
    "b.txt",
    "c.txt",
  ]);
  expect(summary.groups[0].all.map((entry) => entry.path)).toEqual([
    "a.txt",
    "b.txt",
    "c.txt",
    "d.txt",
  ]);
});

it("keeps the full fallback rows available alongside the collapsed visible rows", () => {
  const summary = summarizeRepoChanges(
    groups({
      fallback: ["!! one", "!! two", "!! three", "!! four"],
    }),
  );

  expect(summary.fallback.visible).toEqual(["!! one", "!! two", "!! three"]);
  expect(summary.fallback.all).toEqual(["!! one", "!! two", "!! three", "!! four"]);
});
```

- [ ] **Step 2: Run the helper tests to verify they fail**

Run: `pnpm --dir apps/web test -- src/lib/repoChangeSummary.test.ts`

Expected: FAIL because the summary types do not expose `all` rows yet.

- [ ] **Step 3: Implement the minimal helper shape extension**

Update `apps/web/src/lib/repoChangeSummary.ts`:

```ts
export type SummarizedRepoChangeGroup = {
  title: "Staged" | "Unstaged" | "Untracked";
  count: number;
  all: RepoChangeEntry[];
  visible: RepoChangeEntry[];
  hiddenCount: number;
};

export type SummarizedFallbackRows = {
  all: string[];
  visible: string[];
  hiddenCount: number;
};
```

Return the full rows without changing the existing collapsed defaults:

```ts
function summarizeFallback(lines: string[]): SummarizedFallbackRows {
  return {
    all: lines,
    visible: lines.slice(0, MAX_VISIBLE_REPO_CHANGES),
    hiddenCount: Math.max(lines.length - MAX_VISIBLE_REPO_CHANGES, 0),
  };
}
```

And in grouped sections:

```ts
return {
  title: group.title,
  count: group.entries.length,
  all: group.entries,
  visible: summarized.visible,
  hiddenCount: summarized.hiddenCount,
};
```

- [ ] **Step 4: Re-run the helper tests to verify they pass**

Run: `pnpm --dir apps/web test -- src/lib/repoChangeSummary.test.ts`

Expected: PASS

- [ ] **Step 5: Commit the summary data-shape extension**

```bash
git add apps/web/src/lib/repoChangeSummary.ts apps/web/src/lib/repoChangeSummary.test.ts
git commit -m "feat: expose full repo change rows for expansion"
```

## Task 2: Add per-section expand/collapse behavior in the repo card under TDD

**Files:**
- Modify: `apps/web/src/components/RepoPanel.tsx`
- Modify: `apps/web/src/styles.css`
- Modify: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Write the failing repo-card expand/collapse regressions**

Add these tests near the existing changed-files summary tests in `apps/web/src/test/App.test.tsx`:

```ts
it("expands and collapses a grouped changed-file section", async () => {
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
    }),
  );

  mockFetch.mockImplementation((input: RequestInfo | URL) => {
    const url = String(input);

    if (url.endsWith("/api/v1/templates")) {
      return createCatalogResponse();
    }

    if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
      return createJsonResponse({
        data: {
          ...defaultRepoStatePayload.data,
          dirty: true,
          changed_files: ["M  one.txt", "M  two.txt", "M  three.txt", "M  four.txt"],
        },
      });
    }

    throw new Error(`Unexpected fetch request: ${url}`);
  });

  render(<App />);

  const sessionCard = await screen.findByLabelText("Operational session card");
  expect(within(sessionCard).queryByText("four.txt")).not.toBeInTheDocument();

  fireEvent.click(within(sessionCard).getByRole("button", { name: "Show 1 more" }));

  expect(within(sessionCard).getByText("four.txt")).toBeInTheDocument();
  expect(within(sessionCard).getByRole("button", { name: "Show less" })).toBeInTheDocument();

  fireEvent.click(within(sessionCard).getByRole("button", { name: "Show less" }));

  expect(within(sessionCard).queryByText("four.txt")).not.toBeInTheDocument();
});

it("expands and collapses fallback raw rows", async () => {
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
    }),
  );

  mockFetch.mockImplementation((input: RequestInfo | URL) => {
    const url = String(input);

    if (url.endsWith("/api/v1/templates")) {
      return createCatalogResponse();
    }

    if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
      return createJsonResponse({
        data: {
          ...defaultRepoStatePayload.data,
          dirty: true,
          changed_files: ["!! one", "!! two", "!! three", "!! four"],
        },
      });
    }

    throw new Error(`Unexpected fetch request: ${url}`);
  });

  render(<App />);

  const sessionCard = await screen.findByLabelText("Operational session card");
  expect(within(sessionCard).queryByText("!! four")).not.toBeInTheDocument();

  fireEvent.click(within(sessionCard).getByRole("button", { name: "Show 1 more" }));

  expect(within(sessionCard).getByText("!! four")).toBeInTheDocument();
  expect(within(sessionCard).getByRole("button", { name: "Show less" })).toBeInTheDocument();
});

it("resets expanded changed-file sections when a newer snapshot replaces the current one", async () => {
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
    history: [{ id: "cmd-1", command: "git add .", phase: "stopped", exitCode: 0 }],
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
      return createJsonResponse({
        data:
          repoStateRequestCount === 1
            ? {
                ...defaultRepoStatePayload.data,
                dirty: true,
                changed_files: ["M  one.txt", "M  two.txt", "M  three.txt", "M  four.txt"],
              }
            : {
                ...defaultRepoStatePayload.data,
                dirty: true,
                changed_files: ["M  one.txt", "M  two.txt", "M  three.txt", "M  five.txt"],
                captured_at: "2026-05-23T04:06:00.000Z",
              },
      });
    }

    throw new Error(`Unexpected fetch request: ${url}`);
  });

  const { rerender } = render(<App />);
  const sessionCard = await screen.findByLabelText("Operational session card");

  fireEvent.click(within(sessionCard).getByRole("button", { name: "Show 1 more" }));
  expect(within(sessionCard).getByText("four.txt")).toBeInTheDocument();

  mockUseTerminalSession.mockReturnValue(completedCommandTerminalState);
  rerender(<App />);

  await waitFor(() => {
    expect(within(sessionCard).getByText("Updated after git add .")).toBeInTheDocument();
  });
  expect(within(sessionCard).queryByText("five.txt")).not.toBeInTheDocument();
  expect(within(sessionCard).getByRole("button", { name: "Show 1 more" })).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the focused repo-card regressions to verify they fail**

Run: `pnpm --dir apps/web test -- src/test/App.test.tsx -t "expands and collapses a grouped changed-file section|expands and collapses fallback raw rows|resets expanded changed-file sections when a newer snapshot replaces the current one"`

Expected: FAIL because the repo card does not yet render expand/collapse controls or local expansion state.

- [ ] **Step 3: Add local expansion state and toggle rendering in `RepoPanel`**

Update `apps/web/src/components/RepoPanel.tsx` to import and use local state:

```ts
import { useEffect, useState } from "react";
```

Add a local expansion map:

```ts
const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>({});
```

Reset it when the successful snapshot changes:

```ts
useEffect(() => {
  if (repoState.status === "ready" || repoState.status === "stale") {
    setExpandedSections({});
    return;
  }

  setExpandedSections({});
}, [repoState.status, repoState.status === "ready" || repoState.status === "stale" ? repoState.snapshot.capturedAt : null]);
```

Add a render helper that supports collapsed vs expanded rows:

```tsx
function renderToggleButton(
  expanded: boolean,
  hiddenCount: number,
  onToggle: () => void,
) {
  return (
    <button className="repo-state-change-toggle" onClick={onToggle} type="button">
      {expanded ? "Show less" : `Show ${hiddenCount} more`}
    </button>
  );
}
```

Update grouped rendering to use `group.all` when expanded and `group.visible` when collapsed:

```tsx
const expanded = expandedSections[group.title.toLowerCase()] ?? false;
const rows = expanded ? group.all : group.visible;
```

Render the toggle button when `group.hiddenCount > 0 || expanded`.

Do the same for fallback rows using the key `fallback`.

- [ ] **Step 4: Add compact toggle styling**

Update `apps/web/src/styles.css` with:

```css
.repo-state-change-toggle {
  width: fit-content;
  border: 0;
  padding: 0;
  background: transparent;
  color: rgba(240, 246, 255, 0.78);
  font-family: "Consolas", "SFMono-Regular", "Liberation Mono", monospace;
  font-size: 0.74rem;
  cursor: pointer;
}

.repo-state-change-toggle:hover {
  color: rgba(240, 246, 255, 0.94);
}
```

If needed, add a small container spacing rule near `.repo-state-change-group`.

- [ ] **Step 5: Re-run the focused repo-card regressions to verify they pass**

Run: `pnpm --dir apps/web test -- src/test/App.test.tsx -t "expands and collapses a grouped changed-file section|expands and collapses fallback raw rows|resets expanded changed-file sections when a newer snapshot replaces the current one"`

Expected: PASS

- [ ] **Step 6: Run the full web suite**

Run: `pnpm --dir apps/web test`

Expected: PASS

- [ ] **Step 7: Commit the repo-card expand/collapse UX**

```bash
git add apps/web/src/components/RepoPanel.tsx apps/web/src/styles.css apps/web/src/test/App.test.tsx
git commit -m "feat: add changed files expand collapse ux"
```

## Final Verification

- [ ] Run `git status --short`
- [ ] Confirm the branch is clean
- [ ] Summarize changed behavior and test evidence before merging or presenting finish options
