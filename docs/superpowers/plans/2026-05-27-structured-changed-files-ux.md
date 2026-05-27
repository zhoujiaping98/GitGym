# Structured Changed Files UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the raw changed-files list in the repository snapshot card into a structured staged/unstaged/untracked view without changing the backend repo-state payload.

**Architecture:** Keep the current repo snapshot API contract and derive web-only parsed change entries from the existing `changedFiles: string[]` array. Add a focused parsing helper, render grouped sections in `RepoPanel`, and cover both parser behavior and RTL card output.

**Tech Stack:** React, TypeScript, Vitest, Testing Library

---

## File Structure

### Existing files to modify

- `apps/web/src/components/RepoPanel.tsx`
  - replace the flat dirty-file list with grouped changed-files rendering
- `apps/web/src/types.ts`
  - add web-only parsed change types if they are shared between helper and component
- `apps/web/src/test/App.test.tsx`
  - add RTL coverage for grouped changed-files rendering

### New files to create

- `apps/web/src/lib/repoChanges.ts`
  - parse raw `git status --short` lines into grouped entries
- `apps/web/src/lib/repoChanges.test.ts`
  - unit tests for parser edge cases and fallback behavior

### Files explicitly out of scope

- `services/api/**`
  - no API changes
- `services/runner/**`
  - no runner changes

---

### Task 1: Add the changed-files parser under TDD

**Files:**
- Create: `apps/web/src/lib/repoChanges.ts`
- Create: `apps/web/src/lib/repoChanges.test.ts`
- Modify: `apps/web/src/types.ts`

- [ ] **Step 1: Write the failing parser tests**

Create `apps/web/src/lib/repoChanges.test.ts` with:

```ts
import { describe, expect, it } from "vitest";
import { groupRepoChanges } from "./repoChanges";

describe("groupRepoChanges", () => {
  it("groups staged, unstaged, and untracked entries", () => {
    const grouped = groupRepoChanges([
      "M  staged-only.txt",
      " M unstaged-only.txt",
      "MM both.txt",
      "?? draft.md",
    ]);

    expect(grouped.staged).toEqual([
      { key: "staged:Modified:staged-only.txt", bucket: "staged", label: "Modified", path: "staged-only.txt" },
      { key: "staged:Modified:both.txt", bucket: "staged", label: "Modified", path: "both.txt" },
    ]);
    expect(grouped.unstaged).toEqual([
      { key: "unstaged:Modified:unstaged-only.txt", bucket: "unstaged", label: "Modified", path: "unstaged-only.txt" },
      { key: "unstaged:Modified:both.txt", bucket: "unstaged", label: "Modified", path: "both.txt" },
    ]);
    expect(grouped.untracked).toEqual([
      { key: "untracked:Untracked:draft.md", bucket: "untracked", label: "Untracked", path: "draft.md" },
    ]);
    expect(grouped.fallback).toEqual([]);
  });

  it("preserves rename text and falls back for unknown rows", () => {
    const grouped = groupRepoChanges([
      "R  old.txt -> new.txt",
      "!! ignored.tmp",
    ]);

    expect(grouped.staged).toEqual([
      { key: "staged:Renamed:old.txt -> new.txt", bucket: "staged", label: "Renamed", path: "old.txt -> new.txt" },
    ]);
    expect(grouped.fallback).toEqual(["!! ignored.tmp"]);
  });

  it("treats unmerged rows as conflicted entries", () => {
    const grouped = groupRepoChanges(["UU notes.txt"]);

    expect(grouped.staged).toEqual([
      { key: "staged:Conflicted:notes.txt", bucket: "staged", label: "Conflicted", path: "notes.txt" },
    ]);
    expect(grouped.unstaged).toEqual([
      { key: "unstaged:Conflicted:notes.txt", bucket: "unstaged", label: "Conflicted", path: "notes.txt" },
    ]);
  });
});
```

- [ ] **Step 2: Run the parser tests to verify they fail**

Run:

