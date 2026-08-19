import { getApiBaseUrl } from "@/lib/config";
import { normalizeProfile, normalizeTask, normalizeToday } from "@/lib/api/normalize";
import type {
  CreateTaskInput,
  DevSession,
  Profile,
  Task,
  Today,
} from "@/lib/api/types";

interface ErrorPayload {
  error?: {
    code?: string;
    message?: string;
    fields?: Record<string, string>;
    request_id?: string;
  };
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields: Record<string, string>;
  readonly requestId: string | null;

  constructor(
    message: string,
    options: {
      status: number;
      code?: string;
      fields?: Record<string, string>;
      requestId?: string;
    },
  ) {
    super(message);
    this.name = "ApiError";
    this.status = options.status;
    this.code = options.code ?? "REQUEST_FAILED";
    this.fields = options.fields ?? {};
    this.requestId = options.requestId ?? null;
  }
}

interface RequestOptions extends Omit<RequestInit, "body"> {
  token?: string;
  body?: unknown;
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { token, body, headers, ...init } = options;
  const requestHeaders = new Headers(headers);
  requestHeaders.set("Accept", "application/json");

  if (token) requestHeaders.set("Authorization", `Bearer ${token}`);
  if (body !== undefined) requestHeaders.set("Content-Type", "application/json");

  const apiBaseUrl = getApiBaseUrl();
  let response: Response;
  try {
    response = await fetch(`${apiBaseUrl}${path}`, {
      ...init,
      body: body === undefined ? undefined : JSON.stringify(body),
      cache: "no-store",
      credentials: "omit",
      headers: requestHeaders,
    });
  } catch {
    throw new ApiError("LifeHub belum dapat terhubung ke server. Coba lagi sebentar.", {
      status: 0,
      code: "NETWORK_ERROR",
    });
  }

  const contentType = response.headers.get("content-type") ?? "";
  const payload = contentType.includes("application/json")
    ? ((await response.json()) as unknown)
    : null;

  if (!response.ok) {
    const errorPayload = payload as ErrorPayload | null;
    throw new ApiError(
      errorPayload?.error?.message ?? "Permintaan belum berhasil. Silakan coba lagi.",
      {
        status: response.status,
        code: errorPayload?.error?.code,
        fields: errorPayload?.error?.fields,
        requestId: errorPayload?.error?.request_id,
      },
    );
  }

  return payload as T;
}

export async function createDevSession(email: string): Promise<DevSession> {
  const response = await apiRequest<{ access_token?: string; token?: string }>("/auth/dev-session", {
    method: "POST",
    body: { email },
  });
  const accessToken = response.access_token ?? response.token;

  if (!accessToken) {
    throw new ApiError("Server tidak mengembalikan sesi pengembangan yang valid.", {
      status: 502,
      code: "INVALID_SESSION_RESPONSE",
    });
  }

  return { accessToken };
}

export async function getProfile(token: string, signal?: AbortSignal): Promise<Profile> {
  const response = await apiRequest<unknown>("/profile", { token, signal });
  return normalizeProfile(response);
}

export async function updateProfile(token: string, timezone: string): Promise<Profile> {
  const response = await apiRequest<unknown>("/profile", {
    method: "PATCH",
    token,
    body: { timezone },
  });
  return normalizeProfile(response);
}

export async function getToday(token: string, signal?: AbortSignal): Promise<Today> {
  const response = await apiRequest<unknown>("/today", { token, signal });
  return normalizeToday(response);
}

export async function createTask(token: string, input: CreateTaskInput): Promise<Task> {
  const response = await apiRequest<unknown>("/tasks", {
    method: "POST",
    token,
    body: input,
  });
  return normalizeTask(response);
}

export async function completeTask(token: string, taskId: string): Promise<void> {
  await apiRequest<unknown>(`/tasks/${encodeURIComponent(taskId)}/complete`, {
    method: "POST",
    token,
  });
}
