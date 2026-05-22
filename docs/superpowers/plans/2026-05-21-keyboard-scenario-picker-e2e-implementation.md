# Keyboard Scenario Picker E2E Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add one Playwright smoke test that proves the scenario picker supports keyboard-only scenario selection and traps focus inside the modal.

**Architecture:** Keep all changes inside `apps/web/tests/e2e/smoke.spec.ts`. Reuse the existing active-session smoke setup, terminal websocket stub, and two-scenario catalog payload, then add one standalone keyboard-driven smoke path that verifies both non-default keyboard selection and in-modal focus cycling before session creation.

**Tech Stack:** Playwright, TypeScript, existing smoke test helpers in `apps/web/tests/e2e/smoke.spec.ts`.

---

## File Map

- Modify: `apps/web/tests/e2e/smoke.spec.ts`
  - add one keyboard-specific smoke test
  - add or extend a tiny helper only if it improves readability without hiding assertions

## Task 1: Add the failing keyboard smoke

**Files:**
- Modify: `apps/web/tests/e2e/smoke.spec.ts`

- [ ] **Step 1: Add a failing keyboard smoke test**

Add this test near the other active-session smoke tests in `apps/web/tests/e2e/smoke.spec.ts`:

```ts
  test("supports keyboard scenario selection and traps focus inside the picker", async ({
    page,
  }) => {
    let currentSessionCalls = 0;

    await page.route("**/api/v1/practice-sessions/current", async (route) => {
      currentSessionCalls += 1;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          currentSessionCalls === 1
            ? activeSessionPayload
            : {
                session: {
                  id: 43,
                  user_id: 7,
                  scenario_id: 2,
                  template_id: 1,
                  runner_ref: "runner-43",
                  workspace_path: "/tmp/gitgym/session-43",
                  status: "active",
                  started_at: "2026-05-16T10:10:00.000Z",
                  expires_at: "2026-05-16T12:10:00.000Z",
                  last_activity_at: "2026-05-16T10:10:00.000Z",
                },
              },
        ),
      });
    });

    await page.route("**/api/v1/practice-sessions", async (route) => {
      if (route.request().method() !== "POST") {
        await route.fallback();
        return;
      }

      const body = JSON.parse(route.request().postData() ?? "{}");
      expect(body).toEqual({ scenario_id: 2 });
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          session: {
            id: 43,
            user_id: 7,
            scenario_id: 2,
            template_id: 1,
            runner_ref: "runner-43",
            workspace_path: "/tmp/gitgym/session-43",
            status: "active",
            started_at: "2026-05-16T10:10:00.000Z",
            expires_at: "2026-05-16T12:10:00.000Z",
            last_activity_at: "2026-05-16T10:10:00.000Z",
          },
        }),
      });
    });

    await page.goto("/");
    await expect(page.getByText("runner-42")).toBeVisible();

    await page.getByRole("button", { name: "New Session" }).click();
    const dialog = page.getByRole("dialog", { name: "Choose a practice scenario" });
    await expect(dialog).toBeVisible();

    const firstOption = page.getByRole("option", { name: /Standard Sandbox/i });
    const secondOption = page.getByRole("option", { name: /Recover Branch/i });
    const startSessionButton = page.getByRole("button", { name: "Start Session" });
    const backgroundNewSessionButton = page.getByRole("button", { name: "New Session" });

    await expect(firstOption).toBeFocused();
    await page.keyboard.press("ArrowDown");
    await expect(secondOption).toBeFocused();
    await expect(secondOption).toHaveAttribute("aria-selected", "true");

    await page.keyboard.press("Shift+Tab");
    await expect(startSessionButton).toBeFocused();
    await page.keyboard.press("Tab");
    await expect(secondOption).toBeFocused();
    await expect(backgroundNewSessionButton).not.toBeFocused();

    await page.keyboard.press("Tab");
    await expect(page.getByRole("button", { name: "Cancel" })).toBeFocused();
    await page.keyboard.press("Tab");
    await expect(startSessionButton).toBeFocused();
    await page.keyboard.press("Enter");

    await expect(page.getByText("runner-43")).toBeVisible();
    await expect(page.getByRole("complementary")).toContainText(
      /session #43\s*scenario #2\s*template #1/,
    );
  });
```

