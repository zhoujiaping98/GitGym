# Repo State UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current repository metadata panel with an operational session status card that uses existing session, terminal, and catalog data without adding backend dependencies.

**Architecture:** Keep the workbench structure intact and confine this slice to the web app. Introduce a presentation-oriented boundary for repo/session card data, update the `RepoPanel` to render the new three-layer operational card, and synchronize unit/e2e tests with the new name-first, health-first UI.

**Tech Stack:** React 19, TypeScript, Vitest, Playwright, existing app CSS in `apps/web/src/styles.css`

---

## File Structure

### Existing files to modify

- `apps/web/src/App.tsx`
  - currently owns catalog lookup and passes only `session` into `Workbench`
  - will supply scenario/template names to the workbench-side operational card
- `apps/web/src/components/Workbench.tsx`
  - currently wires `TerminalPanel`, `RepoPanel`, and `CommandHistoryPanel`
  - will become the handoff point for operational card display data
- `apps/web/src/components/RepoPanel.tsx`
  - currently renders preview repo summary and live session metadata
  - will be refactored into the operational status card
- `apps/web/src/styles.css`
  - currently contains `repo-summary`, `commit-rail`, and workbench side panel styling
  - will add card-specific operational styles and keep preview/live layouts aligned
- `apps/web/src/test/App.test.tsx`
  - will be updated to assert the new operational card text hierarchy and degraded terminal state behavior
- `apps/web/tests/e2e/smoke.spec.ts`
  - will be updated where current assertions rely on removed raw metadata text

### Optional new file if needed during implementation

- `apps/web/src/components/repoPanelModel.ts`
  - only create this file if the worker finds `RepoPanel.tsx` becoming too large while mapping display labels and status values
  - keep all logic local to the component if the file stays small enough

---

### Task 1: Thread Operational Display Data Into the Workbench

**Files:**
- Modify: `apps/web/src/App.tsx`
- Modify: `apps/web/src/components/Workbench.tsx`
- Modify: `apps/web/src/components/RepoPanel.tsx`
- Test: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Write the failing unit test for readable scenario/template names**

Add or update an `App.test.tsx` case so the live workbench asserts the repo panel shows human-readable names instead of only raw IDs.

```tsx
it("renders readable scenario and template names in the session card", async () => {
  mockFetch
    .mockResolvedValueOnce(
      jsonResponse({
        templates: [{ id: 1, key: "standard", name: "Standard" }],
        scenarios: [{ id: 1, key: "sandbox-standard", name: "Standard Sandbox", template_id: 1 }],
      }),
    )
    .mockResolvedValueOnce(
      jsonResponse({
        session: {
          id: 42,
          user_id: 7,
          scenario_id: 1,
          template_id: 1,
          runner_ref: "runner-42",
          workspace_path: "/tmp/gitgym/session-42",
          status: "active",
          started_at: "2026-05-16T10:00:00.000Z",
          expires_at: "2026-05-16T12:00:00.000Z",
          last_activity_at: "2026-05-16T10:05:00.000Z",
        },
      }),
    );

  render(<App />);

  expect(await screen.findByText("Standard Sandbox")).toBeInTheDocument();
  expect(screen.getByText("Template: Standard")).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: FAIL because the current repo panel still renders `scenario #1` and `template #1` instead of a readable status card.

- [ ] **Step 3: Thread catalog-derived labels into the workbench**

Update `App.tsx` and `Workbench.tsx` so the repo panel can receive display names without doing new fetches.

```tsx
// apps/web/src/components/Workbench.tsx
type WorkbenchProps = {
  preview?: boolean;
  session?: PracticeSession | null;
  terminal?: TerminalSessionState;
  scenarioName?: string | null;
  templateName?: string | null;
};

export function Workbench({
  preview = false,
  session = null,
  terminal = previewTerminal,
  scenarioName = null,
  templateName = null,
}: WorkbenchProps) {
  return (
    <section className={shellClassName}>
      <TerminalPanel ... />
      <RepoPanel
        preview={preview}
        session={session}
        scenarioName={scenarioName}
        templateName={templateName}
        terminalStatus={terminal.status}
      />
      <CommandHistoryPanel preview={preview} terminal={terminal} />
    </section>
  );
}
```

```tsx
// apps/web/src/App.tsx
const displayedScenario =
  displayedSession && catalog
    ? catalog.scenarios.find((entry) => entry.id === displayedSession.scenarioId) ?? null
    : null;

const displayedTemplate =
  displayedSession && catalog
    ? catalog.templates.find((entry) => entry.id === displayedSession.templateId) ?? null
    : null;

<Workbench
  session={displayedSession}
  terminal={terminalSession}
  scenarioName={displayedScenario?.name ?? null}
  templateName={displayedTemplate?.name ?? null}
/>;
```

- [ ] **Step 4: Re-run the focused test to verify it passes**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: PASS for the new readable-name assertion, even though the card layout is not fully productized yet.

- [ ] **Step 5: Commit the data-threading change**

```bash
git add apps/web/src/App.tsx apps/web/src/components/Workbench.tsx apps/web/src/test/App.test.tsx
git commit -m "refactor: thread repo panel display labels"
```

