import { describe, expect, it } from "vitest";

import {
  ensureLocalDateTimeSeconds,
  formatDateOnlyHeading,
  getDefaultDueLocal,
  getGreeting,
  toLocalDateTimeValue,
} from "@/lib/dates";

describe("LifeHub date helpers", () => {
  it("converts an instant into profile-local wall time", () => {
    const instant = new Date("2026-08-19T07:05:06.000Z");

    expect(toLocalDateTimeValue(instant, "Asia/Jakarta")).toBe("2026-08-19T14:05:06");
    expect(toLocalDateTimeValue(instant, "Asia/Jayapura")).toBe("2026-08-19T16:05:06");
  });

  it("defaults a task one hour ahead on the next quarter hour", () => {
    const instant = new Date("2026-08-19T07:07:00.000Z");

    expect(getDefaultDueLocal(instant, "Asia/Jakarta")).toBe("2026-08-19T15:15:00");
  });

  it("keeps the API wall-time contract explicit to seconds", () => {
    expect(ensureLocalDateTimeSeconds("2026-08-19T15:15")).toBe("2026-08-19T15:15:00");
    expect(ensureLocalDateTimeSeconds("2026-08-19T15:15:30")).toBe("2026-08-19T15:15:30");
  });

  it("uses the profile timezone for greetings", () => {
    const instant = new Date("2026-08-19T01:00:00.000Z");

    expect(getGreeting(instant, "Asia/Jakarta")).toBe("Selamat pagi");
    expect(getGreeting(instant, "America/New_York")).toBe("Selamat malam");
  });

  it("formats a date-only value without timezone shifting", () => {
    expect(formatDateOnlyHeading("2026-08-19")).toMatch(/Rabu, 19 Agustus/i);
    expect(formatDateOnlyHeading("not-a-date")).toBeNull();
  });
});
