"use client";

import {
  CalendarClock,
  CalendarDays,
  ChevronDown,
  FileText,
  ListChecks,
  MapPin,
  Plus,
  ReceiptText,
  Repeat2,
  ShieldCheck,
} from "lucide-react";
import { useEffect, useRef, useState, type FormEvent } from "react";

import { ApiError } from "@/lib/api/client";
import type {
  CreateBillInput,
  CreateDocumentInput,
  CreateEventInput,
  CreateTaskInput,
  Priority,
  RecurrenceFrequency,
  RecurrenceInput,
  SmartCaptureResult,
} from "@/lib/api/types";
import { formatMoney, parseIntegerAmount } from "@/lib/currency";
import {
  ensureLocalDateTimeSeconds,
  getDefaultBillDueLocal,
  getDefaultDocumentExpiryDate,
  getDefaultDueLocal,
  getDefaultEventSchedule,
  reconcilePristineDateValue,
} from "@/lib/dates";
import { DOCUMENT_CATEGORY_OPTIONS } from "@/lib/documents";

type QuickAddKind = "task" | "event" | "bill" | "document";

interface QuickAddFormProps {
  timezone: string;
  profileDate: string;
  onCreateTask: (input: CreateTaskInput) => Promise<void>;
  onCreateEvent: (input: CreateEventInput) => Promise<void>;
  onCreateBill: (input: CreateBillInput) => Promise<void>;
  onCreateDocument: (input: CreateDocumentInput) => Promise<void>;
  onParseSmartCapture: (text: string) => Promise<SmartCaptureResult>;
}

