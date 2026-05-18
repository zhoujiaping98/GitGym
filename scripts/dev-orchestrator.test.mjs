import test from "node:test";
import assert from "node:assert/strict";

import {
  buildServiceSpecs,
  isDirectExecution,
} from "./dev-orchestrator.mjs";

test("buildServiceSpecs returns runner api and web commands in startup order", () => {
  const specs = buildServiceSpecs("win32");

  assert.deepEqual(
    specs.map((spec) => spec.name),
    ["runner", "api", "web"],
  );

  assert.deepEqual(specs[0].command, "npm.cmd");
  assert.deepEqual(specs[0].args, ["run", "runner:dev"]);
  assert.deepEqual(specs[1].args, ["run", "api:dev"]);
  assert.deepEqual(specs[2].args, ["run", "web:dev"]);
});

test("buildServiceSpecs uses npm on non-windows platforms", () => {
  const specs = buildServiceSpecs("linux");

  assert.equal(specs[0].command, "npm");
  assert.equal(specs[1].command, "npm");
  assert.equal(specs[2].command, "npm");
});

test("isDirectExecution recognizes Windows script paths", () => {
  assert.equal(
    isDirectExecution(
      "file:///D:/Project/GitGym/scripts/dev-orchestrator.mjs",
      "D:\\Project\\GitGym\\scripts\\dev-orchestrator.mjs",
    ),
    true,
  );
});
