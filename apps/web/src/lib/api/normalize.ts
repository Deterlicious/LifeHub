import type { Priority, Profile, Task, Today, TodayItem } from "@/lib/api/types";

type JsonRecord = Record<string, unknown>;

function isRecord(value: unknown): value is JsonRecord {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function unwrap(value: unknown, keys: string[]): JsonRecord {
  if (!isRecord(value)) return {};

  for (const key of keys) {
    if (isRecord(value[key])) return value[key];
  }

  return value;
}

function stringValue(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function nullableString(value: unknown): string | null {
  return typeof value === "string" && value.length > 0 ? value : null;
}

function numberValue(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function priorityValue(value: unknown): Priority {
  return value === "low" || value === "high" ? value : "normal";
}

export function normalizeProfile(value: unknown): Profile {
  const profile = unwrap(value, ["profile", "data"]);

  return {
    timezone: stringValue(profile.timezone),
    locale: stringValue(profile.locale, "id-ID"),
    currency: stringValue(profile.currency, "IDR"),
  };
}

export function normalizeTask(value: unknown): Task {
  const task = unwrap(value, ["task", "data"]);

  return {
    id: stringValue(task.id ?? task.entity_id),
    title: stringValue(task.title, "Tugas tanpa judul"),
    notes: nullableString(task.notes),
    priority: priorityValue(task.priority),
    dueAt: nullableString(task.due_at ?? task.due_local),
    completedAt: nullableString(task.completed_at),
  };
}

function normalizeTodayItem(value: unknown): TodayItem | null {
  if (!isRecord(value)) return null;

  const task = normalizeTask(value);
  if (!task.id) return null;

  return {
    ...task,
    kind: stringValue(value.kind ?? value.entity_type, "task"),
    urgency: stringValue(value.urgency, "today"),
    status: stringValue(value.status, task.completedAt ? "completed" : "open"),
  };
}

export function normalizeToday(value: unknown): Today {
  const today = unwrap(value, ["today", "data"]);
  const rawItems = Array.isArray(today.items) ? today.items : [];
  const summary = isRecord(today.summary) ? today.summary : {};

  return {
    date: stringValue(today.date),
    timezone: stringValue(today.timezone),
    items: rawItems.map(normalizeTodayItem).filter((item): item is TodayItem => item !== null),
    summary: {
      open: numberValue(summary.open),
      completed: numberValue(summary.completed),
    },
  };
}
