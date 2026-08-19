import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError, apiRequest, createDevSession } from "@/lib/api/client";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

describe("LifeHub API client", () => {
  beforeEach(() => {
    process.env.NEXT_PUBLIC_AUTH_MODE = "development";
    delete process.env.NEXT_PUBLIC_API_URL;
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("uses bearer authorization and disables fetch caching", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ ok: true }));
    vi.stubGlobal("fetch", fetchMock);

    await apiRequest("/today", { token: "verified-access-token" });

    expect(fetchMock).toHaveBeenCalledOnce();
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    const headers = new Headers(init.headers);
    expect(url).toBe("http://127.0.0.1:8080/api/v1/today");
    expect(headers.get("Authorization")).toBe("Bearer verified-access-token");
    expect(init.cache).toBe("no-store");
    expect(init.credentials).toBe("omit");
  });

  it("maps the safe API error envelope", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse(
          {
            error: {
              code: "VALIDATION_ERROR",
              message: "Periksa data yang belum valid.",
              fields: { title: "Judul wajib diisi." },
              request_id: "req-test",
            },
          },
          422,
        ),
      ),
    );

    const request = apiRequest("/tasks", { method: "POST", body: { title: "" } });
    await expect(request).rejects.toMatchObject({
      name: "ApiError",
      status: 422,
      code: "VALIDATION_ERROR",
      fields: { title: "Judul wajib diisi." },
      requestId: "req-test",
    } satisfies Partial<ApiError>);
  });

  it("accepts the local dev session access_token response", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(jsonResponse({ access_token: "dev-token" })),
    );

    await expect(createDevSession("demo@lifehub.local")).resolves.toEqual({
      accessToken: "dev-token",
    });
  });
});
