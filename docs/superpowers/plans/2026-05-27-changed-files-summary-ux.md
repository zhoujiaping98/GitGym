# Changed Files Summary UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep the repo snapshot card compact by adding per-group counts and truncating long changed-file lists into `+N more` summary rows.

**Architecture:** Keep the existing `groupRepoChanges()` parser as the full-fidelity source of truth, then add a small web-only summarization helper that shapes parsed groups into capped display sections. `RepoPanel` renders the summarized model and existing repo snapshot tests expand to cover both counts and truncation behavior without changing any backend contract.

**Tech Stack:** React, TypeScript, Vitest, Testing Library, CSS

---

## File Map

- Create: `apps/web/src/lib/repoChangeSummary.ts`
- Create: `apps/web/src/lib/repoChangeSummary.test.ts`
- Modify: `apps/web/src/components/RepoPanel.tsx`
- Modify: `apps/web/src/styles.css`
- Modify: `apps/web/src/test/App.test.tsx`

## Task 1: Add a summarized changed-files display model under TDD

**Files:**
- Create: `apps/web/src/lib/repoChangeSummary.ts`
- Create: `apps/web/src/lib/repoChangeSummary.test.ts`
- Modify: `apps/web/src/components/RepoPanel.tsx`

- [ ] **Step 1: Write the failing summary-helper tests**

Create `apps/web/src/lib/repoChangeSummary.test.ts` with:

```ts
import { describe, expect, it } from "vitest";
import type { RepoChangeGroups } from "../types";
import { summarizeRepoChanges } from "./repoChangeSummary";

function groups(overrides: Partial<RepoChangeGroups> = {}): RepoChangeGroups {
  return {
    staged: [],
    unstaged: [],
    untracked: [],
    fallback: [],
    ...overrides,
  };
}

describe("summarizeRepoChanges", () => {
  it("keeps all rows visible when a group has three or fewer entries", () => {
    const summary = summarizeRepoChanges(
      groups({
        staged: [
          { key: "1", bucket: "staged", label: "Modified", path: "a.txt" },
          { key: "2", bucket: "staged", label: "Added", path: "b.txt" },
          { key: "3", bucket: "staged", label: "Deleted", path: "c.txt" },
        ],
      }),
    );

    expect(summary.groups[0].title).toBe("Staged");
    expect(summary.groups[0].count).toBe(3);
    expect(summary.groups[0].visible).toHaveLength(3);
    expect(summary.groups[0].hiddenCount).toBe(0);
  });

  it("caps visible rows at three and reports the hidden remainder", () => {
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

    expect(summary.groups[0].title).toBe("Unstaged");
    expect(summary.groups[0].count).toBe(4);
    expect(summary.groups[0].visible.map((entry) => entry.path)).toEqual([
      "a.txt",
      "b.txt",
      "c.txt",
    ]);
    expect(summary.groups[0].hiddenCount).toBe(1);
  });

  it("caps fallback rows with the same hidden remainder rule", () => {
    const summary = summarizeRepoChanges(
      groups({
        fallback: ["!! one", "!! two", "!! three", "!! four"],
      }),
    );

    expect(summary.fallback.visible).toEqual(["!! one", "!! two", "!! three"]);
    expect(summary.fallback.hiddenCount).toBe(1);
  });
});
```

- [ ] **Step 2: Run the helper tests to verify they fail**

Run: `pnpm --dir apps/web test -- src/lib/repoChangeSummary.test.ts`

Expected: FAIL because `repoChangeSummary.ts` does not exist yet.

- [ ] **Step 3: Implement the minimal summary helper**

Create `apps/web/src/lib/repoChangeSummary.ts` with:

```ts
import type { RepoChangeEntry, RepoChangeGroups } from "../types";

const MAX_VISIBLE_REPO_CHANGES = 3;

export type SummarizedRepoChangeGroup = {
  title: "Staged" | "Unstaged" | "Untracked";
  count: number;
  visible: RepoChangeEntry[];
  hiddenCount: number;
};

export type SummarizedFallbackRows = {
  visible: string[];
  hiddenCount: number;
};

export type SummarizedRepoChanges = {
  groups: SummarizedRepoChangeGroup[];
  fallback: SummarizedFallbackRows;
};

function summarizeEntries(entries: RepoChangeEntry[]) {
  return {
    visible: entries.slice(0, MAX_VISIBLE_REPO_CHANGES),
    hiddenCount: Math.max(entries.length - MAX_VISIBLE_REPO_CHANGES, 0),
  };
}

function summarizeFallback(lines: string[]): SummarizedFallbackRows {
  return {
    visible: lines.slice(0, MAX_VISIBLE_REPO_CHANGES),
    hiddenCount: Math.max(lines.length - MAX_VISIBLE_REPO_CHANGES, 0),
  };
}

export function summarizeRepoChanges(groups: RepoChangeGroups): SummarizedRepoChanges {
  const orderedGroups: Array<{
    title: SummarizedRepoChangeGroup["title"];
    entries: RepoChangeEntry[];
  }> = [
    { title: "Staged", entries: groups.staged },
    { title: "Unstaged", entries: groups.unstaged },
    { title: "Untracked", entries: groups.untracked },
  ];

  return {
    groups: orderedGroups
      .filter((group) => group.entries.length > 0)
      .map((group) => {
        const summarized = summarizeEntries(group.entries);
        return {
          title: group.title,
          count: group.entries.length,
          visible: summarized.visible,
          hiddenCount: summarized.hiddenCount,
        };
      }),
    fallback: summarizeFallback(groups.fallback),
  };
}
```

- [ ] **Step 4: Run the helper tests to verify they pass**

Run: `pnpm --dir apps/web test -- src/lib/repoChangeSummary.test.ts`

Expected: PASS

- [ ] **Step 5: Write the failing RepoPanel truncation regressions**

Add these tests in `apps/web/src/test/App.test.tsx` near the existing changed-files tests:

```ts
it("renders changed-file group counts and a summary row for long groups", async () => {
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
          changed_files: [
            "M  one.txt",
            "M  two.txt",
            "M  three.txt",
            "M  four.txt",
          ],
        },
      });
    }

    throw new Error(`Unexpected fetch request: ${url}`);
  });

  render(<App />);

  const sessionCard = await screen.findByLabelText("Operational session card");
  expect(within(sessionCard).getByText("Staged (4)")).toBeInTheDocument();
  expect(within(sessionCard).getByText("one.txt")).toBeInTheDocument();
  expect(within(sessionCard).getByText("two.txt")).toBeInTheDocument();
  expect(within(sessionCard).getByText("three.txt")).toBeInTheDocument();
  expect(within(sessionCard).queryByText("four.txt")).not.toBeInTheDocument();
  expect(within(sessionCard).getByText("+1 more")).toBeInTheDocument();
});

it("does not render a summary row when a group has exactly three entries", async () => {
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
          changed_files: ["?? one.md", "?? two.md", "?? three.md"],
        },
      });
    }

    throw new Error(`Unexpected fetch request: ${url}`);
  });

  render(<App />);

  const sessionCard = await screen.findByLabelText("Operational session card");
  expect(within(sessionCard).getByText("Untracked (3)")).toBeInTheDocument();
  expect(within(sessionCard).queryByText(/\+\d+ more/)).not.toBeInTheDocument();
});

it("truncates fallback raw rows with the same summary behavior", async () => {
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
  expect(within(sessionCard).getByText("!! one")).toBeInTheDocument();
  expect(within(sessionCard).getByText("!! two")).toBeInTheDocument();
  expect(within(sessionCard).getByText("!! three")).toBeInTheDocument();
  expect(within(sessionCard).queryByText("!! four")).not.toBeInTheDocument();
  expect(within(sessionCard).getByText("+1 more")).toBeInTheDocument();
});
```

- [ ] **Step 6: Run the focused App tests to verify they fail**

Run: `pnpm --dir apps/web test -- src/test/App.test.tsx -t "renders changed-file group counts and a summary row for long groups|does not render a summary row when a group has exactly three entries|truncates fallback raw rows with the same summary behavior"`

Expected: FAIL because the current panel renders uncapped groups and no counts.

- [ ] **Step 7: Wire the summary helper into RepoPanel**

