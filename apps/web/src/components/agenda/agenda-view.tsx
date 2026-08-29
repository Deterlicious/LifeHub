"use client";

import {
  AlertCircle,
  CalendarDays,
  CalendarRange,
  FileText,
  ListChecks,
  MapPin,
  MoreHorizontal,
  ReceiptText,
  RefreshCw,
} from "lucide-react";
import { useMemo, useState } from "react";

import type {
  Agenda,
  AgendaBillItem,
  AgendaItem,
  Bill,
  CorrectableItem,
} from "@/lib/api/types";
import { formatMoney } from "@/lib/currency";
import {
  formatAgendaGroupHeading,
  formatDateOnlyShort,
  formatDateStripDay,
  formatDateStripWeekday,
  formatEventDateRange,
  formatEventTimeRange,
  formatFullMoment,
  formatTime,
  getSevenDayStrip,
  toLocalDateTimeValue,
} from "@/lib/dates";
import { documentCategoryLabel, documentStatusLabel } from "@/lib/documents";

export type AgendaStatus = "idle" | "loading" | "ready" | "error";
export type PaidHistoryStatus = "idle" | "loading" | "ready" | "error";
type AgendaFilter = "all" | AgendaItem["kind"];

interface AgendaViewProps {
  agenda: Agenda | null;
  range: { from: string; to: string };
  status: AgendaStatus;
  error: string;
  paidBills: Bill[];
  paidHistoryStatus: PaidHistoryStatus;
  paidHistoryError: string;
  paidHistoryHasMore: boolean;
  loadingMorePaid: boolean;
  timezone: string;
  onJumpDate: (dateOnly: string) => void;
  onRetry: () => Promise<unknown>;
  onLoadPaidHistory: () => Promise<unknown>;
  onLoadMorePaid: () => Promise<unknown>;
  onEdit: (item: CorrectableItem) => void;
}

const FILTERS: ReadonlyArray<{ value: AgendaFilter; label: string }> = [
  { value: "all", label: "Semua" },
  { value: "task", label: "Tugas" },
  { value: "event", label: "Jadwal" },
  { value: "bill", label: "Tagihan" },
  { value: "document", label: "Dokumen" },
];

function itemTypeLabel(item: AgendaItem): string {
  if (item.kind === "task") return "Tugas";
  if (item.kind === "event") return "Jadwal";
  if (item.kind === "bill") return "Tagihan";
  return "Dokumen";
}

function itemIcon(item: AgendaItem) {
  if (item.kind === "task") return <ListChecks size={17} aria-hidden="true" />;
  if (item.kind === "event") return <CalendarDays size={17} aria-hidden="true" />;
  if (item.kind === "bill") return <ReceiptText size={17} aria-hidden="true" />;
  return <FileText size={17} aria-hidden="true" />;
}

function itemSchedule(item: AgendaItem, timezone: string): string {
  if (item.kind === "task") {
    return item.dueAt ? formatTime(item.dueAt, timezone) : "Fleksibel";
  }
  if (item.kind === "event") {
    return item.allDay
      ? "Seharian"
      : formatEventTimeRange(item.startsAt, item.endsAt, timezone);
  }
  if (item.kind === "bill") return formatTime(item.dueAt, timezone);
  return formatDateOnlyShort(item.expiresOn) ?? item.expiresOn;
}

function AgendaRow({
  item,
  timezone,
  onEdit,
}: {
  item: AgendaItem;
  timezone: string;
  onEdit: (item: CorrectableItem) => void;
}) {
  return (
    <article className={`agenda-item agenda-item-${item.kind}`}>
      <div className="agenda-item-marker" aria-hidden="true">{itemIcon(item)}</div>
      <div className="agenda-item-copy">
        <div className="agenda-item-heading">
          <div>
            <p className="agenda-item-type">{itemTypeLabel(item)}</p>
            <h3>{item.title}</h3>
          </div>
          {item.kind !== "document" ? (
            <button
              aria-label={`Ubah atau hapus ${item.title}`}
              className="icon-button agenda-item-more"
              onClick={() => onEdit(item)}
              type="button"
            >
              <MoreHorizontal size={20} aria-hidden="true" />
            </button>
          ) : null}
        </div>

        <div className="agenda-item-meta">
          <span className="agenda-item-schedule">{itemSchedule(item, timezone)}</span>
          {item.kind === "task" ? <span>Prioritas {item.priority === "high" ? "tinggi" : item.priority === "low" ? "rendah" : "normal"}</span> : null}
          {item.kind === "event" && item.allDay ? <span>{formatEventDateRange(item.startsOn, item.endsOn)}</span> : null}
          {item.kind === "event" && item.location ? <span><MapPin size={12} aria-hidden="true" /> {item.location}</span> : null}
          {item.kind === "bill" ? <span>{formatMoney(item.amount, item.currency)}</span> : null}
          {item.kind === "document" ? <span>{documentCategoryLabel(item.category)} · {documentStatusLabel(item.status)}</span> : null}
        </div>
        {item.notes ? <p className="agenda-item-notes">{item.notes}</p> : null}
      </div>
    </article>
  );
}

