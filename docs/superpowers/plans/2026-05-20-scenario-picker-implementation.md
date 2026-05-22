# Scenario Picker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a shared scenario picker modal so every `New Session` flow selects a scenario explicitly before creating a practice session.

**Architecture:** The frontend keeps session orchestration in `App.tsx` and introduces one presentation-only `ScenarioPickerModal` component. `App.tsx` owns catalog joining, modal state, and the existing optimistic create/reconcile flow, while the modal only renders choices and emits selection and confirm callbacks.

**Tech Stack:** React, TypeScript, Vitest, Playwright, existing app CSS in `apps/web/src/styles.css`.

---

## File Map

- Create: `apps/web/src/components/ScenarioPickerModal.tsx`
  - modal chrome, scenario list, confirm/cancel buttons, modal-local error rendering
- Modify: `apps/web/src/App.tsx`
  - modal state, scenario view-model derivation, `New Session` entry-point rewiring, create-session confirm flow
- Modify: `apps/web/src/styles.css`
  - modal overlay, card list, selected state, action row, mobile layout
- Modify: `apps/web/src/test/App.test.tsx`
  - modal-driven unit coverage for all entry points and failure behavior
- Modify: `apps/web/tests/e2e/smoke.spec.ts`
  - multi-scenario smoke coverage and selected `scenario_id` assertion

## Task 1: Add the shared scenario picker modal component

**Files:**
- Create: `apps/web/src/components/ScenarioPickerModal.tsx`
- Modify: `apps/web/src/styles.css`

- [ ] **Step 1: Create the modal component file with a minimal compile target**

Add `apps/web/src/components/ScenarioPickerModal.tsx`:

```tsx
type ScenarioPickerOption = {
  id: number;
  name: string;
  key: string;
  templateName: string;
};

type ScenarioPickerModalProps = {
  open: boolean;
  title: string;
  body: string;
  scenarios: ScenarioPickerOption[];
  selectedScenarioId: number | null;
  pending: boolean;
  error: string | null;
  confirmLabel?: string;
  onSelect: (scenarioId: number) => void;
  onConfirm: () => void;
  onClose: () => void;
};

export function ScenarioPickerModal(_props: ScenarioPickerModalProps) {
  return null;
}
```

- [ ] **Step 2: Add the modal styles as a failing visual shell**

Append these rules to `apps/web/src/styles.css`:

```css
.scenario-picker-backdrop {
  position: fixed;
  inset: 0;
  z-index: 5;
  display: grid;
  place-items: center;
  padding: 2rem;
  background: rgba(8, 15, 26, 0.48);
  backdrop-filter: blur(10px);
}

.scenario-picker-modal {
  width: min(100%, 42rem);
  display: grid;
  gap: 1rem;
  padding: 1.35rem;
  border-radius: 1.35rem;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(242, 246, 250, 0.98));
  border: 1px solid rgba(255, 255, 255, 0.7);
  box-shadow: 0 28px 72px rgba(16, 33, 58, 0.24);
}

.scenario-picker-list {
  display: grid;
  gap: 0.75rem;
}

.scenario-picker-option {
  width: 100%;
  display: grid;
  gap: 0.35rem;
  padding: 0.95rem 1rem;
  text-align: left;
  border-radius: 1rem;
  border: 1px solid rgba(16, 33, 58, 0.1);
  background: rgba(255, 255, 255, 0.76);
  color: var(--ink-strong);
  cursor: pointer;
}

.scenario-picker-option[data-selected="true"] {
  border-color: rgba(13, 155, 93, 0.5);
  background: rgba(121, 255, 177, 0.12);
  box-shadow: inset 0 0 0 1px rgba(13, 155, 93, 0.14);
}

.scenario-picker-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
}

@media (max-width: 720px) {
  .scenario-picker-backdrop {
    padding: 1rem;
  }

  .scenario-picker-actions {
    flex-direction: column-reverse;
  }
}
```

- [ ] **Step 3: Implement the modal markup**

Replace `ScenarioPickerModal.tsx` with:

