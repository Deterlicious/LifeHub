const DATE_PARTS_FORMAT: Intl.DateTimeFormatOptions = {
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hourCycle: "h23",
};

function getParts(date: Date, timezone: string): Record<string, string> {
  const parts = new Intl.DateTimeFormat("en-CA", {
    ...DATE_PARTS_FORMAT,
    timeZone: timezone,
  }).formatToParts(date);

  return Object.fromEntries(parts.map((part) => [part.type, part.value]));
}

export function toLocalDateTimeValue(date: Date, timezone: string): string {
  const parts = getParts(date, timezone);

  return `${parts.year}-${parts.month}-${parts.day}T${parts.hour}:${parts.minute}:${parts.second}`;
}

export function getDefaultDueLocal(date: Date, timezone: string): string {
  const due = new Date(date.getTime() + 60 * 60 * 1000);
  due.setUTCMinutes(Math.ceil(due.getUTCMinutes() / 15) * 15, 0, 0);
  return toLocalDateTimeValue(due, timezone);
}

export function getDefaultBillDueLocal(date: Date, timezone: string): string {
  return `${toLocalDateTimeValue(date, timezone).slice(0, 10)}T23:59:00`;
}

export function getDefaultDocumentExpiryDate(date: Date, timezone: string): string {
  const localDate = toLocalDateTimeValue(date, timezone).slice(0, 10);
  const [year, month, day] = localDate.split("-").map(Number);
  const expiry = new Date(Date.UTC(year, month - 1, day + 30, 12));
  return expiry.toISOString().slice(0, 10);
}

export function addDateOnlyDays(dateOnly: string, days: number): string {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(dateOnly)) return "";
  const [year, month, day] = dateOnly.split("-").map(Number);
  const date = new Date(Date.UTC(year, month - 1, day + days, 12));
  return date.toISOString().slice(0, 10);
}

export function getDefaultAgendaRange(todayDate: string): { from: string; to: string } {
  return {
    from: addDateOnlyDays(todayDate, 1),
    to: addDateOnlyDays(todayDate, 30),
  };
}

export function reconcileAgendaRangeForToday(
  current: { from: string; to: string },
  followsToday: boolean,
  todayDate: string,
): { from: string; to: string } {
  return followsToday ? getDefaultAgendaRange(todayDate) : current;
}

export function reconcilePristineDateValue(
  current: string,
  edited: boolean,
  nextDefault: string,
): string {
  return edited ? current : nextDefault;
}

export function getAgendaRangeFrom(dateOnly: string): { from: string; to: string } {
  return { from: dateOnly, to: addDateOnlyDays(dateOnly, 30) };
}

export function getSevenDayStrip(dateOnly: string): string[] {
  return Array.from({ length: 7 }, (_, index) => addDateOnlyDays(dateOnly, index));
}

export function toDateTimeLocalInput(instant: string | null, timezone: string): string {
  if (!instant) return "";
  const parsed = new Date(instant);
  if (Number.isNaN(parsed.getTime())) return "";
  return toLocalDateTimeValue(parsed, timezone).slice(0, 16);
}

export interface DefaultEventSchedule {
  startsLocal: string;
  endsLocal: string;
  startsOn: string;
  endsOn: string;
}

export function getDefaultEventSchedule(date: Date, timezone: string): DefaultEventSchedule {
  const startsAt = new Date(date.getTime() + 60 * 60 * 1000);
  startsAt.setUTCMinutes(Math.ceil(startsAt.getUTCMinutes() / 15) * 15, 0, 0);
  const endsAt = new Date(startsAt.getTime() + 60 * 60 * 1000);
  const startsLocal = toLocalDateTimeValue(startsAt, timezone);

  return {
    startsLocal,
    endsLocal: toLocalDateTimeValue(endsAt, timezone),
    startsOn: startsLocal.slice(0, 10),
    endsOn: startsLocal.slice(0, 10),
  };
}

export function ensureLocalDateTimeSeconds(value: string): string {
  if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)) {
    return `${value}:00`;
  }

  return value;
}

export function getBrowserTimezone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Jakarta";
}

