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

export function ensureLocalDateTimeSeconds(value: string): string {
  if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)) {
    return `${value}:00`;
  }

  return value;
}

export function getBrowserTimezone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Jakarta";
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
