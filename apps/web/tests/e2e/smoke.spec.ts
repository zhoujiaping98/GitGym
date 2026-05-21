import { type AddressInfo } from "node:net";
import { expect, test, type Locator, type Page } from "@playwright/test";
import { WebSocketServer } from "ws";

type TerminalStub = {
  close: () => Promise<void>;
  connectionCount: () => number;
  port: number;
};

async function createTerminalStub(): Promise<TerminalStub> {
  const server = new WebSocketServer({ host: "127.0.0.1", port: 0 });
  await new Promise<void>((resolve) => {
    server.once("listening", () => resolve());
  });

  let connectionCount = 0;

  server.on("connection", (socket) => {
    connectionCount += 1;
    let currentCommand = "";

    socket.send(JSON.stringify({ type: "ready", cols: 120, rows: 40 }));
    socket.send(
      JSON.stringify({
        type: "output",
        data: "PS D:\\\\Project\\\\GitGym\\\\var\\\\workspaces\\\\ws-42> ",
      }),
    );

    socket.on("message", (payload) => {
      const frame = JSON.parse(payload.toString()) as
        | { type: "input"; data: string }
        | { type: "resize"; cols: number; rows: number };

      if (frame.type !== "input") {
        return;
      }

      for (const chunk of frame.data) {
        if (chunk === "\r") {
          const command = currentCommand.trim();
          currentCommand = "";
          if (!command) {
            socket.send(JSON.stringify({ type: "output", data: "\r\n" }));
            socket.send(
              JSON.stringify({
                type: "output",
                data: "PS D:\\\\Project\\\\GitGym\\\\var\\\\workspaces\\\\ws-42> ",
              }),
            );
            continue;
          }

          socket.send(
            JSON.stringify({
              type: "status",
              phase: "running",
              detail: command,
            }),
          );
          socket.send(
            JSON.stringify({
              type: "output",
              data: `$ ${command}\r\n`,
            }),
          );
          socket.send(
            JSON.stringify({
              type: "output",
              data:
                command === "pwd"
                  ? "D:\\\\Project\\\\GitGym\\\\var\\\\workspaces\\\\ws-42\r\n"
                  : "On branch main\r\nnothing to commit, working tree clean\r\n",
            }),
          );
          socket.send(JSON.stringify({ type: "exit", exitCode: 0 }));
          socket.send(
            JSON.stringify({
              type: "output",
              data: "PS D:\\\\Project\\\\GitGym\\\\var\\\\workspaces\\\\ws-42> ",
            }),
          );
          continue;
        }

        if (chunk !== "\n") {
          currentCommand += chunk;
        }
      }
    });
  });

  const { port } = server.address() as AddressInfo;

  return {
    port,
    connectionCount: () => connectionCount,
    close: async () => {
      for (const client of server.clients) {
        client.close();
      }
      await new Promise<void>((resolve, reject) => {
        server.close((error) => {
          if (error) {
            reject(error);
            return;
          }
          resolve();
        });
      });
    },
  };
}

async function routeTerminalWebSocketToStub(page: Page, port: number) {
  await page.addInitScript((stubPort: number) => {
    const NativeWebSocket = window.WebSocket;

    class TerminalTestWebSocket extends NativeWebSocket {
      constructor(url: string | URL, protocols?: string | string[]) {
        const nextUrl = new URL(typeof url === "string" ? url : url.toString());
        if (nextUrl.pathname.startsWith("/api/v1/practice-sessions/")) {
          nextUrl.protocol = "ws:";
          nextUrl.hostname = "127.0.0.1";
          nextUrl.port = String(stubPort);
        }
        super(nextUrl.toString(), protocols);
      }
    }

    Object.defineProperties(TerminalTestWebSocket, {
      CONNECTING: { value: NativeWebSocket.CONNECTING },
      OPEN: { value: NativeWebSocket.OPEN },
      CLOSING: { value: NativeWebSocket.CLOSING },
      CLOSED: { value: NativeWebSocket.CLOSED },
    });

    window.WebSocket = TerminalTestWebSocket as typeof WebSocket;
  }, port);
}

