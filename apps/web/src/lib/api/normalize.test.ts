import { describe, expect, it } from "vitest";

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

describe("API DTO normalization", () => {

  it("normalizes a smart-capture draft while preserving required review signals", () => {
    expect(normalizeSmartCapture({
      draft: {
        kind: "bill",
        title: "Internet",
        amount: 350000,
        currency: "idr",
        recurrence: { frequency: "monthly", interval: 1 },
        confidence: 0.72,
      },
      ambiguities: ["Jam jatuh tempo belum disebutkan."],
      provider: "rule",
    })).toEqual({
      draft: {
        kind: "bill",
        title: "Internet",
        notes: "",
        priority: "normal",
        dueLocal: "",
        allDay: null,
        startsLocal: "",
        endsLocal: "",
        startsOn: "",
        endsOn: "",
        location: "",
        amount: 350000,
        currency: "IDR",
        name: "",
        category: "other",
        expiresOn: "",
        recurrence: { frequency: "monthly", interval: 1, ends_on: undefined },
        confidence: 0.72,
      },
      ambiguities: ["Jam jatuh tempo belum disebutkan."],
      provider: "rule",
    });
  });
  it("accepts an enveloped profile and preserves explicit timezone", () => {
    expect(
      normalizeProfile({
        profile: { timezone: "Asia/Makassar", locale: "id-ID", currency: "IDR" },
      }),
    ).toEqual({ timezone: "Asia/Makassar", locale: "id-ID", currency: "IDR" });
  });

  it("normalizes task wire fields without inventing client ownership", () => {
    expect(
      normalizeTask({
        task: {
          id: "task-1",
          title: "Kirim laporan",
          notes: null,
          priority: "high",
          due_at: "2026-08-19T15:00:00+07:00",
          completed_at: null,
        },
      }),
    ).toEqual({
      id: "task-1",
      title: "Kirim laporan",
      notes: null,
      priority: "high",
      dueAt: "2026-08-19T15:00:00+07:00",
      completedAt: null,
    });
  });

  it("accepts entity_id Today rows and safely defaults priority to normal", () => {
    const today = normalizeToday({
      date: "2026-08-19",
      timezone: "Asia/Jakarta",
      items: [
        {
          entity_id: "task-2",
          kind: "task",
          title: "Belajar",
          priority: "unexpected",
          due_at: "2026-08-19T20:00:00+07:00",
          urgency: "today",
          status: "open",
        },
      ],
      summary: { open: 1, completed: 2 },
    });

    expect(today.items[0]).toMatchObject({ id: "task-2", priority: "normal" });
    expect(today.summary).toEqual({ open: 1, completed: 2, upcoming: 0 });
  });

  it("normalizes an all-day event without inventing task fields", () => {
    expect(
      normalizeEvent({
        event: {
          id: "event-1",
          title: "Lokakarya tim",
          notes: "Bawa catatan",
          location: "Ruang Cendana",
          all_day: true,
          timezone: "Asia/Jakarta",
          starts_at: null,
          ends_at: null,
          starts_on: "2026-08-20",
          ends_on: "2026-08-21",
          created_at: "2026-08-19T10:00:00Z",
          updated_at: "2026-08-19T10:00:00Z",
        },
      }),
    ).toEqual({
      id: "event-1",
      title: "Lokakarya tim",
      notes: "Bawa catatan",
      location: "Ruang Cendana",
      allDay: true,
      timezone: "Asia/Jakarta",
      startsAt: null,
      endsAt: null,
      startsOn: "2026-08-20",
      endsOn: "2026-08-21",
      createdAt: "2026-08-19T10:00:00Z",
      updatedAt: "2026-08-19T10:00:00Z",
    });
  });

  it("dispatches Today events by kind and never supplies task priority or completion", () => {
    const today = normalizeToday({
      date: "2026-08-20",
      timezone: "Asia/Jakarta",
      items: [
        {
          id: "event-2",
          kind: "event",
          title: "Review proyek",
          notes: null,
          location: "Online",
          all_day: false,
          timezone: "Asia/Jakarta",
          starts_at: "2026-08-20T07:00:00Z",
          ends_at: "2026-08-20T08:00:00Z",
          starts_on: null,
          ends_on: null,
          urgency: "now",
          status: "in_progress",
          bucket: "happening_now",
        },
      ],
      summary: { open: 1, completed: 0 },
    });

    expect(today.items[0]).toEqual({
      id: "event-2",
      kind: "event",
      title: "Review proyek",
      notes: null,
      location: "Online",
      allDay: false,
      timezone: "Asia/Jakarta",
      startsAt: "2026-08-20T07:00:00Z",
      endsAt: "2026-08-20T08:00:00Z",
      startsOn: null,
      endsOn: null,
      urgency: "now",
      status: "in_progress",
      bucket: "happening_now",
    });
    expect(today.items[0]).not.toHaveProperty("priority");
    expect(today.items[0]).not.toHaveProperty("completedAt");
  });

  it("normalizes integer bill money and paid state", () => {
    expect(
      normalizeBill({
        bill: {
          id: "bill-1",
          title: "Internet rumah",
          notes: null,
          amount: 350000,
          currency: "idr",
          due_at: "2026-08-20T16:59:00Z",
          paid_at: "2026-08-20T09:00:00Z",
          created_at: "2026-08-19T10:00:00Z",
          updated_at: "2026-08-20T09:00:00Z",
        },
      }),
    ).toEqual({
      id: "bill-1",
      title: "Internet rumah",
      notes: null,
      amount: 350000,
      currency: "IDR",
      dueAt: "2026-08-20T16:59:00Z",
      paidAt: "2026-08-20T09:00:00Z",
      createdAt: "2026-08-19T10:00:00Z",
      updatedAt: "2026-08-20T09:00:00Z",
    });
  });

  it("normalizes a Today bill without task or event-only fields", () => {
    const today = normalizeToday({
      date: "2026-08-20",
      timezone: "Asia/Jakarta",
      items: [
        {
          id: "bill-2",
          kind: "bill",
          title: "Listrik",
          notes: "Token bulanan",
          amount: 275000,
          currency: "IDR",
          due_at: "2026-08-20T12:00:00Z",
          paid_at: null,
          urgency: "today",
          status: "unpaid",
          bucket: "due_today",
        },
      ],
      summary: { open: 1, completed: 0 },
    });

    expect(today.items[0]).toEqual({
      id: "bill-2",
      kind: "bill",
      title: "Listrik",
      notes: "Token bulanan",
      amount: 275000,
      currency: "IDR",
      dueAt: "2026-08-20T12:00:00Z",
      paidAt: null,
      urgency: "today",
      status: "unpaid",
      bucket: "due_today",
    });
    expect(today.items[0]).not.toHaveProperty("priority");
    expect(today.items[0]).not.toHaveProperty("allDay");
  });

  it("normalizes owned document metadata with Go-derived expiry state", () => {
    expect(
      normalizeDocument({
        id: "document-1",
        name: "SIM A",
        category: "license",
        notes: "Periksa syarat perpanjangan",
        expires_on: "2026-08-20",
        status: "expiring",
        days_until_expiry: 0,
        created_at: "2026-08-19T10:00:00Z",
        updated_at: "2026-08-19T10:00:00Z",
      }),
    ).toEqual({
      id: "document-1",
      name: "SIM A",
      category: "license",
      notes: "Periksa syarat perpanjangan",
      expiresOn: "2026-08-20",
      status: "expiring",
      daysUntilExpiry: 0,
      createdAt: "2026-08-19T10:00:00Z",
      updatedAt: "2026-08-19T10:00:00Z",
    });

    expect(normalizeDocuments({ documents: [{ id: "document-2", category: "unknown" }] }))
      .toHaveLength(1);
    expect(normalizeDocuments([{ name: "Tanpa ID" }])).toEqual([]);
  });

  it("keeps expiring-soon documents separate from Today open items", () => {
    const today = normalizeToday({
      date: "2026-08-20",
      timezone: "Asia/Jakarta",
      items: [
        {
          id: "document-today",
          kind: "document",
          title: "SIM A",
          notes: null,
          category: "license",
          expires_on: "2026-08-20",
          days_until_expiry: 0,
          status: "expiring",
          bucket: "expires_today",
          urgency: "today",
        },
      ],
      upcoming: [
        {
          id: "document-upcoming",
          kind: "document",
          title: "Polis kesehatan",
          notes: null,
          category: "insurance",
          expires_on: "2026-09-19",
          days_until_expiry: 30,
          status: "expiring",
          bucket: "expiring_soon",
          urgency: "upcoming",
        },
      ],
      upcoming_horizon_days: 30,
      summary: { open: 1, completed: 0, upcoming: 1 },
    });

    expect(today.items[0]).toMatchObject({
      kind: "document",
      title: "SIM A",
      category: "license",
      expiresOn: "2026-08-20",
      daysUntilExpiry: 0,
      bucket: "expires_today",
    });
    expect(today.upcoming).toEqual([
      expect.objectContaining({
        id: "document-upcoming",
        title: "Polis kesehatan",
        daysUntilExpiry: 30,
        urgency: "upcoming",
      }),
    ]);
    expect(today.summary).toEqual({ open: 1, completed: 0, upcoming: 1 });
    expect(today.upcomingHorizonDays).toBe(30);
  });

  it("normalizes all four unresolved domains in Today Upcoming without capping", () => {
    const today = normalizeToday({
      date: "2026-08-20",
      timezone: "Asia/Jakarta",
      items: [],
      upcoming: [
        { kind: "task", id: "t1", title: "Tugas", notes: null, priority: "high", due_at: "2026-08-21T02:00:00Z", completed_at: null, status: "open", bucket: "upcoming", urgency: "upcoming" },
        { kind: "event", id: "e1", title: "Jadwal", notes: null, all_day: true, timezone: "Asia/Jakarta", starts_on: "2026-08-22", ends_on: "2026-08-22", status: "scheduled", bucket: "upcoming", urgency: "upcoming" },
        { kind: "bill", id: "b1", title: "Tagihan", notes: null, amount: 50000, currency: "IDR", due_at: "2026-08-23T02:00:00Z", paid_at: null, status: "unpaid", bucket: "upcoming", urgency: "upcoming" },
        { kind: "document", id: "d1", title: "Dokumen", notes: null, category: "work", expires_on: "2026-08-24", days_until_expiry: 4, status: "expiring", bucket: "expiring_soon", urgency: "upcoming" },
      ],
      upcoming_horizon_days: 30,
      summary: { open: 0, completed: 0, upcoming: 4 },
    });

    expect(today.upcoming.map((item) => item.kind)).toEqual(["task", "event", "bill", "document"]);
    expect(today.summary.upcoming).toBe(4);
  });

  it("normalizes the mixed Agenda contract and preserves Go ordering", () => {
    const agenda = normalizeAgenda({
      from: "2026-08-21",
      to: "2026-09-20",
      timezone: "Asia/Jakarta",
      items: [
        {
          kind: "event",
          id: "event-agenda",
          title: "Review",
          notes: null,
          display_on: "2026-08-21",
          all_day: false,
          timezone: "Asia/Jakarta",
          starts_at: "2026-08-21T01:00:00Z",
          ends_at: "2026-08-21T02:00:00Z",
          starts_on: null,
          ends_on: null,
          status: "scheduled",
          created_at: "2026-08-20T01:00:00Z",
          updated_at: "2026-08-20T01:00:00Z",
        },
        {
          kind: "document",
          id: "document-agenda",
          title: "Kontrak kerja",
          notes: null,
          category: "work",
          expires_on: "2026-09-20",
          days_until_expiry: 31,
          display_on: "2026-09-20",
          status: "valid",
          created_at: "2026-08-20T02:00:00Z",
          updated_at: "2026-08-20T02:00:00Z",
        },
      ],
      summary: { total: 2, tasks: 0, events: 1, bills: 0, documents: 1 },
    });

    expect(agenda.items.map((item) => item.id)).toEqual(["event-agenda", "document-agenda"]);
    expect(agenda.items[0]).toMatchObject({ kind: "event", displayOn: "2026-08-21" });
    expect(agenda.items[1]).toMatchObject({
      kind: "document",
      status: "valid",
      daysUntilExpiry: 31,
    });
    expect(agenda.summary).toEqual({ total: 2, tasks: 0, events: 1, bills: 0, documents: 1 });
  });

  it("normalizes event and paid-bill list envelopes", () => {
    expect(normalizeEvents({ items: [{ id: "event-list", title: "Agenda", all_day: true }] }))
      .toHaveLength(1);
    expect(normalizeBillsPage({
      items: [{ id: "bill-paid", title: "Internet", amount: 1, currency: "IDR", due_at: "2026-08-20T00:00:00Z" }],
      next_cursor: "cursor-next",
    })).toMatchObject({ items: [{ id: "bill-paid" }], nextCursor: "cursor-next" });
  });

  it("normalizes recurrence series and drops rows without an identity", () => {
    const raw = {
      id: "series-1",
      source_kind: "bill",
      title: "Internet rumah",
      frequency: "monthly",
      interval: 1,
      anchor_on: "2026-08-15",
      ends_on: null,
      timezone: "Asia/Jakarta",
      active: true,
      created_at: "2026-08-20T01:00:00Z",
      updated_at: "2026-08-20T01:00:00Z",
    };
    expect(normalizeRecurrenceSeries(raw)).toEqual({
      id: "series-1",
      sourceKind: "bill",
      title: "Internet rumah",
      frequency: "monthly",
      interval: 1,
      anchorOn: "2026-08-15",
      endsOn: null,
      timezone: "Asia/Jakarta",
      active: true,
      createdAt: "2026-08-20T01:00:00Z",
      updatedAt: "2026-08-20T01:00:00Z",
    });
    expect(normalizeRecurrenceSeriesList({ items: [raw, { title: "Tanpa ID" }] }))
      .toHaveLength(1);
  });

  it("normalizes strict moment/date reminder schedules without changing server order", () => {
    const reminders = normalizeReminders({
      items: [
        {
          id: "reminder-moment",
          source_kind: "task",
          source_id: "task-1",
          schedule: { kind: "before_moment", minutes_before: 90 },
          status: "scheduled",
          next_fire_at: "2026-08-21T01:30:00Z",
          created_at: "2026-08-20T01:00:00Z",
          updated_at: "2026-08-20T01:00:00Z",
        },
        {
          id: "reminder-date",
          source_kind: "document",
          source_id: "document-1",
          schedule: { kind: "before_date", days_before: 30, time_local: "09:00" },
          status: "inactive",
          next_fire_at: null,
          created_at: "2026-08-20T02:00:00Z",
          updated_at: "2026-08-20T02:00:00Z",
        },
      ],
    });

    expect(reminders.map((reminder) => reminder.id)).toEqual([
      "reminder-moment",
      "reminder-date",
    ]);
    expect(reminders[0].schedule).toEqual({ kind: "before_moment", minutesBefore: 90 });
    expect(reminders[1]).toMatchObject({
      sourceKind: "document",
      status: "inactive",
      schedule: { kind: "before_date", daysBefore: 30, timeLocal: "09:00" },
    });
    expect(normalizeReminder({ id: "fallback", source_id: "source", schedule: {} }))
      .toMatchObject({ schedule: { kind: "before_moment", minutesBefore: 0 } });
  });

  it("normalizes notification cursors, unread results, and read snapshots", () => {
    const item = {
      id: "notification-1",
      source_kind: "bill",
      source_id: "bill-1",
      title: "Tagihan segera jatuh tempo",
      body: "Internet rumah jatuh tempo hari ini.",
      created_at: "2026-08-20T02:00:00Z",
      read_at: null,
    };
    expect(normalizeNotificationsPage({
      items: [item],
      next_cursor: "opaque cursor",
      unread_count: 1,
    })).toEqual({
      items: [{
        id: "notification-1",
        sourceKind: "bill",
        sourceId: "bill-1",
        title: "Tagihan segera jatuh tempo",
        body: "Internet rumah jatuh tempo hari ini.",
        createdAt: "2026-08-20T02:00:00Z",
        readAt: null,
      }],
      nextCursor: "opaque cursor",
      unreadCount: 1,
    });
    expect(normalizeUnreadCount({ unread_count: 7 })).toBe(7);
    expect(normalizeMarkNotificationRead({
      item: { ...item, read_at: "2026-08-20T03:00:00Z" },
      unread_count: 0,
    })).toMatchObject({ item: { readAt: "2026-08-20T03:00:00Z" }, unreadCount: 0 });
    expect(normalizeMarkAllNotificationsRead({ marked_read: 4, unread_count: 0 }))
      .toEqual({ markedRead: 4, unreadCount: 0 });
  });
});