```bash
pnpm --dir apps/web test -- --run apps/web/src/lib/repoChanges.test.ts
```

Expected:

```text
FAIL
```

- [ ] **Step 3: Add the minimal parser types and implementation**

In `apps/web/src/types.ts`, add:

```ts
export type RepoChangeBucket = "staged" | "unstaged" | "untracked";

export type RepoChangeEntry = {
  key: string;
  bucket: RepoChangeBucket;
  label: string;
  path: string;
};

export type RepoChangeGroups = {
  staged: RepoChangeEntry[];
  unstaged: RepoChangeEntry[];
  untracked: RepoChangeEntry[];
  fallback: string[];
};
```

Create `apps/web/src/lib/repoChanges.ts` with:

```ts
import type { RepoChangeBucket, RepoChangeEntry, RepoChangeGroups } from "../types";

const conflictCodes = new Set(["U", "A"]);

function labelForCode(code: string) {
  if (code === "M") return "Modified";
  if (code === "A") return "Added";
  if (code === "D") return "Deleted";
  if (code === "R") return "Renamed";
  if (code === "C") return "Copied";
  if (code === "U") return "Conflicted";
  return null;
}

function entry(bucket: RepoChangeBucket, label: string, path: string): RepoChangeEntry {
  return {
    key: `${bucket}:${label}:${path}`,
    bucket,
    label,
    path,
  };
}

export function groupRepoChanges(lines: string[]): RepoChangeGroups {
  const groups: RepoChangeGroups = {
    staged: [],
    unstaged: [],
    untracked: [],
    fallback: [],
  };

  for (const line of lines) {
    if (line.startsWith("?? ")) {
      groups.untracked.push(entry("untracked", "Untracked", line.slice(3)));
      continue;
    }

    if (line.length < 4) {
      groups.fallback.push(line);
      continue;
    }

    const stagedCode = line[0];
    const unstagedCode = line[1];
    const path = line.slice(3);

    const isConflict = conflictCodes.has(stagedCode) && conflictCodes.has(unstagedCode);
    const stagedLabel = isConflict ? "Conflicted" : labelForCode(stagedCode);
    const unstagedLabel = isConflict ? "Conflicted" : labelForCode(unstagedCode);

    let parsed = false;
    if (stagedCode !== " " && stagedLabel) {
      groups.staged.push(entry("staged", stagedLabel, path));
      parsed = true;
    }
    if (unstagedCode !== " " && unstagedLabel) {
      groups.unstaged.push(entry("unstaged", unstagedLabel, path));
      parsed = true;
    }

    if (!parsed) {
      groups.fallback.push(line);
    }
  }

  return groups;
}
```

- [ ] **Step 4: Run the parser tests until green**

Run:

```bash
pnpm --dir apps/web test -- --run apps/web/src/lib/repoChanges.test.ts
```

Expected:

```text
PASS
```

- [ ] **Step 5: Commit Task 1**

Run:

```bash
git add apps/web/src/lib/repoChanges.ts apps/web/src/lib/repoChanges.test.ts apps/web/src/types.ts
git commit -m "feat: parse structured repo changes"
```

### Task 2: Render grouped changed-files sections in the repo card under TDD

**Files:**
- Modify: `apps/web/src/components/RepoPanel.tsx`
- Modify: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Write the failing RTL tests for grouped repo changes**

Add these tests to `apps/web/src/test/App.test.tsx`:

