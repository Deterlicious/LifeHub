"use client";

import {
  CalendarClock,
  CalendarDays,
  ChevronDown,
  ListChecks,
  MapPin,
  ReceiptText,
  RefreshCw,
  RotateCcw,
  Trash2,
  X,
} from "lucide-react";
import { useEffect, useRef, useState, type FormEvent } from "react";

import { ApiError } from "@/lib/api/client";
import {
  ReminderControls,
  type ReminderControlsProps,
} from "@/components/reminders/reminder-controls";
import type {
  CorrectableItem,
  Priority,
  UpdateBillInput,
  UpdateEventInput,
  UpdateTaskInput,
} from "@/lib/api/types";
import { formatMoney, parseIntegerAmount } from "@/lib/currency";
import { ensureLocalDateTimeSeconds, toDateTimeLocalInput } from "@/lib/dates";

interface CorrectionSheetProps {
  item: CorrectableItem;
  timezone: string;
  onClose: () => void;
  onSaveTask: (taskId: string, input: UpdateTaskInput) => Promise<void>;
  onSaveEvent: (eventId: string, input: UpdateEventInput) => Promise<void>;
  onSaveBill: (billId: string, input: UpdateBillInput) => Promise<void>;
  onDelete: (kind: "task" | "event" | "bill", id: string) => Promise<void>;
  onUncomplete: (taskId: string) => Promise<void>;
  onMarkUnpaid: (billId: string) => Promise<void>;
  onLoadReminders: ReminderControlsProps["onLoad"];
  onCreateReminder: ReminderControlsProps["onCreate"];
  onUpdateReminder: ReminderControlsProps["onUpdate"];
  onDeleteReminder: ReminderControlsProps["onDelete"];
}

function kindLabel(kind: CorrectableItem["kind"]): string {
  if (kind === "task") return "tugas";
  if (kind === "event") return "jadwal";
  return "tagihan";
}

