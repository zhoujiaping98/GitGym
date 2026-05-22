# Catalog Unavailable Recovery UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the page-level `Practice catalog unavailable` shell into a clearer catalog-specific recovery state while preserving the existing catalog-only retry behavior.

**Architecture:** Keep this slice confined to the web app’s catalog-error shell in `App.tsx`. Update the catalog-error body copy, then align unit coverage so it proves the detail text remains visible and that `Try again` continues to retry only the catalog path. Check e2e for stale wording, but do not add new smoke flows.

**Tech Stack:** React 19, TypeScript, Vitest, Playwright

---

## File Structure

### Existing files to modify

- `apps/web/src/App.tsx`
  - currently renders the `Practice catalog unavailable` shell
  - will receive the new user-facing body copy while keeping the same title, detail source, and `Try again` action
- `apps/web/src/test/App.test.tsx`
  - will be updated to assert the new copy and verify `Try again` remains catalog-only
- `apps/web/tests/e2e/smoke.spec.ts`
  - only modify if it already contains assertions tied to the old catalog-unavailable wording

### Files that should not change

- `apps/web/src/hooks/useCurrentSession.ts`
- `apps/web/src/hooks/useTerminalSession.ts`
- session/picker components
- backend service files

---

### Task 1: Rewrite the Catalog Error Copy in Unit Tests First

**Files:**
- Modify: `apps/web/src/test/App.test.tsx`
- Modify: `apps/web/src/App.tsx`

- [ ] **Step 1: Write the failing unit test for the new catalog-unavailable copy**

Add or update a test so the catalog-error shell expects the new result-oriented body copy while still showing the low-level diagnostic detail.

```tsx
it("renders a recovery-first catalog unavailable shell", async () => {
  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: null,
    absenceReason: "missing",
    error: null,
    refresh: vi.fn().mockResolvedValue(null),
  });

  mockFetch.mockRejectedValueOnce(new Error("api offline"));

  render(<App />);

  expect(
    await screen.findByRole("heading", { name: "Practice catalog unavailable" }),
  ).toBeInTheDocument();
  expect(
    screen.getByText("We couldn’t load the available practice scenarios for this environment."),
  ).toBeInTheDocument();
  expect(screen.getByText("api offline")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Try again" })).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the focused unit test to verify it fails**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: FAIL because `App.tsx` still renders the older catalog-error body copy.

- [ ] **Step 3: Update only the catalog-error body copy in `App.tsx`**

Keep the title, detail source, and `Try again` action unchanged. Only rewrite the body text.

Implementation shape:

```tsx
      ) : catalogState.status === "error" && shouldShowCatalogState ? (
        <AppStateShell
          eyebrow="Catalog unavailable"
          title="Practice catalog unavailable"
          body="We couldn’t load the available practice scenarios for this environment."
          detail={catalogState.error}
          actionLabel="Try again"
          onAction={retryCatalogLoad}
        />
```

- [ ] **Step 4: Re-run the focused unit test to verify it passes**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: PASS for the new copy assertion.

- [ ] **Step 5: Commit the copy change**

```bash
git add apps/web/src/App.tsx apps/web/src/test/App.test.tsx
git commit -m "feat: rewrite catalog unavailable recovery copy"
```

---

### Task 2: Verify `Try Again` Still Retries Only the Catalog Path

**Files:**
- Modify: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Write the failing unit test for catalog-only retry behavior**

Add or update a test that proves clicking `Try again` re-runs catalog loading without triggering `currentSession.refresh()`.

```tsx
it("retries only the catalog request from the catalog unavailable shell", async () => {
  const refresh = vi.fn().mockResolvedValue(null);

  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: null,
    absenceReason: "missing",
    error: null,
    refresh,
  });

  mockFetch
    .mockRejectedValueOnce(new Error("api offline"))
    .mockResolvedValueOnce(createCatalogResponse());

  render(<App />);

  await userEvent.click(
    await screen.findByRole("button", { name: "Try again" }),
  );

  expect(refresh).not.toHaveBeenCalled();
  expect(mockFetch).toHaveBeenCalledTimes(2);
});
```

- [ ] **Step 2: Run the focused unit test to verify it fails if the assertion is new**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: FAIL until the test setup correctly isolates catalog retry behavior.

- [ ] **Step 3: Make the smallest test-only adjustment needed**

If the production behavior already works, keep production code unchanged and only refine the test setup/assertions until they accurately capture the existing behavior.

Do not change `retryCatalogLoad()` or `useCurrentSession`.

- [ ] **Step 4: Re-run the focused unit test to verify it passes**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: PASS, proving `Try again` remains catalog-only.

- [ ] **Step 5: Commit the retry-path coverage**

```bash
git add apps/web/src/test/App.test.tsx
git commit -m "test: cover catalog unavailable retry behavior"
```

---

### Task 3: Check E2E for Stale Catalog-Unavailable Wording

**Files:**
- Modify: `apps/web/tests/e2e/smoke.spec.ts` (only if needed)

- [ ] **Step 1: Search for stale catalog-unavailable assertions**

Inspect `apps/web/tests/e2e/smoke.spec.ts` for assertions tied to the old `Practice catalog unavailable` body copy.

Expected outcome:

- either no e2e changes are needed
- or a small text-only assertion update is needed

- [ ] **Step 2: If stale assertions exist, write/update the failing e2e assertion first**

Use the new copy:

```ts
await expect(
  page.getByText("We couldn’t load the available practice scenarios for this environment."),
).toBeVisible();
await expect(page.getByRole("button", { name: "Try again" })).toBeVisible();
```

- [ ] **Step 3: Run the e2e suite**

Run: `pnpm --dir apps/web run test:e2e`

Expected:

- FAIL only if stale assertions existed and were updated
- otherwise PASS unchanged

- [ ] **Step 4: Make the smallest necessary e2e update**

Only adjust existing assertion text if needed.

Do not add a new catalog-error smoke flow.

- [ ] **Step 5: Run final verification**

Run:

- `pnpm --dir apps/web test`
- `pnpm --dir apps/web run test:e2e`

Expected:

- Vitest: all tests pass
- Playwright: all smoke tests pass

- [ ] **Step 6: Commit the e2e alignment if any file changed**

```bash
git add apps/web/tests/e2e/smoke.spec.ts apps/web/src/test/App.test.tsx apps/web/src/App.tsx
git commit -m "test: align catalog unavailable recovery messaging"
```

If no e2e file changed, commit only the files that actually changed.

---

## Final Verification

- [ ] Run: `pnpm --dir apps/web test`
- [ ] Run: `pnpm --dir apps/web run test:e2e`
- [ ] Confirm the catalog-error shell shows the new result-oriented body copy
- [ ] Confirm the low-level error detail still appears
- [ ] Confirm `Try again` does not call `currentSession.refresh()`

---

## Notes For Implementers

- Keep this slice front-end only
- Keep it copy-and-assertion focused
- Do not change catalog retry semantics
- Do not refresh current session as part of the retry action
- Do not add auto retry or new smoke flows
