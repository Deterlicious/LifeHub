import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  ApiError,
  apiRequest,
  createBill,
  createDevSession,
  createDocument,
  createEvent,
  createReminder,
  deleteBill,
  deleteDocument,
  deleteEvent,
  deleteProfileData,
  deleteReminder,
  deleteTask,
  getAgenda,
  getBills,
  getDocuments,
  getNotificationUnreadCount,
  getNotifications,
  getRecurrenceSeries,
  getReminders,
  markAllNotificationsRead,
  markBillPaid,
  markBillUnpaid,
  markNotificationRead,
  parseSmartCapture,
  uncompleteTask,
  updateBill,
  updateDocument,
  updateEvent,
  updateReminder,
  updateRecurrenceSeries,
  updateTask,
  stopRecurrenceSeries,
} from "@/lib/api/client";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

describe("LifeHub API client", () => {

  it("deletes only the authenticated LifeHub data after explicit confirmation", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(deleteProfileData(
      "verified-access-token",
      "HAPUS DATA LIFEHUB",
    )).resolves.toBeUndefined();

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("http://127.0.0.1:8080/api/v1/profile/data");
    expect(init.method).toBe("DELETE");
    expect(new Headers(init.headers).get("Authorization")).toBe("Bearer verified-access-token");
    expect(JSON.parse(String(init.body))).toEqual({ confirmation: "HAPUS DATA LIFEHUB" });
  });

  it("requests a smart draft without sending a save instruction", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      draft: {
        kind: "bill",
        title: "Internet",
        amount: 350000,
        currency: "IDR",
        recurrence: { frequency: "monthly", interval: 1 },
        confidence: 0.72,
      },
      ambiguities: ["Jam jatuh tempo belum disebutkan."],
      provider: "rule",
    }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(parseSmartCapture(
      "verified-access-token",
      "Bayar internet 350 ribu tanggal 15 tiap bulan",
    )).resolves.toMatchObject({
      draft: { kind: "bill", amount: 350000 },
      provider: "rule",
    });

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("http://127.0.0.1:8080/api/v1/smart-capture/parse");
    expect(init.method).toBe("POST");
    expect(JSON.parse(String(init.body))).toEqual({
      text: "Bayar internet 350 ribu tanggal 15 tiap bulan",
    });
  });
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

  it("creates an all-day event through the owned Go API contract", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse(
        {
          id: "event-1",
          title: "Lokakarya",
          notes: null,
          location: "Ruang Cendana",
          all_day: true,
          timezone: "Asia/Jakarta",
          starts_at: null,
          ends_at: null,
          starts_on: "2026-08-20",
          ends_on: "2026-08-20",
          created_at: "2026-08-19T10:00:00Z",
          updated_at: "2026-08-19T10:00:00Z",
        },
        201,
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      createEvent("verified-access-token", {
        title: "Lokakarya",
        location: "Ruang Cendana",
        all_day: true,
        starts_on: "2026-08-20",
        ends_on: "2026-08-20",
      }),
    ).resolves.toMatchObject({
      id: "event-1",
      allDay: true,
      startsOn: "2026-08-20",
    });

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    const headers = new Headers(init.headers);
    expect(url).toBe("http://127.0.0.1:8080/api/v1/events");
    expect(init.method).toBe("POST");
    expect(headers.get("Authorization")).toBe("Bearer verified-access-token");
    expect(JSON.parse(String(init.body))).toEqual({
      title: "Lokakarya",
      location: "Ruang Cendana",
      all_day: true,
      starts_on: "2026-08-20",
      ends_on: "2026-08-20",
    });
  });

  it("creates an integer-rupiah bill through the owned Go API", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse(
        {
          id: "bill-1",
          title: "Internet rumah",
          notes: null,
          amount: 350000,
          currency: "IDR",
          due_at: "2026-08-20T16:59:00Z",
          paid_at: null,
          created_at: "2026-08-20T01:00:00Z",
          updated_at: "2026-08-20T01:00:00Z",
        },
        201,
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      createBill("verified-access-token", {
        title: "Internet rumah",
        amount: 350000,
        currency: "IDR",
        due_local: "2026-08-20T23:59:00",
      }),
    ).resolves.toMatchObject({ id: "bill-1", amount: 350000, paidAt: null });

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("http://127.0.0.1:8080/api/v1/bills");
    expect(init.method).toBe("POST");
    expect(JSON.parse(String(init.body))).toEqual({
      title: "Internet rumah",
      amount: 350000,
      currency: "IDR",
      due_local: "2026-08-20T23:59:00",
    });
  });

  it("marks a bill paid through the idempotent action endpoint", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({
        id: "bill/owned",
        title: "Internet rumah",
        amount: 350000,
        currency: "IDR",
        due_at: "2026-08-20T16:59:00Z",
        paid_at: "2026-08-20T08:00:00Z",
        created_at: "2026-08-20T01:00:00Z",
        updated_at: "2026-08-20T08:00:00Z",
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(markBillPaid("verified-access-token", "bill/owned")).resolves.toMatchObject({
      paidAt: "2026-08-20T08:00:00Z",
    });

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    const headers = new Headers(init.headers);
    expect(url).toBe("http://127.0.0.1:8080/api/v1/bills/bill%2Fowned/mark-paid");
    expect(init.method).toBe("POST");
    expect(headers.get("Authorization")).toBe("Bearer verified-access-token");
  });

  it("creates, lists, updates, and deletes owned document metadata", async () => {
    const storedDocument = {
      id: "document/owned",
      name: "SIM A",
      category: "license",
      notes: "Periksa syarat perpanjangan",
      expires_on: "2026-09-30",
      status: "valid",
      days_until_expiry: 41,
      created_at: "2026-08-20T01:00:00Z",
      updated_at: "2026-08-20T01:00:00Z",
    };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(storedDocument, 201))
      .mockResolvedValueOnce(jsonResponse([storedDocument]))
      .mockResolvedValueOnce(jsonResponse({ ...storedDocument, notes: null }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(createDocument("verified-access-token", {
      name: "SIM A",
      category: "license",
      notes: "Periksa syarat perpanjangan",
      expires_on: "2026-09-30",
    })).resolves.toMatchObject({ id: "document/owned", daysUntilExpiry: 41 });
    await expect(getDocuments("verified-access-token")).resolves.toHaveLength(1);
    await expect(updateDocument("verified-access-token", "document/owned", {
      name: "SIM A diperbarui",
      category: "license",
      notes: null,
      expires_on: "2026-09-30",
    })).resolves.toMatchObject({ notes: null });
    await expect(deleteDocument("verified-access-token", "document/owned")).resolves.toBeUndefined();

    const [createUrl, createInit] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(createUrl).toBe("http://127.0.0.1:8080/api/v1/documents");
    expect(createInit.method).toBe("POST");
    const [listUrl, listInit] = fetchMock.mock.calls[1] as [string, RequestInit];
    expect(listUrl).toBe("http://127.0.0.1:8080/api/v1/documents");
    expect(listInit.method).toBeUndefined();
    const [updateUrl, updateInit] = fetchMock.mock.calls[2] as [string, RequestInit];
    expect(updateUrl).toBe("http://127.0.0.1:8080/api/v1/documents/document%2Fowned");
    expect(updateInit.method).toBe("PATCH");
    expect(JSON.parse(String(updateInit.body))).toMatchObject({
      name: "SIM A diperbarui",
      notes: null,
    });
    const [deleteUrl, deleteInit] = fetchMock.mock.calls[3] as [string, RequestInit];
    expect(deleteUrl).toBe("http://127.0.0.1:8080/api/v1/documents/document%2Fowned");
    expect(deleteInit.method).toBe("DELETE");
  });

  it("requests an inclusive Agenda range and normalizes its mixed summary", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      from: "2026-08-21",
      to: "2026-09-20",
      timezone: "Asia/Jakarta",
      items: [],
      summary: { total: 0, tasks: 0, events: 0, bills: 0, documents: 0 },
    }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(getAgenda("verified-access-token", {
      from: "2026-08-21",
      to: "2026-09-20",
    })).resolves.toMatchObject({ from: "2026-08-21", to: "2026-09-20" });

    const [url] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("http://127.0.0.1:8080/api/v1/agenda?from=2026-08-21&to=2026-09-20");
  });

  it("sends task clearing semantics and the idempotent uncomplete/delete actions", async () => {
    const taskResponse = {
      id: "task/owned",
      title: "Tugas pindah",
      notes: null,
      priority: "normal",
      due_at: null,
      completed_at: null,
    };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(taskResponse))
      .mockResolvedValueOnce(jsonResponse(taskResponse))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await updateTask("verified-access-token", "task/owned", {
      title: "Tugas pindah",
      notes: null,
      due_local: null,
    });
    await uncompleteTask("verified-access-token", "task/owned");
    await deleteTask("verified-access-token", "task/owned");

    const [updateUrl, updateInit] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(updateUrl).toContain("/tasks/task%2Fowned");
    expect(JSON.parse(String(updateInit.body))).toEqual({
      title: "Tugas pindah",
      notes: null,
      due_local: null,
    });
    expect((fetchMock.mock.calls[1] as [string, RequestInit])[0]).toContain("/uncomplete");
    expect((fetchMock.mock.calls[2] as [string, RequestInit])[1].method).toBe("DELETE");
  });

  it("replaces an event schedule only with the complete all-day union", async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(jsonResponse({
      id: "event/owned",
      title: "Retret",
      notes: null,
      location: null,
      all_day: true,
      timezone: "Asia/Jakarta",
      starts_at: null,
      ends_at: null,
      starts_on: "2026-08-25",
      ends_on: "2026-08-26",
      created_at: "2026-08-20T01:00:00Z",
      updated_at: "2026-08-20T02:00:00Z",
    })).mockResolvedValueOnce(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await updateEvent("verified-access-token", "event/owned", {
      title: "Retret",
      location: null,
      all_day: true,
      starts_on: "2026-08-25",
      ends_on: "2026-08-26",
    });
    await deleteEvent("verified-access-token", "event/owned");

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain("/events/event%2Fowned");
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(String(init.body))).toEqual({
      title: "Retret",
      location: null,
      all_day: true,
      starts_on: "2026-08-25",
      ends_on: "2026-08-26",
    });
    expect((fetchMock.mock.calls[1] as [string, RequestInit])[1].method).toBe("DELETE");
  });

  it("uses stable paid-history cursor queries and bill correction actions", async () => {
    const billResponse = {
      id: "bill/owned",
      title: "Internet",
      notes: null,
      amount: 400000,
      currency: "IDR",
      due_at: "2026-08-25T16:59:00Z",
      paid_at: null,
      created_at: "2026-08-20T01:00:00Z",
      updated_at: "2026-08-20T02:00:00Z",
    };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ items: [billResponse], next_cursor: "v1 cursor" }))
      .mockResolvedValueOnce(jsonResponse(billResponse))
      .mockResolvedValueOnce(jsonResponse(billResponse))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await getBills("verified-access-token", { state: "paid", limit: 50, cursor: "old cursor" });
    await updateBill("verified-access-token", "bill/owned", {
      amount: 400000,
      due_local: "2026-08-25T23:59:00",
    });
    await markBillUnpaid("verified-access-token", "bill/owned");
    await deleteBill("verified-access-token", "bill/owned");

    const [listUrl] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(listUrl).toBe("http://127.0.0.1:8080/api/v1/bills?state=paid&limit=50&cursor=old+cursor");
    expect((fetchMock.mock.calls[1] as [string, RequestInit])[1].method).toBe("PATCH");
    expect((fetchMock.mock.calls[2] as [string, RequestInit])[0]).toContain("/mark-unpaid");
    expect((fetchMock.mock.calls[3] as [string, RequestInit])[1].method).toBe("DELETE");
  });

  it("uses strict reminder unions and owned source queries", async () => {
    const reminder = {
      id: "reminder/owned",
      source_kind: "document",
      source_id: "document/owned",
      schedule: { kind: "before_date", days_before: 30, time_local: "09:00" },
      status: "scheduled",
      next_fire_at: "2026-09-01T02:00:00Z",
      created_at: "2026-08-20T01:00:00Z",
      updated_at: "2026-08-20T01:00:00Z",
    };
    const updated = {
      ...reminder,
      schedule: { kind: "before_date", days_before: 14, time_local: "08:30" },
    };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(reminder, 201))
      .mockResolvedValueOnce(jsonResponse({ items: [reminder] }))
      .mockResolvedValueOnce(jsonResponse(updated))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await createReminder("verified-access-token", {
      source_kind: "document",
      source_id: "document/owned",
      schedule: { kind: "before_date", days_before: 30, time_local: "09:00" },
    });
    await expect(getReminders(
      "verified-access-token",
      "document",
      "document/owned",
    )).resolves.toHaveLength(1);
    await updateReminder("verified-access-token", "reminder/owned", {
      schedule: { kind: "before_date", days_before: 14, time_local: "08:30" },
    });
    await deleteReminder("verified-access-token", "reminder/owned");

    const [createUrl, createInit] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(createUrl).toBe("http://127.0.0.1:8080/api/v1/reminders");
    expect(JSON.parse(String(createInit.body))).toEqual({
      source_kind: "document",
      source_id: "document/owned",
      schedule: { kind: "before_date", days_before: 30, time_local: "09:00" },
    });
    expect((fetchMock.mock.calls[1] as [string, RequestInit])[0]).toBe(
      "http://127.0.0.1:8080/api/v1/reminders?source_kind=document&source_id=document%2Fowned",
    );
    expect(JSON.parse(String((fetchMock.mock.calls[2] as [string, RequestInit])[1].body)))
      .toEqual({ schedule: { kind: "before_date", days_before: 14, time_local: "08:30" } });
    expect((fetchMock.mock.calls[3] as [string, RequestInit])[0]).toContain(
      "/reminders/reminder%2Fowned",
    );
  });

  it("lists, updates, and stops owned recurrence series", async () => {
    const series = {
      id: "series/owned",
      source_kind: "bill",
      title: "Internet",
      frequency: "monthly",
      interval: 1,
      anchor_on: "2026-08-15",
      ends_on: null,
      timezone: "Asia/Jakarta",
      active: true,
      created_at: "2026-08-20T01:00:00Z",
      updated_at: "2026-08-20T01:00:00Z",
    };
    const updated = { ...series, frequency: "weekly", interval: 2 };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ items: [series] }))
      .mockResolvedValueOnce(jsonResponse(updated))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(getRecurrenceSeries("verified-access-token")).resolves.toMatchObject([
      { id: "series/owned", sourceKind: "bill", frequency: "monthly" },
    ]);
    await expect(updateRecurrenceSeries("verified-access-token", "series/owned", {
      frequency: "weekly",
      interval: 2,
    })).resolves.toMatchObject({ frequency: "weekly", interval: 2 });
    await stopRecurrenceSeries("verified-access-token", "series/owned");

    expect((fetchMock.mock.calls[0] as [string, RequestInit])[0]).toBe(
      "http://127.0.0.1:8080/api/v1/recurrence-series",
    );
    const [patchUrl, patchInit] = fetchMock.mock.calls[1] as [string, RequestInit];
    expect(patchUrl).toBe("http://127.0.0.1:8080/api/v1/recurrence-series/series%2Fowned");
    expect(patchInit.method).toBe("PATCH");
    expect(JSON.parse(String(patchInit.body))).toEqual({ frequency: "weekly", interval: 2 });
    expect((fetchMock.mock.calls[2] as [string, RequestInit])[1].method).toBe("DELETE");
  });

  it("lists stable notification pages and persists read actions", async () => {
    const unread = {
      id: "notification/owned",
      source_kind: "task",
      source_id: "task/owned",
      title: "Tugas perlu diperhatikan",
      body: "Kirim laporan segera jatuh tempo.",
      created_at: "2026-08-20T02:00:00Z",
      read_at: null,
    };
    const read = { ...unread, read_at: "2026-08-20T03:00:00Z" };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({
        items: [unread],
        next_cursor: "opaque cursor",
        unread_count: 1,
      }))
      .mockResolvedValueOnce(jsonResponse({ unread_count: 1 }))
      .mockResolvedValueOnce(jsonResponse({ item: read, unread_count: 0 }))
      .mockResolvedValueOnce(jsonResponse({ marked_read: 0, unread_count: 0 }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(getNotifications("verified-access-token", {
      limit: 50,
      cursor: "opaque cursor",
    })).resolves.toMatchObject({ nextCursor: "opaque cursor", unreadCount: 1 });
    await expect(getNotificationUnreadCount("verified-access-token")).resolves.toBe(1);
    await expect(markNotificationRead(
      "verified-access-token",
      "notification/owned",
    )).resolves.toMatchObject({ item: { readAt: "2026-08-20T03:00:00Z" }, unreadCount: 0 });
    await expect(markAllNotificationsRead("verified-access-token")).resolves.toEqual({
      markedRead: 0,
      unreadCount: 0,
    });

    expect((fetchMock.mock.calls[0] as [string, RequestInit])[0]).toBe(
      "http://127.0.0.1:8080/api/v1/notifications?limit=50&cursor=opaque+cursor",
    );
    expect((fetchMock.mock.calls[1] as [string, RequestInit])[0]).toContain(
      "/notifications/unread-count",
    );
    expect((fetchMock.mock.calls[2] as [string, RequestInit])[0]).toContain(
      "/notifications/notification%2Fowned/mark-read",
    );
    expect((fetchMock.mock.calls[3] as [string, RequestInit])[0]).toContain(
      "/notifications/mark-all-read",
    );
  });
});
