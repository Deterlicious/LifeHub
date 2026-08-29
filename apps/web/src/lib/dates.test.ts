import { describe, expect, it } from "vitest";

import {
  ensureLocalDateTimeSeconds,
  addDateOnlyDays,
  formatAgendaGroupHeading,
  formatDateOnlyHeading,
  formatEventDateRange,
  formatEventTimeRange,
  formatDateOnlyShort,
  formatDateStripDay,
  formatDateStripWeekday,
  getAgendaRangeFrom,
  getDefaultBillDueLocal,
  getDefaultDocumentExpiryDate,
  getDefaultAgendaRange,
  getDefaultDueLocal,
  getDefaultEventSchedule,
  getGreeting,
  getProfileDateOnly,
  getSevenDayStrip,
  reconcileAgendaRangeForToday,
  reconcilePristineDateValue,
  toDateTimeLocalInput,
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

  it("defaults a bill to the end of the current profile-local day", () => {
    const instant = new Date("2026-08-19T17:30:00.000Z");

    expect(getDefaultBillDueLocal(instant, "Asia/Jakarta")).toBe("2026-08-20T23:59:00");
    expect(getDefaultBillDueLocal(instant, "America/New_York")).toBe("2026-08-19T23:59:00");
  });

  it("defaults document expiry with date-only calendar arithmetic", () => {
    const instant = new Date("2026-08-19T17:30:00.000Z");

    expect(getDefaultDocumentExpiryDate(instant, "Asia/Jakarta")).toBe("2026-09-19");
    expect(getDefaultDocumentExpiryDate(instant, "America/New_York")).toBe("2026-09-18");
  });

  it("defaults a one-hour event range in the profile timezone", () => {
    const instant = new Date("2026-08-19T07:07:00.000Z");

    expect(getDefaultEventSchedule(instant, "Asia/Jakarta")).toEqual({
      startsLocal: "2026-08-19T15:15:00",
      endsLocal: "2026-08-19T16:15:00",
      startsOn: "2026-08-19",
      endsOn: "2026-08-19",
    });
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

  it("formats event time ranges in the profile timezone", () => {
    expect(
      formatEventTimeRange(
        "2026-08-19T07:00:00Z",
        "2026-08-19T08:30:00Z",
        "Asia/Jakarta",
      ),
    ).toMatch(/^14[.:]00–15[.:]30$/);
  });

  it("formats inclusive all-day event ranges with date-only semantics", () => {
    expect(formatEventDateRange("2026-08-19", "2026-08-19")).toMatch(/19 Agu 2026/i);
    expect(formatEventDateRange("2026-08-19", "2026-08-21")).toMatch(
      /19 Agu 2026–21 Agu 2026/i,
    );
    expect(formatEventDateRange(null, null)).toBe("Tanggal belum tersedia");
  });

  it("formats a document expiry date without applying an instant timezone", () => {
    expect(formatDateOnlyShort("2026-08-20")).toMatch(/20 Agu 2026/i);
    expect(formatDateOnlyShort("not-a-date")).toBeNull();
  });

  it("builds profile-calendar Agenda ranges without DST-sensitive millisecond math", () => {
    expect(addDateOnlyDays("2026-03-01", -1)).toBe("2026-02-28");
    expect(getDefaultAgendaRange("2026-08-20")).toEqual({
      from: "2026-08-21",
      to: "2026-09-19",
    });
    expect(getAgendaRangeFrom("2026-09-20")).toEqual({
      from: "2026-09-20",
      to: "2026-10-20",
    });
    expect(getSevenDayStrip("2026-08-21")).toEqual([
      "2026-08-21",
      "2026-08-22",
      "2026-08-23",
      "2026-08-24",
      "2026-08-25",
      "2026-08-26",
      "2026-08-27",
    ]);
  });

  it("moves only the default Agenda range when the profile day changes", () => {
    const current = { from: "2026-08-21", to: "2026-09-19" };

    expect(reconcileAgendaRangeForToday(current, true, "2026-08-21")).toEqual({
      from: "2026-08-22",
      to: "2026-09-20",
    });
    expect(reconcileAgendaRangeForToday(current, false, "2026-08-21")).toBe(current);
  });

  it("refreshes pristine date defaults while preserving user-edited values", () => {
    expect(reconcilePristineDateValue("2026-08-20T23:59", false, "2026-08-21T23:59"))
      .toBe("2026-08-21T23:59");
    expect(reconcilePristineDateValue("2026-09-01T08:00", true, "2026-08-21T23:59"))
      .toBe("2026-09-01T08:00");
  });

  it("derives the current calendar date in the profile timezone", () => {
    const instant = new Date("2026-08-20T18:00:00Z");
    expect(getProfileDateOnly(instant, "Asia/Jakarta")).toBe("2026-08-21");
    expect(getProfileDateOnly(instant, "America/New_York")).toBe("2026-08-20");
  });

  it("formats Agenda dates and stored moments for accessible controls", () => {
    expect(formatDateStripWeekday("2026-08-21")).toMatch(/Jum/i);
    expect(formatDateStripDay("2026-08-21")).toMatch(/21 Agu/i);
    expect(formatAgendaGroupHeading("2026-08-21")).toMatch(/Jumat, 21 Agustus 2026/i);
    expect(toDateTimeLocalInput("2026-08-21T01:00:00Z", "Asia/Jakarta"))
      .toBe("2026-08-21T08:00");
  });
});
