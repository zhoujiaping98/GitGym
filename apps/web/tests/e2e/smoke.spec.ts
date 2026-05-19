import { expect, test } from "@playwright/test";

test.describe("GitGym shell", () => {
  const activeSessionPayload = {
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
  };

  test("shows the signed-out login shell when there is no active session", async ({
    page,
  }) => {
    await page.route("**/api/v1/practice-sessions/current", async (route) => {
      await route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ error: "unauthorized" }),
      });
    });

    await page.goto("/");

    await expect(
      page.getByRole("link", { name: "Continue with GitHub" }),
    ).toBeVisible();
    await expect(page.getByText("Workbench preview")).toBeVisible();
  });

  test("shows the live workbench when the current session exists", async ({
    page,
  }) => {
    let createSessionCalls = 0;
    let currentSessionCalls = 0;
    let resetOldSessionCalls = 0;
    let resetNewSessionCalls = 0;
    let releaseRefresh: (() => void) | null = null;
    const refreshGate = new Promise<void>((resolve) => {
      releaseRefresh = resolve;
    });

    await page.route("**/api/v1/practice-sessions/current", async (route) => {
      currentSessionCalls += 1;
      if (currentSessionCalls > 2) {
        await refreshGate;
      }

      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          currentSessionCalls <= 2
            ? activeSessionPayload
            : {
                session: {
                  id: 43,
                  user_id: 7,
                  scenario_id: 1,
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

      createSessionCalls += 1;
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          session: {
            id: 43,
            user_id: 7,
            scenario_id: 1,
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
    await page.route("**/api/v1/practice-sessions/42/reset", async (route) => {
      resetOldSessionCalls += 1;
      await route.fulfill({
        status: 202,
        contentType: "application/json",
        body: JSON.stringify({ status: "resetting" }),
      });
    });
    await page.route("**/api/v1/practice-sessions/43/reset", async (route) => {
      resetNewSessionCalls += 1;
      await route.fulfill({
        status: 202,
        contentType: "application/json",
        body: JSON.stringify({ status: "resetting" }),
      });
    });

    await page.goto("/");

    await expect(page.getByText("Session live")).toBeVisible();
    await expect(page.getByText("runner-42")).toBeVisible();
    await expect(page.getByText("Repository", { exact: true })).toBeVisible();
    await expect(page.getByText("History", { exact: true })).toBeVisible();
    await expect(
      page.getByRole("button", { name: "New Session" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Reset" }),
    ).toBeVisible();

    await page.getByRole("button", { name: "New Session" }).click();
    await expect(page.getByText("runner-43")).toBeVisible();
    await expect(page.getByText("/tmp/gitgym/session-43")).toBeVisible();
    await expect(page.getByText("session #43")).toBeVisible();
    await expect(page.getByRole("button", { name: "Reset" })).toBeVisible();
    releaseRefresh?.();
    await page.getByRole("button", { name: "Reset" }).click();

    expect(createSessionCalls).toBe(1);
    expect(resetOldSessionCalls).toBe(0);
    expect(resetNewSessionCalls).toBe(1);
  });

  test("shows a retryable session error state when lookup fails", async ({
    page,
  }) => {
    await page.route("**/api/v1/practice-sessions/current", async (route) => {
      await route.fulfill({
        status: 500,
        contentType: "application/json",
        body: JSON.stringify({ error: "api offline" }),
      });
    });

    await page.goto("/");

    await expect(
      page.getByRole("heading", { name: "Session unavailable" }),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Try again" })).toBeVisible();
    await expect(
      page.getByRole("link", { name: "Continue with GitHub" }),
    ).toHaveCount(0);
  });

  test("shows a reconciliation error when a new session cannot be confirmed", async ({
    page,
  }) => {
    let currentSessionCalls = 0;

    await page.route("**/api/v1/practice-sessions/current", async (route) => {
      currentSessionCalls += 1;

      if (currentSessionCalls <= 2) {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(activeSessionPayload),
        });
        return;
      }

      await route.fulfill({
        status: 404,
        contentType: "application/json",
        body: JSON.stringify({ error: "not found" }),
      });
    });

    await page.route("**/api/v1/practice-sessions", async (route) => {
      if (route.request().method() !== "POST") {
        await route.fallback();
        return;
      }

      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          session: {
            id: 43,
            user_id: 7,
            scenario_id: 1,
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

    await expect(
      page.getByRole("heading", { name: "Session unavailable" }),
    ).toBeVisible();
    await expect(
      page.getByText("Created a new session, but the server did not return it as current."),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Try again" })).toBeVisible();
    await expect(page.getByText("runner-43")).toHaveCount(0);
  });

  test("keeps the current workbench visible when reconciliation refresh fails", async ({
    page,
  }) => {
    let currentSessionCalls = 0;

    await page.route("**/api/v1/practice-sessions/current", async (route) => {
      currentSessionCalls += 1;

      if (currentSessionCalls <= 2) {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(activeSessionPayload),
        });
        return;
      }

      await route.fulfill({
        status: 500,
        contentType: "application/json",
        body: JSON.stringify({ error: "api offline" }),
      });
    });

    await page.route("**/api/v1/practice-sessions", async (route) => {
      if (route.request().method() !== "POST") {
        await route.fallback();
        return;
      }

      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          session: {
            id: 43,
            user_id: 7,
            scenario_id: 1,
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

    await expect(page.getByText("runner-42")).toBeVisible();
    await expect(page.getByText("Terminal", { exact: true })).toBeVisible();
    await expect(
      page.getByText("Created a new session, but refreshing it failed: api offline"),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Retry sync" })).toBeVisible();
    await expect(page.getByText("runner-43")).toHaveCount(0);
  });
});
