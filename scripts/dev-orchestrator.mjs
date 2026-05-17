import { spawn } from "node:child_process";
import process from "node:process";

export function buildServiceSpecs(platform = process.platform) {
  const npmCommand = platform === "win32" ? "npm.cmd" : "npm";

  return [
    { name: "runner", command: npmCommand, args: ["run", "runner:dev"] },
    { name: "api", command: npmCommand, args: ["run", "api:dev"] },
    { name: "web", command: npmCommand, args: ["run", "web:dev"] },
  ];
}

function log(prefix, message) {
  process.stdout.write(`[${prefix}] ${message}`);
}

function killProcessTree(child) {
  if (!child || child.killed) {
    return Promise.resolve();
  }

  if (process.platform === "win32") {
    return new Promise((resolve) => {
      const killer = spawn("taskkill", ["/pid", String(child.pid), "/t", "/f"], {
        stdio: "ignore",
        windowsHide: true,
      });
      killer.on("exit", () => resolve());
      killer.on("error", () => resolve());
    });
  }

  try {
    process.kill(-child.pid, "SIGTERM");
  } catch {}

  return Promise.resolve();
}

async function main() {
  const specs = buildServiceSpecs();
  const children = [];
  let shuttingDown = false;

  const shutdown = async (exitCode = 0) => {
    if (shuttingDown) {
      return;
    }
    shuttingDown = true;

    await Promise.all(children.map(({ child }) => killProcessTree(child)));
    process.exit(exitCode);
  };

  for (const signal of ["SIGINT", "SIGTERM"]) {
    process.on(signal, () => {
      void shutdown(0);
    });
  }

  for (const spec of specs) {
    const child = spawn(spec.command, spec.args, {
      cwd: process.cwd(),
      stdio: ["ignore", "pipe", "pipe"],
      detached: process.platform !== "win32",
      windowsHide: false,
    });

    child.stdout.on("data", (chunk) => {
      log(spec.name, chunk.toString());
    });
    child.stderr.on("data", (chunk) => {
      process.stderr.write(`[${spec.name}] ${chunk}`);
    });
    child.on("exit", (code, signal) => {
      if (shuttingDown) {
        return;
      }

      const suffix = signal ? `signal ${signal}` : `code ${code ?? 0}`;
      process.stderr.write(`[${spec.name}] exited with ${suffix}\n`);
      void shutdown(code ?? 1);
    });
    child.on("error", (error) => {
      if (shuttingDown) {
        return;
      }

      process.stderr.write(`[${spec.name}] failed to start: ${error.message}\n`);
      void shutdown(1);
    });

    children.push({ spec, child });
  }
}

if (import.meta.url === new URL(process.argv[1], "file://").href) {
  void main();
}
