import { getApiBaseUrl } from "@/lib/config";
import {
  normalizeAgenda,
  normalizeBill,
  normalizeBillsPage,
  normalizeDocument,
  normalizeDocuments,
  normalizeEvent,
  normalizeEvents,
  normalizeMarkAllNotificationsRead,
  normalizeMarkNotificationRead,
  normalizeNotificationsPage,
  normalizeProfile,
  normalizeRecurrenceSeries,
  normalizeRecurrenceSeriesList,
  normalizeReminder,
  normalizeReminders,
  normalizeSmartCapture,
  normalizeTask,
  normalizeToday,
  normalizeUnreadCount,
} from "@/lib/api/normalize";
import type {
  Agenda,
  Bill,
  BillListPage,
  CreateBillInput,
  CreateDocumentInput,
  CreateEventInput,
  CreateReminderInput,
  CreateTaskInput,
  DevSession,
  DocumentRecord,
  Event,
  MarkAllNotificationsReadResult,
  MarkNotificationReadResult,
  NotificationsPage,
  Profile,
  RecurrenceInput,
  RecurrenceSeries,
  Reminder,
  ReminderSourceKind,
  SmartCaptureResult,
  Task,
  Today,
  UpdateBillInput,
  UpdateDocumentInput,
  UpdateEventInput,
  UpdateReminderInput,
  UpdateTaskInput,
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
  } catch (reason) {
    if (reason instanceof DOMException && reason.name === "AbortError") throw reason;
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

export async function deleteProfileData(token: string, confirmation: string): Promise<void> {
  await apiRequest<unknown>("/profile/data", {
    method: "DELETE",
    token,
    body: { confirmation },
  });
}

export async function parseSmartCapture(token: string, text: string): Promise<SmartCaptureResult> {
  const response = await apiRequest<unknown>("/smart-capture/parse", {
    method: "POST",
    token,
    body: { text },
  });
  return normalizeSmartCapture(response);
}

export async function getToday(token: string, signal?: AbortSignal): Promise<Today> {
  const response = await apiRequest<unknown>("/today", { token, signal });
  return normalizeToday(response);
}

export async function getAgenda(
  token: string,
  range?: { from: string; to: string },
  signal?: AbortSignal,
): Promise<Agenda> {
  const query = range
    ? `?${new URLSearchParams({ from: range.from, to: range.to }).toString()}`
    : "";
  const response = await apiRequest<unknown>(`/agenda${query}`, { token, signal });
  return normalizeAgenda(response);
}

export async function createTask(token: string, input: CreateTaskInput): Promise<Task> {
  const response = await apiRequest<unknown>("/tasks", {
    method: "POST",
    token,
    body: input,
  });
  return normalizeTask(response);
}

export async function getTask(token: string, taskId: string): Promise<Task> {
  const response = await apiRequest<unknown>(`/tasks/${encodeURIComponent(taskId)}`, { token });
  return normalizeTask(response);
}

export async function updateTask(
  token: string,
  taskId: string,
  input: UpdateTaskInput,
): Promise<Task> {
  const response = await apiRequest<unknown>(`/tasks/${encodeURIComponent(taskId)}`, {
    method: "PATCH",
    token,
    body: input,
  });
  return normalizeTask(response);
}

export async function deleteTask(token: string, taskId: string): Promise<void> {
  await apiRequest<unknown>(`/tasks/${encodeURIComponent(taskId)}`, {
    method: "DELETE",
    token,
  });
}

export async function uncompleteTask(token: string, taskId: string): Promise<Task> {
  const response = await apiRequest<unknown>(`/tasks/${encodeURIComponent(taskId)}/uncomplete`, {
    method: "POST",
    token,
  });
  return normalizeTask(response);
}

export async function createEvent(token: string, input: CreateEventInput): Promise<Event> {
  const response = await apiRequest<unknown>("/events", {
    method: "POST",
    token,
    body: input,
  });
  return normalizeEvent(response);
}

export async function getEvents(
  token: string,
  range?: { from: string; to: string },
): Promise<Event[]> {
  const query = range
    ? `?${new URLSearchParams({ from: range.from, to: range.to }).toString()}`
    : "";
  const response = await apiRequest<unknown>(`/events${query}`, { token });
  return normalizeEvents(response);
}

export async function getEvent(token: string, eventId: string): Promise<Event> {
  const response = await apiRequest<unknown>(`/events/${encodeURIComponent(eventId)}`, { token });
  return normalizeEvent(response);
}

export async function updateEvent(
  token: string,
  eventId: string,
  input: UpdateEventInput,
): Promise<Event> {
  const response = await apiRequest<unknown>(`/events/${encodeURIComponent(eventId)}`, {
    method: "PATCH",
    token,
    body: input,
  });
  return normalizeEvent(response);
}

export async function deleteEvent(token: string, eventId: string): Promise<void> {
  await apiRequest<unknown>(`/events/${encodeURIComponent(eventId)}`, {
    method: "DELETE",
    token,
  });
}

export async function createBill(token: string, input: CreateBillInput): Promise<Bill> {
  const response = await apiRequest<unknown>("/bills", {
    method: "POST",
    token,
    body: input,
  });
  return normalizeBill(response);
}

export async function getRecurrenceSeries(
  token: string,
  signal?: AbortSignal,
): Promise<RecurrenceSeries[]> {
  const response = await apiRequest<unknown>("/recurrence-series", { token, signal });
  return normalizeRecurrenceSeriesList(response);
}

export async function updateRecurrenceSeries(
  token: string,
  seriesId: string,
  input: RecurrenceInput,
): Promise<RecurrenceSeries> {
  const response = await apiRequest<unknown>(`/recurrence-series/${encodeURIComponent(seriesId)}`, {
    method: "PATCH",
    token,
    body: input,
  });
  return normalizeRecurrenceSeries(response);
}

export async function stopRecurrenceSeries(token: string, seriesId: string): Promise<void> {
  await apiRequest<unknown>(`/recurrence-series/${encodeURIComponent(seriesId)}`, {
    method: "DELETE",
    token,
  });
}

export async function getBills(
  token: string,
  options: { state?: "unpaid" | "paid"; limit?: number; cursor?: string } = {},
): Promise<BillListPage> {
  const query = new URLSearchParams();
  if (options.state) query.set("state", options.state);
  if (options.limit) query.set("limit", String(options.limit));
  if (options.cursor) query.set("cursor", options.cursor);
  const suffix = query.size > 0 ? `?${query.toString()}` : "";
  const response = await apiRequest<unknown>(`/bills${suffix}`, { token });
  return normalizeBillsPage(response);
}

export async function getBill(token: string, billId: string): Promise<Bill> {
  const response = await apiRequest<unknown>(`/bills/${encodeURIComponent(billId)}`, { token });
  return normalizeBill(response);
}

export async function updateBill(
  token: string,
  billId: string,
  input: UpdateBillInput,
): Promise<Bill> {
  const response = await apiRequest<unknown>(`/bills/${encodeURIComponent(billId)}`, {
    method: "PATCH",
    token,
    body: input,
  });
  return normalizeBill(response);
}

export async function deleteBill(token: string, billId: string): Promise<void> {
  await apiRequest<unknown>(`/bills/${encodeURIComponent(billId)}`, {
    method: "DELETE",
    token,
  });
}

export async function createDocument(
  token: string,
  input: CreateDocumentInput,
): Promise<DocumentRecord> {
  const response = await apiRequest<unknown>("/documents", {
    method: "POST",
    token,
    body: input,
  });
  return normalizeDocument(response);
}

export async function getDocuments(token: string, signal?: AbortSignal): Promise<DocumentRecord[]> {
  const response = await apiRequest<unknown>("/documents", { token, signal });
  return normalizeDocuments(response);
}

export async function updateDocument(
  token: string,
  documentId: string,
  input: UpdateDocumentInput,
): Promise<DocumentRecord> {
  const response = await apiRequest<unknown>(`/documents/${encodeURIComponent(documentId)}`, {
    method: "PATCH",
    token,
    body: input,
  });
  return normalizeDocument(response);
}

export async function deleteDocument(token: string, documentId: string): Promise<void> {
  await apiRequest<unknown>(`/documents/${encodeURIComponent(documentId)}`, {
    method: "DELETE",
    token,
  });
}

export async function markBillPaid(token: string, billId: string): Promise<Bill> {
  const response = await apiRequest<unknown>(`/bills/${encodeURIComponent(billId)}/mark-paid`, {
    method: "POST",
    token,
  });
  return normalizeBill(response);
}

export async function markBillUnpaid(token: string, billId: string): Promise<Bill> {
  const response = await apiRequest<unknown>(`/bills/${encodeURIComponent(billId)}/mark-unpaid`, {
    method: "POST",
    token,
  });
  return normalizeBill(response);
}

export async function completeTask(token: string, taskId: string): Promise<void> {
  await apiRequest<unknown>(`/tasks/${encodeURIComponent(taskId)}/complete`, {
    method: "POST",
    token,
  });
}

export async function getReminders(
  token: string,
  sourceKind: ReminderSourceKind,
  sourceId: string,
  signal?: AbortSignal,
): Promise<Reminder[]> {
  const query = new URLSearchParams({ source_kind: sourceKind, source_id: sourceId });
  const response = await apiRequest<unknown>(`/reminders?${query.toString()}`, { token, signal });
  return normalizeReminders(response);
}

export async function createReminder(
  token: string,
  input: CreateReminderInput,
): Promise<Reminder> {
  const response = await apiRequest<unknown>("/reminders", {
    method: "POST",
    token,
    body: input,
  });
  return normalizeReminder(response);
}

export async function updateReminder(
  token: string,
  reminderId: string,
  input: UpdateReminderInput,
): Promise<Reminder> {
  const response = await apiRequest<unknown>(`/reminders/${encodeURIComponent(reminderId)}`, {
    method: "PATCH",
    token,
    body: input,
  });
  return normalizeReminder(response);
}

export async function deleteReminder(token: string, reminderId: string): Promise<void> {
  await apiRequest<unknown>(`/reminders/${encodeURIComponent(reminderId)}`, {
    method: "DELETE",
    token,
  });
}

export async function getNotifications(
  token: string,
  options: { limit?: number; cursor?: string } = {},
  signal?: AbortSignal,
): Promise<NotificationsPage> {
  const query = new URLSearchParams();
  if (options.limit) query.set("limit", String(options.limit));
  if (options.cursor) query.set("cursor", options.cursor);
  const suffix = query.size > 0 ? `?${query.toString()}` : "";
  const response = await apiRequest<unknown>(`/notifications${suffix}`, { token, signal });
  return normalizeNotificationsPage(response);
}

export async function getNotificationUnreadCount(
  token: string,
  signal?: AbortSignal,
): Promise<number> {
  const response = await apiRequest<unknown>("/notifications/unread-count", { token, signal });
  return normalizeUnreadCount(response);
}

export async function markNotificationRead(
  token: string,
  notificationId: string,
): Promise<MarkNotificationReadResult> {
  const response = await apiRequest<unknown>(
    `/notifications/${encodeURIComponent(notificationId)}/mark-read`,
    { method: "POST", token },
  );
  return normalizeMarkNotificationRead(response);
}

export async function markAllNotificationsRead(
  token: string,
): Promise<MarkAllNotificationsReadResult> {
  const response = await apiRequest<unknown>("/notifications/mark-all-read", {
    method: "POST",
    token,
  });
  return normalizeMarkAllNotificationsRead(response);
}