```tsx
type ScenarioPickerOption = {
  id: number;
  name: string;
  key: string;
  templateName: string;
};

type ScenarioPickerModalProps = {
  open: boolean;
  title: string;
  body: string;
  scenarios: ScenarioPickerOption[];
  selectedScenarioId: number | null;
  pending: boolean;
  error: string | null;
  confirmLabel?: string;
  onSelect: (scenarioId: number) => void;
  onConfirm: () => void;
  onClose: () => void;
};

export function ScenarioPickerModal({
  open,
  title,
  body,
  scenarios,
  selectedScenarioId,
  pending,
  error,
  confirmLabel = "Start Session",
  onSelect,
  onConfirm,
  onClose,
}: ScenarioPickerModalProps) {
  if (!open) {
    return null;
  }

  return (
    <div className="scenario-picker-backdrop" role="presentation">
      <section
        aria-labelledby="scenario-picker-title"
        aria-modal="true"
        className="scenario-picker-modal"
        role="dialog"
      >
        <header>
          <span className="preview-label">Scenario picker</span>
          <h2 id="scenario-picker-title">{title}</h2>
          <p>{body}</p>
        </header>
        <div className="scenario-picker-list" role="listbox" aria-label="Practice scenarios">
          {scenarios.map((scenario) => (
            <button
              key={scenario.id}
              aria-selected={selectedScenarioId === scenario.id}
              className="scenario-picker-option"
              data-selected={selectedScenarioId === scenario.id}
              onClick={() => onSelect(scenario.id)}
              type="button"
            >
              <strong>{scenario.name}</strong>
              <span>{scenario.key}</span>
              <span>Template: {scenario.templateName}</span>
            </button>
          ))}
        </div>
        {error ? <div className="session-state-detail">{error}</div> : null}
        <div className="scenario-picker-actions">
          <button className="top-bar-button" disabled={pending} onClick={onClose} type="button">
            Cancel
          </button>
          <button
            className="primary-button"
            disabled={pending || selectedScenarioId === null}
            onClick={onConfirm}
            type="button"
          >
            {pending ? "Starting..." : confirmLabel}
          </button>
        </div>
      </section>
    </div>
  );
}
```

- [ ] **Step 4: Run the app test file to confirm the modal shell is non-breaking**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: PASS because the modal shell is not wired in yet and should not change existing behavior.

- [ ] **Step 5: Commit the modal shell**

```bash
git add apps/web/src/components/ScenarioPickerModal.tsx apps/web/src/styles.css
git commit -m "feat: add scenario picker modal shell"
```

## Task 2: Wire App session creation entry points to the modal

**Files:**
- Modify: `apps/web/src/App.tsx`
- Create: `apps/web/src/components/ScenarioPickerModal.tsx`

- [ ] **Step 1: Add a failing App test for modal opening from the top bar**

Add this test to `apps/web/src/test/App.test.tsx`:

```tsx
it("opens the scenario picker before creating a new session from the top bar", async () => {
  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: activeSession,
    absenceReason: null,
    error: null,
    refresh: vi.fn().mockResolvedValue(activeSession),
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("button", { name: "New Session" })).toBeEnabled();
  });

  fireEvent.click(screen.getByRole("button", { name: "New Session" }));

  expect(
    screen.getByRole("dialog", { name: "Choose a practice scenario" }),
  ).toBeInTheDocument();
  expect(mockCreatePracticeSession).not.toHaveBeenCalled();
});
```

- [ ] **Step 2: Run the focused test and verify it fails**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx -t "opens the scenario picker before creating a new session from the top bar"`

Expected: FAIL because `New Session` still creates directly.

- [ ] **Step 3: Add modal state and scenario view-model derivation to App**

In `apps/web/src/App.tsx`, add:

```tsx
import { ScenarioPickerModal } from "./components/ScenarioPickerModal";
```

Add state near the existing `catalogState`:

```tsx
type ScenarioPickerSource = "topbar" | "empty" | "orphaned";

type ScenarioPickerState =
  | { status: "closed" }
  | {
      status: "open";
      source: ScenarioPickerSource;
      selectedScenarioId: number | null;
      error: string | null;
    };
```

Initialize it:

```tsx
const [scenarioPickerState, setScenarioPickerState] = useState<ScenarioPickerState>({
  status: "closed",
});
```

Derive modal options after `defaultScenario`:

```tsx
const scenarioOptions =
  catalog?.scenarios.map((scenario) => {
    const template = catalog.templates.find((entry) => entry.id === scenario.templateId);
    return {
      id: scenario.id,
      name: scenario.name,
      key: scenario.key,
      templateName: template?.name ?? `Template #${scenario.templateId}`,
    };
  }) ?? [];
```

- [ ] **Step 4: Add modal open/close/select/confirm handlers**

In `apps/web/src/App.tsx`, add:

```tsx
function openScenarioPicker(source: ScenarioPickerSource) {
  if (catalogState.status !== "ready" || !defaultScenario) {
    return;
  }

  setActionError(null);
  setScenarioPickerState({
    status: "open",
    source,
    selectedScenarioId: defaultScenario.id,
    error: null,
  });
}

function closeScenarioPicker() {
  if (pendingAction === "new-session") {
    return;
  }

  setScenarioPickerState({ status: "closed" });
}

function selectScenario(scenarioId: number) {
  setScenarioPickerState((previous) =>
    previous.status !== "open"
      ? previous
      : { ...previous, selectedScenarioId: scenarioId, error: null },
  );
}

