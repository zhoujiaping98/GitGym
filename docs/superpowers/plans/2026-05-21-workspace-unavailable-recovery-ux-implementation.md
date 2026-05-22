# Workspace Unavailable Recovery UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the page-level `Workspace unavailable` shell into a recovery-first state that clearly explains the outcome, keeps diagnostics secondary, and preserves the existing `New Session -> scenario picker` recovery path.

**Architecture:** Keep all lifecycle logic unchanged and limit the work to the web app’s page-level orphaned-session shell. Update the recovery copy in `App.tsx`, then align unit and end-to-end assertions so the tests validate the new outcome-first messaging and unchanged recovery action.

**Tech Stack:** React 19, TypeScript, Vitest, Playwright

---

## File Structure

### Existing files to modify

- `apps/web/src/App.tsx`
  - currently renders the `Workspace unavailable` page-level shell for orphaned sessions
  - will receive the new recovery-first body copy while preserving the existing detail source and `New Session` action
- `apps/web/src/test/App.test.tsx`
  - will be updated to assert the new recovery copy and unchanged picker behavior
- `apps/web/tests/e2e/smoke.spec.ts`
  - only update if there is already a flow that lands on the unavailable shell and asserts the old text

### Files that should not change

- `apps/web/src/hooks/useCurrentSession.ts`
- `apps/web/src/hooks/useTerminalSession.ts`
- `apps/web/src/components/ScenarioPickerModal.tsx`
- backend service files

---

### Task 1: Rewrite the Recovery Shell Copy in Unit Tests First

**Files:**
- Modify: `apps/web/src/test/App.test.tsx`
- Modify: `apps/web/src/App.tsx`

- [ ] **Step 1: Write the failing unit test for the new recovery-first copy**

Add or update the orphaned-session test so it expects the new body copy and still expects the diagnostic detail to remain visible.

```tsx
it("renders a recovery-first workspace unavailable shell for orphaned sessions", async () => {
  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: null,
    absenceReason: "orphaned",
    error: "workspace path is no longer available",
    refresh: vi.fn().mockResolvedValue(null),
  });

  render(<App />);

  expect(screen.getByRole("heading", { name: "Workspace unavailable" })).toBeInTheDocument();
  expect(
    screen.getByText(
      "Your previous sandbox can no longer be reopened. Start a fresh session to keep practicing.",
    ),
  ).toBeInTheDocument();
  expect(screen.getByText("workspace path is no longer available")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "New Session" })).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the focused unit test to verify it fails**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: FAIL because `App.tsx` still renders the older orphaned-session body copy.

- [ ] **Step 3: Update the orphaned-session shell copy in `App.tsx`**

Change only the page-level `Workspace unavailable` shell body text while preserving:

- the heading
- the `detail={actionError?.message ?? currentSession.error}` source
- the `New Session` action
- the current picker-opening behavior

Implementation shape:

```tsx
      ) : hasOrphanedSessionState ? (
        <AppStateShell
          eyebrow="Workspace recovery"
          title="Workspace unavailable"
          body="Your previous sandbox can no longer be reopened. Start a fresh session to keep practicing."
          detail={actionError?.message ?? currentSession.error}
          actionLabel="New Session"
          onAction={() => {
            setHasAttemptedAutoCreate(true);
            openScenarioPicker("orphaned");
          }}
        />
```

- [ ] **Step 4: Re-run the focused unit test to verify it passes**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: PASS for the new recovery-copy assertion.

- [ ] **Step 5: Commit the copy change**

```bash
git add apps/web/src/App.tsx apps/web/src/test/App.test.tsx
git commit -m "feat: rewrite workspace unavailable recovery copy"
```

---

### Task 2: Verify the Recovery Action Still Opens the Existing Scenario Picker

**Files:**
- Modify: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Write the failing unit test for the unchanged recovery path**

Add or update a test that proves clicking `New Session` from the orphaned recovery shell still opens the scenario picker instead of introducing a different recovery flow.

```tsx
it("opens the existing scenario picker from the workspace unavailable shell", async () => {
  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: null,
    absenceReason: "orphaned",
    error: "workspace path is no longer available",
    refresh: vi.fn().mockResolvedValue(null),
  });

  mockFetch.mockImplementationOnce(() => createCatalogResponse());

  render(<App />);

  await userEvent.click(screen.getByRole("button", { name: "New Session" }));

  expect(
    await screen.findByRole("dialog", { name: "Choose a practice scenario" }),
  ).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the focused unit test to verify it fails if the assertion is new**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: FAIL until the test is aligned with the current orphaned-shell flow or any missing setup is fixed.

- [ ] **Step 3: Make the smallest test/setup adjustment needed**

If the flow already works, keep production code unchanged and only make the unit test setup precise enough to exercise the existing behavior.

If any adjustment is needed, keep it bounded to test setup rather than changing the recovery flow itself.

- [ ] **Step 4: Re-run the focused unit tests**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: PASS, proving the new recovery messaging did not change the action flow.

- [ ] **Step 5: Commit the recovery-path coverage**

```bash
git add apps/web/src/test/App.test.tsx
git commit -m "test: cover workspace unavailable recovery flow"
```

---

### Task 3: Align Existing E2E Assertions If They Depend on the Old Copy

**Files:**
- Modify: `apps/web/tests/e2e/smoke.spec.ts` (only if needed)

- [ ] **Step 1: Search for stale orphaned-shell expectations**

Check `apps/web/tests/e2e/smoke.spec.ts` for assertions tied to the old `Workspace unavailable` wording.

Expected outcome:

- either no e2e changes are needed
- or a small assertion update is needed in an existing flow

- [ ] **Step 2: If stale assertions exist, write/update the failing e2e assertion first**

Use the new recovery-first copy:

```ts
await expect(
  page.getByText(
    "Your previous sandbox can no longer be reopened. Start a fresh session to keep practicing.",
  ),
).toBeVisible();
```

and keep the action expectation:

```ts
await expect(page.getByRole("button", { name: "New Session" })).toBeVisible();
```

- [ ] **Step 3: Run the relevant e2e suite**

Run: `pnpm --dir apps/web run test:e2e`

Expected:

- FAIL only if stale orphaned-shell assertions existed and had to be updated
- otherwise PASS without production changes

- [ ] **Step 4: Make the smallest necessary e2e update**

Only touch the assertion text if the suite depended on the old copy.

Do not add new backend setup or new smoke flows for this task.

- [ ] **Step 5: Run full verification**

Run:

- `pnpm --dir apps/web test`
- `pnpm --dir apps/web run test:e2e`

Expected:

- Vitest: all tests pass
- Playwright: all smoke tests pass

- [ ] **Step 6: Commit the e2e alignment (if any)**

```bash
git add apps/web/tests/e2e/smoke.spec.ts apps/web/src/test/App.test.tsx apps/web/src/App.tsx
git commit -m "test: align workspace unavailable recovery messaging"
```

If no e2e file changed, commit only the files that actually changed.

---

## Final Verification

- [ ] Run: `pnpm --dir apps/web test`
- [ ] Run: `pnpm --dir apps/web run test:e2e`
- [ ] Confirm the orphaned-session shell shows the new recovery-first copy
- [ ] Confirm the low-level detail text still appears
- [ ] Confirm clicking `New Session` still opens the existing scenario picker

---

## Notes For Implementers

- Keep this slice front-end only
- Keep it copy-and-assertion focused
- Do not change orphan detection or terminal attach behavior
- Do not add retry/repair actions
- Do not auto-open the picker
