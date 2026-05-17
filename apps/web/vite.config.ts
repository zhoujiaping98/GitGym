import path from "node:path";
import react from "@vitejs/plugin-react";
import { loadEnv } from "vite";
import { defineConfig } from "vitest/config";

const envRoot = path.resolve(__dirname, "../..");

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, envRoot, "");

  return {
    envDir: envRoot,
    plugins: [react()],
    server: {
      port: 5173,
      proxy: {
        "/api": {
          target: env.VITE_API_PROXY_TARGET || env.API_BASE_URL || "http://127.0.0.1:8080",
          changeOrigin: true,
          ws: true,
        },
      },
    },
    test: {
      environment: "jsdom",
      globals: true,
      setupFiles: "./src/test/setup.ts",
      include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
      exclude: ["tests/e2e/**"],
    },
  };
});
