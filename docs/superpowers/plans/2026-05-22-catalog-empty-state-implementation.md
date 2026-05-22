# Catalog Empty State Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the page-level `Practice catalog empty` shell into a clearer administrative empty state that explains the environment has no published scenarios and intentionally offers no primary CTA.

**Architecture:** Keep this slice confined to the empty-catalog branch in `App.tsx` and the tests that describe it. Rewrite the empty-state body copy, keep the administrator guidance detail, and align unit assertions so they verify that the shell remains CTA-free. Then check e2e for stale wording, but do not add new smoke flows.

**Tech Stack:** React 19, TypeScript, Vitest, Playwright

---

## File Structure

### Existing files to modify

- `apps/web/src/App.tsx`
  - currently renders the `Practice catalog empty` shell when catalog load succeeds with zero scenarios
  - will receive the new administrative empty-state body copy
- `apps/web/src/test/App.test.tsx`
  - will be updated to assert the new administrative copy and the absence of action buttons
- `apps/web/tests/e2e/smoke.spec.ts`
  - only modify if it already contains assertions tied to the old empty-catalog wording

### Files that should not change

- `apps/web/src/hooks/useCurrentSession.ts`
- `apps/web/src/hooks/useTerminalSession.ts`
- catalog loading logic
- session creation logic
- scenario picker components
- backend service files

---

### Task 1: Rewrite the Empty-Catalog Copy in Unit Tests First

**Files:**
- Modify: `apps/web/src/test/App.test.tsx`
- Modify: `apps/web/src/App.tsx`

- [ ] **Step 1: Write the failing unit test for the new administrative empty copy**

Update the existing empty-catalog test so it expects the new body copy and the existing administrator guidance detail.

```tsx
it("renders an administrative empty state when the practice catalog has no scenarios", async () => {
  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: null,
    absenceReason: "missing",
    error: null,
    refresh: vi.fn().mockResolvedValue(null),
  });

  mockFetch.mockResolvedValueOnce(
    createCatalogResponse({
      templates: [{ id: 1, key: "standard", name: "Standard" }],
      scenarios: [],
    }),
  );

  render(<App />);

  expect(
    await screen.findByRole("heading", { name: "Practice catalog empty" }),
  ).toBeInTheDocument();
  expect(
    screen.getByText("This environment doesn’t have any published practice scenarios yet."),
  ).toBeInTheDocument();
  expect(
    screen.getByText(
      "Ask an administrator to publish at least one scenario before creating a session.",
    ),
  ).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the focused unit test to verify it fails**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: FAIL because `App.tsx` still renders the older empty-catalog body copy.

- [ ] **Step 3: Update only the empty-catalog body copy in `App.tsx`**

Keep the title and administrator guidance detail intact. Only rewrite the body text.

Implementation shape:

```tsx
      ) : hasEmptyCatalogState ? (
        <AppStateShell
          eyebrow="Catalog empty"
          title="Practice catalog empty"
          body="This environment doesn’t have any published practice scenarios yet."
          detail="Ask an administrator to publish at least one scenario before creating a session."
        />
```

- [ ] **Step 4: Re-run the focused unit test to verify it passes**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: PASS for the new administrative empty-state copy assertion.

- [ ] **Step 5: Commit the copy change**

```bash
git add apps/web/src/App.tsx apps/web/src/test/App.test.tsx
git commit -m "feat: rewrite catalog empty state copy"
```

---

### Task 2: Prove the Empty State Stays CTA-Free

**Files:**
- Modify: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Write or tighten the failing CTA-absence assertions**

Expand the empty-catalog unit test so it explicitly proves the state does not render either `Try again` or `New Session`.

```tsx
expect(screen.queryByRole("button", { name: "Try again" })).not.toBeInTheDocument();
expect(screen.queryByRole("button", { name: "New Session" })).not.toBeInTheDocument();
expect(mockCreatePracticeSession).not.toHaveBeenCalled();
```

- [ ] **Step 2: Run the focused unit test to verify it fails if the assertions are new**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: FAIL until the test is aligned with the intended CTA-free empty state.

- [ ] **Step 3: Keep this task test-only unless a real bug is discovered**

If the production shell already renders no CTA, leave `App.tsx` unchanged and adjust only the test.

Do not add buttons, disabled buttons, or retry behavior.

- [ ] **Step 4: Re-run the focused unit test to verify it passes**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: PASS, proving the empty state remains administrative and non-actionable.

- [ ] **Step 5: Commit the CTA-free coverage**

```bash
git add apps/web/src/test/App.test.tsx
git commit -m "test: cover catalog empty state shell"
```

---

### Task 3: Check E2E for Stale Empty-Catalog Wording

**Files:**
- Modify: `apps/web/tests/e2e/smoke.spec.ts` (only if needed)

- [ ] **Step 1: Search for stale empty-catalog assertions**

Inspect `apps/web/tests/e2e/smoke.spec.ts` for assertions tied to the old `Practice catalog empty` body or detail copy.

Expected outcome:

- either no e2e changes are needed
- or a small text-only assertion update is needed

- [ ] **Step 2: If stale assertions exist, write/update the failing e2e assertion first**

Use the new body/detail copy:

```ts
await expect(
  page.getByText("This environment doesn’t have any published practice scenarios yet."),
).toBeVisible();
await expect(
  page.getByText(
    "Ask an administrator to publish at least one scenario before creating a session.",
  ),
).toBeVisible();
```

- [ ] **Step 3: Run the e2e suite**

Run: `pnpm --dir apps/web run test:e2e`

Expected:

- FAIL only if stale assertions existed and were updated
- otherwise PASS unchanged

- [ ] **Step 4: Make the smallest necessary e2e update**

Only adjust existing assertion text if needed.

Do not add a new empty-catalog smoke flow.

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
git commit -m "test: align catalog empty state messaging"
```

If no e2e file changed, commit only the files that actually changed.

---

## Final Verification

- [ ] Run: `pnpm --dir apps/web test`
- [ ] Run: `pnpm --dir apps/web run test:e2e`
- [ ] Confirm the empty-catalog shell shows the new administrative body copy
- [ ] Confirm the administrator guidance detail still appears
- [ ] Confirm the shell renders no `Try again` or `New Session` button

---

## Notes For Implementers

- Keep this slice front-end only
- Keep it copy-and-assertion focused
- Do not change catalog loading behavior
- Do not add retry or session-creation actions
- Do not add new smoke flows