export function CorrectionSheet({
  item,
  timezone,
  onClose,
  onSaveTask,
  onSaveEvent,
  onSaveBill,
  onDelete,
  onUncomplete,
  onMarkUnpaid,
  onLoadReminders,
  onCreateReminder,
  onUpdateReminder,
  onDeleteReminder,
}: CorrectionSheetProps) {
  const [title, setTitle] = useState(item.title);
  const [notes, setNotes] = useState(item.notes ?? "");
  const [priority, setPriority] = useState<Priority>(item.kind === "task" ? item.priority : "normal");
  const initialTaskDue = item.kind === "task" ? toDateTimeLocalInput(item.dueAt, timezone) : "";
  const [taskDueLocal, setTaskDueLocal] = useState(initialTaskDue);
  const [location, setLocation] = useState(item.kind === "event" ? item.location ?? "" : "");
  const initialAllDay = item.kind === "event" ? item.allDay : false;
  const [allDay, setAllDay] = useState(initialAllDay);
  const initialStartsLocal = item.kind === "event" ? toDateTimeLocalInput(item.startsAt, timezone) : "";
  const initialEndsLocal = item.kind === "event" ? toDateTimeLocalInput(item.endsAt, timezone) : "";
  const initialStartsOn = item.kind === "event" ? item.startsOn ?? "" : "";
  const initialEndsOn = item.kind === "event" ? item.endsOn ?? "" : "";
  const [startsLocal, setStartsLocal] = useState(initialStartsLocal);
  const [endsLocal, setEndsLocal] = useState(initialEndsLocal);
  const [startsOn, setStartsOn] = useState(initialStartsOn);
  const [endsOn, setEndsOn] = useState(initialEndsOn);
  const [amount, setAmount] = useState(item.kind === "bill" ? String(item.amount) : "");
  const [currency, setCurrency] = useState(item.kind === "bill" ? item.currency : "IDR");
  const initialBillDue = item.kind === "bill" ? toDateTimeLocalInput(item.dueAt, timezone) : "";
  const [billDueLocal, setBillDueLocal] = useState(initialBillDue);
  const [submitting, setSubmitting] = useState(false);
  const [reminderBusy, setReminderBusy] = useState(false);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const dialogRef = useRef<HTMLElement>(null);
  const closeRef = useRef<HTMLButtonElement>(null);
  const cancelDeleteRef = useRef<HTMLButtonElement>(null);
  const deleteTriggerRef = useRef<HTMLButtonElement>(null);
  const closeCallbackRef = useRef(onClose);
  const submittingRef = useRef(submitting);
  const interactionBusy = submitting || reminderBusy;

  function updateSubmitting(nextSubmitting: boolean) {
    submittingRef.current = nextSubmitting;
    setSubmitting(nextSubmitting);
  }

  useEffect(() => {
    closeCallbackRef.current = onClose;
  }, [onClose]);

  useEffect(() => {
    submittingRef.current = interactionBusy;
  }, [interactionBusy]);

  useEffect(() => {
    const previouslyFocused = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        event.preventDefault();
        if (submittingRef.current) return;
        closeCallbackRef.current();
        return;
      }
      if (event.key !== "Tab" || !dialogRef.current) return;
      const focusable = Array.from(dialogRef.current.querySelectorAll<HTMLElement>(
        'button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [href], [tabindex]:not([tabindex="-1"])',
      ));
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (!focusable.includes(document.activeElement as HTMLElement)) {
        event.preventDefault();
        (event.shiftKey ? last : first).focus();
      } else if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }

    document.addEventListener("keydown", handleKeyDown);
    closeRef.current?.focus();
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
      document.body.style.overflow = previousOverflow;
      if (previouslyFocused?.isConnected) previouslyFocused.focus();
      else document.getElementById("main-content")?.focus();
    };
  }, []);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (reminderBusy) return;
    updateSubmitting(true);
    setError(null);
    setFieldErrors({});
    try {
      if (item.kind === "task") {
        await onSaveTask(item.id, {
          title: title.trim(),
          notes: notes.trim() || null,
          priority,
          due_local: taskDueLocal ? ensureLocalDateTimeSeconds(taskDueLocal) : null,
        });
      } else if (item.kind === "event") {
        const metadata = {
          title: title.trim(),
          notes: notes.trim() || null,
          location: location.trim() || null,
        };
        const scheduleChanged = allDay !== initialAllDay
          || startsLocal !== initialStartsLocal
          || endsLocal !== initialEndsLocal
          || startsOn !== initialStartsOn
          || endsOn !== initialEndsOn;
        let input: UpdateEventInput = metadata;
        if (scheduleChanged && allDay) {
          input = {
            ...metadata,
            all_day: true,
            starts_on: startsOn,
            ends_on: endsOn || undefined,
          };
        } else if (scheduleChanged) {
          input = {
            ...metadata,
            all_day: false,
            starts_local: ensureLocalDateTimeSeconds(startsLocal),
            ends_local: endsLocal ? ensureLocalDateTimeSeconds(endsLocal) : undefined,
          };
        }
        await onSaveEvent(item.id, input);
      } else {
        const parsedAmount = parseIntegerAmount(amount);
        if (parsedAmount === null) {
          setFieldErrors({ amount: "Masukkan nominal rupiah utuh lebih dari nol." });
          setError("Periksa data yang belum valid.");
          return;
        }
        await onSaveBill(item.id, {
          title: title.trim(),
          notes: notes.trim() || null,
          amount: parsedAmount,
          currency: currency.trim().toUpperCase(),
          due_local: ensureLocalDateTimeSeconds(billDueLocal),
        });
      }
      onClose();
    } catch (reason) {
      if (reason instanceof ApiError) {
        setFieldErrors(reason.fields);
        setError(reason.message);
      } else {
        setError(reason instanceof Error ? reason.message : "Perubahan belum dapat disimpan.");
      }
    } finally {
      updateSubmitting(false);
    }
  }

  async function performDelete() {
    updateSubmitting(true);
    setError(null);
    try {
      await onDelete(item.kind, item.id);
      onClose();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Item belum dapat dihapus.");
    } finally {
      updateSubmitting(false);
    }
  }

  async function performInverseAction() {
    updateSubmitting(true);
    setError(null);
    try {
      if (item.kind === "task") await onUncomplete(item.id);
      if (item.kind === "bill") await onMarkUnpaid(item.id);
      onClose();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Status belum dapat dibatalkan.");
    } finally {
      updateSubmitting(false);
    }
  }

  const titleFieldError = fieldErrors.title;
  const notesFieldError = fieldErrors.notes;

  return (
    <div
      className="sheet-backdrop"
      onMouseDown={(event) => {
        if (!interactionBusy && event.currentTarget === event.target) onClose();
      }}
      role="presentation"
    >
      <section
        aria-labelledby="correction-sheet-title"
        aria-modal="true"
        aria-busy={interactionBusy}
        className="correction-sheet"
        ref={dialogRef}
        role="dialog"
      >
        <header className="correction-sheet-header">
          <div>
            <p className="eyebrow">Koreksi dengan jelas</p>
            <h2 id="correction-sheet-title">Ubah {kindLabel(item.kind)}</h2>
          </div>
          <button aria-label="Tutup editor" className="icon-button" disabled={interactionBusy} onClick={onClose} ref={closeRef} type="button">
            <X size={20} aria-hidden="true" />
          </button>
        </header>

        <div className="correction-sheet-body">
          {error ? <div className="inline-alert inline-alert-error" role="alert">{error}</div> : null}

          <form className="correction-form" onSubmit={submit}>
            <label className="field">
              <span>{item.kind === "event" ? "Nama jadwal" : item.kind === "bill" ? "Nama tagihan" : "Nama tugas"}</span>
              <input
                aria-describedby={titleFieldError ? "correction-title-error" : undefined}
                aria-invalid={Boolean(titleFieldError)}
                maxLength={200}
                onChange={(event) => setTitle(event.target.value)}
                required
                value={title}
              />
              {titleFieldError ? <small className="field-error" id="correction-title-error">{titleFieldError}</small> : null}
            </label>

            {item.kind === "task" ? (
              <>
                <label className="field">
                  <span>Prioritas</span>
                  <div className="select-wrap">
                    <select onChange={(event) => setPriority(event.target.value as Priority)} value={priority}>
                      <option value="low">Rendah</option>
                      <option value="normal">Normal</option>
                      <option value="high">Tinggi</option>
                    </select>
                    <ChevronDown size={17} aria-hidden="true" />
                  </div>
                </label>
                <label className="field">
                  <span>Tenggat lokal <small>Boleh dikosongkan</small></span>
                  <div className="input-with-icon">
                    <CalendarClock size={17} aria-hidden="true" />
                    <input
                      aria-describedby={fieldErrors.due_local ? "correction-task-due-error" : "correction-task-due-help"}
                      aria-invalid={Boolean(fieldErrors.due_local)}
                      onChange={(event) => setTaskDueLocal(event.target.value)}
                      step={60}
                      type="datetime-local"
                      value={taskDueLocal}
                    />
                  </div>
                  <small className={fieldErrors.due_local ? "field-error" : "field-help"} id={fieldErrors.due_local ? "correction-task-due-error" : "correction-task-due-help"}>
                    {fieldErrors.due_local ?? timezone}
                  </small>
                </label>
              </>
            ) : null}

            {item.kind === "event" ? (
              <>
                <label className="field">
                  <span>Lokasi <small>Opsional</small></span>
                  <div className="input-with-icon">
                    <MapPin size={17} aria-hidden="true" />
                    <input
                      aria-describedby={fieldErrors.location ? "correction-location-error" : undefined}
                      aria-invalid={Boolean(fieldErrors.location)}
                      maxLength={500}
                      onChange={(event) => setLocation(event.target.value)}
                      value={location}
                    />
                  </div>
                  {fieldErrors.location ? <small className="field-error" id="correction-location-error">{fieldErrors.location}</small> : null}
                </label>
                <label className="check-field">
                  <input checked={allDay} onChange={(event) => setAllDay(event.target.checked)} type="checkbox" />
                  <span><strong>Sepanjang hari</strong><small>Ganti antara jadwal berjam dan tanggal penuh.</small></span>
                </label>
                {allDay ? (
                  <div className="correction-schedule-grid">
                    <label className="field">
                      <span>Tanggal mulai</span>
                      <input
                        aria-describedby={fieldErrors.starts_on ? "correction-starts-on-error" : undefined}
                        aria-invalid={Boolean(fieldErrors.starts_on)}
                        onChange={(event) => setStartsOn(event.target.value)}
                        required
                        type="date"
                        value={startsOn}
                      />
                      {fieldErrors.starts_on ? <small className="field-error" id="correction-starts-on-error">{fieldErrors.starts_on}</small> : null}
                    </label>
                    <label className="field">
                      <span>Tanggal selesai <small>Inklusif</small></span>
                      <input
                        aria-describedby={fieldErrors.ends_on ? "correction-ends-on-error" : undefined}
                        aria-invalid={Boolean(fieldErrors.ends_on)}
                        min={startsOn}
                        onChange={(event) => setEndsOn(event.target.value)}
                        type="date"
                        value={endsOn}
                      />
                      {fieldErrors.ends_on ? <small className="field-error" id="correction-ends-on-error">{fieldErrors.ends_on}</small> : null}
                    </label>
                  </div>
                ) : (
                  <div className="correction-schedule-grid">
                    <label className="field">
                      <span>Mulai lokal</span>
                      <input
                        aria-describedby={fieldErrors.starts_local ? "correction-starts-local-error" : "correction-timezone-help"}
                        aria-invalid={Boolean(fieldErrors.starts_local)}
                        onChange={(event) => setStartsLocal(event.target.value)}
                        required
                        step={60}
                        type="datetime-local"
                        value={startsLocal}
                      />
                      <small className={fieldErrors.starts_local ? "field-error" : "field-help"} id={fieldErrors.starts_local ? "correction-starts-local-error" : "correction-timezone-help"}>
                        {fieldErrors.starts_local ?? timezone}
                      </small>
                    </label>
                    <label className="field">
                      <span>Selesai lokal <small>Opsional</small></span>
                      <input
                        aria-describedby={fieldErrors.ends_local ? "correction-ends-local-error" : undefined}
                        aria-invalid={Boolean(fieldErrors.ends_local)}
                        min={startsLocal}
                        onChange={(event) => setEndsLocal(event.target.value)}
                        step={60}
                        type="datetime-local"
                        value={endsLocal}
                      />
                      {fieldErrors.ends_local ? <small className="field-error" id="correction-ends-local-error">{fieldErrors.ends_local}</small> : null}
                    </label>
                  </div>
                )}
              </>
            ) : null}

            {item.kind === "bill" ? (
              <>
                <label className="field">
                  <span>Nominal <small>{currency}</small></span>
                  <div className="money-input">
                    <span aria-hidden="true">Rp</span>
                    <input
                      aria-describedby={fieldErrors.amount ? "correction-amount-error" : "correction-amount-help"}
                      aria-invalid={Boolean(fieldErrors.amount)}
                      inputMode="numeric"
                      onChange={(event) => setAmount(event.target.value.replace(/\D/g, ""))}
                      pattern="[0-9]+"
                      required
                      value={amount}
                    />
                  </div>
                  <small className={fieldErrors.amount ? "field-error" : "field-help"} id={fieldErrors.amount ? "correction-amount-error" : "correction-amount-help"}>
                    {fieldErrors.amount ?? (parseIntegerAmount(amount) ? formatMoney(Number(amount), currency) : "Rupiah utuh tanpa desimal.")}
                  </small>
                </label>
                <label className="field">
                  <span>Mata uang</span>
                  <input maxLength={3} onChange={(event) => setCurrency(event.target.value.toUpperCase())} pattern="[A-Z]{3}" required value={currency} />
                </label>
                <label className="field">
                  <span>Jatuh tempo lokal</span>
                  <input
                    aria-describedby={fieldErrors.due_local ? "correction-bill-due-error" : "correction-bill-due-help"}
                    aria-invalid={Boolean(fieldErrors.due_local)}
                    onChange={(event) => setBillDueLocal(event.target.value)}
                    required
                    step={60}
                    type="datetime-local"
                    value={billDueLocal}
                  />
                  <small className={fieldErrors.due_local ? "field-error" : "field-help"} id={fieldErrors.due_local ? "correction-bill-due-error" : "correction-bill-due-help"}>
                    {fieldErrors.due_local ?? timezone}
                  </small>
                </label>
              </>
            ) : null}

            <label className="field">
              <span>Catatan <small>Opsional</small></span>
              <textarea
                aria-describedby={notesFieldError ? "correction-notes-error" : undefined}
                aria-invalid={Boolean(notesFieldError)}
                maxLength={5000}
                onChange={(event) => setNotes(event.target.value)}
                rows={4}
                value={notes}
              />
              {notesFieldError ? <small className="field-error" id="correction-notes-error">{notesFieldError}</small> : null}
            </label>

            <button className="button button-primary button-full" disabled={interactionBusy} type="submit">
              {submitting ? <RefreshCw className="spin" size={18} aria-hidden="true" /> : item.kind === "task" ? <ListChecks size={18} aria-hidden="true" /> : item.kind === "event" ? <CalendarDays size={18} aria-hidden="true" /> : <ReceiptText size={18} aria-hidden="true" />}
              {submitting ? "Menyimpan…" : "Simpan perubahan"}
            </button>
          </form>

          <ReminderControls
            anchorLabel={item.kind === "task" ? "tenggat tugas" : item.kind === "event" ? (item.allDay ? "tanggal jadwal" : "waktu mulai jadwal") : "jatuh tempo tagihan"}
            disabled={submitting}
            onCreate={onCreateReminder}
            onDelete={onDeleteReminder}
            onLoad={onLoadReminders}
            onBusyChange={setReminderBusy}
            onUpdate={onUpdateReminder}
            scheduleKind={item.kind === "task" ? (item.dueAt ? "before_moment" : null) : item.kind === "event" && item.allDay ? "before_date" : "before_moment"}
            sourceId={item.id}
            sourceKind={item.kind}
            timezone={timezone}
          />

          {(item.kind === "task" && Boolean(item.completedAt)) || (item.kind === "bill" && Boolean(item.paidAt)) ? (
            <section className="correction-state-action" aria-label="Batalkan status">
              <div>
                <strong>{item.kind === "task" ? "Tugas sudah selesai" : "Tagihan sudah lunas"}</strong>
                <p>Status dapat dibatalkan tanpa menghapus item.</p>
              </div>
              <button className="quiet-button" disabled={interactionBusy} onClick={() => void performInverseAction()} type="button">
                <RotateCcw size={17} aria-hidden="true" />
                {item.kind === "task" ? "Batalkan selesai" : "Tandai belum lunas"}
              </button>
            </section>
          ) : null}

          <section className="correction-danger-zone" aria-label="Hapus item">
            {!confirmingDelete ? (
              <button
                className="quiet-button correction-delete-trigger"
                disabled={interactionBusy}
                onClick={() => {
                  setConfirmingDelete(true);
                  window.requestAnimationFrame(() => cancelDeleteRef.current?.focus());
                }}
                ref={deleteTriggerRef}
                type="button"
              >
                <Trash2 size={17} aria-hidden="true" /> Hapus {kindLabel(item.kind)}
              </button>
            ) : (
              <div className="correction-delete-confirm" role="group" aria-label={`Konfirmasi hapus ${item.title}`}>
                <p><strong>Hapus {item.title}?</strong> Tindakan ini tidak dapat dibatalkan.</p>
                <div>
                  <button
                    className="quiet-button"
                    disabled={interactionBusy}
                    onClick={() => {
                      setConfirmingDelete(false);
                      window.requestAnimationFrame(() => deleteTriggerRef.current?.focus());
                    }}
                    ref={cancelDeleteRef}
                    type="button"
                  >
                    Batal
                  </button>
                  <button className="button button-danger" disabled={interactionBusy} onClick={() => void performDelete()} type="button">
                    {submitting ? <RefreshCw className="spin" size={17} aria-hidden="true" /> : <Trash2 size={17} aria-hidden="true" />}
                    {submitting ? "Menghapus…" : "Ya, hapus"}
                  </button>
                </div>
              </div>
            )}
          </section>
        </div>
      </section>
    </div>
  );
}
