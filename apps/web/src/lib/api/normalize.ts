import type {
  Agenda,
  AgendaBillItem,
  AgendaDocumentItem,
  AgendaEventItem,
  AgendaItem,
  AgendaTaskItem,
  Bill,
  BillListPage,
  DocumentCategory,
  DocumentRecord,
  Event,
  MarkAllNotificationsReadResult,
  MarkNotificationReadResult,
  NotificationItem,
  NotificationsPage,
  Priority,
  Profile,
  RecurrenceFrequency,
  RecurrenceSeries,
  SmartCaptureDraft,
  SmartCaptureKind,
  SmartCaptureResult,
  Reminder,
  ReminderSchedule,
  ReminderSourceKind,
  Task,
  Today,
  TodayBillItem,
  TodayDocumentItem,
  TodayEventItem,
  TodayItem,
  TodayTaskItem,
} from "@/lib/api/types";

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

function integerValue(value: unknown): number {
  return typeof value === "number" && Number.isSafeInteger(value) ? value : 0;
}

function nonNegativeIntegerValue(value: unknown): number {
  const normalized = integerValue(value);
  return normalized >= 0 ? normalized : 0;
}

function reminderSourceKindValue(value: unknown): ReminderSourceKind {
  if (value === "event" || value === "bill" || value === "document") return value;
  return "task";
}

function priorityValue(value: unknown): Priority {
  return value === "low" || value === "high" ? value : "normal";
}

function recurrenceFrequencyValue(value: unknown): RecurrenceFrequency {
  if (value === "daily" || value === "weekly" || value === "yearly") return value;
  return "monthly";
}

function recurrenceSourceKindValue(value: unknown): RecurrenceSeries["sourceKind"] {
  if (value === "event" || value === "bill") return value;
  return "task";
}

function documentCategoryValue(value: unknown): DocumentCategory {
  if (
    value === "identity"
    || value === "license"
    || value === "insurance"
    || value === "education"
    || value === "work"
  ) {
    return value;
  }
  return "other";
}

function smartCaptureKindValue(value: unknown): SmartCaptureKind {
  if (value === "event" || value === "bill" || value === "document") return value;
  return "task";
}

function normalizeSmartCaptureDraft(value: unknown): SmartCaptureDraft {
  const draft = isRecord(value) ? value : {};
  const recurrence = isRecord(draft.recurrence) ? draft.recurrence : null;
  const rawAmount = draft.amount;

  return {
    kind: smartCaptureKindValue(draft.kind),
    title: stringValue(draft.title),
    notes: stringValue(draft.notes),
    priority: priorityValue(draft.priority),
    dueLocal: stringValue(draft.due_local),
    allDay: typeof draft.all_day === "boolean" ? draft.all_day : null,
    startsLocal: stringValue(draft.starts_local),
    endsLocal: stringValue(draft.ends_local),
    startsOn: stringValue(draft.starts_on),
    endsOn: stringValue(draft.ends_on),
    location: stringValue(draft.location),
    amount: typeof rawAmount === "number" && Number.isSafeInteger(rawAmount) && rawAmount > 0
      ? rawAmount
      : null,
    currency: stringValue(draft.currency, "IDR").toUpperCase(),
    name: stringValue(draft.name),
    category: documentCategoryValue(draft.category),
    expiresOn: stringValue(draft.expires_on),
    recurrence: recurrence
      ? {
          frequency: recurrenceFrequencyValue(recurrence.frequency),
          interval: Math.max(1, integerValue(recurrence.interval) || 1),
          ends_on: stringValue(recurrence.ends_on) || undefined,
        }
      : null,
    confidence: typeof draft.confidence === "number" && Number.isFinite(draft.confidence)
      ? Math.min(1, Math.max(0, draft.confidence))
      : 0,
  };
}

