"use client";

import {
  CalendarDays,
  Check,
  CheckCircle2,
  Circle,
  Clock3,
  FileText,
  Inbox,
  ListChecks,
  MapPin,
  MoreHorizontal,
  ReceiptText,
  RefreshCw,
} from "lucide-react";

import { QuickAddForm } from "@/components/today/quick-task-form";
import type {
  CreateBillInput,
  CreateDocumentInput,
  CreateEventInput,
  CreateTaskInput,
  EditableTodayItem,
  Priority,
  SmartCaptureResult,
  Today,
  TodayBillItem,
  TodayEventItem,
  TodayItem,
} from "@/lib/api/types";
import { formatMoney } from "@/lib/currency";
import {
  formatDateOnlyHeading,
  formatDateOnlyShort,
  formatDayHeading,
  formatEventDateRange,
  formatEventTimeRange,
  formatFullMoment,
  formatTime,
  getGreeting,
} from "@/lib/dates";
import { isHappeningEvent } from "@/lib/today";
import { documentCategoryLabel, documentStatusLabel } from "@/lib/documents";

interface TodayViewProps {
  today: Today;
  timezone: string;
  completingTaskId: string | null;
  markingBillId: string | null;
  refreshing: boolean;
  onComplete: (taskId: string) => Promise<void>;
  onMarkBillPaid: (billId: string) => Promise<void>;
  onCreateBill: (input: CreateBillInput) => Promise<void>;
  onCreateDocument: (input: CreateDocumentInput) => Promise<void>;
  onCreateTask: (input: CreateTaskInput) => Promise<void>;
  onCreateEvent: (input: CreateEventInput) => Promise<void>;
  onParseSmartCapture: (text: string) => Promise<SmartCaptureResult>;
  onRefresh: () => Promise<void>;
  onEdit: (item: EditableTodayItem) => void;
}

function isResolved(item: TodayItem): boolean {
  if (item.kind === "task") {
    return Boolean(item.completedAt) || item.status === "completed" || item.status === "done";
  }
  if (item.kind === "bill") {
    return Boolean(item.paidAt) || item.status === "paid";
  }
  return false;
}

function isPaid(item: TodayBillItem): boolean {
  return Boolean(item.paidAt) || item.status === "paid";
}

function priorityLabel(priority: Priority): string {
  if (priority === "high") return "Prioritas tinggi";
  if (priority === "low") return "Prioritas rendah";
  return "Prioritas normal";
}

function eventSchedule(item: TodayEventItem, timezone: string) {
  if (item.allDay) {
    const dateRange = formatEventDateRange(item.startsOn, item.endsOn);
    return {
      primary: "Seharian",
      secondary: dateRange,
      full: `Sepanjang hari, ${dateRange}`,
      dateTime: item.startsOn,
    };
  }

  const range = formatEventTimeRange(item.startsAt, item.endsAt, timezone);
  return {
    primary: formatTime(item.startsAt, timezone),
    secondary: item.endsAt ? `s.d. ${formatTime(item.endsAt, timezone)}` : "Waktu lokal",
    full: item.startsAt ? formatFullMoment(item.startsAt, timezone) : range,
    dateTime: item.startsAt,
  };
}