function confirmScenarioPicker() {
  if (scenarioPickerState.status !== "open" || scenarioPickerState.selectedScenarioId === null) {
    return;
  }

  const selectedScenarioId = scenarioPickerState.selectedScenarioId;
  const fallbackSession = displayedSession;

  setPendingAction("new-session");
  void createPracticeSession({ scenarioId: selectedScenarioId })
    .then((nextSession) => {
      setScenarioPickerState({ status: "closed" });
      if (!fallbackSession) {
        setSessionOverride(nextSession);
      }
      return reconcileSessionAction("new-session", nextSession.id, {
        fallbackSession,
        optimisticSession: nextSession,
      });
    })
    .catch((error: unknown) => {
      setScenarioPickerState((previous) =>
        previous.status !== "open"
          ? previous
          : {
              ...previous,
              error:
                error instanceof Error ? error.message : "Unable to create a new session.",
            },
      );
    })
    .finally(() => {
      setPendingAction(null);
    });
}
```

- [ ] **Step 5: Rewire all New Session entry points to the modal**

Update the top-bar action in `App.tsx`:

```tsx
{
  label: "New Session",
  onClick: () => {
    openScenarioPicker("topbar");
  },
  disabled: pendingAction !== null || catalogState.status !== "ready" || !defaultScenario,
},
```

Update the empty-state action:

```tsx
onAction={() => {
  setHasAttemptedAutoCreate(true);
  openScenarioPicker("empty");
}}
```

Update the orphaned-state action:

```tsx
onAction={() => {
  setHasAttemptedAutoCreate(true);
  openScenarioPicker("orphaned");
}}
```

Replace the auto-create effect body:

```tsx
setHasAttemptedAutoCreate(true);
openScenarioPicker("empty");
```

And render the modal before closing `</div>`:

```tsx
<ScenarioPickerModal
  body="Pick a sandbox before creating the next session."
  confirmLabel="Start Session"
  error={scenarioPickerState.status === "open" ? scenarioPickerState.error : null}
  onClose={closeScenarioPicker}
  onConfirm={confirmScenarioPicker}
  onSelect={selectScenario}
  open={scenarioPickerState.status === "open"}
  pending={pendingAction === "new-session"}
  scenarios={scenarioOptions}
  selectedScenarioId={
    scenarioPickerState.status === "open" ? scenarioPickerState.selectedScenarioId : null
  }
  title="Choose a practice scenario"
/>
```

- [ ] **Step 6: Run the focused modal-open test and verify it passes**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx -t "opens the scenario picker before creating a new session from the top bar"`

Expected: PASS

- [ ] **Step 7: Commit the App wiring**

```bash
git add apps/web/src/App.tsx apps/web/src/components/ScenarioPickerModal.tsx
git commit -m "feat: route new session flows through scenario picker"
```

## Task 3: Cover selection, confirm, and modal-local failure behavior

**Files:**
- Modify: `apps/web/src/test/App.test.tsx`
- Modify: `apps/web/src/App.tsx`

- [ ] **Step 1: Add the failing selection and confirm tests**

Add these tests to `apps/web/src/test/App.test.tsx`:

```tsx
it("uses the selected scenario when the modal confirms a new session", async () => {
  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: activeSession,
    absenceReason: null,
    error: null,
    refresh: vi.fn().mockResolvedValue(nextSession),
  });

  mockFetch.mockImplementationOnce(() =>
    createCatalogResponse({
      templates: [{ id: 1, key: "standard", name: "Standard" }],
      scenarios: [
        { id: 1, key: "sandbox-standard", name: "Standard Sandbox", template_id: 1 },
        { id: 2, key: "recover-branch", name: "Recover Branch", template_id: 1 },
      ],
    }),
  );

  render(<App />);

  await waitForNewSessionAction();
  fireEvent.click(screen.getByRole("button", { name: "New Session" }));
  fireEvent.click(screen.getByRole("button", { name: /Recover Branch/i }));
  fireEvent.click(screen.getByRole("button", { name: "Start Session" }));

  await waitFor(() => {
    expect(mockCreatePracticeSession).toHaveBeenCalledWith({ scenarioId: 2 });
  });
});

it("keeps create-session failures inside the scenario picker", async () => {
  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: activeSession,
    absenceReason: null,
    error: null,
    refresh: vi.fn().mockResolvedValue(activeSession),
  });

  mockCreatePracticeSession.mockRejectedValueOnce(new Error("scenario create failed"));

  render(<App />);

  await waitForNewSessionAction();
  fireEvent.click(screen.getByRole("button", { name: "New Session" }));
  fireEvent.click(screen.getByRole("button", { name: "Start Session" }));

  await waitFor(() => {
    expect(screen.getByText("scenario create failed")).toBeInTheDocument();
  });

  expect(
    screen.getByRole("dialog", { name: "Choose a practice scenario" }),
  ).toBeInTheDocument();
});
```

