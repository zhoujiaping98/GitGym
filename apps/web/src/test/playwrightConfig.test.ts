import { afterEach, describe, expect, test, vi } from "vitest";

async function loadConfig() {
  vi.resetModules();
  const module = await import("../../playwright.config");
  return module.default;
}

afterEach(() => {
  delete process.env.CI;
  delete process.env.PLAYWRIGHT_REUSE_WEB_SERVER;
});

describe("playwright webServer config", () => {
  test("does not reuse an existing server unless explicitly requested", async () => {
    delete process.env.CI;
    delete process.env.PLAYWRIGHT_REUSE_WEB_SERVER;

    const config = await loadConfig();

    expect(config.webServer).toMatchObject({
      reuseExistingServer: false,
    });
  });

  test("allows explicit reuse of an existing server for local debugging", async () => {
    process.env.PLAYWRIGHT_REUSE_WEB_SERVER = "1";

    const config = await loadConfig();

    expect(config.webServer).toMatchObject({
      reuseExistingServer: true,
    });
  });
});