---

### Task 2: Replace Raw Metadata With the Operational Session Card

**Files:**
- Modify: `apps/web/src/components/RepoPanel.tsx`
- Modify: `apps/web/src/styles.css`
- Test: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Write failing tests for operational card structure and degraded state**

Add or update tests that assert:

1. the card leads with a health label such as `Live`
2. the card shows `runner`, `workspace`, and `session id`
3. degraded terminal state changes the card status without removing the workbench

```tsx
it("renders the operational session card for an active session", async () => {
  render(<App />);

  expect(await screen.findByText("Live")).toBeInTheDocument();
  expect(screen.getByText("Standard Sandbox")).toBeInTheDocument();
  expect(screen.getByText("Template: Standard")).toBeInTheDocument();
  expect(screen.getByText("Runner")).toBeInTheDocument();
  expect(screen.getByText("runner-42")).toBeInTheDocument();
  expect(screen.getByText("Workspace")).toBeInTheDocument();
  expect(screen.getByText("/tmp/gitgym/session-42")).toBeInTheDocument();
  expect(screen.getByText("Session ID")).toBeInTheDocument();
  expect(screen.getByText("42")).toBeInTheDocument();
});

it("keeps the workbench visible and marks the card recovering when the terminal degrades", async () => {
  mockTerminalState.status = "unavailable";

  render(<App />);

  expect(await screen.findByText("Recovering")).toBeInTheDocument();
  expect(screen.getByText("Terminal")).toBeInTheDocument();
  expect(screen.getByText("unavailable")).toBeInTheDocument();
  expect(screen.getByText("History")).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: FAIL because the current component still renders `live session metadata` and the commit rail, not the new operational status card.

- [ ] **Step 3: Implement the new `RepoPanel` structure**

Replace the existing repo summary/commit rail structure with a three-layer card.

```tsx
// apps/web/src/components/RepoPanel.tsx
type RepoPanelProps = {
  preview?: boolean;
  session: PracticeSession | null;
  scenarioName?: string | null;
  templateName?: string | null;
  terminalStatus?: TerminalSessionState["status"];
};

function operationalStatus(
  preview: boolean,
  session: PracticeSession | null,
  terminalStatus: TerminalSessionState["status"] = "idle",
) {
  if (preview || !session) {
    return "Preview";
  }
  if (terminalStatus === "ready" || terminalStatus === "connecting" || terminalStatus === "idle") {
    return "Live";
  }
  return "Recovering";
}

export function RepoPanel({
  preview = false,
  session,
  scenarioName = null,
  templateName = null,
  terminalStatus = "idle",
}: RepoPanelProps) {
  const statusLabel = operationalStatus(preview, session, terminalStatus);

  if (preview || !session) {
    return (
      <aside className="workbench-side repo-state-card repo-state-card-preview">
        <div className="panel-header">
          <span>Repository</span>
          <span className="panel-kicker">operational view</span>
        </div>
        <div className="repo-state-status">
          <span className="repo-state-badge">Preview</span>
          <div className="repo-state-heading">
            <strong>Sandbox status</strong>
            <p>Operational details appear after a live session is attached.</p>
          </div>
        </div>
      </aside>
    );
  }

  return (
    <aside className="workbench-side repo-state-card">
      <div className="panel-header">
        <span>Repository</span>
        <span className="panel-kicker">operational view</span>
      </div>

      <section className="repo-state-status">
        <span className={`repo-state-badge repo-state-badge-${statusLabel.toLowerCase()}`}>
          {statusLabel}
        </span>
        <div className="repo-state-heading">
          <strong>{scenarioName ?? `Scenario #${session.scenarioId}`}</strong>
          <p>{templateName ? `Template: ${templateName}` : `Template #${session.templateId}`}</p>
        </div>
      </section>

      <dl className="repo-state-facts">
        <div>
          <dt>Runner</dt>
          <dd>{session.runnerRef}</dd>
        </div>
        <div>
          <dt>Workspace</dt>
          <dd className="repo-summary-break">{session.workspacePath}</dd>
        </div>
        <div>
          <dt>Session ID</dt>
          <dd>{session.id}</dd>
        </div>
      </dl>

      <dl className="repo-state-lifecycle">
        <div>
          <dt>Started</dt>
          <dd>{formatDate(session.startedAt)}</dd>
        </div>
        <div>
          <dt>Last activity</dt>
          <dd>{formatDate(session.lastActivityAt)}</dd>
        </div>
        <div>
          <dt>Expires</dt>
          <dd>{formatDate(session.expiresAt)}</dd>
        </div>
        <div>
          <dt>Terminal</dt>
          <dd>{terminalStatus}</dd>
        </div>
      </dl>
    </aside>
  );
}
```

- [ ] **Step 4: Add minimal CSS for the operational card**

Update `styles.css` so the new card has distinct health, facts, and lifecycle groupings without redesigning the whole workbench.

```css
.repo-state-card {
  gap: 1rem;
}

.repo-state-status,
.repo-state-facts,
.repo-state-lifecycle {
  display: grid;
  gap: 0.75rem;
}

