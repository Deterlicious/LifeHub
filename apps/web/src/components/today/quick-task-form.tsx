"use client";

import { CalendarClock, ChevronDown, Plus } from "lucide-react";
import { useState, type FormEvent } from "react";

import { ApiError } from "@/lib/api/client";
import type { CreateTaskInput, Priority } from "@/lib/api/types";
import { ensureLocalDateTimeSeconds, getDefaultDueLocal } from "@/lib/dates";

interface QuickTaskFormProps {
  timezone: string;
  onCreate: (input: CreateTaskInput) => Promise<void>;
}

export function QuickTaskForm({ timezone, onCreate }: QuickTaskFormProps) {
  const [title, setTitle] = useState("");
  const [notes, setNotes] = useState("");
  const [priority, setPriority] = useState<Priority>("normal");
  const [dueLocal, setDueLocal] = useState(() => getDefaultDueLocal(new Date(), timezone));
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError(null);
    setFieldErrors({});

    try {
      await onCreate({
        title: title.trim(),
        notes: notes.trim() || undefined,
        priority,
        due_local: dueLocal ? ensureLocalDateTimeSeconds(dueLocal) : undefined,
      });
      setTitle("");
      setNotes("");
      setPriority("normal");
      setDueLocal(getDefaultDueLocal(new Date(), timezone));
    } catch (reason) {
      if (reason instanceof ApiError) {
        setFieldErrors(reason.fields);
        setError(reason.message);
      } else {
        setError(reason instanceof Error ? reason.message : "Tugas belum dapat ditambahkan.");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <section className="quick-add-card" id="quick-add" aria-labelledby="quick-add-title">
      <div className="quick-add-heading">
        <span className="quick-add-icon" aria-hidden="true"><Plus size={19} /></span>
        <div>
          <p className="eyebrow">Tangkap sebelum terlupa</p>
          <h2 id="quick-add-title">Tambah tugas</h2>
        </div>
      </div>

      {error ? <div className="inline-alert inline-alert-error" role="alert">{error}</div> : null}

      <form className="quick-add-form" onSubmit={handleSubmit}>
        <label className="field">
          <span>Apa yang perlu dilakukan?</span>
          <input
            aria-describedby={fieldErrors.title ? "task-title-error" : undefined}
            aria-invalid={Boolean(fieldErrors.title)}
            autoComplete="off"
            maxLength={160}
            onChange={(event) => setTitle(event.target.value)}
            placeholder="Contoh: Kirim laporan mingguan"
            required
            value={title}
          />
          {fieldErrors.title ? <small className="field-error" id="task-title-error">{fieldErrors.title}</small> : null}
        </label>

        <div className="quick-add-grid">
          <label className="field">
            <span>Tenggat lokal</span>
            <div className="input-with-icon">
              <CalendarClock size={17} aria-hidden="true" />
              <input
                aria-describedby={fieldErrors.due_local ? "task-due-error" : "task-due-help"}
                aria-invalid={Boolean(fieldErrors.due_local)}
                onChange={(event) => setDueLocal(event.target.value)}
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
              maxLength={2000}
              onChange={(event) => setNotes(event.target.value)}
              placeholder="Konteks singkat agar tugas lebih mudah dimulai…"
              rows={3}
              value={notes}
            />
          </label>
        </details>

        <button className="button button-primary button-full" disabled={submitting} type="submit">
          <Plus size={18} aria-hidden="true" />
          {submitting ? "Menambahkan…" : "Tambah ke Today"}
        </button>
      </form>
    </section>
  );
}