function TodayRow({
  item,
  timezone,
  completing,
  markingPaid,
  onComplete,
  onMarkBillPaid,
  onEdit,
}: {
  item: TodayItem;
  timezone: string;
  completing: boolean;
  markingPaid: boolean;
  onComplete: (taskId: string) => Promise<void>;
  onMarkBillPaid: (billId: string) => Promise<void>;
  onEdit: (item: EditableTodayItem) => void;
}) {
  const resolved = isResolved(item);
  const schedule = item.kind === "event" ? eventSchedule(item, timezone) : null;
  const rowLabel = item.kind === "event"
    ? `${item.title}, jadwal ${schedule?.full ?? "hari ini"}`
    : item.kind === "bill"
      ? `${item.title}, tagihan ${formatMoney(item.amount, item.currency)}, jatuh tempo ${formatFullMoment(item.dueAt, timezone)}${isPaid(item) ? ", lunas" : ""}`
      : item.kind === "document"
        ? `${item.title}, dokumen ${documentCategoryLabel(item.category)}, ${documentStatusLabel(item.status)} pada ${formatDateOnlyShort(item.expiresOn) ?? item.expiresOn}`
        : `${item.title}${resolved ? ", selesai" : ""}`;

  return (
    <article
      aria-label={rowLabel}
      className={`timeline-item${resolved ? " timeline-item-completed" : ""}`}
    >
      <div className="timeline-time">
        {item.kind === "event" ? (
          <>
            {schedule?.dateTime ? (
              <time dateTime={schedule.dateTime}><strong>{schedule.primary}</strong></time>
            ) : <strong>{schedule?.primary}</strong>}
            <span>{schedule?.secondary}</span>
          </>
        ) : item.kind === "document" ? (
          <>
            <time dateTime={item.expiresOn}>
              <strong>{formatDateOnlyShort(item.expiresOn)?.replace(/ \d{4}$/, "") ?? item.expiresOn}</strong>
            </time>
            <span>Masa berlaku</span>
          </>
        ) : item.kind === "bill" ? (
          <>
            {item.dueAt ? (
              <time dateTime={item.dueAt}><strong>{formatTime(item.dueAt, timezone)}</strong></time>
            ) : <strong>Belum ada</strong>}
            <span>Jatuh tempo</span>
          </>
        ) : (
          <>
            {item.dueAt ? (
              <time dateTime={item.dueAt}><strong>{formatTime(item.dueAt, timezone)}</strong></time>
            ) : <strong>{formatTime(null, timezone)}</strong>}
            <span>{item.dueAt ? "Waktu lokal" : "Fleksibel"}</span>
          </>
        )}
      </div>
      <span
        className={`timeline-dot${item.kind === "task" ? ` priority-${item.priority}` : item.kind === "event" ? " event-dot" : item.kind === "bill" ? " bill-dot" : " document-dot"}`}
        aria-hidden="true"
      >
        {resolved ? <Check size={13} /> : null}
      </span>
      <div className="timeline-content">
        <div className="timeline-title-row">
          <div>
            <h3>{item.title}</h3>
            {item.kind === "bill" ? (
              <p className="bill-value">{formatMoney(item.amount, item.currency)}</p>
            ) : null}
            {item.notes ? <p>{item.notes}</p> : null}
          </div>
          <div className="timeline-actions">
            {item.kind === "task" && !resolved ? (
              <button
                aria-label={`Selesaikan ${item.title}`}
                className="complete-button"
                disabled={completing}
                onClick={() => void onComplete(item.id)}
                title="Tandai selesai"
                type="button"
              >
                {completing
                  ? <RefreshCw className="spin" size={19} aria-hidden="true" />
                  : <Circle size={21} aria-hidden="true" />}
              </button>
            ) : item.kind === "bill" && !resolved ? (
              <button
                aria-label={`Bayar ${item.title}`}
                className="pay-button"
                disabled={markingPaid}
                onClick={() => void onMarkBillPaid(item.id)}
                type="button"
              >
                {markingPaid
                  ? <RefreshCw className="spin" size={17} aria-hidden="true" />
                  : <Check size={17} aria-hidden="true" />}
                <span>{markingPaid ? "Menyimpan" : "Bayar"}</span>
              </button>
            ) : resolved ? (
              <CheckCircle2
                className="completed-check"
                size={20}
                aria-label={item.kind === "bill" ? "Lunas" : "Selesai"}
              />
            ) : item.kind === "document" ? (
              <FileText className="document-row-icon" size={20} aria-hidden="true" />
            ) : null}
            {item.kind !== "document" ? (
              <button
                aria-label={`Ubah atau hapus ${item.title}`}
                className="icon-button timeline-more-button"
                onClick={() => onEdit(item)}
                type="button"
              >
                <MoreHorizontal size={19} aria-hidden="true" />
              </button>
            ) : null}
          </div>
        </div>
        <div className="timeline-meta">
          {item.kind === "task" ? (
            <>
              <span className={`priority-label priority-label-${item.priority}`}>{priorityLabel(item.priority)}</span>
              {item.urgency === "overdue" ? <span className="overdue-label">Terlambat</span> : null}
              <span>Tugas</span>
            </>
          ) : item.kind === "event" ? (
            <>
              <span className="event-type-label"><CalendarDays size={12} aria-hidden="true" /> Jadwal</span>
              {isHappeningEvent(item) ? <span className="happening-label">Sedang berlangsung</span> : null}
              {item.location ? (
                <span className="event-location"><MapPin size={12} aria-hidden="true" /> {item.location}</span>
              ) : null}
              {item.allDay ? <span>{formatEventDateRange(item.startsOn, item.endsOn)}</span> : null}
            </>
          ) : item.kind === "bill" ? (
            <>
              <span className="bill-type-label"><ReceiptText size={12} aria-hidden="true" /> Tagihan</span>
              {item.urgency === "overdue" && !isPaid(item) ? <span className="overdue-label">Terlambat</span> : null}
              {isPaid(item) ? <span className="paid-label">Lunas</span> : null}
              <span>{item.currency}</span>
            </>
          ) : (
            <>
              <span className="document-type-label"><FileText size={12} aria-hidden="true" /> Dokumen</span>
              <span>{documentCategoryLabel(item.category)}</span>
              <span className={`document-today-status document-today-status-${item.status}`}>
                {documentStatusLabel(item.status)}
              </span>
            </>
          )}
        </div>
      </div>
    </article>
  );
}

