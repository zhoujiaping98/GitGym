# Session Reconciliation UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Normalize live-workbench reconciliation feedback into explicit informational vs retryable outcomes, with predictable `Retry sync` behavior.

**Architecture:** Keep all changes inside the existing `App.tsx` live-shell reconciliation path and its tests. Extend the current inline `actionError` model just enough to distinguish informational outcomes from retryable ones, then prove the rendered behavior with focused RTL coverage and the existing reconciliation smoke flows.

**Tech Stack:** React, TypeScript, Vitest, React Testing Library, Playwright

---

## File Structure

**Modify:**
- `apps/web/src/App.tsx`
  - Keep reconciliation logic in the existing `actionError` path.
  - Add an explicit reconciliation-result classification so the live-shell render can decide whether `Retry sync` is meaningful.
  - Preserve the current create/reset result messages unless the plan explicitly changes them.
- `apps/web/src/test/App.test.tsx`
  - Tighten existing reconciliation assertions around `Retry sync`.
  - Add a retry-success regression proving the inline reconciliation message clears when sync succeeds.

**Verify only:**
- `apps/web/tests/e2e/smoke.spec.ts`
  - Reuse the current reconciliation smoke flows.
  - Only update assertions if the inline wording or `Retry sync` visibility changes.

---

### Task 1: Classify live-shell reconciliation outcomes under TDD

**Files:**
- Modify: `apps/web/src/test/App.test.tsx`
- Modify: `apps/web/src/App.tsx`

- [ ] **Step 1: Write the failing test**

Add a focused regression near the existing reset/no-current-session coverage proving that informational no-current-session results do not show `Retry sync`.

```tsx
it("does not show retry sync for informational reset reconciliation results", async () => {
  const refresh = vi.fn().mockResolvedValue(null);

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

  fireEvent.click(screen.getByRole("button", { name: "Reset" }));

  await waitFor(() => {
    expect(mockResetPracticeSession).toHaveBeenCalledWith(42);
    expect(refresh).toHaveBeenCalledTimes(1);
  });

  expect(
    screen.getByText("Reset completed, but the server did not return a current session."),
  ).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Retry sync" })).not.toBeInTheDocument();
});
```

This should fail against the current implementation if informational and retryable outcomes are not explicitly separated enough.

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
pnpm --dir apps/web test -- src/test/App.test.tsx -t "does not show retry sync for informational reset reconciliation results"
```

Expected:

```text
FAIL
Expected the Retry sync button not to be present
```

Any equivalent failure proving the current render path still treats the informational result as retryable is acceptable.

- [ ] **Step 3: Write minimal implementation**

Extend the live-shell error state with an explicit reconciliation kind, then use that kind to gate `Retry sync`.

In `apps/web/src/App.tsx`, update the action-error type:

```tsx
type ReconciliationFeedbackKind = "informational" | "retryable";

type ActionErrorState = {
  kind?: ReconciliationFeedbackKind;
  message: string;
  retryExpectedSessionId?: number;
};
```

Set `kind: "informational"` for no-current-session results in `reconcileSessionAction()`:

```tsx
      if (!refreshedSession) {
        setSessionOverride(fallbackSession ?? optimisticSession);
        setActionError({
          kind: "informational",
          message:
            action === "new-session"
              ? "Created a new session, but the server did not return it as current."
              : "Reset completed, but the server did not return a current session.",
        });
        return;
      }
```

Set `kind: "retryable"` for mismatch and refresh-failure cases in both reconciliation functions:

```tsx
        setActionError({
          kind: "retryable",
          message:
            action === "new-session"
              ? `Created session #${expectedSessionId}, but the server returned session #${refreshedSession.id}.`
              : `Reset session #${expectedSessionId}, but the server returned session #${refreshedSession.id}.`,
          retryExpectedSessionId: expectedSessionId,
        });
```

```tsx
      setActionError({
        kind: "retryable",
        message: `${
          action === "new-session"
            ? "Created a new session"
            : "Reset completed"
        }, but refreshing it failed: ${
          error instanceof Error ? error.message : "Unable to refresh the current session."
        }`,
        retryExpectedSessionId: expectedSessionId,
      });
```

```tsx
        setActionError({
          kind: "retryable",
          message: `Expected session #${actionError.retryExpectedSessionId}, but the server returned session #${refreshedSession.id}.`,
          retryExpectedSessionId: actionError.retryExpectedSessionId,
        });
```

```tsx
      setActionError({
        kind: "retryable",
        message: error instanceof Error ? error.message : "Unable to refresh the current session.",
        retryExpectedSessionId: actionError?.retryExpectedSessionId,
      });
