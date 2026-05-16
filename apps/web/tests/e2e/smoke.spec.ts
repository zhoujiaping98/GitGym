import { expect, test } from "@playwright/test";

test.describe("GitGym shell", () => {
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
    let resetSessionCalls = 0;

    await page.route("**/api/v1/practice-sessions/current", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          session: {
            id: 42,
            user_id: 7,
            scenario_id: 9,
            template_id: 1,
            runner_ref: "runner-42",
            workspace_path: "/tmp/gitgym/session-42",
            status: "active",
            started_at: "2026-05-16T10:00:00.000Z",
            expires_at: "2026-05-16T12:00:00.000Z",
            last_activity_at: "2026-05-16T10:05:00.000Z",
          },
        }),
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
            scenario_id: 9,
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
      resetSessionCalls += 1;
      await route.fulfill({
        status: 202,
        contentType: "application/json",
        body: JSON.stringify({ status: "resetting" }),
      });
    });

    await page.goto("/");

    await expect(page.getByText("Session live")).toBeVisible();
    await expect(page.getByText("Repository")).toBeVisible();
    await expect(page.getByText("History")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "New Session" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Reset" }),
    ).toBeVisible();

    await page.getByRole("button", { name: "New Session" }).click();
    await expect(page.getByRole("button", { name: "Reset" })).toBeVisible();
    await page.getByRole("button", { name: "Reset" }).click();

    expect(createSessionCalls).toBe(1);
    expect(resetSessionCalls).toBe(1);
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

    await expect(page.getByText("Session unavailable")).toBeVisible();
    await expect(page.getByRole("button", { name: "Try again" })).toBeVisible();
    await expect(
      page.getByRole("link", { name: "Continue with GitHub" }),
    ).toHaveCount(0);
  });
});