- [ ] **Step 2: Add the empty-state and orphaned entry-point tests**

Add:

```tsx
it("opens the shared scenario picker from the authenticated empty state", async () => {
  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: null,
    absenceReason: "missing",
    error: null,
    refresh: vi.fn().mockResolvedValue(null),
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("button", { name: "New Session" })).toBeInTheDocument();
  });

  fireEvent.click(screen.getByRole("button", { name: "New Session" }));

  expect(
    screen.getByRole("dialog", { name: "Choose a practice scenario" }),
  ).toBeInTheDocument();
});

it("opens the shared scenario picker from the orphaned recovery state", async () => {
  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: null,
    absenceReason: "orphaned",
    error: "workspace unavailable",
    refresh: vi.fn().mockResolvedValue(null),
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("button", { name: "New Session" })).toBeInTheDocument();
  });

  fireEvent.click(screen.getByRole("button", { name: "New Session" }));

  expect(
    screen.getByRole("dialog", { name: "Choose a practice scenario" }),
  ).toBeInTheDocument();
});
```

- [ ] **Step 3: Run the focused App tests to verify they pass**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: PASS

- [ ] **Step 4: Commit the modal behavior tests**

```bash
git add apps/web/src/test/App.test.tsx apps/web/src/App.tsx
git commit -m "test: cover scenario picker session creation flows"
```

## Task 4: Update smoke coverage for multi-scenario selection

**Files:**
- Modify: `apps/web/tests/e2e/smoke.spec.ts`

- [ ] **Step 1: Add a failing smoke assertion for selected scenario payload**

In `apps/web/tests/e2e/smoke.spec.ts`, update the catalog mock for the live-workbench session test to return:

```ts
const catalogPayload = {
  templates: [{ id: 1, key: "standard", name: "Standard" }],
  scenarios: [
    { id: 1, key: "sandbox-standard", name: "Standard Sandbox", template_id: 1 },
    { id: 2, key: "recover-branch", name: "Recover Branch", template_id: 1 },
  ],
};
```

And add this request assertion inside the `POST /api/v1/practice-sessions` route:

```ts
const body = JSON.parse(route.request().postData() ?? "{}");
expect(body).toEqual({ scenario_id: 2 });
```

- [ ] **Step 2: Run e2e to verify it fails before UI interaction changes**

Run: `pnpm --dir apps/web run test:e2e`

Expected: FAIL because the smoke flow still clicks `New Session` without selecting the second scenario in the modal.

- [ ] **Step 3: Update the smoke interaction to use the picker**

In the live-workbench and reconciliation smoke tests, replace direct creation flow with:

```ts
await page.getByRole("button", { name: "New Session" }).click();
await expect(
  page.getByRole("dialog", { name: "Choose a practice scenario" }),
).toBeVisible();
await page.getByRole("button", { name: /Recover Branch/i }).click();
await page.getByRole("button", { name: "Start Session" }).click();
```

Use the same modal interaction in:

- `shows the live workbench when the current session exists`
- `shows a reconciliation error when a new session cannot be confirmed`
- `keeps the current workbench visible when reconciliation refresh fails`

- [ ] **Step 4: Run the smoke suite and verify it passes**

Run: `pnpm --dir apps/web run test:e2e`

Expected: PASS

- [ ] **Step 5: Commit the smoke update**

```bash
git add apps/web/tests/e2e/smoke.spec.ts
git commit -m "test: cover scenario picker smoke flow"
```

## Task 5: Full verification

**Files:**
- Verify only: `apps/web/src/App.tsx`
- Verify only: `apps/web/src/components/ScenarioPickerModal.tsx`
- Verify only: `apps/web/src/styles.css`
- Verify only: `apps/web/src/test/App.test.tsx`
- Verify only: `apps/web/tests/e2e/smoke.spec.ts`

- [ ] **Step 1: Run frontend unit tests**

Run: `pnpm --dir apps/web test`

Expected: PASS

- [ ] **Step 2: Run frontend smoke tests**

Run: `pnpm --dir apps/web run test:e2e`

Expected: PASS

- [ ] **Step 3: Check worktree changes**

Run: `git status --short`

Expected: only the scenario-picker implementation files are modified in this slice.

- [ ] **Step 4: Record intended limitations**

Confirm these are still true after implementation:

```text
- the modal shows only name, key, and template name
- selection resets to the first scenario each time the modal opens
- auto-create now opens the shared picker flow rather than silently creating immediately
```

- [ ] **Step 5: Final commit if verification fixes were needed**

```bash
git add apps/web/src/App.tsx apps/web/src/components/ScenarioPickerModal.tsx apps/web/src/styles.css apps/web/src/test/App.test.tsx apps/web/tests/e2e/smoke.spec.ts
git commit -m "feat: add shared scenario picker flow"
```