export function getProfileDateOnly(date: Date, timezone: string): string {
  return toLocalDateTimeValue(date, timezone).slice(0, 10);
}

export function getGreeting(date: Date, timezone: string): string {
  const hour = Number(getParts(date, timezone).hour);

  if (hour < 11) return "Selamat pagi";
  if (hour < 15) return "Selamat siang";
  if (hour < 19) return "Selamat sore";
  return "Selamat malam";
}

export function formatDayHeading(date: Date, timezone: string): string {
  return new Intl.DateTimeFormat("id-ID", {
    timeZone: timezone,
    weekday: "long",
    day: "numeric",
    month: "long",
  }).format(date);
}

export function formatDateOnlyHeading(dateOnly: string): string | null {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(dateOnly)) return null;

  const [year, month, day] = dateOnly.split("-").map(Number);
  const parsed = new Date(Date.UTC(year, month - 1, day, 12));

  return new Intl.DateTimeFormat("id-ID", {
    timeZone: "UTC",
    weekday: "long",
    day: "numeric",
    month: "long",
  }).format(parsed);
}

export function formatTime(instant: string | null, timezone: string): string {
  if (!instant) return "Kapan saja";

  const parsed = new Date(instant);
  if (Number.isNaN(parsed.getTime())) return "Waktu belum tersedia";

  return new Intl.DateTimeFormat("id-ID", {
    timeZone: timezone,
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  }).format(parsed);
}

export function formatEventTimeRange(
  startsAt: string | null,
  endsAt: string | null,
  timezone: string,
): string {
  const starts = formatTime(startsAt, timezone);
  if (!endsAt || !startsAt) return starts;

  return `${starts}–${formatTime(endsAt, timezone)}`;
}

export function formatEventDateRange(startsOn: string | null, endsOn: string | null): string {
  const start = formatDateOnlyShort(startsOn);
  if (!start) return "Tanggal belum tersedia";

  const end = formatDateOnlyShort(endsOn);
  if (!end || endsOn === startsOn) return start;

  return `${start}–${end}`;
}

export function formatDateOnlyShort(dateOnly: string | null): string | null {
  if (!dateOnly || !/^\d{4}-\d{2}-\d{2}$/.test(dateOnly)) return null;

  const [year, month, day] = dateOnly.split("-").map(Number);
  const parsed = new Date(Date.UTC(year, month - 1, day, 12));

  return new Intl.DateTimeFormat("id-ID", {
    timeZone: "UTC",
    day: "numeric",
    month: "short",
    year: "numeric",
  }).format(parsed);
}

export function formatDateStripWeekday(dateOnly: string): string {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(dateOnly)) return "";
  const [year, month, day] = dateOnly.split("-").map(Number);
  return new Intl.DateTimeFormat("id-ID", {
    timeZone: "UTC",
    weekday: "short",
  }).format(new Date(Date.UTC(year, month - 1, day, 12)));
}

export function formatDateStripDay(dateOnly: string): string {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(dateOnly)) return "";
  const [year, month, day] = dateOnly.split("-").map(Number);
  return new Intl.DateTimeFormat("id-ID", {
    timeZone: "UTC",
    day: "numeric",
    month: "short",
  }).format(new Date(Date.UTC(year, month - 1, day, 12)));
}

export function formatAgendaGroupHeading(dateOnly: string): string {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(dateOnly)) return dateOnly;
  const [year, month, day] = dateOnly.split("-").map(Number);
  return new Intl.DateTimeFormat("id-ID", {
    timeZone: "UTC",
    weekday: "long",
    day: "numeric",
    month: "long",
    year: "numeric",
  }).format(new Date(Date.UTC(year, month - 1, day, 12)));
}

export function formatFullMoment(instant: string | null, timezone: string): string {
  if (!instant) return "Tanpa tenggat waktu";

  const parsed = new Date(instant);
  if (Number.isNaN(parsed.getTime())) return "Waktu belum tersedia";

  return new Intl.DateTimeFormat("id-ID", {
    timeZone: timezone,
    weekday: "long",
    day: "numeric",
    month: "long",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  }).format(parsed);
}
