import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";

const configPath = path.resolve("apps/web/vite.config.ts");

test("web dev server binds explicitly to 127.0.0.1", () => {
  const source = readFileSync(configPath, "utf8");

  assert.match(source, /host:\s*["']127\.0\.0\.1["']/);
  assert.match(source, /port:\s*5173/);
});
