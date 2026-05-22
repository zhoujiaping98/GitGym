# Session Unavailable Recovery UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the page-level `Session unavailable` UX into a retryable lookup shell and a recovery-first no-current-session shell that routes users to the existing scenario picker.

**Architecture:** Keep the existing `App.tsx` control flow and branch by recovery semantics instead of transport source. `currentSession.status === "error"` continues to render the retry shell, while page-level `actionError` with no displayed session renders a recovery-first shell with `New Session`, preserving live-shell reconciliation behavior and existing detail text handling.

**Tech Stack:** React, TypeScript, Vitest, React Testing Library, Playwright

---

## File Structure

**Modify:**
- `apps/web/src/App.tsx`
  - Keep `AppStateShell` as the shared page-level shell.
  - Add a dedicated recovery-first branch for page-level `actionError` when no session is displayed.
  - Reuse `openScenarioPicker("orphaned")` for the new recovery action so the existing modal path stays unchanged.
- `apps/web/src/test/App.test.tsx`
  - Preserve the lookup-failure regression test.
  - Add a page-level recovery-shell test for `actionError` with no displayed session.
  - Add a picker-opening test from the new recovery shell.
  - Keep live-shell reconciliation tests proving inline behavior.

**Verify only:**
- `apps/web/tests/e2e/smoke.spec.ts`
  - Only inspect after RTL is green to decide whether existing smoke already covers enough. Do not expand coverage unless the page-level branch lacks a route elsewhere.

---

### Task 1: Add the recovery-first page shell under TDD

**Files:**
- Modify: `apps/web/src/test/App.test.tsx`
- Modify: `apps/web/src/App.tsx`

- [ ] **Step 1: Write the failing test**

Add a regression test near the current-session shell tests in `apps/web/src/test/App.test.tsx` proving that a page-level `actionError` with no displayed session shows a recovery-first shell instead of the retry shell.

```tsx
it("renders a recovery-first session unavailable shell when no current session can be restored", async () => {
  const refresh = vi
    .fn()
    .mockResolvedValueOnce(mismatchedSession)
    .mockResolvedValueOnce(null);

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
    }),
  );

  render(<App />);

  await waitForNewSessionAction();
  fireEvent.click(screen.getByRole("button", { name: "New Session" }));
  await confirmScenarioPicker();

  await waitFor(() => {
    expect(screen.getByRole("button", { name: "Retry sync" })).toBeInTheDocument();
  });

  fireEvent.click(screen.getByRole("button", { name: "Retry sync" }));

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "Session unavailable" })).toBeInTheDocument();
  });

  expect(screen.getByText("Session recovery")).toBeInTheDocument();
  expect(
    screen.getByText(
      "Your previous practice session is no longer available. Start a fresh session to keep practicing.",
    ),
  ).toBeInTheDocument();
  expect(screen.getByText("The server did not return a current session.")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "New Session" })).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Try again" })).not.toBeInTheDocument();
});
```

Notes for the engineer:
- Do not replace the existing lookup-failure test.
- This test should fail because the current implementation still renders the generic retry shell for `actionError`.

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
pnpm --dir apps/web test -- --runInBand src/test/App.test.tsx -t "renders a recovery-first session unavailable shell when no current session can be restored"
```

Expected:

```text
FAIL
Unable to find an element with the text: Session recovery
```

Any equivalent failure proving the app still renders the retry-oriented shell is acceptable.

- [ ] **Step 3: Write minimal implementation**

Update the page-level shell branching in `apps/web/src/App.tsx` so `actionError` without a displayed session becomes a recovery-first branch.

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
      ) : actionError ? (
        <AppStateShell
          eyebrow="Session recovery"
          title="Session unavailable"
          body="Your previous practice session is no longer available. Start a fresh session to keep practicing."
          detail={actionError.message}
          actionLabel="New Session"
          onAction={() => {
            setHasAttemptedAutoCreate(true);
            openScenarioPicker("orphaned");
          }}
        />
      ) : currentSession.status === "error" ? (
        <AppStateShell
          eyebrow="Session lookup"
          title="Session unavailable"
          body="We could not restore your current practice session."
          detail={currentSession.error}
          actionLabel="Try again"
          onAction={() => {
            setActionError(null);
            void currentSession.refresh().catch(() => undefined);
          }}
        />
      ) : (
```