.repo-state-status {
  padding: 0.95rem;
  border-radius: 0.95rem;
  background: rgba(255, 255, 255, 0.03);
}

.repo-state-badge {
  display: inline-flex;
  width: fit-content;
  padding: 0.35rem 0.6rem;
  border-radius: 999px;
  font-size: 0.72rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.repo-state-badge-live {
  color: rgba(121, 255, 177, 0.96);
  background: rgba(121, 255, 177, 0.12);
}

.repo-state-badge-recovering {
  color: rgba(255, 209, 102, 0.96);
  background: rgba(255, 209, 102, 0.14);
}

.repo-state-heading strong {
  color: rgba(240, 246, 255, 0.96);
  font-size: 1rem;
}

.repo-state-heading p {
  margin: 0.25rem 0 0;
  color: rgba(240, 246, 255, 0.64);
}

.repo-state-facts,
.repo-state-lifecycle {
  margin: 0;
}

.repo-state-facts div,
.repo-state-lifecycle div {
  display: grid;
  gap: 0.2rem;
}
```

- [ ] **Step 5: Re-run the focused unit tests to verify they pass**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: PASS for the operational-card tests, including the degraded terminal state assertion.

- [ ] **Step 6: Commit the operational card implementation**

```bash
git add apps/web/src/components/RepoPanel.tsx apps/web/src/styles.css apps/web/src/test/App.test.tsx
git commit -m "feat: add operational repo session card"
```

---

### Task 3: Align Preview Text and End-to-End Assertions With the New Card

**Files:**
- Modify: `apps/web/src/test/App.test.tsx`
- Modify: `apps/web/tests/e2e/smoke.spec.ts`
- Modify: `apps/web/src/components/RepoPanel.tsx` (only if preview copy needs a small follow-up)

- [ ] **Step 1: Write or tighten failing assertions for preview and smoke coverage**

Update tests so they stop relying on removed text like `session #43 scenario #2 template #1` and instead assert the new operational structure.

```tsx
it("renders a simplified preview session card without fake live metadata", () => {
  render(<LoginScreen preview={<Workbench preview />} />);

  expect(screen.getByText("Repository")).toBeInTheDocument();
  expect(screen.getByText("Preview")).toBeInTheDocument();
  expect(
    screen.getByText("Operational details appear after a live session is attached."),
  ).toBeInTheDocument();
  expect(screen.queryByText("runner-42")).not.toBeInTheDocument();
});
```

```ts
// apps/web/tests/e2e/smoke.spec.ts
await expect(page.getByText("Recover Branch")).toBeVisible();
await expect(page.getByText("Template: Standard")).toBeVisible();
await expect(page.getByText("Session ID")).toBeVisible();
await expect(page.getByText("43")).toBeVisible();
```

- [ ] **Step 2: Run the targeted test commands to verify they fail**

Run:

- `pnpm --dir apps/web test -- --run src/test/App.test.tsx`
- `pnpm --dir apps/web run test:e2e`

Expected:

- unit test fails until preview copy matches the new card
- e2e fails because it still expects the old raw metadata rail text

- [ ] **Step 3: Update preview and e2e assertions to the new status card**

Make the smallest follow-up needed so preview copy and smoke coverage match the new UI.

```tsx
// keep preview aligned with live layout but without fake operational values
<div className="repo-state-status">
  <span className="repo-state-badge">Preview</span>
  <div className="repo-state-heading">
    <strong>Sandbox status</strong>
    <p>Operational details appear after a live session is attached.</p>
  </div>
</div>
```

```ts
// replace old metadata-rail assertions
await expect(page.getByText("Recover Branch")).toBeVisible();
await expect(page.getByText("Template: Standard")).toBeVisible();
await expect(page.getByText("runner-43")).toBeVisible();
await expect(page.getByText("/tmp/gitgym/session-43")).toBeVisible();
await expect(page.getByText("Session ID")).toBeVisible();
```

- [ ] **Step 4: Run the full verification suite**

Run:

- `pnpm --dir apps/web test`
- `pnpm --dir apps/web run test:e2e`

Expected:

- Vitest: all tests pass
- Playwright: all smoke tests pass with updated repo-card assertions

- [ ] **Step 5: Commit the test-alignment changes**

```bash
git add apps/web/src/test/App.test.tsx apps/web/tests/e2e/smoke.spec.ts apps/web/src/components/RepoPanel.tsx
git commit -m "test: align repo state card coverage"
```

---

## Final Verification

- [ ] Run: `pnpm --dir apps/web test`
- [ ] Run: `pnpm --dir apps/web run test:e2e`
- [ ] Confirm the live workbench still shows terminal, repository card, and history together
- [ ] Confirm degraded terminal state keeps the workbench visible while downgrading the card status

---

## Notes For Implementers

- Keep this slice front-end only
- Do not introduce backend repo snapshot fields in this plan
- Do not redesign `CommandHistoryPanel` in the same branch
- Prefer a small presentational helper only if `RepoPanel.tsx` gets too large; otherwise keep logic local
- Preserve existing page-level unavailable/orphaned flows in `App.tsx`
