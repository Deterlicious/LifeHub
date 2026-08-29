"use client";

import { CalendarRange, ChevronDown, Pencil, Repeat2, X } from "lucide-react";
import { useState, type FormEvent } from "react";

import type {
  RecurrenceFrequency,
  RecurrenceInput,
  RecurrenceSeries,
} from "@/lib/api/types";
import { formatDateOnlyShort } from "@/lib/dates";

export type RecurrenceStatus = "loading" | "ready" | "error";

interface RecurrenceManagerProps {
  items: RecurrenceSeries[];
  status: RecurrenceStatus;
  error: string;
  onRetry: () => Promise<boolean>;
  onUpdate: (seriesId: string, input: RecurrenceInput) => Promise<boolean>;
  onStop: (seriesId: string) => Promise<boolean>;
}

const frequencyLabels: Record<RecurrenceFrequency, string> = {
  daily: "Harian",
  weekly: "Mingguan",
  monthly: "Bulanan",
  yearly: "Tahunan",
};

function kindLabel(kind: RecurrenceSeries["sourceKind"]): string {
  if (kind === "event") return "Jadwal";
  if (kind === "bill") return "Tagihan";
  return "Tugas";
}

function ruleLabel(item: RecurrenceSeries): string {
  const frequency = frequencyLabels[item.frequency].toLowerCase();
  const interval = item.interval === 1 ? frequency : `setiap ${item.interval} ${frequency}`;
  return item.endsOn
    ? `${interval}, sampai ${formatDateOnlyShort(item.endsOn) ?? item.endsOn}`
    : interval;
}

export function RecurrenceManager({
  items,
  status,
  error,
  onRetry,
  onUpdate,
  onStop,
}: RecurrenceManagerProps) {
  const [editingId, setEditingId] = useState<string | null>(null);
  const [confirmingId, setConfirmingId] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const activeItems = items.filter((item) => item.active);

  return (
    <section className="recurrence-management" aria-labelledby="recurrence-manager-title">
      <details className="recurrence-disclosure">
        <summary>
          <span className="recurrence-summary-icon" aria-hidden="true"><Repeat2 size={18} /></span>
          <span>
            <strong id="recurrence-manager-title">Seri berulang</strong>
            <small>{activeItems.length === 0 ? "Belum ada seri aktif" : `${activeItems.length} seri aktif`}</small>
          </span>
          <ChevronDown className="recurrence-summary-chevron" size={18} aria-hidden="true" />
        </summary>

        <div className="recurrence-panel">
          <div className="recurrence-panel-heading">
            <div>
              <p className="eyebrow">Rutinitas tetap terkendali</p>
              <h2>Kelola pengulangan</h2>
            </div>
            {status === "error" ? (
              <button className="quiet-button" onClick={() => void onRetry()} type="button">Coba lagi</button>
            ) : null}
          </div>

          {status === "loading" ? <p className="recurrence-state" role="status">Memuat seri…</p> : null}
          {status === "error" ? <p className="recurrence-state recurrence-state-error" role="alert">{error}</p> : null}
          {status === "ready" && activeItems.length === 0 ? (
            <div className="recurrence-empty">
              <CalendarRange size={21} aria-hidden="true" />
              <p>Aktifkan “Jadikan berulang” saat menambah tugas, jadwal, atau tagihan.</p>
            </div>
          ) : null}

          {status === "ready" && activeItems.length > 0 ? (
            <div className="recurrence-list">
              {activeItems.map((item) => (
                <article className="recurrence-item" key={item.id}>
                  <div className="recurrence-item-copy">
                    <span className="recurrence-kind">{kindLabel(item.sourceKind)}</span>
                    <h3>{item.title}</h3>
                    <p>{ruleLabel(item)}</p>
                    <small>Mulai {formatDateOnlyShort(item.anchorOn) ?? item.anchorOn} · {item.timezone}</small>
                  </div>
                  {editingId === item.id ? (
                    <RecurrenceEditForm
                      item={item}
                      busy={busyId === item.id}
                      onCancel={() => setEditingId(null)}
                      onSave={async (input) => {
                        setBusyId(item.id);
                        try {
                          if (await onUpdate(item.id, input)) setEditingId(null);
                        } finally {
                          setBusyId(null);
                        }
                      }}
                    />
                  ) : confirmingId === item.id ? (
                    <div className="recurrence-confirm" role="group" aria-label={`Konfirmasi hentikan ${item.title}`}>
                      <p>Occurrence mendatang yang belum selesai atau belum lunas akan dihapus.</p>
                      <div>
                        <button className="quiet-button" disabled={busyId === item.id} onClick={() => setConfirmingId(null)} type="button">Batal</button>
                        <button
                          className="button button-danger"
                          disabled={busyId === item.id}
                          onClick={async () => {
                            setBusyId(item.id);
                            try {
                              if (await onStop(item.id)) setConfirmingId(null);
                            } finally {
                              setBusyId(null);
                            }
                          }}
                          type="button"
                        >
                          {busyId === item.id ? "Menghentikan…" : "Ya, hentikan"}
                        </button>
                      </div>
                    </div>
                  ) : (
                    <div className="recurrence-actions">
                      <button className="quiet-button" onClick={() => { setEditingId(item.id); setConfirmingId(null); }} type="button">
                        <Pencil size={15} aria-hidden="true" /> Ubah aturan
                      </button>
                      <button className="quiet-button recurrence-stop-button" onClick={() => { setConfirmingId(item.id); setEditingId(null); }} type="button">
                        <X size={15} aria-hidden="true" /> Hentikan
                      </button>
                    </div>
                  )}
                </article>
              ))}
            </div>
          ) : null}
        </div>
      </details>
    </section>
  );
}

function RecurrenceEditForm({
  item,
  busy,
  onCancel,
  onSave,
}: {
  item: RecurrenceSeries;
  busy: boolean;
  onCancel: () => void;
  onSave: (input: RecurrenceInput) => Promise<void>;
}) {
  const [frequency, setFrequency] = useState(item.frequency);
  const [interval, setInterval] = useState(String(item.interval));
  const [endsOn, setEndsOn] = useState(item.endsOn ?? "");

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void onSave({ frequency, interval: Number(interval), ends_on: endsOn || undefined });
  }

  return (
    <form className="recurrence-edit-form" onSubmit={submit}>
      <label className="field">
        <span>Frekuensi</span>
        <div className="select-wrap">
          <select onChange={(event) => setFrequency(event.target.value as RecurrenceFrequency)} value={frequency}>
            <option value="daily">Harian</option>
            <option value="weekly">Mingguan</option>
            <option value="monthly">Bulanan</option>
            <option value="yearly">Tahunan</option>
          </select>
          <ChevronDown size={16} aria-hidden="true" />
        </div>
      </label>
      <label className="field">
        <span>Setiap</span>
        <input min={1} max={365} onChange={(event) => setInterval(event.target.value)} required type="number" value={interval} />
      </label>
      <label className="field">
        <span>Berakhir <small>Opsional</small></span>
        <input min={item.anchorOn} onChange={(event) => setEndsOn(event.target.value)} type="date" value={endsOn} />
      </label>
      <div className="recurrence-edit-actions">
        <button className="quiet-button" disabled={busy} onClick={onCancel} type="button">Batal</button>
        <button className="button button-primary" disabled={busy} type="submit">{busy ? "Menyimpan…" : "Simpan aturan"}</button>
      </div>
    </form>
  );
}