export function QuickAddForm({
  timezone,
  profileDate,
  onCreateTask,
  onCreateEvent,
  onCreateBill,
  onCreateDocument,
  onParseSmartCapture,
}: QuickAddFormProps) {
  const eventDefaults = getDefaultEventSchedule(new Date(), timezone);
  const [kind, setKind] = useState<QuickAddKind>("task");
  const [taskTitle, setTaskTitle] = useState("");
  const [taskNotes, setTaskNotes] = useState("");
  const [priority, setPriority] = useState<Priority>("normal");
  const [dueLocal, setDueLocal] = useState(() => getDefaultDueLocal(new Date(), timezone));
  const [eventTitle, setEventTitle] = useState("");
  const [eventNotes, setEventNotes] = useState("");
  const [location, setLocation] = useState("");
  const [allDay, setAllDay] = useState(false);
  const [startsLocal, setStartsLocal] = useState(eventDefaults.startsLocal);
  const [endsLocal, setEndsLocal] = useState(eventDefaults.endsLocal);
  const [startsOn, setStartsOn] = useState(eventDefaults.startsOn);
  const [endsOn, setEndsOn] = useState(eventDefaults.endsOn);
  const [billTitle, setBillTitle] = useState("");
  const [billNotes, setBillNotes] = useState("");
  const [billAmount, setBillAmount] = useState("");
  const [billDueLocal, setBillDueLocal] = useState(() =>
    getDefaultBillDueLocal(new Date(), timezone),
  );
  const [documentName, setDocumentName] = useState("");
  const [documentCategory, setDocumentCategory] = useState<CreateDocumentInput["category"]>("identity");
  const [documentNotes, setDocumentNotes] = useState("");
  const [expiresOn, setExpiresOn] = useState(() =>
    getDefaultDocumentExpiryDate(new Date(), timezone),
  );
  const [recurrenceEnabled, setRecurrenceEnabled] = useState(false);
  const [recurrenceFrequency, setRecurrenceFrequency] = useState<RecurrenceFrequency>("weekly");
  const [recurrenceInterval, setRecurrenceInterval] = useState("1");
  const [recurrenceEndsOn, setRecurrenceEndsOn] = useState("");
  const [smartText, setSmartText] = useState("");
  const [smartAmbiguities, setSmartAmbiguities] = useState<string[]>([]);
  const [smartReady, setSmartReady] = useState(false);
  const [smartParsing, setSmartParsing] = useState(false);
  const [smartError, setSmartError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const dateFieldEditedRef = useRef({
    taskDue: false,
    eventStartsLocal: false,
    eventEndsLocal: false,
    eventStartsOn: false,
    eventEndsOn: false,
    billDue: false,
    documentExpiry: false,
  });

  useEffect(() => {
    const now = new Date();
    const nextEventDefaults = getDefaultEventSchedule(now, timezone);
    setDueLocal((current) => reconcilePristineDateValue(
      current,
      dateFieldEditedRef.current.taskDue,
      getDefaultDueLocal(now, timezone),
    ));
    setStartsLocal((current) => reconcilePristineDateValue(
      current,
      dateFieldEditedRef.current.eventStartsLocal,
      nextEventDefaults.startsLocal,
    ));
    setEndsLocal((current) => reconcilePristineDateValue(
      current,
      dateFieldEditedRef.current.eventEndsLocal,
      nextEventDefaults.endsLocal,
    ));
    setStartsOn((current) => reconcilePristineDateValue(
      current,
      dateFieldEditedRef.current.eventStartsOn,
      nextEventDefaults.startsOn,
    ));
    setEndsOn((current) => reconcilePristineDateValue(
      current,
      dateFieldEditedRef.current.eventEndsOn,
      nextEventDefaults.endsOn,
    ));
    setBillDueLocal((current) => reconcilePristineDateValue(
      current,
      dateFieldEditedRef.current.billDue,
      getDefaultBillDueLocal(now, timezone),
    ));
    setExpiresOn((current) => reconcilePristineDateValue(
      current,
      dateFieldEditedRef.current.documentExpiry,
      getDefaultDocumentExpiryDate(now, timezone),
    ));
  }, [profileDate, timezone]);

  function selectKind(nextKind: QuickAddKind) {
    setKind(nextKind);
    setError(null);
    setFieldErrors({});
    setRecurrenceEnabled(false);
    setRecurrenceFrequency(nextKind === "bill" ? "monthly" : "weekly");
    setRecurrenceInterval("1");
    setRecurrenceEndsOn("");
  }

  function recurrenceInput(): RecurrenceInput | undefined {
    if (!recurrenceEnabled) return undefined;
    return {
      frequency: recurrenceFrequency,
      interval: Number(recurrenceInterval),
      ends_on: recurrenceEndsOn || undefined,
    };
  }

  function resetRecurrence() {
    setRecurrenceEnabled(false);
    setRecurrenceInterval("1");
    setRecurrenceEndsOn("");
  }

  function resetTask() {
    dateFieldEditedRef.current.taskDue = false;
    setTaskTitle("");
    setTaskNotes("");
    setPriority("normal");
    setDueLocal(getDefaultDueLocal(new Date(), timezone));
  }

  function resetEvent() {
    dateFieldEditedRef.current.eventStartsLocal = false;
    dateFieldEditedRef.current.eventEndsLocal = false;
    dateFieldEditedRef.current.eventStartsOn = false;
    dateFieldEditedRef.current.eventEndsOn = false;
    const defaults = getDefaultEventSchedule(new Date(), timezone);
    setEventTitle("");
    setEventNotes("");
    setLocation("");
    setAllDay(false);
    setStartsLocal(defaults.startsLocal);
    setEndsLocal(defaults.endsLocal);
    setStartsOn(defaults.startsOn);
    setEndsOn(defaults.endsOn);
  }

  function resetBill() {
    dateFieldEditedRef.current.billDue = false;
    setBillTitle("");
    setBillNotes("");
    setBillAmount("");
    setBillDueLocal(getDefaultBillDueLocal(new Date(), timezone));
  }

  function resetDocument() {
    dateFieldEditedRef.current.documentExpiry = false;
    setDocumentName("");
    setDocumentCategory("identity");
    setDocumentNotes("");
    setExpiresOn(getDefaultDocumentExpiryDate(new Date(), timezone));
  }

  function resetSmartCapture() {
    setSmartText("");
    setSmartAmbiguities([]);
    setSmartReady(false);
    setSmartError(null);
  }

  function applySmartCapture(result: SmartCaptureResult) {
    const { draft } = result;
    setKind(draft.kind);
    setError(null);
    setFieldErrors({});
    setSmartAmbiguities(result.ambiguities);
    setSmartReady(true);

    if (draft.recurrence && draft.kind !== "document") {
      setRecurrenceEnabled(true);
      setRecurrenceFrequency(draft.recurrence.frequency);
      setRecurrenceInterval(String(draft.recurrence.interval ?? 1));
      setRecurrenceEndsOn(draft.recurrence.ends_on ?? "");
    } else {
      resetRecurrence();
    }

    if (draft.kind === "task") {
      dateFieldEditedRef.current.taskDue = true;
      setTaskTitle(draft.title);
      setTaskNotes(draft.notes);
      setPriority(draft.priority);
      setDueLocal(draft.dueLocal);
    } else if (draft.kind === "event") {
      const nextAllDay = draft.allDay === true;
      dateFieldEditedRef.current.eventStartsLocal = true;
      dateFieldEditedRef.current.eventEndsLocal = true;
      dateFieldEditedRef.current.eventStartsOn = true;
      dateFieldEditedRef.current.eventEndsOn = true;
      setEventTitle(draft.title);
      setEventNotes(draft.notes);
      setLocation(draft.location);
      setAllDay(nextAllDay);
      setStartsLocal(draft.startsLocal);
      setEndsLocal(draft.endsLocal);
      setStartsOn(draft.startsOn);
      setEndsOn(draft.endsOn);
    } else if (draft.kind === "bill") {
      dateFieldEditedRef.current.billDue = true;
      setBillTitle(draft.title);
      setBillNotes(draft.notes);
      setBillAmount(draft.amount === null ? "" : String(draft.amount));
      setBillDueLocal(draft.dueLocal);
    } else {
      dateFieldEditedRef.current.documentExpiry = true;
      setDocumentName(draft.name || draft.title);
      setDocumentCategory(draft.category);
      setDocumentNotes(draft.notes);
      setExpiresOn(draft.expiresOn);
    }
  }

  async function handleSmartCapture() {
    const text = smartText.trim();
    if (!text) {
      setSmartError("Ceritakan dulu hal yang ingin ditambahkan.");
      return;
    }
    setSmartParsing(true);
    setSmartError(null);
    setSmartReady(false);
    setSmartAmbiguities([]);
    try {
      applySmartCapture(await onParseSmartCapture(text));
    } catch (reason) {
      setSmartError(
        reason instanceof Error
          ? reason.message
          : "Draf belum dapat dibuat. Form manual tetap bisa digunakan.",
      );
    } finally {
      setSmartParsing(false);
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError(null);
    setFieldErrors({});

    try {
      if (kind === "task") {
        await onCreateTask({
          title: taskTitle.trim(),
          notes: taskNotes.trim() || undefined,
          priority,
          due_local: dueLocal ? ensureLocalDateTimeSeconds(dueLocal) : undefined,
          recurrence: recurrenceInput(),
        });
        resetTask();
        resetRecurrence();
        resetSmartCapture();
      } else if (kind === "event") {
        if (allDay) {
          await onCreateEvent({
            title: eventTitle.trim(),
            notes: eventNotes.trim() || undefined,
            location: location.trim() || undefined,
            all_day: true,
            starts_on: startsOn,
            ends_on: endsOn || undefined,
            recurrence: recurrenceInput(),
          });
        } else {
          await onCreateEvent({
            title: eventTitle.trim(),
            notes: eventNotes.trim() || undefined,
            location: location.trim() || undefined,
            all_day: false,
            starts_local: ensureLocalDateTimeSeconds(startsLocal),
            ends_local: endsLocal ? ensureLocalDateTimeSeconds(endsLocal) : undefined,
            recurrence: recurrenceInput(),
          });
        }
        resetEvent();
        resetRecurrence();
        resetSmartCapture();
      } else if (kind === "bill") {
        const amount = parseIntegerAmount(billAmount);
        if (amount === null) {
          setFieldErrors({ amount: "Masukkan nominal rupiah utuh lebih dari nol." });
          setError("Periksa data yang belum valid.");
          return;
        }
        await onCreateBill({
          title: billTitle.trim(),
          notes: billNotes.trim() || undefined,
          amount,
          currency: "IDR",
          due_local: ensureLocalDateTimeSeconds(billDueLocal),
          recurrence: recurrenceInput(),
        });
        resetBill();
        resetRecurrence();
        resetSmartCapture();
      } else {
        await onCreateDocument({
          name: documentName.trim(),
          category: documentCategory,
          notes: documentNotes.trim() || undefined,
          expires_on: expiresOn,
        });
        resetDocument();
        resetSmartCapture();
      }
    } catch (reason) {
      if (reason instanceof ApiError) {
        setFieldErrors(reason.fields);
        setError(reason.message);
      } else {
        setError(
          reason instanceof Error
            ? reason.message
            : kind === "task"
              ? "Tugas belum dapat ditambahkan."
              : kind === "event"
                ? "Jadwal belum dapat ditambahkan."
                : kind === "bill"
                  ? "Tagihan belum dapat ditambahkan."
                  : "Dokumen belum dapat disimpan.",
        );
      }
    } finally {
      setSubmitting(false);
    }
  }

  const titleError = fieldErrors.title;
  const notesError = fieldErrors.notes;
  const recurrenceAnchorDate = kind === "task"
    ? dueLocal.slice(0, 10)
    : kind === "event"
      ? (allDay ? startsOn : startsLocal.slice(0, 10))
      : billDueLocal.slice(0, 10);

  return (
    <section className="quick-add-card" id="quick-add" aria-labelledby="quick-add-title">
      <div className="quick-add-heading">
        <span className="quick-add-icon" aria-hidden="true"><Plus size={19} /></span>
        <div>
          <p className="eyebrow">Tangkap sebelum terlupa</p>
          <h2 id="quick-add-title">Tambah cepat</h2>
        </div>
      </div>

      <div className="smart-capture-panel">
        <div className="smart-capture-heading">
          <div>
            <strong>Buat draf dari kalimat</strong>
            <p>LifeHub mengisi form untuk diperiksa. Tidak akan disimpan otomatis.</p>
          </div>
          <span>Draf</span>
        </div>
        <label className="field" htmlFor="smart-capture-text">
          <span>Ceritakan yang ingin ditambahkan</span>
          <textarea
            aria-describedby="smart-capture-help"
            id="smart-capture-text"
            maxLength={1000}
            onChange={(event) => {
              setSmartText(event.target.value);
              setSmartError(null);
            }}
            placeholder="Contoh: Bayar internet 350 ribu tanggal 15 tiap bulan"
            rows={3}
            value={smartText}
          />
          <small className="field-help" id="smart-capture-help">Bahasa Indonesia, maksimal 1.000 karakter.</small>
        </label>
        <button
          className="button button-full smart-capture-button"
          disabled={smartParsing || !smartText.trim()}
          onClick={() => void handleSmartCapture()}
          type="button"
        >
          {smartParsing ? "Menyusun draf…" : "Buat draf"}
        </button>
        {smartError ? <div className="inline-alert inline-alert-error" role="alert">{smartError}</div> : null}
        {smartReady ? (
          <div className="smart-capture-review" role="status">
            <strong>Draf siap diperiksa. Belum disimpan.</strong>
            {smartAmbiguities.length > 0 ? (
              <ul>
                {smartAmbiguities.map((ambiguity) => <li key={ambiguity}>{ambiguity}</li>)}
              </ul>
            ) : <p>Periksa kembali detail di bawah sebelum menyimpan.</p>}
          </div>
        ) : null}
      </div>

      {error ? <div className="inline-alert inline-alert-error" role="alert">{error}</div> : null}

      <form className="quick-add-form" onSubmit={handleSubmit}>
        <fieldset className="quick-add-kind">
          <legend className="sr-only">Pilih jenis yang ingin ditambahkan</legend>
          <label>
            <input
              checked={kind === "task"}
              className="sr-only"
              name="quick-add-kind"
              onChange={() => selectKind("task")}
              type="radio"
              value="task"
            />
            <span><ListChecks size={17} aria-hidden="true" /> Tugas</span>
          </label>
          <label>
            <input
              checked={kind === "event"}
              className="sr-only"
              name="quick-add-kind"
              onChange={() => selectKind("event")}
              type="radio"
              value="event"
            />
            <span><CalendarDays size={17} aria-hidden="true" /> Jadwal</span>
          </label>
          <label>
            <input
              checked={kind === "bill"}
              className="sr-only"
              name="quick-add-kind"
              onChange={() => selectKind("bill")}
              type="radio"
              value="bill"
            />
            <span><ReceiptText size={17} aria-hidden="true" /> Tagihan</span>
          </label>
          <label>
            <input
              checked={kind === "document"}
              className="sr-only"
              name="quick-add-kind"
              onChange={() => selectKind("document")}
              type="radio"
              value="document"
            />
            <span><FileText size={17} aria-hidden="true" /> Dokumen</span>
          </label>
        </fieldset>

        {kind === "task" ? (
          <>
            <label className="field">
              <span>Apa yang perlu dilakukan?</span>
              <input
                aria-describedby={titleError ? "quick-add-title-error" : undefined}
                aria-invalid={Boolean(titleError)}
                autoComplete="off"
                maxLength={200}
                onChange={(event) => setTaskTitle(event.target.value)}
                placeholder="Contoh: Kirim laporan mingguan"
                required
                value={taskTitle}
              />
              {titleError ? <small className="field-error" id="quick-add-title-error">{titleError}</small> : null}
            </label>

            <div className="quick-add-grid">
              <label className="field">
                <span>Tenggat lokal</span>
                <div className="input-with-icon">
                  <CalendarClock size={17} aria-hidden="true" />
                  <input
                    aria-describedby={fieldErrors.due_local ? "task-due-error" : "task-due-help"}
                    aria-invalid={Boolean(fieldErrors.due_local)}
                    onChange={(event) => {
                      dateFieldEditedRef.current.taskDue = true;
                      setDueLocal(event.target.value);
                    }}
                    step={60}
                    type="datetime-local"
                    value={dueLocal.slice(0, 16)}
                  />
                </div>
                <small className={fieldErrors.due_local ? "field-error" : "field-help"} id={fieldErrors.due_local ? "task-due-error" : "task-due-help"}>
                  {fieldErrors.due_local ?? timezone}
                </small>
              </label>

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
            </div>

            <details className="notes-disclosure">
              <summary>Tambahkan catatan <span>Opsional</span></summary>
              <label className="field">
                <span className="sr-only">Catatan tugas</span>
                <textarea
                  aria-describedby={notesError ? "quick-add-notes-error" : undefined}
                  aria-invalid={Boolean(notesError)}
                  maxLength={5000}
                  onChange={(event) => setTaskNotes(event.target.value)}
                  placeholder="Konteks singkat agar tugas lebih mudah dimulai…"
                  rows={3}
                  value={taskNotes}
                />
                {notesError ? <small className="field-error" id="quick-add-notes-error">{notesError}</small> : null}
              </label>
            </details>
          </>
        ) : kind === "event" ? (
          <>
            <label className="field">
              <span>Apa jadwalnya?</span>
              <input
                aria-describedby={titleError ? "quick-add-title-error" : undefined}
                aria-invalid={Boolean(titleError)}
                autoComplete="off"
                maxLength={200}
                onChange={(event) => setEventTitle(event.target.value)}
                placeholder="Contoh: Meeting proyek"
                required
                value={eventTitle}
              />
              {titleError ? <small className="field-error" id="quick-add-title-error">{titleError}</small> : null}
            </label>

            <label className="check-field">
              <input
                checked={allDay}
                onChange={(event) => setAllDay(event.target.checked)}
                type="checkbox"
              />
              <span>
                <strong>Sepanjang hari</strong>
                <small>Tanpa jam mulai dan selesai.</small>
              </span>
            </label>

            {allDay ? (
              <div className="event-schedule-grid">
                <label className="field">
                  <span>Tanggal mulai</span>
                  <input
                    aria-describedby={fieldErrors.starts_on ? "event-starts-on-error" : undefined}
                    aria-invalid={Boolean(fieldErrors.starts_on)}
                    onChange={(event) => {
                      dateFieldEditedRef.current.eventStartsOn = true;
                      setStartsOn(event.target.value);
                    }}
                    required
                    type="date"
                    value={startsOn}
                  />
                  {fieldErrors.starts_on ? <small className="field-error" id="event-starts-on-error">{fieldErrors.starts_on}</small> : null}
                </label>
                <label className="field">
                  <span>Tanggal selesai</span>
                  <input
                    aria-describedby={fieldErrors.ends_on ? "event-ends-on-error" : "event-ends-on-help"}
                    aria-invalid={Boolean(fieldErrors.ends_on)}
                    min={startsOn}
                    onChange={(event) => {
                      dateFieldEditedRef.current.eventEndsOn = true;
                      setEndsOn(event.target.value);
                    }}
                    type="date"
                    value={endsOn}
                  />
                  <small className={fieldErrors.ends_on ? "field-error" : "field-help"} id={fieldErrors.ends_on ? "event-ends-on-error" : "event-ends-on-help"}>
                    {fieldErrors.ends_on ?? "Termasuk tanggal ini."}
                  </small>
                </label>
              </div>
            ) : (
              <div className="event-schedule-grid">
                <label className="field">
                  <span>Mulai</span>
                  <div className="input-with-icon">
                    <CalendarClock size={17} aria-hidden="true" />
                    <input
                      aria-describedby={fieldErrors.starts_local ? "event-starts-local-error" : "event-timezone-help"}
                      aria-invalid={Boolean(fieldErrors.starts_local)}
                      onChange={(event) => {
                        dateFieldEditedRef.current.eventStartsLocal = true;
                        setStartsLocal(event.target.value);
                      }}
                      required
                      step={60}
                      type="datetime-local"
                      value={startsLocal.slice(0, 16)}
                    />
                  </div>
                  <small className={fieldErrors.starts_local ? "field-error" : "field-help"} id={fieldErrors.starts_local ? "event-starts-local-error" : "event-timezone-help"}>
                    {fieldErrors.starts_local ?? timezone}
                  </small>
                </label>
                <label className="field">
                  <span>Selesai</span>
                  <input
                    aria-describedby={fieldErrors.ends_local ? "event-ends-local-error" : undefined}
                    aria-invalid={Boolean(fieldErrors.ends_local)}
                    min={startsLocal.slice(0, 16)}
                    onChange={(event) => {
                      dateFieldEditedRef.current.eventEndsLocal = true;
                      setEndsLocal(event.target.value);
                    }}
                    step={60}
                    type="datetime-local"
                    value={endsLocal.slice(0, 16)}
                  />
                  {fieldErrors.ends_local ? <small className="field-error" id="event-ends-local-error">{fieldErrors.ends_local}</small> : null}
                </label>
              </div>
            )}

            <label className="field">
              <span>Lokasi <small>Opsional</small></span>
              <div className="input-with-icon">
                <MapPin size={17} aria-hidden="true" />
                <input
                  aria-describedby={fieldErrors.location ? "event-location-error" : undefined}
                  aria-invalid={Boolean(fieldErrors.location)}
                  autoComplete="off"
                  maxLength={500}
                  onChange={(event) => setLocation(event.target.value)}
                  placeholder="Contoh: Online atau Ruang 3"
                  value={location}
                />
              </div>
              {fieldErrors.location ? <small className="field-error" id="event-location-error">{fieldErrors.location}</small> : null}
            </label>

            <details className="notes-disclosure">
              <summary>Tambahkan catatan <span>Opsional</span></summary>
              <label className="field">
                <span className="sr-only">Catatan jadwal</span>
                <textarea
                  aria-describedby={notesError ? "quick-add-notes-error" : undefined}
                  aria-invalid={Boolean(notesError)}
                  maxLength={5000}
                  onChange={(event) => setEventNotes(event.target.value)}
                  placeholder="Konteks singkat untuk jadwal ini…"
                  rows={3}
                  value={eventNotes}
                />
                {notesError ? <small className="field-error" id="quick-add-notes-error">{notesError}</small> : null}
              </label>
            </details>
          </>
        ) : kind === "bill" ? (
          <>
            <label className="field">
              <span>Tagihan apa yang perlu dibayar?</span>
              <input
                aria-describedby={titleError ? "quick-add-title-error" : undefined}
                aria-invalid={Boolean(titleError)}
                autoComplete="off"
                maxLength={200}
                onChange={(event) => setBillTitle(event.target.value)}
                placeholder="Contoh: Internet rumah"
                required
                value={billTitle}
              />
              {titleError ? <small className="field-error" id="quick-add-title-error">{titleError}</small> : null}
            </label>

            <label className="field">
              <span>Nominal <small>IDR</small></span>
              <div className="money-input">
                <span aria-hidden="true">Rp</span>
                <input
                  aria-describedby={fieldErrors.amount ? "bill-amount-error" : "bill-amount-help"}
                  aria-invalid={Boolean(fieldErrors.amount)}
                  autoComplete="off"
                  inputMode="numeric"
                  maxLength={16}
                  onChange={(event) => setBillAmount(event.target.value.replace(/\D/g, ""))}
                  pattern="[0-9]+"
                  placeholder="350000"
                  required
                  value={billAmount}
                />
              </div>
              <small className={fieldErrors.amount ? "field-error" : "field-help"} id={fieldErrors.amount ? "bill-amount-error" : "bill-amount-help"}>
                {fieldErrors.amount
                  ?? (parseIntegerAmount(billAmount) === null
                    ? "Masukkan rupiah utuh tanpa desimal."
                    : formatMoney(Number(billAmount), "IDR"))}
              </small>
            </label>

            <label className="field">
              <span>Jatuh tempo lokal</span>
              <div className="input-with-icon">
                <CalendarClock size={17} aria-hidden="true" />
                <input
                  aria-describedby={fieldErrors.due_local ? "bill-due-error" : "bill-due-help"}
                  aria-invalid={Boolean(fieldErrors.due_local)}
                    onChange={(event) => {
                      dateFieldEditedRef.current.billDue = true;
                      setBillDueLocal(event.target.value);
                    }}
                  required
                  step={60}
                  type="datetime-local"
                  value={billDueLocal.slice(0, 16)}
                />
              </div>
              <small className={fieldErrors.due_local ? "field-error" : "field-help"} id={fieldErrors.due_local ? "bill-due-error" : "bill-due-help"}>
                {fieldErrors.due_local ?? timezone}
              </small>
            </label>

            <details className="notes-disclosure">
              <summary>Tambahkan catatan <span>Opsional</span></summary>
              <label className="field">
                <span className="sr-only">Catatan tagihan</span>
                <textarea
                  aria-describedby={notesError ? "quick-add-notes-error" : undefined}
                  aria-invalid={Boolean(notesError)}
                  maxLength={5000}
                  onChange={(event) => setBillNotes(event.target.value)}
                  placeholder="Konteks singkat untuk tagihan ini…"
                  rows={3}
                  value={billNotes}
                />
                {notesError ? <small className="field-error" id="quick-add-notes-error">{notesError}</small> : null}
              </label>
            </details>
          </>
        ) : (
          <>
            <div className="document-privacy-note">
              <ShieldCheck size={17} aria-hidden="true" />
              <p>Simpan metadata saja. Jangan masukkan nomor dokumen atau unggah scan.</p>
            </div>

            <label className="field">
              <span>Nama dokumen</span>
              <input
                aria-describedby={fieldErrors.name ? "document-name-error" : undefined}
                aria-invalid={Boolean(fieldErrors.name)}
                autoComplete="off"
                maxLength={200}
                onChange={(event) => setDocumentName(event.target.value)}
                placeholder="Contoh: SIM A"
                required
                value={documentName}
              />
              {fieldErrors.name ? <small className="field-error" id="document-name-error">{fieldErrors.name}</small> : null}
            </label>

            <label className="field">
              <span>Kategori</span>
              <div className="select-wrap">
                <select
                  aria-describedby={fieldErrors.category ? "document-category-error" : undefined}
                  aria-invalid={Boolean(fieldErrors.category)}
                  onChange={(event) => setDocumentCategory(event.target.value as CreateDocumentInput["category"])}
                  value={documentCategory}
                >
                  {DOCUMENT_CATEGORY_OPTIONS.map((option) => (
                    <option key={option.value} value={option.value}>{option.label}</option>
                  ))}
                </select>
                <ChevronDown size={17} aria-hidden="true" />
              </div>
              {fieldErrors.category ? <small className="field-error" id="document-category-error">{fieldErrors.category}</small> : null}
            </label>

            <label className="field">
              <span>Tanggal kedaluwarsa</span>
              <input
                aria-describedby={fieldErrors.expires_on ? "document-expiry-error" : "document-expiry-help"}
                aria-invalid={Boolean(fieldErrors.expires_on)}
                onChange={(event) => {
                  dateFieldEditedRef.current.documentExpiry = true;
                  setExpiresOn(event.target.value);
                }}
                required
                type="date"
                value={expiresOn}
              />
              <small className={fieldErrors.expires_on ? "field-error" : "field-help"} id={fieldErrors.expires_on ? "document-expiry-error" : "document-expiry-help"}>
                {fieldErrors.expires_on ?? "Tanggal disimpan apa adanya, tanpa konversi zona waktu."}
              </small>
            </label>

            <details className="notes-disclosure">
              <summary>Tambahkan catatan <span>Opsional</span></summary>
              <label className="field">
                <span className="sr-only">Catatan dokumen</span>
                <textarea
                  aria-describedby={fieldErrors.notes ? "quick-add-notes-error" : undefined}
                  aria-invalid={Boolean(fieldErrors.notes)}
                  maxLength={5000}
                  onChange={(event) => setDocumentNotes(event.target.value)}
                  placeholder="Konteks aman tanpa nomor dokumen…"
                  rows={3}
                  value={documentNotes}
                />
                {fieldErrors.notes ? <small className="field-error" id="quick-add-notes-error">{fieldErrors.notes}</small> : null}
              </label>
            </details>
          </>
        )}

        {kind !== "document" ? (
          <div className="recurrence-control">
            <label className="check-field">
              <input
                checked={recurrenceEnabled}
                onChange={(event) => setRecurrenceEnabled(event.target.checked)}
                type="checkbox"
              />
              <span>
                <strong><Repeat2 size={16} aria-hidden="true" /> Jadikan berulang</strong>
                <small>Buat occurrence terpisah yang tetap bisa diedit atau diselesaikan sendiri.</small>
              </span>
            </label>
            {recurrenceEnabled ? (
              <div className="recurrence-fields">
                <label className="field">
                  <span>Frekuensi</span>
                  <div className="select-wrap">
                    <select
                      aria-describedby={fieldErrors["recurrence.frequency"] ? "recurrence-frequency-error" : undefined}
                      aria-invalid={Boolean(fieldErrors["recurrence.frequency"])}
                      onChange={(event) => setRecurrenceFrequency(event.target.value as RecurrenceFrequency)}
                      value={recurrenceFrequency}
                    >
                      <option value="daily">Harian</option>
                      <option value="weekly">Mingguan</option>
                      <option value="monthly">Bulanan</option>
                      <option value="yearly">Tahunan</option>
                    </select>
                    <ChevronDown size={17} aria-hidden="true" />
                  </div>
                  {fieldErrors["recurrence.frequency"] ? <small className="field-error" id="recurrence-frequency-error">{fieldErrors["recurrence.frequency"]}</small> : null}
                </label>
                <label className="field">
                  <span>Setiap</span>
                  <input
                    aria-describedby={fieldErrors["recurrence.interval"] ? "recurrence-interval-error" : "recurrence-interval-help"}
                    aria-invalid={Boolean(fieldErrors["recurrence.interval"])}
                    inputMode="numeric"
                    max={365}
                    min={1}
                    onChange={(event) => setRecurrenceInterval(event.target.value)}
                    required
                    type="number"
                    value={recurrenceInterval}
                  />
                  <small className={fieldErrors["recurrence.interval"] ? "field-error" : "field-help"} id={fieldErrors["recurrence.interval"] ? "recurrence-interval-error" : "recurrence-interval-help"}>
                    {fieldErrors["recurrence.interval"] ?? "Satuan mengikuti frekuensi."}
                  </small>
                </label>
                <label className="field">
                  <span>Berakhir <small>Opsional</small></span>
                  <input
                    aria-describedby={fieldErrors["recurrence.ends_on"] ? "recurrence-end-error" : "recurrence-end-help"}
                    aria-invalid={Boolean(fieldErrors["recurrence.ends_on"])}
                    min={recurrenceAnchorDate}
                    onChange={(event) => setRecurrenceEndsOn(event.target.value)}
                    type="date"
                    value={recurrenceEndsOn}
                  />
                  <small className={fieldErrors["recurrence.ends_on"] ? "field-error" : "field-help"} id={fieldErrors["recurrence.ends_on"] ? "recurrence-end-error" : "recurrence-end-help"}>
                    {fieldErrors["recurrence.ends_on"] ?? "Kosongkan agar terus berulang."}
                  </small>
                </label>
              </div>
            ) : null}
          </div>
        ) : null}

        <button className="button button-primary button-full" disabled={submitting} type="submit">
          {kind === "task"
            ? <Plus size={18} aria-hidden="true" />
            : kind === "event"
              ? <CalendarDays size={18} aria-hidden="true" />
              : kind === "bill"
                ? <ReceiptText size={18} aria-hidden="true" />
                : <FileText size={18} aria-hidden="true" />}
          {submitting
            ? "Menyimpan…"
            : kind === "task"
              ? "Simpan tugas"
              : kind === "event"
                ? "Simpan jadwal"
                : kind === "bill"
                  ? "Simpan tagihan"
                  : "Simpan dokumen"}
        </button>
      </form>
    </section>
  );
}