export function TodayView({
  today,
  timezone,
  completingTaskId,
  markingBillId,
  refreshing,
  onComplete,
  onMarkBillPaid,
  onCreateTask,
  onCreateEvent,
  onCreateBill,
  onCreateDocument,
  onParseSmartCapture,
  onEdit,
  onRefresh,
}: TodayViewProps) {
  const now = new Date();
  const openItems = today.items.filter((item) => !isResolved(item));
  const completedItems = today.items.filter(isResolved);
  const dateHeading = formatDateOnlyHeading(today.date) ?? formatDayHeading(now, timezone);

  return (
    <div className="today-page" id="today">
      <header className="today-hero">
        <div>
          <p className="date-heading">{dateHeading}</p>
          <h1>{getGreeting(now, timezone)}.</h1>
          <p className="today-promise">Hal penting hari ini, ada di satu tempat.</p>
        </div>
        <button
          aria-label={refreshing ? "Sedang memperbarui Today" : "Perbarui Today"}
          className="refresh-button"
          disabled={refreshing}
          onClick={() => void onRefresh()}
          type="button"
        >
          <RefreshCw className={refreshing ? "spin" : undefined} size={17} aria-hidden="true" />
          <span>{refreshing ? "Memperbarui…" : "Perbarui"}</span>
        </button>
      </header>

      <div className="today-grid">
        <div className="today-primary-column">
          <section className="timeline-card" aria-labelledby="timeline-title">
            <div className="section-heading">
              <div>
                <p className="eyebrow">Fokus hari ini</p>
                <h2 id="timeline-title">Today</h2>
              </div>
              <div className="summary-copy" aria-label={`${openItems.length} terbuka, ${today.summary.completed} dituntaskan`}>
                <strong>{openItems.length}</strong> perlu perhatian
                <span>· {today.summary.completed} tuntas hari ini</span>
              </div>
            </div>

            {openItems.length > 0 ? (
              <div className="timeline-list">
                {openItems.map((item) => (
                  <TodayRow
                    completing={item.kind === "task" && completingTaskId === item.id}
                    item={item}
                    key={`${item.kind}-${item.id}`}
                    markingPaid={item.kind === "bill" && markingBillId === item.id}
                    onComplete={onComplete}
                    onMarkBillPaid={onMarkBillPaid}
                    onEdit={onEdit}
                    timezone={timezone}
                  />
                ))}
              </div>
            ) : (
              <div className="empty-state">
                <span className="empty-icon" aria-hidden="true"><Inbox size={25} /></span>
                <h3>Hari ini masih lapang.</h3>
                <p>Tambahkan tugas, jadwal, tagihan, atau metadata dokumen penting—cukup satu hal untuk memulai.</p>
                <a className="text-button" href="#quick-add">Tambah hal pertama</a>
              </div>
            )}

            {completedItems.length > 0 ? (
              <details className="completed-section">
                <summary><ListChecks size={17} aria-hidden="true" /> Tuntas hari ini ({completedItems.length})</summary>
                <div className="timeline-list timeline-list-completed">
                  {completedItems.map((item) => (
                    <TodayRow
                      completing={false}
                      item={item}
                      key={`${item.kind}-${item.id}`}
                      markingPaid={false}
                      onComplete={onComplete}
                      onMarkBillPaid={onMarkBillPaid}
                      onEdit={onEdit}
                      timezone={timezone}
                    />
                  ))}
                </div>
              </details>
            ) : null}

            <footer className="timeline-footer">
              <Clock3 size={16} aria-hidden="true" />
              Diurutkan oleh LifeHub berdasarkan waktu dan urgensi.
            </footer>
          </section>

          {today.upcoming.length > 0 ? (
            <section className="upcoming-documents" aria-labelledby="upcoming-documents-title">
              <div className="upcoming-documents-heading">
                <div>
                  <p className="eyebrow">Siapkan lebih awal</p>
                  <h2 id="upcoming-documents-title">Berikutnya</h2>
                </div>
                <p>
                  {today.summary.upcoming} hal dalam {today.upcomingHorizonDays || 30} hari ke depan
                </p>
              </div>
              <div className="timeline-list upcoming-documents-list">
                {today.upcoming.slice(0, 5).map((item) => (
                  <TodayRow
                    completing={false}
                    item={item}
                    key={`${item.kind}-${item.id}`}
                    markingPaid={false}
                    onComplete={onComplete}
                    onMarkBillPaid={onMarkBillPaid}
                    onEdit={onEdit}
                    timezone={timezone}
                  />
                ))}
              </div>
              <footer className="upcoming-documents-footer">
                <a className="text-button" href="#agenda">Lihat semua {today.summary.upcoming} di Agenda</a>
              </footer>
            </section>
          ) : null}
        </div>

        <aside className="quick-add-column" aria-label="Tambah cepat">
          <QuickAddForm
            key={timezone}
            onCreateBill={onCreateBill}
            onCreateDocument={onCreateDocument}
            onCreateEvent={onCreateEvent}
            onCreateTask={onCreateTask}
            onParseSmartCapture={onParseSmartCapture}
            profileDate={today.date}
            timezone={timezone}
          />
        </aside>
      </div>
    </div>
  );
}
