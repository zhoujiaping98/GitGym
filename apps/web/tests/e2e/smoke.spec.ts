import { expect, test } from "@playwright/test";

test.describe("GitGym shell", () => {
  test("shows the signed-out login shell when there is no active session", async ({
    page,
  }) => {
    await page.route("**/api/v1/practice-sessions/current", async (route) => {
      await route.fulfill({
        status: 404,
        contentType: "application/json",
        body: JSON.stringify({ error: "no active session" }),
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

    await page.goto("/");

    await expect(page.getByText("Session live")).toBeVisible();
    await expect(page.getByText("Repository")).toBeVisible();
    await expect(page.getByText("History")).toBeVisible();
  });
});
