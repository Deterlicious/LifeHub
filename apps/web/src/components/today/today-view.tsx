"use client";

import {
  Check,
  CheckCircle2,
  Circle,
  Clock3,
  Inbox,
  ListChecks,
  RefreshCw,
} from "lucide-react";

import { QuickTaskForm } from "@/components/today/quick-task-form";
import type { CreateTaskInput, Today, TodayItem } from "@/lib/api/types";
import {
  formatDateOnlyHeading,
  formatDayHeading,
  formatFullMoment,
  formatTime,
  getGreeting,
} from "@/lib/dates";

interface TodayViewProps {
  today: Today;
  timezone: string;
  completingTaskId: string | null;
  refreshing: boolean;
  onComplete: (taskId: string) => Promise<void>;
  onCreate: (input: CreateTaskInput) => Promise<void>;
  onRefresh: () => Promise<void>;
}

function isCompleted(item: TodayItem): boolean {
  return Boolean(item.completedAt) || item.status === "completed" || item.status === "done";
}

function priorityLabel(priority: TodayItem["priority"]): string {
  if (priority === "high") return "Prioritas tinggi";
  if (priority === "low") return "Prioritas rendah";
  return "Prioritas sedang";
}

function TodayRow({
  item,
  timezone,
  completing,
  onComplete,
}: {
  item: TodayItem;
  timezone: string;
  completing: boolean;
  onComplete: (taskId: string) => Promise<void>;
}) {
  const completed = isCompleted(item);

  return (
    <article
      aria-label={`${item.title}${completed ? ", selesai" : ""}`}
      className={`timeline-item${completed ? " timeline-item-completed" : ""}`}
    >
      <div className="timeline-time">
        <strong>{formatTime(item.dueAt, timezone)}</strong>
        <span>{item.dueAt ? "Waktu lokal" : "Fleksibel"}</span>
      </div>
      <span className={`timeline-dot priority-${item.priority}`} aria-hidden="true">
        {completed ? <Check size={13} /> : null}
      </span>
      <div className="timeline-content">
        <div className="timeline-title-row">
          <div>
            <h3>{item.title}</h3>
            {item.notes ? <p>{item.notes}</p> : null}
          </div>
          {!completed && item.kind === "task" ? (
            <button
              aria-label={`Selesaikan ${item.title}`}
              className="complete-button"
              disabled={completing}
              onClick={() => void onComplete(item.id)}
              title="Tandai selesai"
              type="button"
            >
              {completing ? <RefreshCw className="spin" size={19} aria-hidden="true" /> : <Circle size={21} aria-hidden="true" />}
            </button>
          ) : <CheckCircle2 className="completed-check" size={20} aria-label="Selesai" />}
        </div>
        <div className="timeline-meta">
          <span className={`priority-label priority-label-${item.priority}`}>{priorityLabel(item.priority)}</span>
          {item.urgency === "overdue" ? <span className="overdue-label">Terlambat</span> : null}
          <span title={formatFullMoment(item.dueAt, timezone)}>{item.kind === "task" ? "Tugas" : item.kind}</span>
        </div>
      </div>
    </article>
  );
}

export function TodayView({
  today,
  timezone,
  completingTaskId,
  refreshing,
  onComplete,
  onCreate,
  onRefresh,
}: TodayViewProps) {
  const now = new Date();
  const openItems = today.items.filter((item) => !isCompleted(item));
  const completedItems = today.items.filter(isCompleted);
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
        <section className="timeline-card" aria-labelledby="timeline-title">
          <div className="section-heading">
            <div>
              <p className="eyebrow">Fokus hari ini</p>
              <h2 id="timeline-title">Today</h2>
            </div>
            <div className="summary-copy" aria-label={`${openItems.length} terbuka, ${today.summary.completed} selesai`}>
              <strong>{openItems.length}</strong> perlu perhatian
              <span>· {today.summary.completed} selesai</span>
            </div>
          </div>

          {openItems.length > 0 ? (
            <div className="timeline-list">
              {openItems.map((item) => (
                <TodayRow
                  completing={completingTaskId === item.id}
                  item={item}
                  key={item.id}
                  onComplete={onComplete}
                  timezone={timezone}
                />
              ))}
            </div>
          ) : (
            <div className="empty-state">
              <span className="empty-icon" aria-hidden="true"><Inbox size={25} /></span>
              <h3>Hari ini masih lapang.</h3>
              <p>Tambahkan tugas penting berikutnya—cukup satu hal untuk memulai.</p>
              <a className="text-button" href="#quick-add">Tambah tugas pertama</a>
            </div>
          )}

          {completedItems.length > 0 ? (
            <details className="completed-section">
              <summary><ListChecks size={17} aria-hidden="true" /> Selesai hari ini ({completedItems.length})</summary>
              <div className="timeline-list timeline-list-completed">
                {completedItems.map((item) => (
                  <TodayRow
                    completing={false}
                    item={item}
                    key={item.id}
                    onComplete={onComplete}
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

        <aside className="quick-add-column" aria-label="Tambah tugas cepat">
          <QuickTaskForm key={timezone} onCreate={onCreate} timezone={timezone} />
        </aside>
      </div>
    </div>
  );
}