async function expectFocusInsideDialog(dialog: Locator, target: Locator) {
  await expect
    .poll(async () => {
      const [activeIsTarget, activeIsInsideDialog] = await Promise.all([
        target.evaluate((element) => document.activeElement === element),
        dialog.evaluate((element) => {
          const activeElement = document.activeElement;
          return activeElement instanceof HTMLElement && element.contains(activeElement);
        }),
      ]);
      return activeIsTarget && activeIsInsideDialog;
    })
    .toBe(true);
}

async function expectNotActiveElement(target: Locator) {
  await expect
    .poll(async () => target.evaluate((element) => document.activeElement !== element))
    .toBe(true);
}

test.describe("GitGym shell", () => {
  let terminalStub: TerminalStub;
  const catalogPayload = {
    templates: [{ id: 1, key: "standard", name: "Standard" }],
    scenarios: [
      { id: 1, key: "sandbox-standard", name: "Standard Sandbox", template_id: 1 },
      { id: 2, key: "recover-branch", name: "Recover Branch", template_id: 1 },
    ],
  };

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

  test.beforeEach(async ({ page }) => {
    terminalStub = await createTerminalStub();
    await routeTerminalWebSocketToStub(page, terminalStub.port);
    await page.route("**/api/v1/templates", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(catalogPayload),
      });
    });
  });

  test.afterEach(async () => {
    await terminalStub.close();
  });

  async function startSecondScenario(page: Page) {
    await page.getByRole("button", { name: "New Session" }).click();
    await expect(
      page.getByRole("dialog", { name: "Choose a practice scenario" }),
    ).toBeVisible();
    await page.getByRole("option", { name: /Recover Branch/i }).click();
    await page.getByRole("button", { name: "Start Session" }).click();
  }

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
      createSessionCalls += 1;
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

    await startSecondScenario(page);
    await expect(
      page.getByRole("heading", { name: "Checking session" }),
    ).toBeVisible();
    releaseRefresh?.();
    const sessionCard = page.getByLabel("Operational session card");
    await expect(page.getByText("runner-43")).toBeVisible();
    await expect(sessionCard.getByText("Recover Branch")).toBeVisible();
    await expect(sessionCard.getByText("Template: Standard")).toBeVisible();
    await expect(sessionCard.getByText("runner-43")).toBeVisible();
    await expect(sessionCard.getByText("/tmp/gitgym/session-43")).toBeVisible();
    await expect(sessionCard.getByText("Session ID")).toBeVisible();
    await expect(sessionCard.getByText("43", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "Reset" })).toBeVisible();
    await page.getByRole("button", { name: "Reset" }).click();

    expect(createSessionCalls).toBe(1);
    expect(resetOldSessionCalls).toBe(0);
    expect(resetNewSessionCalls).toBe(1);
  });

  test("supports keyboard scenario selection and traps focus inside the picker", async ({
    page,
  }) => {
    let createSessionCalls = 0;
    let gatedReconcileHits = 0;
    let shouldGateRefresh = false;
    let releaseRefresh: (() => void) | null = null;
    const refreshGate = new Promise<void>((resolve) => {
      releaseRefresh = resolve;
    });

    await page.route("**/api/v1/practice-sessions/current", async (route) => {
      if (shouldGateRefresh) {
        gatedReconcileHits += 1;
        await refreshGate;
      }

      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          shouldGateRefresh
            ? {
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
              }
            : activeSessionPayload,
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
      createSessionCalls += 1;
      shouldGateRefresh = true;
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

    await expect(page.getByText("Session live")).toBeVisible();
    await expect(page.getByText("runner-42")).toBeVisible();

    const backgroundNewSessionButton = page.getByRole("button", { name: "New Session" });
    const backgroundResetButton = page.getByRole("button", { name: "Reset" });

    await backgroundNewSessionButton.click();

    const dialog = page.getByRole("dialog", { name: "Choose a practice scenario" });
    const firstOption = page.getByRole("option", { name: /Standard Sandbox/i });
    const secondOption = page.getByRole("option", { name: /Recover Branch/i });
    const cancelButton = page.getByRole("button", { name: "Cancel" });
    const startSessionButton = page.getByRole("button", { name: "Start Session" });

    await expect(dialog).toBeVisible();
    await expect(firstOption).toBeFocused();
    await expect(firstOption).toHaveAttribute("aria-selected", "true");
    await expect(secondOption).toHaveAttribute("aria-selected", "false");

    await firstOption.press("ArrowDown");

    await expect(secondOption).toBeFocused();
    await expect(secondOption).toHaveAttribute("aria-selected", "true");
    await expect(firstOption).toHaveAttribute("aria-selected", "false");

    await secondOption.press("Tab");
    await expectFocusInsideDialog(dialog, cancelButton);
    await expectNotActiveElement(backgroundNewSessionButton);
    await expectNotActiveElement(backgroundResetButton);

    await cancelButton.press("Tab");
    await expectFocusInsideDialog(dialog, startSessionButton);
    await expectNotActiveElement(backgroundNewSessionButton);
    await expectNotActiveElement(backgroundResetButton);

    await startSessionButton.press("Tab");

    await expectFocusInsideDialog(dialog, firstOption);
    await expectNotActiveElement(backgroundNewSessionButton);
    await expectNotActiveElement(backgroundResetButton);

    await firstOption.press("Shift+Tab");
    await expectFocusInsideDialog(dialog, startSessionButton);

    await startSessionButton.press("Enter");

    await expect(
      page.getByRole("heading", { name: "Checking session" }),
    ).toBeVisible();
    releaseRefresh?.();
    const sessionCard = page.getByLabel("Operational session card");
    await expect(page.getByText("runner-43")).toBeVisible();
    await expect(sessionCard.getByText("Recover Branch")).toBeVisible();
    await expect(sessionCard.getByText("Template: Standard")).toBeVisible();
    await expect(sessionCard.getByText("runner-43")).toBeVisible();
    await expect(sessionCard.getByText("/tmp/gitgym/session-43")).toBeVisible();
    await expect(sessionCard.getByText("Session ID")).toBeVisible();
    await expect(sessionCard.getByText("43", { exact: true })).toBeVisible();
    expect(createSessionCalls).toBe(1);
    expect(gatedReconcileHits).toBeGreaterThan(0);
  });

  test("keeps the terminal interactive across a page refresh", async ({ page }) => {
    await page.route("**/api/v1/practice-sessions/current", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(activeSessionPayload),
      });
    });

    await page.goto("/");

    await expect(page.getByText("Session live")).toBeVisible();
    await expect(page.getByText("interactive")).toBeVisible();
    await expect(page.locator(".terminal-window")).toContainText("ws-42>");

    await page.getByTestId("live-terminal").click();
    await page.keyboard.type("pwd");
    await page.keyboard.press("Enter");

    await expect(page.locator(".terminal-window")).toContainText("$ pwd");
    await expect(page.locator(".terminal-window")).toContainText(
      /workspaces\\+ws-42/,
    );

    await page.reload();

    await expect(page.getByText("Session live")).toBeVisible();
    await expect(page.getByText("interactive")).toBeVisible();
    await expect.poll(() => terminalStub.connectionCount()).toBe(2);

    await page.getByTestId("live-terminal").click();
    await page.keyboard.type("git status");
    await page.keyboard.press("Enter");

    await expect(page.locator(".terminal-window")).toContainText("$ git status");
    await expect(page.locator(".terminal-window")).toContainText(
      /nothing to commit, working tree clean/,
    );
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

    await startSecondScenario(page);

    await expect(page.getByText("runner-42")).toBeVisible();
    await expect(page.getByTestId("live-terminal")).toBeVisible();
    await expect(
      page.getByText("Created a new session, but the server did not return it as current."),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Retry sync" })).toHaveCount(0);
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

    await startSecondScenario(page);

    await expect(page.getByText("runner-42")).toBeVisible();
    await expect(page.getByTestId("live-terminal")).toBeVisible();
    await expect(
      page.getByText("Created a new session, but refreshing it failed: api offline"),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Retry sync" })).toBeVisible();
    await expect(page.getByText("runner-43")).toHaveCount(0);
  });
});
