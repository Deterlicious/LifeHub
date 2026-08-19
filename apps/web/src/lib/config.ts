export type AuthMode = "development" | "supabase";

const DEFAULT_DEVELOPMENT_API_URL = "http://127.0.0.1:8080/api/v1";

export function getAuthMode(): AuthMode {
  const configuredMode = process.env.NEXT_PUBLIC_AUTH_MODE;

  if (configuredMode === "development" || configuredMode === "supabase") {
    return configuredMode;
  }

  return "supabase";
}

export function getApiBaseUrl(): string {
  const configuredUrl = process.env.NEXT_PUBLIC_API_URL?.trim();

  if (configuredUrl) {
    return configuredUrl.replace(/\/$/, "");
  }

  if (getAuthMode() === "development") {
    return DEFAULT_DEVELOPMENT_API_URL;
  }

  throw new Error("NEXT_PUBLIC_API_URL belum dikonfigurasi.");
}

export function getSupabaseConfig(): { url: string; publishableKey: string } | null {
  const url = process.env.NEXT_PUBLIC_SUPABASE_URL?.trim();
  const publishableKey = process.env.NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY?.trim();

  if (!url || !publishableKey) {
    return null;
  }

  return { url, publishableKey };
}
