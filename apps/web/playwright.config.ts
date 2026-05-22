import { defineConfig } from "@playwright/test";

const reuseExistingServer =
  process.env.PLAYWRIGHT_REUSE_WEB_SERVER === "1" ||
  process.env.PLAYWRIGHT_REUSE_WEB_SERVER === "true";

export default defineConfig({
  testDir: "./tests/e2e",
  timeout: 30_000,
  use: {
    baseURL: "http://127.0.0.1:4173",
    trace: "on-first-retry",
  },
  webServer: {
    command: "node ./node_modules/vite/bin/vite.js --host 127.0.0.1 --port 4173",
    port: 4173,
    reuseExistingServer,
  },
});