```ts
  it("renders grouped changed files for staged, unstaged, and untracked rows", async () => {
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
            changed_files: ["M  staged-only.txt", " M unstaged-only.txt", "?? draft.md"],
          },
        });
      }

      throw new Error(`Unexpected fetch request: ${url}`);
    });

    render(<App />);

    const sessionCard = await screen.findByLabelText("Operational session card");
    expect(within(sessionCard).getByText("Staged")).toBeInTheDocument();
    expect(within(sessionCard).getByText("Unstaged")).toBeInTheDocument();
    expect(within(sessionCard).getByText("Untracked")).toBeInTheDocument();
    expect(within(sessionCard).getByText("staged-only.txt")).toBeInTheDocument();
    expect(within(sessionCard).getByText("unstaged-only.txt")).toBeInTheDocument();
    expect(within(sessionCard).getByText("draft.md")).toBeInTheDocument();
  });

  it("renders mixed status rows in both staged and unstaged groups", async () => {
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
            changed_files: ["MM both.txt"],
          },
        });
      }

      throw new Error(`Unexpected fetch request: ${url}`);
    });

    render(<App />);

    const sessionCard = await screen.findByLabelText("Operational session card");
    const pathMatches = within(sessionCard).getAllByText("both.txt");
    expect(pathMatches).toHaveLength(2);
  });

  it("falls back to raw rows when a change line cannot be parsed", async () => {
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
            changed_files: ["!! ignored.tmp"],
          },
        });
      }

      throw new Error(`Unexpected fetch request: ${url}`);
    });

    render(<App />);

    const sessionCard = await screen.findByLabelText("Operational session card");
    expect(within(sessionCard).getByText("!! ignored.tmp")).toBeInTheDocument();
  });
```

- [ ] **Step 2: Run the new RTL tests to verify they fail**

Run:

```bash
pnpm --dir apps/web test -- --runInBand --testNamePattern "renders grouped changed files|renders mixed status rows|falls back to raw rows"
```

Expected:

```text
FAIL
```

- [ ] **Step 3: Implement grouped changed-files rendering**

In `apps/web/src/components/RepoPanel.tsx`:

- import `groupRepoChanges`
- compute grouped changes only when `repoState.status` is `ready` or `stale`
- replace the flat `<ul className="repo-state-changes">` with grouped sections

Use a compact rendering shape like:

```tsx
const groupedChanges =
  repoState.status === "ready" || repoState.status === "stale"
    ? groupRepoChanges(repoState.snapshot.changedFiles)
    : null;
```

```tsx
{repoState.snapshot.dirty && groupedChanges ? (
  <section className="repo-state-changes" aria-label="Changed files">
    {groupedChanges.staged.length > 0 ? (
      <div>
        <strong>Staged</strong>
        <ul>
          {groupedChanges.staged.map((change) => (
            <li key={change.key}>
              <span>{change.label}</span>
              <span>{change.path}</span>
            </li>
          ))}
        </ul>
      </div>
    ) : null}
    {groupedChanges.unstaged.length > 0 ? (
      <div>
        <strong>Unstaged</strong>
        <ul>
          {groupedChanges.unstaged.map((change) => (
            <li key={change.key}>
              <span>{change.label}</span>
              <span>{change.path}</span>
            </li>
          ))}
        </ul>
      </div>
    ) : null}
    {groupedChanges.untracked.length > 0 ? (
      <div>
        <strong>Untracked</strong>
        <ul>
          {groupedChanges.untracked.map((change) => (
            <li key={change.key}>
              <span>{change.label}</span>
              <span>{change.path}</span>
            </li>
          ))}
        </ul>
      </div>
    ) : null}
    {groupedChanges.fallback.length > 0 ? (
      <ul>
        {groupedChanges.fallback.map((line) => (
          <li key={line}>{line}</li>
        ))}
      </ul>
    ) : null}
  </section>
) : null}
```

Do not touch `useRepoState` or API code.

- [ ] **Step 4: Run the focused RTL tests until green**

Run:

```bash
pnpm --dir apps/web test -- --runInBand --testNamePattern "renders grouped changed files|renders mixed status rows|falls back to raw rows"
```

Expected:

```text
PASS
```

- [ ] **Step 5: Run the full web test suite**

Run:

```bash
pnpm --dir apps/web test
```

Expected:

```text
PASS
```

- [ ] **Step 6: Commit Task 2**

Run:

```bash
git add apps/web/src/components/RepoPanel.tsx apps/web/src/test/App.test.tsx
git commit -m "feat: group changed files in repo panel"
```
