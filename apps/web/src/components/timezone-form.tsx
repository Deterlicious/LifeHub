"use client";

import { ArrowRight, Clock3, Globe2, Leaf, LogOut, X } from "lucide-react";
import { useEffect, useRef, useState, type FormEvent } from "react";

import { getBrowserTimezone } from "@/lib/dates";

const COMMON_TIMEZONES = [
  "Asia/Jakarta",
  "Asia/Makassar",
  "Asia/Jayapura",
  "Asia/Singapore",
  "Asia/Kuala_Lumpur",
  "Asia/Bangkok",
  "UTC",
];

interface TimezoneFormProps {
  initialTimezone?: string;
  mode: "onboarding" | "dialog";
  onCancel?: () => void;
  onSave: (timezone: string) => Promise<void>;
  onSignOut?: () => void;
}

export function TimezoneForm({
  initialTimezone,
  mode,
  onCancel,
  onSave,
  onSignOut,
}: TimezoneFormProps) {
  const [timezone, setTimezone] = useState(initialTimezone || getBrowserTimezone());
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const dialogRef = useRef<HTMLElement>(null);
  const cancelRef = useRef(onCancel);

  useEffect(() => {
    cancelRef.current = onCancel;
  }, [onCancel]);

  useEffect(() => {
    if (mode !== "dialog") inputRef.current?.focus();
  }, [mode]);

  useEffect(() => {
    if (mode !== "dialog") return;

    const previouslyFocused = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        cancelRef.current?.();
        return;
      }
      if (event.key !== "Tab" || !dialogRef.current) return;

      const focusable = Array.from(dialogRef.current.querySelectorAll<HTMLElement>(
        'button:not([disabled]), input:not([disabled]), [href], select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
      ));
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    inputRef.current?.focus();
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
      previouslyFocused?.focus();
    };
  }, [mode]);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError(null);

    try {
      await onSave(timezone.trim());
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Zona waktu belum dapat disimpan.");
    } finally {
      setSubmitting(false);
    }
  }

  const content = (
    <div className={mode === "dialog" ? "timezone-card timezone-card-dialog" : "timezone-card"}>
      {mode === "dialog" && onCancel ? (
        <button className="dialog-close icon-button" onClick={onCancel} type="button">
          <X size={19} aria-hidden="true" /><span className="sr-only">Tutup pengaturan</span>
        </button>
      ) : null}

      <span className="feature-icon" aria-hidden="true"><Globe2 size={25} /></span>
      <p className="eyebrow">Waktu yang benar, hari yang jelas</p>
      <h1>{mode === "onboarding" ? "Di zona waktu mana kamu berada?" : "Atur zona waktu"}</h1>
      <p className="timezone-intro">
        LifeHub memakai zona waktu ini untuk menentukan batas Today dan menampilkan tenggat
        tanpa bergantung pada waktu server.
      </p>

      {error ? <div className="inline-alert inline-alert-error" role="alert">{error}</div> : null}

      <form className="timezone-form" onSubmit={handleSubmit}>
        <label className="field">
          <span>Zona waktu IANA</span>
          <div className="input-with-icon">
            <Clock3 size={18} aria-hidden="true" />
            <input
              aria-describedby="timezone-help"
              autoComplete="off"
              list="lifehub-timezones"
              onChange={(event) => setTimezone(event.target.value)}
              placeholder="Asia/Jakarta"
              ref={inputRef}
              required
              value={timezone}
            />
          </div>
        </label>
        <datalist id="lifehub-timezones">
          {COMMON_TIMEZONES.map((zone) => <option key={zone} value={zone} />)}
        </datalist>
        <p className="field-help" id="timezone-help">
          Contoh Indonesia: Asia/Jakarta, Asia/Makassar, atau Asia/Jayapura.
        </p>

        <button className="button button-primary button-full" disabled={submitting} type="submit">
          {submitting ? "Menyimpan…" : mode === "onboarding" ? "Lanjut ke Today" : "Simpan perubahan"}
          {!submitting ? <ArrowRight size={18} aria-hidden="true" /> : null}
        </button>
      </form>

      {mode === "onboarding" && onSignOut ? (
        <button className="quiet-button onboarding-signout" onClick={onSignOut} type="button">
          <LogOut size={16} aria-hidden="true" /> Keluar dari sesi
        </button>
      ) : null}
    </div>
  );

  if (mode === "dialog") {
    return (
      <div className="dialog-backdrop" role="presentation" onMouseDown={(event) => {
        if (event.currentTarget === event.target) onCancel?.();
      }}>
        <section aria-labelledby="timezone-dialog-title" aria-modal="true" ref={dialogRef} role="dialog">
          <span className="sr-only" id="timezone-dialog-title">Pengaturan zona waktu</span>
          {content}
        </section>
      </div>
    );
  }

  return (
    <main className="onboarding-layout">
      <header className="onboarding-header">
        <span className="brand"><span className="brand-mark" aria-hidden="true"><Leaf size={20} /></span> LifeHub</span>
        <span>Langkah 1 dari 1</span>
      </header>
      {content}
    </main>
  );
}
