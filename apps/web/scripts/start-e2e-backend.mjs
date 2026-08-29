import { spawn } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const currentDirectory = dirname(fileURLToPath(import.meta.url));
const apiDirectory = resolve(currentDirectory, "../../../services/api");
const children = new Set();
let shuttingDown = false;
let requestedExitCode = 0;

function startGo(args) {
  const child = spawn("go", args, {
    cwd: apiDirectory,
    env: process.env,
    stdio: "inherit",
    windowsHide: true,
  });
  children.add(child);
  return child;
}

function waitForExit(child) {
  return new Promise((resolveExit, rejectExit) => {
    child.once("error", rejectExit);
    child.once("exit", (code, signal) => resolveExit({ code, signal }));
  });
}

function finishIfStopped() {
  if (children.size === 0) process.exit(requestedExitCode);
}

function stop(signal = "SIGTERM", exitCode = 0) {
  if (!shuttingDown) {
    shuttingDown = true;
    requestedExitCode = exitCode;
    for (const child of children) {
      if (child.exitCode === null && child.signalCode === null) child.kill(signal);
    }
    const fallback = setTimeout(() => process.exit(requestedExitCode), 8_000);
    fallback.unref();
  } else if (exitCode !== 0) {
    requestedExitCode = exitCode;
  }
  finishIfStopped();
}

for (const signal of ["SIGINT", "SIGTERM"]) {
  process.on(signal, () => stop(signal, 0));
}

const migrate = startGo(["run", "./cmd/migrate"]);
const migrationExit = await waitForExit(migrate);
children.delete(migrate);
if (migrationExit.code !== 0) {
  process.exit(migrationExit.code ?? 1);
}

const api = startGo(["run", "./cmd/api"]);
const worker = startGo(["run", "./cmd/worker"]);

for (const child of [api, worker]) {
  child.once("exit", (code) => {
    children.delete(child);
    if (!shuttingDown) stop("SIGTERM", code === 0 ? 1 : code ?? 1);
    else finishIfStopped();
  });
  child.once("error", () => {
    children.delete(child);
    stop("SIGTERM", 1);
  });
}
