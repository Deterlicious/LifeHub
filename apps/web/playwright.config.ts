import { defineConfig, devices } from "@playwright/test";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const currentDirectory = dirname(fileURLToPath(import.meta.url));
const webBaseUrl = process.env.LIFEHUB_E2E_WEB_URL ?? "http://127.0.0.1:3000";
const apiBaseUrl = process.env.LIFEHUB_E2E_API_URL ?? "http://127.0.0.1:8080/api/v1";
const customApiCommand = process.env.LIFEHUB_E2E_API_COMMAND;
const apiCommand = customApiCommand ?? "node ./scripts/start-e2e-backend.mjs";
const e2eEnvironment: Record<string, string> = {
  APP_ENV: "development",
  HTTP_ADDR: "127.0.0.1:8080",
  DATABASE_URL: process.env.LIFEHUB_E2E_DATABASE_URL
    ?? "postgres://lifehub:lifehub@localhost:55432/lifehub?sslmode=disable",
  WEB_ORIGIN: webBaseUrl,
  DEV_AUTH_SECRET: "lifehub-e2e-only-secret-0123456789abcdef",
  NEXT_PUBLIC_AUTH_MODE: "development",
  NEXT_PUBLIC_API_URL: apiBaseUrl,
};

const webServers = [
  {
    command: apiCommand,
    cwd: customApiCommand ? resolve(currentDirectory, "../../services/api") : currentDirectory,
    url: apiBaseUrl.replace(/\/api\/v1$/, "/healthz"),
    reuseExistingServer: false,
    timeout: 120_000,
    env: e2eEnvironment,
  },
  {
    command: "pnpm dev --hostname 127.0.0.1 --port 3000",
    cwd: currentDirectory,
    url: webBaseUrl,
    reuseExistingServer: false,
    timeout: 120_000,
    env: e2eEnvironment,
  },
];

export default defineConfig({
  testDir: "./tests/e2e",
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? "github" : "list",
  // Keep Next dev, the Go API, and the worker responsive on small CI/Windows hosts.
  workers: 2,
  use: {
    baseURL: webBaseUrl,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  webServer: webServers,
  projects: [
    {
      name: "chromium-mobile",
      grep: /@mobile/,
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 390, height: 844 },
      },
    },
    {
      name: "chromium-desktop",
      grep: /@desktop/,
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 1280, height: 900 },
      },
    },
  ],
});
