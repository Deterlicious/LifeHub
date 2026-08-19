import { describe, expect, it } from "vitest";

import { normalizeProfile, normalizeTask, normalizeToday } from "@/lib/api/normalize";

describe("API DTO normalization", () => {
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
    expect(today.summary).toEqual({ open: 1, completed: 2 });
  });
});