- [ ] **Step 2: Run the new smoke test and verify it fails**

Run: `pnpm --dir apps/web run test:e2e -- --grep "supports keyboard scenario selection and traps focus inside the picker"`

Expected: FAIL because the current smoke file does not yet include this keyboard path or, if partially added, the keyboard sequence does not yet match the real browser behavior.

- [ ] **Step 3: Commit the failing test only if your workflow requires it**

Do not commit a red build to `main`. Keep the failing test in your working tree and proceed directly to the minimal fix in the same task.

## Task 2: Make the keyboard smoke pass with minimal smoke-only changes

**Files:**
- Modify: `apps/web/tests/e2e/smoke.spec.ts`

- [ ] **Step 1: Add a tiny helper for opening the picker only if it improves readability**

If the file reads better with a helper, add this near `startSecondScenario`:

```ts
  async function openScenarioPicker(page: Page) {
    await page.getByRole("button", { name: "New Session" }).click();
    await expect(
      page.getByRole("dialog", { name: "Choose a practice scenario" }),
    ).toBeVisible();
  }
```

If the inline test is already clear enough, skip this helper and keep the flow local to the new test.

- [ ] **Step 2: Wire the new keyboard smoke to the existing active-session fixtures**

Ensure the new test:

```ts
    await page.goto("/");
    await expect(page.getByText("runner-42")).toBeVisible();
```

stays aligned with the existing active-session setup rather than inventing new fixture patterns.

- [ ] **Step 3: Keep all scenario assertions internally consistent**

Ensure all new-session stubs in the keyboard test use scenario `2` consistently:

```ts
scenario_id: 2
```

and the request assertion stays:

```ts
expect(body).toEqual({ scenario_id: 2 });
```

- [ ] **Step 4: Verify focus-trap assertions are checking dialog-internal controls**

Use these explicit in-dialog assertions:

```ts
await page.keyboard.press("Shift+Tab");
await expect(startSessionButton).toBeFocused();
await page.keyboard.press("Tab");
await expect(secondOption).toBeFocused();
await expect(backgroundNewSessionButton).not.toBeFocused();
```

and keep the remaining `Tab` assertions inside the modal so the smoke is validating trap behavior, not incidental page order.

- [ ] **Step 5: Run the focused smoke and verify it passes**

Run: `pnpm --dir apps/web run test:e2e -- --grep "supports keyboard scenario selection and traps focus inside the picker"`

Expected: PASS

- [ ] **Step 6: Commit the smoke update**

```bash
git add apps/web/tests/e2e/smoke.spec.ts
git commit -m "test: cover keyboard scenario picker smoke flow"
```

## Task 3: Full verification

**Files:**
- Verify only: `apps/web/tests/e2e/smoke.spec.ts`

- [ ] **Step 1: Run the full smoke suite**

Run: `pnpm --dir apps/web run test:e2e`

Expected: PASS

- [ ] **Step 2: Confirm the broader frontend unit suite is still green**

Run: `pnpm --dir apps/web test`

Expected: PASS

- [ ] **Step 3: Check worktree status**

Run: `git status --short`

Expected:

```text
?? docs/superpowers/plans/2026-05-20-scenario-picker-implementation.md
```

or the same unrelated plan-doc status plus no unexpected feature-file diffs.

- [ ] **Step 4: Record intended limits**

Confirm these remain true after implementation:

```text
- only one new keyboard-specific smoke test was added
- existing mouse-based smoke paths remain intact
- no production code changes were required for this e2e-only subproject
```

- [ ] **Step 5: Final commit only if verification required a follow-up edit**

If a verification-only fix was needed:

```bash
git add apps/web/tests/e2e/smoke.spec.ts
git commit -m "test: stabilize keyboard scenario picker smoke"
```

If no follow-up edit was needed, skip this step.
