"use client";

import { Bell, Check, CheckCheck, RefreshCw, X } from "lucide-react";
import { useEffect, useRef } from "react";

import type { NotificationItem } from "@/lib/api/types";

export type NotificationCenterStatus = "loading" | "ready" | "error";

interface NotificationCenterProps {
  items: NotificationItem[];
  status: NotificationCenterStatus;
  error: string;
  unreadCount: number;
  hasMore: boolean;
  loadingMore: boolean;
  markingId: string | null;
  markingAll: boolean;
  timezone: string;
  onClose: () => void;
  onRetry: () => Promise<unknown>;
  onLoadMore: () => Promise<unknown>;
  onMarkRead: (notificationId: string) => Promise<unknown>;
  onMarkAllRead: () => Promise<unknown>;
}

function sourceLabel(kind: NotificationItem["sourceKind"]): string {
  if (kind === "event") return "Jadwal";
  if (kind === "bill") return "Tagihan";
  if (kind === "document") return "Dokumen";
  return "Tugas";
}

function formatCreatedAt(value: string, timezone: string): string {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  try {
    return new Intl.DateTimeFormat("id-ID", {
      dateStyle: "medium",
      timeStyle: "short",
      timeZone: timezone,
    }).format(parsed);
  } catch {
    return value;
  }
}

export function NotificationCenter({
  items,
  status,
  error,
  unreadCount,
  hasMore,
  loadingMore,
  markingId,
  markingAll,
  timezone,
  onClose,
  onRetry,
  onLoadMore,
  onMarkRead,
  onMarkAllRead,
}: NotificationCenterProps) {
  const dialogRef = useRef<HTMLElement>(null);
  const closeRef = useRef<HTMLButtonElement>(null);
  const onCloseRef = useRef(onClose);
  const busy = loadingMore || markingAll || Boolean(markingId);
  const busyRef = useRef(busy);
  const wasBusyRef = useRef(false);

  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  useEffect(() => {
    busyRef.current = busy;
    const wasBusy = wasBusyRef.current;
    wasBusyRef.current = busy;
    if (!wasBusy || busy) return;
    window.requestAnimationFrame(() => {
      const active = document.activeElement;
      if (
        active === dialogRef.current
        || !(active instanceof HTMLElement)
        || !dialogRef.current?.contains(active)
      ) {
        closeRef.current?.focus();
      }
    });
  }, [busy]);

  useEffect(() => {
    const previouslyFocused = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        event.preventDefault();
        if (!busyRef.current) onCloseRef.current();
        return;
      }
      if (event.key !== "Tab" || !dialogRef.current) return;
      const focusable = Array.from(dialogRef.current.querySelectorAll<HTMLElement>(
        'button:not([disabled]), [href], [tabindex]:not([tabindex="-1"])',
      ));
      if (focusable.length === 0) {
        event.preventDefault();
        dialogRef.current.focus();
        return;
      }
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

  return (
    <div
      className="sheet-backdrop notification-backdrop"
      onMouseDown={(event) => {
        if (!busy && event.currentTarget === event.target) onClose();
      }}
      role="presentation"
    >
      <section
        aria-busy={busy}
        aria-labelledby="notification-center-title"
        aria-modal="true"
        className="notification-center"
        ref={dialogRef}
        role="dialog"
        tabIndex={-1}
      >
        <header className="notification-center-header">
          <div>
            <p className="eyebrow">Tetap tenang dan tepat waktu</p>
            <h2 id="notification-center-title"><Bell size={21} aria-hidden="true" /> Notifikasi</h2>
          </div>
          <button aria-label="Tutup pusat notifikasi" className="icon-button" disabled={busy} onClick={onClose} ref={closeRef} type="button">
            <X size={20} aria-hidden="true" />
          </button>
        </header>

        <div className="notification-center-toolbar">
          <p aria-live="polite"><strong>{unreadCount.toLocaleString("id-ID")}</strong> belum dibaca</p>
          {unreadCount > 0 ? (
            <button className="quiet-button" disabled={busy} onClick={() => void onMarkAllRead()} type="button">
              {markingAll ? <RefreshCw className="spin" size={16} aria-hidden="true" /> : <CheckCheck size={16} aria-hidden="true" />}
              {markingAll ? "Menandai…" : "Tandai semua dibaca"}
            </button>
          ) : null}
        </div>

        <div className="notification-center-body">
          {status === "loading" ? (
            <div className="notification-state" role="status">
              <RefreshCw className="spin" size={20} aria-hidden="true" />
              <p>Memuat notifikasi…</p>
            </div>
          ) : null}

          {status === "error" ? (
            <div className="notification-state notification-state-error" role="alert">
              <Bell size={21} aria-hidden="true" />
              <p>{error || "Notifikasi belum dapat dimuat."}</p>
              <button className="quiet-button" onClick={() => void onRetry()} type="button">Coba lagi</button>
            </div>
          ) : null}

          {status === "ready" && items.length === 0 ? (
            <div className="notification-state notification-state-empty">
              <Bell size={22} aria-hidden="true" />
              <div>
                <strong>Belum ada notifikasi</strong>
                <p>Pengingat yang waktunya tiba akan muncul di sini.</p>
              </div>
            </div>
          ) : null}

          {status === "ready" && items.length > 0 ? (
            <div className="notification-list" aria-label="Daftar notifikasi">
              {items.map((notification) => {
                const unread = notification.readAt === null;
                const isMarking = markingId === notification.id;
                return (
                  <article className={`notification-item${unread ? " notification-item-unread" : ""}`} key={notification.id}>
                    <div className="notification-item-marker" aria-hidden="true" />
                    <div className="notification-item-copy">
                      <div className="notification-item-meta">
                        <span>{sourceLabel(notification.sourceKind)}</span>
                        <time dateTime={notification.createdAt}>{formatCreatedAt(notification.createdAt, timezone)}</time>
                      </div>
                      <h3>{notification.title}</h3>
                      {notification.body ? <p>{notification.body}</p> : null}
                    </div>
                    {unread ? (
                      <button
                        aria-label={`Tandai ${notification.title} sebagai dibaca`}
                        className="icon-button notification-read-button"
                        disabled={busy}
                        onClick={() => void onMarkRead(notification.id)}
                        type="button"
                      >
                        {isMarking ? <RefreshCw className="spin" size={17} aria-hidden="true" /> : <Check size={17} aria-hidden="true" />}
                      </button>
                    ) : (
                      <span className="notification-read-label"><Check size={14} aria-hidden="true" /> Dibaca</span>
                    )}
                  </article>
                );
              })}
            </div>
          ) : null}

          {status === "ready" && error ? (
            <div className="notification-state notification-state-error notification-page-error" role="alert">
              <p>{error}</p>
              <button className="quiet-button" onClick={() => void onLoadMore()} type="button">Coba muat lagi</button>
            </div>
          ) : null}

          {status === "ready" && hasMore && !error ? (
            <button className="quiet-button notification-load-more" disabled={busy} onClick={() => void onLoadMore()} type="button">
              {loadingMore ? <RefreshCw className="spin" size={17} aria-hidden="true" /> : null}
              {loadingMore ? "Memuat…" : "Muat notifikasi lainnya"}
            </button>
          ) : null}
        </div>
      </section>
    </div>
  );
}
