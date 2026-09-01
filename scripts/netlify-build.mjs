import { spawnSync } from "node:child_process";

function required(name) {
  const value = process.env[name]?.trim();
  if (!value) {
    throw new Error(`${name} is required for the Netlify production build.`);
  }
  return value;
}

function productionApiUrl() {
  const configured = process.env.NEXT_PUBLIC_API_URL?.trim();
  if (configured) return configured.replace(/\/$/, "");

  const siteUrl = new URL(required("URL"));
  return `${siteUrl.origin}/api/v1`;
}

function run(command, args, environment) {
  const result = spawnSync(command, args, {
    env: environment,
    shell: process.platform === "win32",
    stdio: "inherit",
  });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}

required("NEXT_PUBLIC_SUPABASE_URL");
required("NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY");

const environment = {
  ...process.env,
  LIFEHUB_STATIC_EXPORT: "true",
  NEXT_PUBLIC_API_URL: productionApiUrl(),
  NEXT_PUBLIC_AUTH_MODE: "supabase",
};

run("corepack", ["pnpm", "install", "--frozen-lockfile"], environment);
run("corepack", ["pnpm", "--dir", "apps/web", "build"], environment);