```

Gate the live-shell button on `kind === "retryable"`:

```tsx
                {actionError.kind === "retryable" && actionError.retryExpectedSessionId ? (
                  <button className="top-bar-button" onClick={() => void retrySessionRefresh()} type="button">
                    Retry sync
                  </button>
                ) : null}
```

Notes for the engineer:
- Leave page-level recovery behavior unchanged.
- Keep transport failures before reconciliation untouched.
- Do not rewrite the existing message strings in this task.

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
pnpm --dir apps/web test -- src/test/App.test.tsx -t "does not show retry sync for informational reset reconciliation results"
```

Expected:

```text
PASS
```

- [ ] **Step 5: Run the focused reconciliation regression set**

Run:

```bash
pnpm --dir apps/web test -- src/test/App.test.tsx -t "replaces the optimistic new session when refresh returns a different current session|reverts the optimistic new session and shows an error when refresh fails|does not show retry sync for informational reset reconciliation results|does not show retry sync when creating a new session fails before reconciliation"
```

Expected:

```text
4 passed
```

This confirms:
- mismatch remains retryable
- refresh failure remains retryable
- informational no-current-session is not retryable
- early create failure still never surfaces `Retry sync`

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/App.tsx apps/web/src/test/App.test.tsx
git commit -m "feat: classify session reconciliation feedback"
```

---

### Task 2: Prove retry sync success clears inline reconciliation feedback

**Files:**
- Modify: `apps/web/src/test/App.test.tsx`
- Verify: `apps/web/tests/e2e/smoke.spec.ts`

- [ ] **Step 1: Write the failing test**

Add a focused retry-success regression proving the inline reconciliation message disappears once sync finally returns the expected session.

```tsx
it("clears the inline reconciliation message when retry sync succeeds", async () => {
  const refresh = vi
    .fn()
    .mockResolvedValueOnce(mismatchedSession)
    .mockResolvedValueOnce(nextSession);

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
    expect(screen.getByText("Created session #43, but the server returned session #99.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry sync" })).toBeInTheDocument();
  });

  fireEvent.click(screen.getByRole("button", { name: "Retry sync" }));

  await waitFor(() => {
    expect(screen.getByText("runner-43")).toBeInTheDocument();
  });

  expect(
    screen.queryByText("Created session #43, but the server returned session #99."),
  ).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Retry sync" })).not.toBeInTheDocument();
});
```

This should fail before the implementation is complete if successful retry does not fully clear the inline reconciliation state.

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
pnpm --dir apps/web test -- src/test/App.test.tsx -t "clears the inline reconciliation message when retry sync succeeds"
```

Expected:

```text
FAIL
Expected the old reconciliation message to be removed
```

Any equivalent failure proving the success path still leaves stale inline feedback is acceptable.

- [ ] **Step 3: Write minimal implementation**

If Task 1 was implemented as specified, the success path should already be close. Ensure `retrySessionRefresh()` clears the inline error when the expected session arrives:

```tsx
      if (refreshedSession.id !== actionError.retryExpectedSessionId) {
        setSessionOverride(refreshedSession);
        setActionError({
          kind: "retryable",
          message: `Expected session #${actionError.retryExpectedSessionId}, but the server returned session #${refreshedSession.id}.`,
          retryExpectedSessionId: actionError.retryExpectedSessionId,
        });
        return;
      }

      setSessionOverride(refreshedSession);
      setActionError(null);
```

Do not add extra success banners in this slice. Clearing the stale reconciliation feedback is enough.

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
pnpm --dir apps/web test -- src/test/App.test.tsx -t "clears the inline reconciliation message when retry sync succeeds"
```

Expected:

```text
PASS
```

- [ ] **Step 5: Run full frontend unit tests**

Run:

```bash
pnpm --dir apps/web test
```

Expected:

```text
All tests passed
```

If the runner prints an exact count, record it in the implementation notes before handoff.

- [ ] **Step 6: Re-run focused reconciliation e2e**

Run:

```bash
pnpm --dir apps/web run test:e2e -- --grep "session unavailable|reconciliation"
```

Expected:

```text
PASS
```

Update `apps/web/tests/e2e/smoke.spec.ts` only if current assertions depend on old `Retry sync` visibility or old inline wording.

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/App.tsx apps/web/src/test/App.test.tsx apps/web/tests/e2e/smoke.spec.ts
git commit -m "test: cover session reconciliation feedback"
```

If the e2e file does not change, commit only the modified frontend source and unit test files with the same message.
