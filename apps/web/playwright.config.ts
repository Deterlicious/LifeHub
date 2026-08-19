import { defineConfig, devices } from "@playwright/test";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const currentDirectory = dirname(fileURLToPath(import.meta.url));
const webBaseUrl = process.env.LIFEHUB_E2E_WEB_URL ?? "http://127.0.0.1:3000";
const apiBaseUrl = process.env.LIFEHUB_E2E_API_URL ?? "http://127.0.0.1:8080/api/v1";
const apiCommand = process.env.LIFEHUB_E2E_API_COMMAND
  ?? "docker compose -f ../../compose.yaml up -d --wait postgres && go run ./cmd/migrate && go run ./cmd/api";
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
    cwd: resolve(currentDirectory, "../../services/api"),
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
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 390, height: 844 },
      },
    },
  ],
});