function paidBillToAgendaItem(bill: Bill, timezone: string): AgendaBillItem {
  const paidDate = bill.paidAt
    ? toLocalDateTimeValue(new Date(bill.paidAt), timezone).slice(0, 10)
    : toLocalDateTimeValue(new Date(bill.dueAt), timezone).slice(0, 10);
  return {
    ...bill,
    kind: "bill",
    displayOn: paidDate,
    status: "paid",
  };
}

export function AgendaView({
  agenda,
  range,
  status,
  error,
  paidBills,
  paidHistoryStatus,
  paidHistoryError,
  paidHistoryHasMore,
  loadingMorePaid,
  timezone,
  onJumpDate,
  onRetry,
  onLoadPaidHistory,
  onLoadMorePaid,
  onEdit,
}: AgendaViewProps) {
  const [filter, setFilter] = useState<AgendaFilter>("all");
  const [billMode, setBillMode] = useState<"unpaid" | "paid">("unpaid");
  const loadedAgenda = agenda?.from === range.from && agenda.to === range.to ? agenda : null;
  const stripDates = range.from ? getSevenDayStrip(range.from) : [];
  const showingPaidHistory = filter === "bill" && billMode === "paid";
  const filteredItems = useMemo(
    () => (loadedAgenda?.items ?? []).filter((item) => filter === "all" || item.kind === filter),
    [loadedAgenda?.items, filter],
  );
  const groupedItems = useMemo(() => {
    const groups = new Map<string, AgendaItem[]>();
    for (const item of filteredItems) {
      const group = groups.get(item.displayOn);
      if (group) group.push(item);
      else groups.set(item.displayOn, [item]);
    }
    return Array.from(groups.entries());
  }, [filteredItems]);
  const paidItems = paidBills.map((bill) => paidBillToAgendaItem(bill, timezone));

  function chooseBillMode(mode: "unpaid" | "paid") {
    setBillMode(mode);
    if (mode === "paid" && paidHistoryStatus === "idle") void onLoadPaidHistory();
  }

  return (
    <div className="agenda-page" id="agenda">
      <header className="agenda-hero">
        <div>
          <p className="eyebrow">Rencana yang tetap tenang</p>
          <h1>Agenda</h1>
          <p>Lihat yang akan datang tanpa kehilangan fokus pada Today.</p>
        </div>
        <a className="quiet-button agenda-back-today" href="#today">Kembali ke Today</a>
      </header>

      <section className="agenda-controls" aria-label="Rentang dan filter Agenda">
        <div className="agenda-date-tools">
          <div>
            <p className="agenda-range-copy">
              {range.from && range.to
                ? `${formatDateOnlyShort(range.from)}–${formatDateOnlyShort(range.to)}`
                : "Memuat rentang…"}
            </p>
            <p>{loadedAgenda?.summary.total ?? 0} hal terjadwal</p>
          </div>
          <label className="agenda-date-jump">
            <span>Lompat ke tanggal</span>
            <input
              onChange={(event) => onJumpDate(event.target.value)}
              type="date"
              value={range.from}
            />
          </label>
        </div>

        <div className="agenda-date-strip" aria-label="Tujuh hari dari tanggal pilihan">
          {stripDates.map((dateOnly, index) => (
            <button
              aria-pressed={index === 0}
              className={index === 0 ? "agenda-date-active" : undefined}
              key={dateOnly}
              onClick={() => onJumpDate(dateOnly)}
              type="button"
            >
              <span>{formatDateStripWeekday(dateOnly)}</span>
              <strong>{formatDateStripDay(dateOnly)}</strong>
            </button>
          ))}
        </div>

        <div className="agenda-filter-chips" aria-label="Filter jenis Agenda">
          {FILTERS.map((option) => (
            <button
              aria-pressed={filter === option.value}
              key={option.value}
              onClick={() => setFilter(option.value)}
              type="button"
            >
              {option.label}
            </button>
          ))}
        </div>

        {filter === "bill" ? (
          <div className="agenda-bill-mode" aria-label="Status tagihan">
            <button aria-pressed={billMode === "unpaid"} onClick={() => chooseBillMode("unpaid")} type="button">Belum lunas</button>
            <button aria-pressed={billMode === "paid"} onClick={() => chooseBillMode("paid")} type="button">Riwayat lunas</button>
          </div>
        ) : null}
      </section>

      {status === "loading" ? (
        <div className="agenda-state" role="status"><RefreshCw className="spin" size={20} aria-hidden="true" /> Menyusun rentang Agenda…</div>
      ) : null}

      {status === "error" ? (
        <div className="agenda-state agenda-state-error" role="alert">
          <AlertCircle size={20} aria-hidden="true" />
          <span>{error || "Agenda belum dapat dimuat."}</span>
          <button className="quiet-button" onClick={() => void onRetry()} type="button">Coba lagi</button>
        </div>
      ) : null}

      {showingPaidHistory ? (
        <section className="agenda-groups" aria-label="Riwayat tagihan lunas">
          <div className="agenda-history-heading">
            <div>
              <p className="eyebrow">Jejak pembayaran</p>
              <h2>Riwayat lunas</h2>
            </div>
            <ReceiptText size={20} aria-hidden="true" />
          </div>
          {paidHistoryStatus === "loading" && paidItems.length === 0 ? (
            <div className="agenda-state" role="status"><RefreshCw className="spin" size={19} aria-hidden="true" /> Memuat riwayat…</div>
          ) : null}
          {paidHistoryStatus === "error" ? (
            <div className="agenda-state agenda-state-error" role="alert">
              <AlertCircle size={19} aria-hidden="true" /> {paidHistoryError || "Riwayat belum dapat dimuat."}
              <button className="quiet-button" onClick={() => void onLoadPaidHistory()} type="button">Coba lagi</button>
            </div>
          ) : null}
          {paidHistoryStatus === "ready" && paidItems.length === 0 ? (
            <div className="agenda-empty"><ReceiptText size={24} aria-hidden="true" /><h2>Belum ada riwayat lunas.</h2><p>Tagihan yang dibayar akan tetap bisa ditemukan di sini.</p></div>
          ) : null}
          {paidItems.map((item) => (
            <section className="agenda-group" key={item.id}>
              <h2>{item.paidAt ? `Dibayar ${formatFullMoment(item.paidAt, timezone)}` : "Sudah dibayar"}</h2>
              <AgendaRow item={item} onEdit={onEdit} timezone={timezone} />
            </section>
          ))}
          {paidHistoryStatus === "ready" && paidHistoryHasMore ? (
            <button className="button agenda-load-more" disabled={loadingMorePaid} onClick={() => void onLoadMorePaid()} type="button">
              {loadingMorePaid ? <RefreshCw className="spin" size={18} aria-hidden="true" /> : null}
              {loadingMorePaid ? "Memuat…" : "Muat riwayat berikutnya"}
            </button>
          ) : null}
        </section>
      ) : (
        <section className="agenda-groups" aria-label="Daftar Agenda">
          {status === "ready" && loadedAgenda && groupedItems.length === 0 ? (
            <div className="agenda-empty">
              <CalendarRange size={25} aria-hidden="true" />
              <h2>Rentang ini masih lapang.</h2>
              <p>Coba tanggal lain atau tampilkan semua jenis.</p>
            </div>
          ) : null}
          {groupedItems.map(([dateOnly, items]) => (
            <section className="agenda-group" key={dateOnly} aria-labelledby={`agenda-date-${dateOnly}`}>
              <h2 id={`agenda-date-${dateOnly}`}><time dateTime={dateOnly}>{formatAgendaGroupHeading(dateOnly)}</time></h2>
              <div className="agenda-group-list">
                {items.map((item) => <AgendaRow item={item} key={`${item.kind}-${item.id}`} onEdit={onEdit} timezone={timezone} />)}
              </div>
            </section>
          ))}
        </section>
      )}
    </div>
  );
}
