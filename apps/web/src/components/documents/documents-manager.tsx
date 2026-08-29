"use client";

import {
  AlertCircle,
  CalendarDays,
  ChevronDown,
  FileText,
  Pencil,
  RefreshCw,
  ShieldCheck,
  Trash2,
  X,
} from "lucide-react";
import { useState, type FormEvent } from "react";

import { ApiError } from "@/lib/api/client";
import {
  ReminderControls,
  type ReminderControlsProps,
} from "@/components/reminders/reminder-controls";
import type { DocumentRecord, UpdateDocumentInput } from "@/lib/api/types";
import { formatDateOnlyShort } from "@/lib/dates";
import {
  DOCUMENT_CATEGORY_OPTIONS,
  documentCategoryLabel,
  documentStatusLabel,
} from "@/lib/documents";

export type DocumentsStatus = "loading" | "ready" | "error";

interface DocumentsManagerProps {
  documents: DocumentRecord[];
  status: DocumentsStatus;
  error: string;
  onRetry: () => Promise<unknown>;
  onUpdate: (documentId: string, input: UpdateDocumentInput) => Promise<void>;
  onDelete: (documentId: string) => Promise<void>;
  timezone: string;
  onLoadReminders: ReminderControlsProps["onLoad"];
  onCreateReminder: ReminderControlsProps["onCreate"];
  onUpdateReminder: ReminderControlsProps["onUpdate"];
  onDeleteReminder: ReminderControlsProps["onDelete"];
}

interface EditDraft {
  name: string;
  category: UpdateDocumentInput["category"];
  notes: string;
  expiresOn: string;
}