Notes for the engineer:
- Keep the live-shell `actionError` block above this untouched.
- Do not change the lookup shell copy.
- Do not introduce a second CTA or a new picker source for this slice.

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
pnpm --dir apps/web test -- --runInBand src/test/App.test.tsx -t "renders a recovery-first session unavailable shell when no current session can be restored"
```

Expected:

```text
PASS
```

- [ ] **Step 5: Run the focused shell regression set**

Run:

```bash
pnpm --dir apps/web test -- --runInBand src/test/App.test.tsx -t "renders a retryable error shell when current session lookup fails|renders a recovery-first session unavailable shell when no current session can be restored|renders a recovery-first workspace unavailable shell for orphaned sessions|surfaces a reset reconciliation error when refresh returns no current session"
```

Expected:

```text
4 passed
```

This confirms:
- lookup failure still retries
- new recovery shell is active
- orphaned shell did not regress
- reset/no-current-session still surfaces detail

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/App.tsx apps/web/src/test/App.test.tsx
git commit -m "feat: split session unavailable recovery shell"
```

---

### Task 2: Prove the recovery shell reuses the existing picker flow

**Files:**
- Modify: `apps/web/src/test/App.test.tsx`
- Verify: `apps/web/tests/e2e/smoke.spec.ts`

- [ ] **Step 1: Write the failing test**

Add a second RTL test next to the new recovery-shell coverage proving the page-level recovery shell opens the existing scenario picker.

```tsx
it("opens the existing scenario picker from the session unavailable recovery shell", async () => {
  const refresh = vi
    .fn()
    .mockResolvedValueOnce(mismatchedSession)
    .mockResolvedValueOnce(null);

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
    }),
  );

  render(<App />);

  await waitForNewSessionAction();
  fireEvent.click(screen.getByRole("button", { name: "New Session" }));
  await confirmScenarioPicker();

  await waitFor(() => {
    expect(screen.getByRole("button", { name: "Retry sync" })).toBeInTheDocument();
  });

  fireEvent.click(screen.getByRole("button", { name: "Retry sync" }));

  await waitFor(() => {
    expect(screen.getByRole("button", { name: "New Session" })).toBeInTheDocument();
  });

  fireEvent.click(screen.getByRole("button", { name: "New Session" }));

  expect(
    await screen.findByRole("dialog", { name: "Choose a practice scenario" }),
  ).toBeInTheDocument();
});
```

Notes for the engineer:
- Reuse the existing `confirmScenarioPicker()` helper.
- Do not assert that `createPracticeSession` is called a second time; this test is only about reopening the modal path from the recovery shell after `Retry sync` clears the displayed session.

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
pnpm --dir apps/web test -- --runInBand src/test/App.test.tsx -t "opens the existing scenario picker from the session unavailable recovery shell"
```

Expected:

```text
FAIL
Unable to find role="dialog" with name "Choose a practice scenario"
```

Any equivalent failure proving the recovery-shell CTA does not yet reopen the picker is acceptable.

- [ ] **Step 3: Write minimal implementation**

If Task 1 used the same `openScenarioPicker("orphaned")` action for the recovery shell, this test should pass without more production changes.

If it still fails, make the recovery-shell CTA exactly match the orphaned-shell action:

```tsx
onAction={() => {
  setHasAttemptedAutoCreate(true);
  openScenarioPicker("orphaned");
}}
```

Do not add a new scenario picker source such as `"recovery"` in this slice.

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
pnpm --dir apps/web test -- --runInBand src/test/App.test.tsx -t "opens the existing scenario picker from the session unavailable recovery shell"
```

Expected:

```text
PASS
```

- [ ] **Step 5: Run the full frontend unit suite**

Run:

```bash
pnpm --dir apps/web test
```

Expected:

```text
All tests passed
```

If the runner prints an exact count, record it in the implementation notes before the final handoff.

- [ ] **Step 6: Inspect smoke coverage and only expand if needed**

Run:

```bash
pnpm --dir apps/web run test:e2e -- --grep "session unavailable|reconciliation"
```

Expected:

```text
PASS
```

Review `apps/web/tests/e2e/smoke.spec.ts` only if this command exposes missing coverage for the new page-level branch. If the existing reconciliation smoke already demonstrates the live-shell behavior and no page-level route is obvious, do not force a new Playwright case in this slice.

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/test/App.test.tsx apps/web/tests/e2e/smoke.spec.ts
git commit -m "test: cover session unavailable recovery flow"
```

If no e2e file changed, commit only the updated unit test file with the same message.