Update `apps/web/src/components/RepoPanel.tsx` so the card renders summarized groups:

```tsx
import {
  summarizeRepoChanges,
  type SummarizedRepoChangeGroup,
} from "../lib/repoChangeSummary";

function renderChangeGroup(group: SummarizedRepoChangeGroup) {
  return (
    <section className="repo-state-change-group" aria-label={group.title} key={group.title}>
      <strong>{`${group.title} (${group.count})`}</strong>
      <ul className="repo-state-change-list">
        {group.visible.map((change) => (
          <li key={change.key}>
            <span className="repo-state-change-pill">{change.label}</span>
            <span>{change.path}</span>
          </li>
        ))}
        {group.hiddenCount > 0 ? (
          <li className="repo-state-change-more">{`+${group.hiddenCount} more`}</li>
        ) : null}
      </ul>
    </section>
  );
}

const summarizedChanges =
  repoState.status === "ready" || repoState.status === "stale"
    ? summarizeRepoChanges(groupRepoChanges(repoState.snapshot.changedFiles))
    : null;

{repoState.snapshot.dirty && summarizedChanges ? (
  <section className="repo-state-changes" aria-label="Changed files">
    {summarizedChanges.groups.map((group) => renderChangeGroup(group))}
    {summarizedChanges.fallback.visible.length > 0 ||
    summarizedChanges.fallback.hiddenCount > 0 ? (
      <ul className="repo-state-change-list repo-state-change-fallback">
        {summarizedChanges.fallback.visible.map((line) => (
          <li key={line}>{line}</li>
        ))}
        {summarizedChanges.fallback.hiddenCount > 0 ? (
          <li className="repo-state-change-more">
            {`+${summarizedChanges.fallback.hiddenCount} more`}
          </li>
        ) : null}
      </ul>
    ) : null}
  </section>
) : null}
```

- [ ] **Step 8: Re-run the focused App tests to verify they pass**

Run: `pnpm --dir apps/web test -- src/test/App.test.tsx -t "renders changed-file group counts and a summary row for long groups|does not render a summary row when a group has exactly three entries|truncates fallback raw rows with the same summary behavior"`

Expected: PASS

- [ ] **Step 9: Commit the summary model and panel behavior**

```bash
git add apps/web/src/lib/repoChangeSummary.ts apps/web/src/lib/repoChangeSummary.test.ts apps/web/src/components/RepoPanel.tsx apps/web/src/test/App.test.tsx
git commit -m "feat: summarize repo change groups"
```

## Task 2: Polish summary-row styling and verify the full web suite

**Files:**
- Modify: `apps/web/src/styles.css`
- Modify: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Add the failing styling-adjacent regression if needed**

If the focused tests from Task 1 already assert the summary row text and no extra behavior is needed, skip new test creation. Otherwise add one minimal assertion that `+N more` appears as text without a status pill.

- [ ] **Step 2: Implement minimal summary-row styles**

Update `apps/web/src/styles.css` with:

```css
.repo-state-change-more {
  color: rgba(240, 246, 255, 0.58);
  font-family: "Sora", "Inter", sans-serif;
  font-size: 0.7rem;
  letter-spacing: 0.02em;
  list-style: none;
}
```

Keep the new rule visually lighter than ordinary rows and do not add hover or button styles.

- [ ] **Step 3: Run the focused helper and App tests**

Run:

```bash
pnpm --dir apps/web test -- src/lib/repoChangeSummary.test.ts
pnpm --dir apps/web test -- src/test/App.test.tsx -t "renders grouped changed files|renders changed-file group counts and a summary row for long groups|does not render a summary row when a group has exactly three entries|truncates fallback raw rows with the same summary behavior"
```

Expected: PASS

- [ ] **Step 4: Run the full web test suite**

Run: `pnpm --dir apps/web test`

Expected: PASS

- [ ] **Step 5: Commit the styling and final test coverage**

```bash
git add apps/web/src/styles.css apps/web/src/test/App.test.tsx
git commit -m "test: cover changed files summary ux"
```

## Final Verification

- [ ] Run `git status --short`
- [ ] Confirm the branch is clean
- [ ] Summarize changed behavior and test evidence before presenting finish options
