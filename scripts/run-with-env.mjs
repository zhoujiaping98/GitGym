import { readFileSync, existsSync } from "node:fs";
import { resolve } from "node:path";
import { spawn } from "node:child_process";

function parseEnvFile(filePath) {
  const env = {};
  const contents = readFileSync(filePath, "utf8");

  for (const rawLine of contents.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || line.startsWith("#")) {
      continue;
    }

    const separatorIndex = line.indexOf("=");
    if (separatorIndex === -1) {
      continue;
    }

    const key = line.slice(0, separatorIndex).trim();
    let value = line.slice(separatorIndex + 1).trim();

    if (
      (value.startsWith("\"") && value.endsWith("\"")) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }

    env[key] = value;
  }

  return env;
}

const envPath = resolve(process.cwd(), ".env.local");
const localEnv = existsSync(envPath) ? parseEnvFile(envPath) : {};
const [command, ...args] = process.argv.slice(2);

if (!command) {
  console.error("usage: node scripts/run-with-env.mjs <command> [args...]");
  process.exit(1);
}

const child = spawn(command, args, {
  cwd: process.cwd(),
  env: { ...process.env, ...localEnv },
  shell: false,
  stdio: "inherit",
});

child.on("error", (error) => {
  console.error(`failed to start ${command}: ${error.message}`);
  process.exit(1);
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 1);
});