export function normalizeSmartCapture(value: unknown): SmartCaptureResult {
  const payload = unwrap(value, ["data"]);
  const ambiguities = Array.isArray(payload.ambiguities)
    ? payload.ambiguities.filter((item): item is string => typeof item === "string")
    : [];
  return {
    draft: normalizeSmartCaptureDraft(payload.draft),
    ambiguities,
    provider: stringValue(payload.provider),
  };
}

function documentStatusValue(value: unknown): DocumentRecord["status"] {
  if (value === "expired" || value === "expiring") return value;
  return "valid";
}

function todayDocumentStatusValue(value: unknown): TodayDocumentItem["status"] {
  return value === "expired" ? "expired" : "expiring";
}

function todayDocumentBucketValue(value: unknown): TodayDocumentItem["bucket"] {
  if (value === "expired" || value === "expires_today") return value;
  return "expiring_soon";
}

function todayDocumentUrgencyValue(value: unknown): TodayDocumentItem["urgency"] {
  if (value === "overdue" || value === "today") return value;
  return "upcoming";
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

export function normalizeEvent(value: unknown): Event {
  const event = unwrap(value, ["event", "data"]);

  return {
    id: stringValue(event.id ?? event.entity_id),
    title: stringValue(event.title, "Jadwal tanpa judul"),
    notes: nullableString(event.notes),
    location: nullableString(event.location),
    allDay: event.all_day === true,
    timezone: stringValue(event.timezone),
    startsAt: nullableString(event.starts_at),
    endsAt: nullableString(event.ends_at),
    startsOn: nullableString(event.starts_on),
    endsOn: nullableString(event.ends_on),
    createdAt: stringValue(event.created_at),
    updatedAt: stringValue(event.updated_at),
  };
}

export function normalizeBill(value: unknown): Bill {
  const bill = unwrap(value, ["bill", "data"]);

  return {
    id: stringValue(bill.id ?? bill.entity_id),
    title: stringValue(bill.title, "Tagihan tanpa judul"),
    notes: nullableString(bill.notes),
    amount: integerValue(bill.amount),
    currency: stringValue(bill.currency, "IDR").toUpperCase(),
    dueAt: stringValue(bill.due_at),
    paidAt: nullableString(bill.paid_at),
    createdAt: stringValue(bill.created_at),
    updatedAt: stringValue(bill.updated_at),
  };
}

export function normalizeBillsPage(value: unknown): BillListPage {
  const page = unwrap(value, ["bills", "data"]);
  const rawItems = Array.isArray(page.items) ? page.items : [];
  return {
    items: rawItems.map(normalizeBill).filter((bill) => bill.id.length > 0),
    nextCursor: nullableString(page.next_cursor),
  };
}

export function normalizeEvents(value: unknown): Event[] {
  const page = unwrap(value, ["events", "data"]);
  const rawItems = Array.isArray(page.items) ? page.items : [];
  return rawItems.map(normalizeEvent).filter((event) => event.id.length > 0);
}

export function normalizeRecurrenceSeries(value: unknown): RecurrenceSeries {
  const series = unwrap(value, ["series", "data"]);
  return {
    id: stringValue(series.id),
    sourceKind: recurrenceSourceKindValue(series.source_kind),
    title: stringValue(series.title, "Seri tanpa judul"),
    frequency: recurrenceFrequencyValue(series.frequency),
    interval: Math.max(1, integerValue(series.interval)),
    anchorOn: stringValue(series.anchor_on),
    endsOn: nullableString(series.ends_on),
    timezone: stringValue(series.timezone),
    active: series.active === true,
    createdAt: stringValue(series.created_at),
    updatedAt: stringValue(series.updated_at),
  };
}

export function normalizeRecurrenceSeriesList(value: unknown): RecurrenceSeries[] {
  const collection = unwrap(value, ["recurrence_series", "data"]);
  const rawItems = Array.isArray(collection.items) ? collection.items : [];
  return rawItems
    .map(normalizeRecurrenceSeries)
    .filter((series) => series.id.length > 0);
}

export function normalizeDocument(value: unknown): DocumentRecord {
  const documentRecord = unwrap(value, ["document", "data"]);

  return {
    id: stringValue(documentRecord.id ?? documentRecord.entity_id),
    name: stringValue(documentRecord.name, "Dokumen tanpa nama"),
    category: documentCategoryValue(documentRecord.category),
    notes: nullableString(documentRecord.notes),
    expiresOn: stringValue(documentRecord.expires_on),
    status: documentStatusValue(documentRecord.status),
    daysUntilExpiry: integerValue(documentRecord.days_until_expiry),
    createdAt: stringValue(documentRecord.created_at),
    updatedAt: stringValue(documentRecord.updated_at),
  };
}

export function normalizeDocuments(value: unknown): DocumentRecord[] {
  const rawDocuments = Array.isArray(value)
    ? value
    : isRecord(value) && Array.isArray(value.documents)
      ? value.documents
      : isRecord(value) && isRecord(value.data) && Array.isArray(value.data.documents)
        ? value.data.documents
        : [];

  return rawDocuments
    .map(normalizeDocument)
    .filter((documentRecord) => documentRecord.id.length > 0);
}

function normalizeTodayTask(value: JsonRecord): TodayTaskItem | null {
  const task = normalizeTask(value);
  if (!task.id) return null;

  return {
    ...task,
    kind: "task",
    urgency: stringValue(value.urgency, "today"),
    status: stringValue(value.status, task.completedAt ? "completed" : "open"),
    bucket: stringValue(value.bucket, task.completedAt ? "completed_today" : "due_today"),
  };
}

function normalizeTodayEvent(value: JsonRecord): TodayEventItem | null {
  const event = normalizeEvent(value);
  if (!event.id) return null;

  return {
    id: event.id,
    kind: "event",
    title: event.title,
    notes: event.notes,
    location: event.location,
    allDay: event.allDay,
    timezone: event.timezone,
    startsAt: event.startsAt,
    endsAt: event.endsAt,
    startsOn: event.startsOn,
    endsOn: event.endsOn,
    urgency: stringValue(value.urgency, "today"),
    status: stringValue(value.status, "scheduled"),
    bucket: stringValue(value.bucket, event.allDay ? "all_day" : "scheduled_today"),
  };
}

function normalizeTodayBill(value: JsonRecord): TodayBillItem | null {
  const bill = normalizeBill(value);
  if (!bill.id) return null;

  return {
    id: bill.id,
    kind: "bill",
    title: bill.title,
    notes: bill.notes,
    amount: bill.amount,
    currency: bill.currency,
    dueAt: bill.dueAt,
    paidAt: bill.paidAt,
    urgency: stringValue(value.urgency, "today"),
    status: stringValue(value.status, bill.paidAt ? "paid" : "unpaid"),
    bucket: stringValue(value.bucket, bill.paidAt ? "paid_today" : "due_today"),
  };
}

function normalizeTodayDocument(value: JsonRecord): TodayDocumentItem | null {
  const documentRecord = normalizeDocument(value);
  if (!documentRecord.id) return null;

  return {
    id: documentRecord.id,
    kind: "document",
    title: stringValue(value.title, documentRecord.name),
    category: documentRecord.category,
    notes: documentRecord.notes,
    expiresOn: documentRecord.expiresOn,
    daysUntilExpiry: documentRecord.daysUntilExpiry,
    urgency: todayDocumentUrgencyValue(value.urgency),
    status: todayDocumentStatusValue(value.status),
    bucket: todayDocumentBucketValue(value.bucket),
  };
}

function normalizeTodayItem(value: unknown): TodayItem | null {
  if (!isRecord(value)) return null;

  const kind = stringValue(value.kind ?? value.entity_type, "task");
  if (kind === "event") return normalizeTodayEvent(value);
  if (kind === "bill") return normalizeTodayBill(value);
  if (kind === "document") return normalizeTodayDocument(value);
  if (kind === "task") return normalizeTodayTask(value);

  return null;
}

export function normalizeToday(value: unknown): Today {
  const today = unwrap(value, ["today", "data"]);
  const rawItems = Array.isArray(today.items) ? today.items : [];
  const rawUpcoming = Array.isArray(today.upcoming) ? today.upcoming : [];
  const summary = isRecord(today.summary) ? today.summary : {};

  return {
    date: stringValue(today.date),
    timezone: stringValue(today.timezone),
    items: rawItems.map(normalizeTodayItem).filter((item): item is TodayItem => item !== null),
    upcoming: rawUpcoming
      .map(normalizeTodayItem)
      .filter((item): item is TodayItem => item !== null),
    upcomingHorizonDays: numberValue(today.upcoming_horizon_days),
    summary: {
      open: numberValue(summary.open),
      completed: numberValue(summary.completed),
      upcoming: numberValue(summary.upcoming),
    },
  };
}

function agendaItemBase(value: JsonRecord) {
  return {
    id: stringValue(value.id ?? value.entity_id),
    title: stringValue(value.title, "Item tanpa judul"),
    notes: nullableString(value.notes),
    displayOn: stringValue(value.display_on),
    createdAt: stringValue(value.created_at),
    updatedAt: stringValue(value.updated_at),
    status: stringValue(value.status),
  };
}

function normalizeAgendaTask(value: JsonRecord): AgendaTaskItem | null {
  const task = normalizeTask(value);
  const base = agendaItemBase(value);
  if (!base.id || !base.displayOn) return null;
  return {
    ...base,
    kind: "task",
    title: task.title,
    notes: task.notes,
    priority: task.priority,
    dueAt: task.dueAt,
    completedAt: task.completedAt,
  };
}

function normalizeAgendaEvent(value: JsonRecord): AgendaEventItem | null {
  const event = normalizeEvent(value);
  const base = agendaItemBase(value);
  if (!base.id || !base.displayOn) return null;
  return {
    ...base,
    kind: "event",
    title: event.title,
    notes: event.notes,
    location: event.location,
    allDay: event.allDay,
    timezone: event.timezone,
    startsAt: event.startsAt,
    endsAt: event.endsAt,
    startsOn: event.startsOn,
    endsOn: event.endsOn,
  };
}

function normalizeAgendaBill(value: JsonRecord): AgendaBillItem | null {
  const bill = normalizeBill(value);
  const base = agendaItemBase(value);
  if (!base.id || !base.displayOn) return null;
  return {
    ...base,
    kind: "bill",
    title: bill.title,
    notes: bill.notes,
    amount: bill.amount,
    currency: bill.currency,
    dueAt: bill.dueAt,
    paidAt: bill.paidAt,
  };
}

function normalizeAgendaDocument(value: JsonRecord): AgendaDocumentItem | null {
  const documentRecord = normalizeDocument(value);
  const base = agendaItemBase(value);
  if (!base.id || !base.displayOn) return null;
  return {
    ...base,
    kind: "document",
    title: stringValue(value.title, documentRecord.name),
    notes: documentRecord.notes,
    category: documentRecord.category,
    expiresOn: documentRecord.expiresOn,
    daysUntilExpiry: documentRecord.daysUntilExpiry,
    status: documentRecord.status,
  };
}

function normalizeAgendaItem(value: unknown): AgendaItem | null {
  if (!isRecord(value)) return null;
  const kind = stringValue(value.kind ?? value.entity_type);
  if (kind === "task") return normalizeAgendaTask(value);
  if (kind === "event") return normalizeAgendaEvent(value);
  if (kind === "bill") return normalizeAgendaBill(value);
  if (kind === "document") return normalizeAgendaDocument(value);
  return null;
}

export function normalizeAgenda(value: unknown): Agenda {
  const agenda = unwrap(value, ["agenda", "data"]);
  const rawItems = Array.isArray(agenda.items) ? agenda.items : [];
  const items = rawItems
    .map(normalizeAgendaItem)
    .filter((item): item is AgendaItem => item !== null);
  const summary = isRecord(agenda.summary) ? agenda.summary : {};

  return {
    from: stringValue(agenda.from),
    to: stringValue(agenda.to),
    timezone: stringValue(agenda.timezone),
    items,
    summary: {
      total: typeof summary.total === "number" ? numberValue(summary.total) : items.length,
      tasks: typeof summary.tasks === "number"
        ? numberValue(summary.tasks)
        : items.filter((item) => item.kind === "task").length,
      events: typeof summary.events === "number"
        ? numberValue(summary.events)
        : items.filter((item) => item.kind === "event").length,
      bills: typeof summary.bills === "number"
        ? numberValue(summary.bills)
        : items.filter((item) => item.kind === "bill").length,
      documents: typeof summary.documents === "number"
        ? numberValue(summary.documents)
        : items.filter((item) => item.kind === "document").length,
    },
  };
}

function normalizeReminderSchedule(value: unknown): ReminderSchedule {
  const schedule = isRecord(value) ? value : {};
  if (schedule.kind === "before_date") {
    return {
      kind: "before_date",
      daysBefore: nonNegativeIntegerValue(schedule.days_before),
      timeLocal: stringValue(schedule.time_local),
    };
  }
  return {
    kind: "before_moment",
    minutesBefore: nonNegativeIntegerValue(schedule.minutes_before),
  };
}

export function normalizeReminder(value: unknown): Reminder {
  const reminder = unwrap(value, ["reminder", "data"]);
  const status = reminder.status === "delivered" || reminder.status === "inactive"
    ? reminder.status
    : "scheduled";
  return {
    id: stringValue(reminder.id),
    sourceKind: reminderSourceKindValue(reminder.source_kind),
    sourceId: stringValue(reminder.source_id),
    schedule: normalizeReminderSchedule(reminder.schedule),
    status,
    nextFireAt: nullableString(reminder.next_fire_at),
    createdAt: stringValue(reminder.created_at),
    updatedAt: stringValue(reminder.updated_at),
  };
}

export function normalizeReminders(value: unknown): Reminder[] {
  const collection = unwrap(value, ["reminders", "data"]);
  const rawItems = Array.isArray(collection.items) ? collection.items : [];
  return rawItems
    .map(normalizeReminder)
    .filter((reminder) => reminder.id.length > 0 && reminder.sourceId.length > 0);
}

export function normalizeNotification(value: unknown): NotificationItem {
  const notification = unwrap(value, ["item", "notification", "data"]);
  return {
    id: stringValue(notification.id),
    sourceKind: reminderSourceKindValue(notification.source_kind),
    sourceId: stringValue(notification.source_id),
    title: stringValue(notification.title, "Pengingat LifeHub"),
    body: stringValue(notification.body),
    createdAt: stringValue(notification.created_at),
    readAt: nullableString(notification.read_at),
  };
}

export function normalizeNotificationsPage(value: unknown): NotificationsPage {
  const collection = unwrap(value, ["notifications", "data"]);
  const rawItems = Array.isArray(collection.items) ? collection.items : [];
  return {
    items: rawItems
      .map(normalizeNotification)
      .filter((notification) => notification.id.length > 0),
    nextCursor: nullableString(collection.next_cursor),
    unreadCount: nonNegativeIntegerValue(collection.unread_count),
  };
}

export function normalizeUnreadCount(value: unknown): number {
  const payload = unwrap(value, ["data"]);
  return nonNegativeIntegerValue(payload.unread_count);
}

export function normalizeMarkNotificationRead(value: unknown): MarkNotificationReadResult {
  const payload = unwrap(value, ["data"]);
  return {
    item: normalizeNotification(payload.item),
    unreadCount: nonNegativeIntegerValue(payload.unread_count),
  };
}

export function normalizeMarkAllNotificationsRead(
  value: unknown,
): MarkAllNotificationsReadResult {
  const payload = unwrap(value, ["data"]);
  return {
    markedRead: nonNegativeIntegerValue(payload.marked_read),
    unreadCount: nonNegativeIntegerValue(payload.unread_count),
  };
}