export function DocumentsManager({
  documents,
  status,
  error,
  onRetry,
  onUpdate,
  onDelete,
  timezone,
  onLoadReminders,
  onCreateReminder,
  onUpdateReminder,
  onDeleteReminder,
}: DocumentsManagerProps) {
  const [editingId, setEditingId] = useState<string | null>(null);
  const [confirmingDeleteId, setConfirmingDeleteId] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [reminderBusyId, setReminderBusyId] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [draft, setDraft] = useState<EditDraft | null>(null);

  function beginEdit(documentRecord: DocumentRecord) {
    setEditingId(documentRecord.id);
    setConfirmingDeleteId(null);
    setActionError(null);
    setFieldErrors({});
    setDraft({
      name: documentRecord.name,
      category: documentRecord.category,
      notes: documentRecord.notes ?? "",
      expiresOn: documentRecord.expiresOn,
    });
  }

  function cancelEdit() {
    setEditingId(null);
    setDraft(null);
    setActionError(null);
    setFieldErrors({});
  }

  async function submitEdit(event: FormEvent<HTMLFormElement>, documentId: string) {
    event.preventDefault();
    if (!draft) return;
    setBusyId(documentId);
    setActionError(null);
    setFieldErrors({});
    try {
      await onUpdate(documentId, {
        name: draft.name.trim(),
        category: draft.category,
        notes: draft.notes.trim() || null,
        expires_on: draft.expiresOn,
      });
      cancelEdit();
    } catch (reason) {
      if (reason instanceof ApiError) {
        setFieldErrors(reason.fields);
        setActionError(reason.message);
      } else {
        setActionError(reason instanceof Error ? reason.message : "Dokumen belum dapat diperbarui.");
      }
    } finally {
      setBusyId(null);
    }
  }

  async function confirmDelete(documentRecord: DocumentRecord) {
    setBusyId(documentRecord.id);
    setActionError(null);
    try {
      await onDelete(documentRecord.id);
      setConfirmingDeleteId(null);
    } catch (reason) {
      setActionError(reason instanceof Error ? reason.message : "Dokumen belum dapat dihapus.");
    } finally {
      setBusyId(null);
    }
  }

  return (
    <section className="documents-management" aria-label="Kelola metadata dokumen">
      <details className="documents-disclosure">
        <summary>
          <span className="documents-summary-icon" aria-hidden="true"><FileText size={19} /></span>
          <span>
            <strong>Dokumen saya</strong>
            <small>{status === "ready" ? `${documents.length} metadata tersimpan` : "Metadata penting tetap mudah dijangkau"}</small>
          </span>
          <ChevronDown className="documents-summary-chevron" size={19} aria-hidden="true" />
        </summary>

        <div className="documents-panel">
          <div className="document-privacy-note document-privacy-note-wide">
            <ShieldCheck size={17} aria-hidden="true" />
            <p>Simpan metadata saja. Jangan masukkan nomor dokumen atau unggah scan.</p>
          </div>

          {status === "loading" ? (
            <div className="documents-state" role="status">
              <RefreshCw className="spin" size={19} aria-hidden="true" /> Memuat metadata dokumen…
            </div>
          ) : null}

          {status === "error" ? (
            <div className="documents-state documents-state-error" role="alert">
              <AlertCircle size={19} aria-hidden="true" />
              <span>{error || "Metadata dokumen belum dapat dimuat."}</span>
              <button className="quiet-button" onClick={() => void onRetry()} type="button">Coba lagi</button>
            </div>
          ) : null}

          {status === "ready" && documents.length === 0 ? (
            <div className="documents-empty">
              <FileText size={23} aria-hidden="true" />
              <p>Belum ada metadata dokumen. Tambahkan melalui panel Tambah cepat.</p>
            </div>
          ) : null}

          {status === "ready" && documents.length > 0 ? (
            <div className="documents-list">
              {documents.map((documentRecord) => {
                const isEditing = editingId === documentRecord.id && draft;
                const isConfirmingDelete = confirmingDeleteId === documentRecord.id;
                const isBusy = busyId === documentRecord.id || reminderBusyId === documentRecord.id;

                return (
                  <article className="document-record" key={documentRecord.id}>
                    {isEditing ? (
                      <>
                      <form className="document-edit-form" onSubmit={(event) => void submitEdit(event, documentRecord.id)}>
                        <div className="document-record-heading">
                          <div>
                            <p className="eyebrow">Ubah metadata</p>
                            <h3>{documentRecord.name}</h3>
                          </div>
                          <button aria-label="Batal mengubah dokumen" className="icon-button" disabled={isBusy} onClick={cancelEdit} type="button">
                            <X size={18} aria-hidden="true" />
                          </button>
                        </div>

                        {actionError ? <div className="inline-alert inline-alert-error" role="alert">{actionError}</div> : null}

                        <div className="document-edit-grid">
                          <label className="field">
                            <span>Nama dokumen</span>
                            <input
                              aria-describedby={fieldErrors.name ? "edit-document-name-error" : undefined}
                              aria-invalid={Boolean(fieldErrors.name)}
                              maxLength={200}
                              onChange={(event) => setDraft({ ...draft, name: event.target.value })}
                              required
                              value={draft.name}
                            />
                            {fieldErrors.name ? <small className="field-error" id="edit-document-name-error">{fieldErrors.name}</small> : null}
                          </label>

                          <label className="field">
                            <span>Kategori</span>
                            <div className="select-wrap">
                              <select
                                aria-describedby={fieldErrors.category ? "edit-document-category-error" : undefined}
                                aria-invalid={Boolean(fieldErrors.category)}
                                onChange={(event) => setDraft({
                                  ...draft,
                                  category: event.target.value as UpdateDocumentInput["category"],
                                })}
                                value={draft.category}
                              >
                                {DOCUMENT_CATEGORY_OPTIONS.map((option) => (
                                  <option key={option.value} value={option.value}>{option.label}</option>
                                ))}
                              </select>
                              <ChevronDown size={17} aria-hidden="true" />
                            </div>
                            {fieldErrors.category ? <small className="field-error" id="edit-document-category-error">{fieldErrors.category}</small> : null}
                          </label>

                          <label className="field">
                            <span>Tanggal kedaluwarsa</span>
                            <input
                              aria-describedby={fieldErrors.expires_on ? "edit-document-expiry-error" : undefined}
                              aria-invalid={Boolean(fieldErrors.expires_on)}
                              onChange={(event) => setDraft({ ...draft, expiresOn: event.target.value })}
                              required
                              type="date"
                              value={draft.expiresOn}
                            />
                            {fieldErrors.expires_on ? <small className="field-error" id="edit-document-expiry-error">{fieldErrors.expires_on}</small> : null}
                          </label>
                        </div>

                        <label className="field">
                          <span>Catatan <small>Opsional</small></span>
                          <textarea
                            aria-describedby={fieldErrors.notes ? "edit-document-notes-error" : undefined}
                            aria-invalid={Boolean(fieldErrors.notes)}
                            maxLength={5000}
                            onChange={(event) => setDraft({ ...draft, notes: event.target.value })}
                            placeholder="Konteks aman tanpa nomor dokumen…"
                            rows={3}
                            value={draft.notes}
                          />
                          {fieldErrors.notes ? <small className="field-error" id="edit-document-notes-error">{fieldErrors.notes}</small> : null}
                        </label>

                        <div className="document-form-actions">
                          <button className="quiet-button" disabled={isBusy} onClick={cancelEdit} type="button">Batal</button>
                          <button className="button button-primary" disabled={isBusy} type="submit">
                            {isBusy ? <RefreshCw className="spin" size={17} aria-hidden="true" /> : null}
                            {isBusy ? "Menyimpan…" : "Simpan perubahan"}
                          </button>
                        </div>
                      </form>
                      <ReminderControls
                        anchorLabel="tanggal kedaluwarsa dokumen"
                        disabled={busyId === documentRecord.id}
                        onCreate={onCreateReminder}
                        onDelete={onDeleteReminder}
                        onLoad={onLoadReminders}
                        onBusyChange={(nextBusy) => setReminderBusyId(nextBusy ? documentRecord.id : null)}
                        onUpdate={onUpdateReminder}
                        scheduleKind="before_date"
                        sourceId={documentRecord.id}
                        sourceKind="document"
                        timezone={timezone}
                      />
                      </>
                    ) : (
                      <>
                        <div className="document-record-heading">
                          <div>
                            <h3>{documentRecord.name}</h3>
                            <div className="document-record-meta">
                              <span>{documentCategoryLabel(documentRecord.category)}</span>
                              <span className={`document-status document-status-${documentRecord.status}`}>
                                {documentStatusLabel(documentRecord.status)}
                              </span>
                            </div>
                          </div>
                          <div className="document-record-actions">
                            <button aria-label={`Ubah ${documentRecord.name}`} className="icon-button" onClick={() => beginEdit(documentRecord)} type="button">
                              <Pencil size={17} aria-hidden="true" />
                            </button>
                            <button
                              aria-label={`Hapus ${documentRecord.name}`}
                              className="icon-button icon-button-danger"
                              onClick={() => {
                                setEditingId(null);
                                setConfirmingDeleteId(documentRecord.id);
                                setActionError(null);
                              }}
                              type="button"
                            >
                              <Trash2 size={17} aria-hidden="true" />
                            </button>
                          </div>
                        </div>

                        <p className="document-expiry">
                          <CalendarDays size={15} aria-hidden="true" />
                          Kedaluwarsa <time dateTime={documentRecord.expiresOn}>{formatDateOnlyShort(documentRecord.expiresOn) ?? documentRecord.expiresOn}</time>
                        </p>
                        {documentRecord.notes ? <p className="document-notes">{documentRecord.notes}</p> : null}

                        {isConfirmingDelete ? (
                          <div className="document-delete-confirm" role="group" aria-label={`Konfirmasi hapus ${documentRecord.name}`}>
                            <p><strong>Hapus metadata {documentRecord.name}?</strong> Tindakan ini tidak dapat dibatalkan.</p>
                            {actionError ? <div className="inline-alert inline-alert-error" role="alert">{actionError}</div> : null}
                            <div>
                              <button className="quiet-button" disabled={isBusy} onClick={() => setConfirmingDeleteId(null)} type="button">Batal</button>
                              <button className="button button-danger" disabled={isBusy} onClick={() => void confirmDelete(documentRecord)} type="button">
                                {isBusy ? <RefreshCw className="spin" size={17} aria-hidden="true" /> : <Trash2 size={17} aria-hidden="true" />}
                                {isBusy ? "Menghapus…" : "Ya, hapus"}
                              </button>
                            </div>
                          </div>
                        ) : null}
                      </>
                    )}
                  </article>
                );
              })}
            </div>
          ) : null}
        </div>
      </details>
    </section>
  );
}
