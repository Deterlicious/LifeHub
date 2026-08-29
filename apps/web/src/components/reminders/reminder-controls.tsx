"use client";

import { BellRing, Clock3, Pencil, Plus, RefreshCw, Trash2, X } from "lucide-react";
import { useEffect, useId, useRef, useState, type FormEvent } from "react";

import { ApiError } from "@/lib/api/client";
import type {
  CreateReminderInput,
  Reminder,
  ReminderSchedule,
  ReminderScheduleInput,
  ReminderSourceKind,
  UpdateReminderInput,
} from "@/lib/api/types";

export interface ReminderControlsProps {
  sourceKind: ReminderSourceKind;
  sourceId: string;
  scheduleKind: ReminderSchedule["kind"] | null;
  anchorLabel: string;
  timezone: string;
  onLoad: (
    sourceKind: ReminderSourceKind,
    sourceId: string,
    signal?: AbortSignal,
  ) => Promise<Reminder[]>;
  onCreate: (input: CreateReminderInput) => Promise<Reminder | null>;
  onUpdate: (reminderId: string, input: UpdateReminderInput) => Promise<Reminder | null>;
  onDelete: (reminderId: string) => Promise<boolean>;
  disabled?: boolean;
  onBusyChange?: (busy: boolean) => void;
}

type LoadStatus = "loading" | "ready" | "error";

function scheduleLabel(schedule: ReminderSchedule): string {
  if (schedule.kind === "before_moment") {
    if (schedule.minutesBefore === 0) return "Saat waktunya tiba";
    return `${schedule.minutesBefore.toLocaleString("id-ID")} menit sebelumnya`;
  }
  const dayLabel = schedule.daysBefore === 0
    ? "Pada hari yang sama"
    : `${schedule.daysBefore.toLocaleString("id-ID")} hari sebelumnya`;
  return `${dayLabel}, pukul ${schedule.timeLocal}`;
}

function statusLabel(status: Reminder["status"]): string {
  if (status === "delivered") return "Sudah terkirim";
  if (status === "inactive") return "Tidak aktif";
  return "Terjadwal";
}

function formatNextFireAt(value: string | null, timezone: string): string | null {
  if (!value) return null;
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return null;
  try {
    return new Intl.DateTimeFormat("id-ID", {
      dateStyle: "medium",
      timeStyle: "short",
      timeZone: timezone,
    }).format(parsed);
  } catch {
    return null;
  }
}

function fieldError(fields: Record<string, string>, name: string): string | undefined {
  return fields[name] ?? fields[`schedule.${name}`];
}

export function ReminderControls({
  sourceKind,
  sourceId,
  scheduleKind,
  anchorLabel,
  timezone,
  onLoad,
  onCreate,
  onUpdate,
  onDelete,
  disabled = false,
  onBusyChange,
}: ReminderControlsProps) {
  const instanceId = useId();
  const minutesHelpId = `${instanceId}-reminder-minutes`;
  const daysHelpId = `${instanceId}-reminder-days`;
  const timeHelpId = `${instanceId}-reminder-time`;
  const [status, setStatus] = useState<LoadStatus>("loading");
  const [reminders, setReminders] = useState<Reminder[]>([]);
  const [loadError, setLoadError] = useState("");
  const [editingId, setEditingId] = useState<string | "new" | null>(null);
  const [minutesBefore, setMinutesBefore] = useState("60");
  const [daysBefore, setDaysBefore] = useState(sourceKind === "document" ? "30" : "1");
  const [timeLocal, setTimeLocal] = useState("09:00");
  const [actionError, setActionError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [confirmingDeleteId, setConfirmingDeleteId] = useState<string | null>(null);
  const cancelDeleteRef = useRef<HTMLButtonElement>(null);
  const addTriggerRef = useRef<HTMLButtonElement>(null);
  const actionOriginRef = useRef<HTMLButtonElement | null>(null);
  const deleteTriggerRef = useRef<HTMLButtonElement | null>(null);
  const firstFormFieldRef = useRef<HTMLInputElement>(null);
  const interactionBusy = busy || disabled;

  function updateBusy(next: boolean) {
    setBusy(next);
    onBusyChange?.(next);
  }

  async function load(signal?: AbortSignal) {
    setStatus("loading");
    setLoadError("");
    try {
      const next = await onLoad(sourceKind, sourceId, signal);
      if (signal?.aborted) return;
      setReminders(next);
      setStatus("ready");
    } catch (reason) {
      if (signal?.aborted || (reason instanceof DOMException && reason.name === "AbortError")) return;
      setLoadError(reason instanceof Error ? reason.message : "Pengingat belum dapat dimuat.");
      setStatus("error");
    }
  }

  useEffect(() => {
    const controller = new AbortController();
    const timeout = window.setTimeout(() => void load(controller.signal), 0);
    return () => {
      window.clearTimeout(timeout);
      controller.abort();
    };
    // Source identity/timezone are lifecycle boundaries. Parent callbacks are stable.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sourceId, sourceKind, timezone]);

  function focusForm() {
    window.requestAnimationFrame(() => firstFormFieldRef.current?.focus());
  }

  function restoreActionFocus() {
    window.requestAnimationFrame(() => {
      if (actionOriginRef.current?.isConnected) actionOriginRef.current.focus();
      else addTriggerRef.current?.focus();
    });
  }

  function beginCreate(trigger: HTMLButtonElement) {
    if (!scheduleKind) return;
    actionOriginRef.current = trigger;
    setEditingId("new");
    setMinutesBefore("60");
    setDaysBefore(sourceKind === "document" ? "30" : "1");
    setTimeLocal("09:00");
    setActionError("");
    setFieldErrors({});
    focusForm();
  }

  function beginEdit(reminder: Reminder, trigger: HTMLButtonElement) {
    if (!scheduleKind) return;
    actionOriginRef.current = trigger;
    setEditingId(reminder.id);
    setMinutesBefore(
      reminder.schedule.kind === "before_moment"
        ? String(reminder.schedule.minutesBefore)
        : "60",
    );
    setDaysBefore(
      reminder.schedule.kind === "before_date"
        ? String(reminder.schedule.daysBefore)
        : sourceKind === "document" ? "30" : "1",
    );
    setTimeLocal(
      reminder.schedule.kind === "before_date" ? reminder.schedule.timeLocal : "09:00",
    );
    setConfirmingDeleteId(null);
    setActionError("");
    setFieldErrors({});
    focusForm();
  }

  function cancelEdit() {
    setEditingId(null);
    setActionError("");
    setFieldErrors({});
    restoreActionFocus();
  }

  function buildSchedule(): ReminderScheduleInput | null {
    if (scheduleKind === "before_moment") {
      const parsed = Number(minutesBefore);
      if (!Number.isSafeInteger(parsed) || parsed < 0 || parsed > 525_600) {
        setFieldErrors({ minutes_before: "Masukkan 0–525.600 menit." });
        return null;
      }
      return { kind: "before_moment", minutes_before: parsed };
    }
    if (scheduleKind === "before_date") {
      const parsed = Number(daysBefore);
      const errors: Record<string, string> = {};
      if (!Number.isSafeInteger(parsed) || parsed < 0 || parsed > 3_650) {
        errors.days_before = "Masukkan 0–3.650 hari.";
      }
      if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(timeLocal)) {
        errors.time_local = "Pilih waktu lokal yang valid.";
      }
      if (Object.keys(errors).length > 0) {
        setFieldErrors(errors);
        return null;
      }
      return { kind: "before_date", days_before: parsed, time_local: timeLocal };
    }
    return null;
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (disabled) return;
    const schedule = buildSchedule();
    if (!schedule || !editingId) return;
    updateBusy(true);
    setActionError("");
    setFieldErrors({});
    try {
      const saved = editingId === "new"
        ? await onCreate({ source_kind: sourceKind, source_id: sourceId, schedule })
        : await onUpdate(editingId, { schedule });
      if (!saved) return;
      setReminders((current) => {
        const exists = current.some((reminder) => reminder.id === saved.id);
        return exists
          ? current.map((reminder) => reminder.id === saved.id ? saved : reminder)
          : [...current, saved];
      });
      setEditingId(null);
      restoreActionFocus();
    } catch (reason) {
      if (reason instanceof ApiError) {
        setFieldErrors(reason.fields);
        setActionError(reason.message);
      } else {
        setActionError(reason instanceof Error ? reason.message : "Pengingat belum dapat disimpan.");
      }
    } finally {
      updateBusy(false);
    }
  }

  async function remove(reminderId: string) {
    if (disabled) return;
    updateBusy(true);
    setActionError("");
    try {
      if (!(await onDelete(reminderId))) return;
      setReminders((current) => current.filter((reminder) => reminder.id !== reminderId));
      setConfirmingDeleteId(null);
      window.requestAnimationFrame(() => addTriggerRef.current?.focus());
    } catch (reason) {
      setActionError(reason instanceof Error ? reason.message : "Pengingat belum dapat dihapus.");
    } finally {
      updateBusy(false);
    }
  }

  return (
    <section className="reminder-controls" aria-labelledby={`reminders-${sourceKind}-${sourceId}`}>
      <div className="reminder-controls-heading">
        <div>
          <p className="eyebrow">Pengingat satu kali</p>
          <h3 id={`reminders-${sourceKind}-${sourceId}`}><BellRing size={18} aria-hidden="true" /> Pengingat</h3>
        </div>
        {status === "ready" && scheduleKind && editingId === null ? (
          <button className="quiet-button" disabled={interactionBusy} onClick={(event) => beginCreate(event.currentTarget)} ref={addTriggerRef} type="button">
            <Plus size={17} aria-hidden="true" /> Tambah
          </button>
        ) : null}
      </div>

      <p className="reminder-help">
        Atur secara manual berdasarkan {anchorLabel}. Ini bukan pengingat berulang.
        Simpan perubahan waktu atau tanggal item terlebih dahulu bila jangkar berubah.
      </p>

      {!scheduleKind ? (
        <div className="reminder-state">
          Tambahkan {anchorLabel} terlebih dahulu agar pengingat dapat dijadwalkan.
        </div>
      ) : null}

      {status === "loading" ? (
        <div className="reminder-state" role="status">
          <RefreshCw className="spin" size={17} aria-hidden="true" /> Memuat pengingat…
        </div>
      ) : null}

      {status === "error" ? (
        <div className="reminder-state reminder-state-error" role="alert">
          <span>{loadError || "Pengingat belum dapat dimuat."}</span>
          <button className="quiet-button" onClick={() => void load()} type="button">Coba lagi</button>
        </div>
      ) : null}

      {status === "ready" && reminders.length === 0 && editingId === null ? (
        <div className="reminder-state">Belum ada pengingat untuk item ini.</div>
      ) : null}

      {status === "ready" && reminders.length > 0 ? (
        <div className="reminder-list">
          {reminders.map((reminder) => {
            const nextFire = formatNextFireAt(reminder.nextFireAt, timezone);
            return (
              <article className="reminder-item" key={reminder.id}>
                <div className="reminder-item-copy">
                  <strong>{scheduleLabel(reminder.schedule)}</strong>
                  <span className={`reminder-status reminder-status-${reminder.status}`}>
                    {statusLabel(reminder.status)}
                  </span>
                  {nextFire ? (
                    <small><Clock3 size={14} aria-hidden="true" /> Berikutnya <time dateTime={reminder.nextFireAt ?? undefined}>{nextFire}</time></small>
                  ) : null}
                </div>
                <div className="reminder-item-actions">
                  <button
                    aria-label={`Ubah pengingat ${scheduleLabel(reminder.schedule)}`}
                    className="icon-button"
                    disabled={interactionBusy || !scheduleKind}
                    onClick={(event) => beginEdit(reminder, event.currentTarget)}
                    type="button"
                  >
                    <Pencil size={16} aria-hidden="true" />
                  </button>
                  <button
                    aria-label={`Hapus pengingat ${scheduleLabel(reminder.schedule)}`}
                    className="icon-button icon-button-danger"
                    disabled={interactionBusy}
                    onClick={(event) => {
                      deleteTriggerRef.current = event.currentTarget;
                      setEditingId(null);
                      setConfirmingDeleteId(reminder.id);
                      setActionError("");
                      window.requestAnimationFrame(() => cancelDeleteRef.current?.focus());
                    }}
                    type="button"
                  >
                    <Trash2 size={16} aria-hidden="true" />
                  </button>
                </div>
                {confirmingDeleteId === reminder.id ? (
                  <div className="reminder-delete-confirm" role="group" aria-label="Konfirmasi hapus pengingat">
                    <p><strong>Hapus pengingat ini?</strong> Jadwal yang belum terkirim akan dibatalkan.</p>
                    <div>
                      <button
                        className="quiet-button"
                        disabled={interactionBusy}
                        onClick={() => {
                          setConfirmingDeleteId(null);
                          window.requestAnimationFrame(() => deleteTriggerRef.current?.focus());
                        }}
                        ref={cancelDeleteRef}
                        type="button"
                      >Batal</button>
                      <button className="button button-danger" disabled={interactionBusy} onClick={() => void remove(reminder.id)} type="button">
                        {busy ? <RefreshCw className="spin" size={16} aria-hidden="true" /> : <Trash2 size={16} aria-hidden="true" />}
                        {busy ? "Menghapus…" : "Ya, hapus"}
                      </button>
                    </div>
                  </div>
                ) : null}
              </article>
            );
          })}
        </div>
      ) : null}

      {editingId ? (
        <form className="reminder-form" onSubmit={(event) => void submit(event)}>
          <div className="reminder-form-heading">
            <strong>{editingId === "new" ? "Tambah pengingat" : "Ubah pengingat"}</strong>
            <button aria-label="Batal mengubah pengingat" className="icon-button" disabled={interactionBusy} onClick={cancelEdit} type="button">
              <X size={17} aria-hidden="true" />
            </button>
          </div>

          {actionError ? <div className="inline-alert inline-alert-error" role="alert">{actionError}</div> : null}

          {scheduleKind === "before_moment" ? (
            <label className="field">
              <span>Menit sebelum {anchorLabel}</span>
              <input
                aria-describedby={minutesHelpId}
                aria-invalid={Boolean(fieldError(fieldErrors, "minutes_before"))}
                inputMode="numeric"
                max={525600}
                min={0}
                onChange={(event) => setMinutesBefore(event.target.value)}
                ref={firstFormFieldRef}
                required
                step={1}
                type="number"
                value={minutesBefore}
              />
              <small className={fieldError(fieldErrors, "minutes_before") ? "field-error" : "field-help"} id={minutesHelpId}>
                {fieldError(fieldErrors, "minutes_before") ?? "0 berarti saat waktunya tiba."}
              </small>
            </label>
          ) : (
            <div className="reminder-date-grid">
              <label className="field">
                <span>Hari sebelum {anchorLabel}</span>
                <input
                  aria-describedby={daysHelpId}
                  aria-invalid={Boolean(fieldError(fieldErrors, "days_before"))}
                  inputMode="numeric"
                  max={3650}
                  min={0}
                  onChange={(event) => setDaysBefore(event.target.value)}
                  ref={firstFormFieldRef}
                  required
                  step={1}
                  type="number"
                  value={daysBefore}
                />
                <small className={fieldError(fieldErrors, "days_before") ? "field-error" : "field-help"} id={daysHelpId}>
                  {fieldError(fieldErrors, "days_before") ?? "0 berarti pada tanggal yang sama."}
                </small>
              </label>
              <label className="field">
                <span>Waktu lokal</span>
                <input
                  aria-describedby={timeHelpId}
                  aria-invalid={Boolean(fieldError(fieldErrors, "time_local"))}
                  onChange={(event) => setTimeLocal(event.target.value)}
                  required
                  step={60}
                  type="time"
                  value={timeLocal}
                />
                <small className={fieldError(fieldErrors, "time_local") ? "field-error" : "field-help"} id={timeHelpId}>
                  {fieldError(fieldErrors, "time_local") ?? timezone}
                </small>
              </label>
            </div>
          )}

          <button className="button button-primary button-full" disabled={interactionBusy} type="submit">
            {busy ? <RefreshCw className="spin" size={17} aria-hidden="true" /> : <BellRing size={17} aria-hidden="true" />}
            {busy ? "Menyimpan…" : "Simpan pengingat"}
          </button>
        </form>
      ) : null}

      {actionError && !editingId ? <div className="inline-alert inline-alert-error" role="alert">{actionError}</div> : null}
    </section>
  );
}
