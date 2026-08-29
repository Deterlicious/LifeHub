"use client";

import { AlertCircle, CheckCircle2, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";

import { AppShell } from "@/components/app-shell";
import {
  AgendaView,
  type AgendaStatus,
  type PaidHistoryStatus,
} from "@/components/agenda/agenda-view";
import { useAuth } from "@/components/auth/auth-provider";
import { LoginScreen } from "@/components/auth/login-screen";
import { CorrectionSheet } from "@/components/corrections/correction-sheet";
import {
  DocumentsManager,
  type DocumentsStatus,
} from "@/components/documents/documents-manager";
import {
  NotificationCenter,
  type NotificationCenterStatus,
} from "@/components/notifications/notification-center";
import {
  RecurrenceManager,
  type RecurrenceStatus,
} from "@/components/recurrence/recurrence-manager";
import { AppLoadingScreen, WorkspaceError } from "@/components/status-screens";
import { TimezoneForm } from "@/components/timezone-form";
import { TodayView } from "@/components/today/today-view";
import {
  ApiError,
  completeTask,
  createBill,
  createDocument,
  createEvent,
  createReminder,
  createTask,
  deleteBill,
  deleteProfileData,
  deleteReminder,
  getProfile,
  deleteEvent,
  deleteTask,
  getDocuments,
  getAgenda,
  getBills,
  getNotificationUnreadCount,
  getNotifications,
  getReminders,
  getRecurrenceSeries,
  getToday,
  markAllNotificationsRead,
  markBillPaid,
  markBillUnpaid,
  markNotificationRead,
  parseSmartCapture,
  deleteDocument,
  updateProfile,
  updateReminder,
  updateRecurrenceSeries,
  updateDocument,
  updateBill,
  updateEvent,
  updateTask,
  uncompleteTask,
  stopRecurrenceSeries,
} from "@/lib/api/client";
import type {
  Agenda,
  Bill,
  CorrectableItem,
  CreateBillInput,
  CreateDocumentInput,
  CreateEventInput,
  CreateReminderInput,
  CreateTaskInput,
  Profile,
  NotificationItem,
  Reminder,
  ReminderSourceKind,
  RecurrenceInput,
  RecurrenceSeries,
  SmartCaptureResult,
  Today,
  UpdateBillInput,
  UpdateDocumentInput,
  UpdateEventInput,
  UpdateReminderInput,
  UpdateTaskInput,
  DocumentRecord,
} from "@/lib/api/types";
import {
  getAgendaRangeFrom,
  getDefaultAgendaRange,
  getProfileDateOnly,
  reconcileAgendaRangeForToday,
} from "@/lib/dates";
import { appViewFromHash, type AppView } from "@/lib/navigation";
import {
  createLatestRequestGate,
  mergeNewestPageById,
  mergeUniqueById,
} from "@/lib/request-state";

type WorkspaceStatus = "loading" | "ready" | "error";

interface ToastState {
  message: string;
  tone: "success" | "error";
  action?: {
    label: string;
    run: () => Promise<void>;
  };
}

function errorMessage(reason: unknown): string {
  return reason instanceof Error ? reason.message : "Terjadi kendala yang tidak terduga.";
}

function AuthenticatedWorkspace({ token }: { token: string }) {
  const { email, signOut } = useAuth();
  const [status, setStatus] = useState<WorkspaceStatus>("loading");
  const [profile, setProfile] = useState<Profile | null>(null);
  const [today, setToday] = useState<Today | null>(null);
  const [documents, setDocuments] = useState<DocumentRecord[]>([]);
  const [documentsStatus, setDocumentsStatus] = useState<DocumentsStatus>("loading");
  const [documentsError, setDocumentsError] = useState("");
  const [activeView, setActiveView] = useState<AppView>("today");
  const [agenda, setAgenda] = useState<Agenda | null>(null);
  const [agendaStatus, setAgendaStatus] = useState<AgendaStatus>("idle");
  const [agendaError, setAgendaError] = useState("");
  const [agendaRange, setAgendaRange] = useState({ from: "", to: "" });
  const [paidBills, setPaidBills] = useState<Bill[]>([]);
  const [paidHistoryStatus, setPaidHistoryStatus] = useState<PaidHistoryStatus>("idle");
  const [paidHistoryError, setPaidHistoryError] = useState("");
  const [paidHistoryCursor, setPaidHistoryCursor] = useState<string | null>(null);
  const [loadingMorePaid, setLoadingMorePaid] = useState(false);
  const [correctionItem, setCorrectionItem] = useState<CorrectableItem | null>(null);
  const [workspaceError, setWorkspaceError] = useState<string>("");
  const [refreshing, setRefreshing] = useState(false);
  const [completingTaskId, setCompletingTaskId] = useState<string | null>(null);
  const [markingBillId, setMarkingBillId] = useState<string | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [notificationCenterOpen, setNotificationCenterOpen] = useState(false);
  const [notifications, setNotifications] = useState<NotificationItem[]>([]);
  const [notificationStatus, setNotificationStatus] = useState<NotificationCenterStatus>("loading");
  const [notificationError, setNotificationError] = useState("");
  const [notificationCursor, setNotificationCursor] = useState<string | null>(null);
  const [notificationUnreadCount, setNotificationUnreadCount] = useState(0);
  const [recurrenceSeries, setRecurrenceSeries] = useState<RecurrenceSeries[]>([]);
  const [recurrenceStatus, setRecurrenceStatus] = useState<RecurrenceStatus>("loading");
  const [recurrenceError, setRecurrenceError] = useState("");
  const [loadingMoreNotifications, setLoadingMoreNotifications] = useState(false);
  const [markingNotificationId, setMarkingNotificationId] = useState<string | null>(null);
  const [markingAllNotifications, setMarkingAllNotifications] = useState(false);
  const [toast, setToast] = useState<ToastState | null>(null);
  const [toastActionBusy, setToastActionBusy] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const agendaRangeFollowsTodayRef = useRef(true);
  const todayDateRef = useRef("");
  const rolloverRefreshBusyRef = useRef(false);
  const agendaRequestGateRef = useRef<ReturnType<typeof createLatestRequestGate> | null>(null);
  const paidHistoryRequestGateRef = useRef<ReturnType<typeof createLatestRequestGate> | null>(null);
  const notificationRequestGateRef = useRef<ReturnType<typeof createLatestRequestGate> | null>(null);
  const unreadRequestGateRef = useRef<ReturnType<typeof createLatestRequestGate> | null>(null);
  const notificationCenterOpenRef = useRef(false);
  const notificationCursorRef = useRef<string | null>(null);
  const notificationLoadedMoreRef = useRef(false);
  agendaRequestGateRef.current ??= createLatestRequestGate();
  paidHistoryRequestGateRef.current ??= createLatestRequestGate();
  notificationRequestGateRef.current ??= createLatestRequestGate();
  unreadRequestGateRef.current ??= createLatestRequestGate();

  const handleAuthError = useCallback(
    async (reason: unknown): Promise<boolean> => {
      if (reason instanceof ApiError && reason.status === 401) {
        await signOut();
        return true;
      }
      return false;
    },
    [signOut],
  );

  const executeAuthenticatedMutation = useCallback(async <T,>(
    operation: () => Promise<T>,
  ): Promise<{ ok: true; value: T } | { ok: false }> => {
    try {
      return { ok: true, value: await operation() };
    } catch (reason) {
      if (await handleAuthError(reason)) return { ok: false };
      throw reason;
    }
  }, [handleAuthError]);

  useEffect(() => {
    function syncViewFromHash() {
      const hash = window.location.hash;
      setActiveView(appViewFromHash(hash));
    }

    if (!window.location.hash) {
      window.history.replaceState(null, "", "#today");
    }
    syncViewFromHash();
    window.addEventListener("hashchange", syncViewFromHash);
    return () => window.removeEventListener("hashchange", syncViewFromHash);
  }, []);

  useEffect(() => {
    if (activeView !== "today" || window.location.hash !== "#quick-add") return;
    window.requestAnimationFrame(() => document.getElementById("quick-add")?.scrollIntoView());
  }, [activeView]);

  useEffect(() => {
    notificationCenterOpenRef.current = notificationCenterOpen;
  }, [notificationCenterOpen]);

  useEffect(() => {
    let active = true;
    const controller = new AbortController();

    async function loadWorkspace() {
      setStatus("loading");
      setWorkspaceError("");
      setDocumentsStatus("loading");
      setDocumentsError("");
      setRecurrenceStatus("loading");
      setRecurrenceError("");

      try {
        const nextProfile = await getProfile(token, controller.signal);
        if (!active) return;
        setProfile(nextProfile);

        if (!nextProfile.timezone) {
          setStatus("ready");
          return;
        }

        const nextToday = await getToday(token, controller.signal);
        if (!active) return;
        todayDateRef.current = nextToday.date;
        setToday(nextToday);
        setAgendaRange((current) => reconcileAgendaRangeForToday(
          current,
          agendaRangeFollowsTodayRef.current || !current.from,
          nextToday.date,
        ));

        try {
          const nextDocuments = await getDocuments(token, controller.signal);
          if (!active) return;
          setDocuments(nextDocuments);
          setDocumentsStatus("ready");
        } catch (reason) {
          if (!active) return;
          if (reason instanceof ApiError && reason.status === 401) throw reason;
          setDocumentsError(errorMessage(reason));
          setDocumentsStatus("error");
        }
        try {
          const nextSeries = await getRecurrenceSeries(token, controller.signal);
          if (!active) return;
          setRecurrenceSeries(nextSeries);
          setRecurrenceStatus("ready");
        } catch (reason) {
          if (!active) return;
          if (reason instanceof ApiError && reason.status === 401) throw reason;
          setRecurrenceError(errorMessage(reason));
          setRecurrenceStatus("error");
        }
        setStatus("ready");
      } catch (reason) {
        if (!active) return;
        if (await handleAuthError(reason)) return;
        setWorkspaceError(errorMessage(reason));
        setStatus("error");
      }
    }

    void loadWorkspace();
    return () => {
      active = false;
      controller.abort();
    };
  }, [handleAuthError, reloadKey, token]);

  useEffect(() => {
    if (!toast) return;
    const timer = window.setTimeout(() => setToast(null), 4500);
    return () => window.clearTimeout(timer);
  }, [toast]);

  const refreshToday = useCallback(async (): Promise<boolean> => {
    setRefreshing(true);
    try {
      const nextToday = await getToday(token);
      const previousDate = todayDateRef.current;
      todayDateRef.current = nextToday.date;
      setToday(nextToday);
      if (previousDate !== nextToday.date && agendaRangeFollowsTodayRef.current) {
        setAgendaRange((current) => reconcileAgendaRangeForToday(
          current,
          true,
          nextToday.date,
        ));
      }
      return true;
    } catch (reason) {
      if (await handleAuthError(reason)) return false;
      setToast({ message: errorMessage(reason), tone: "error" });
      return false;
    } finally {
      setRefreshing(false);
    }
  }, [handleAuthError, token]);

  const refreshAgenda = useCallback(async (
    range: { from: string; to: string },
    signal?: AbortSignal,
  ): Promise<boolean> => {
    if (!range.from || !range.to) return false;
    const gate = agendaRequestGateRef.current;
    if (!gate) return false;
    const requestId = gate.begin();
    setAgendaStatus("loading");
    setAgendaError("");
    try {
      const nextAgenda = await getAgenda(token, range, signal);
      if (!gate.isCurrent(requestId)) return true;
      setAgenda(nextAgenda);
      setAgendaStatus("ready");
      return true;
    } catch (reason) {
      if (!gate.isCurrent(requestId)) return true;
      if (reason instanceof DOMException && reason.name === "AbortError") return true;
      if (await handleAuthError(reason)) return false;
      setAgendaError(errorMessage(reason));
      setAgendaStatus("error");
      return false;
    }
  }, [handleAuthError, token]);

  useEffect(() => {
    if (activeView !== "agenda" || !agendaRange.from || !agendaRange.to) return;
    const controller = new AbortController();
    const timeout = window.setTimeout(() => {
      void refreshAgenda(agendaRange, controller.signal);
    }, 0);
    return () => {
      window.clearTimeout(timeout);
      controller.abort();
    };
  }, [activeView, agendaRange, refreshAgenda]);

  const refreshPaidHistory = useCallback(async (append = false): Promise<boolean> => {
    const gate = paidHistoryRequestGateRef.current;
    if (!gate) return false;
    const requestId = gate.begin();
    if (append) setLoadingMorePaid(true);
    else {
      setLoadingMorePaid(false);
      setPaidHistoryStatus("loading");
      setPaidHistoryError("");
    }
    try {
      const page = await getBills(token, {
        state: "paid",
        limit: 50,
        cursor: append ? paidHistoryCursor ?? undefined : undefined,
      });
      if (!gate.isCurrent(requestId)) return true;
      setPaidBills((current) => append ? mergeUniqueById(current, page.items) : page.items);
      setPaidHistoryCursor(page.nextCursor);
      setPaidHistoryStatus("ready");
      return true;
    } catch (reason) {
      if (!gate.isCurrent(requestId)) return true;
      if (await handleAuthError(reason)) return false;
      setPaidHistoryError(errorMessage(reason));
      setPaidHistoryStatus("error");
      return false;
    } finally {
      if (gate.isCurrent(requestId)) setLoadingMorePaid(false);
    }
  }, [handleAuthError, paidHistoryCursor, token]);

  const refreshUnreadCount = useCallback(async (signal?: AbortSignal): Promise<boolean> => {
    const gate = unreadRequestGateRef.current;
    if (!gate) return false;
    const requestId = gate.begin();
    try {
      const nextCount = await getNotificationUnreadCount(token, signal);
      if (!gate.isCurrent(requestId)) return true;
      setNotificationUnreadCount(nextCount);
      return true;
    } catch (reason) {
      if (!gate.isCurrent(requestId)) return true;
      if (reason instanceof DOMException && reason.name === "AbortError") return true;
      if (await handleAuthError(reason)) return false;
      return false;
    }
  }, [handleAuthError, token]);

  const refreshNotifications = useCallback(async (
    append = false,
    signal?: AbortSignal,
    background = false,
  ): Promise<boolean> => {
    const gate = notificationRequestGateRef.current;
    if (!gate) return false;
    if (append && !notificationCursorRef.current) return true;
    const requestId = gate.begin();
    const cursor = append ? notificationCursorRef.current ?? undefined : undefined;
    if (append) {
      setLoadingMoreNotifications(true);
      setNotificationError("");
    }
    else if (!background) {
      setLoadingMoreNotifications(false);
      setNotificationStatus("loading");
      setNotificationError("");
    }
    try {
      const page = await getNotifications(token, { limit: 50, cursor }, signal);
      if (!gate.isCurrent(requestId)) return true;
      setNotifications((current) => append
        ? mergeUniqueById(current, page.items)
        : mergeNewestPageById(current, page.items));
      if (append) notificationLoadedMoreRef.current = true;
      if (append || !notificationLoadedMoreRef.current) {
        notificationCursorRef.current = page.nextCursor;
        setNotificationCursor(page.nextCursor);
      }
      setNotificationUnreadCount(page.unreadCount);
      setNotificationStatus("ready");
      return true;
    } catch (reason) {
      if (!gate.isCurrent(requestId)) return true;
      if (reason instanceof DOMException && reason.name === "AbortError") return true;
      if (await handleAuthError(reason)) return false;
      if (background) return false;
      setNotificationError(errorMessage(reason));
      setNotificationStatus(append ? "ready" : "error");
      return false;
    } finally {
      if (gate.isCurrent(requestId)) setLoadingMoreNotifications(false);
    }
  }, [handleAuthError, token]);

  const refreshNotificationSurface = useCallback(async (): Promise<boolean> => {
    return notificationCenterOpenRef.current
      ? refreshNotifications(false, undefined, true)
      : refreshUnreadCount();
  }, [refreshNotifications, refreshUnreadCount]);

  const refreshDocuments = useCallback(async (): Promise<boolean> => {
    setDocumentsStatus("loading");
    setDocumentsError("");
    try {
      setDocuments(await getDocuments(token));
      setDocumentsStatus("ready");
      return true;
    } catch (reason) {
      if (await handleAuthError(reason)) return false;
      setDocumentsError(errorMessage(reason));
      setDocumentsStatus("error");
      return false;
    }
  }, [handleAuthError, token]);

  const refreshRecurrences = useCallback(async (): Promise<boolean> => {
    setRecurrenceStatus("loading");
    setRecurrenceError("");
    try {
      setRecurrenceSeries(await getRecurrenceSeries(token));
      setRecurrenceStatus("ready");
      return true;
    } catch (reason) {
      if (await handleAuthError(reason)) return false;
      setRecurrenceError(errorMessage(reason));
      setRecurrenceStatus("error");
      return false;
    }
  }, [handleAuthError, token]);

  const refreshDomainViews = useCallback(async (): Promise<boolean> => {
    const results = await Promise.all([
      refreshToday(),
      agendaRange.from && agendaRange.to
        ? refreshAgenda(agendaRange)
        : Promise.resolve(true),
      paidHistoryStatus === "ready"
        ? refreshPaidHistory(false)
        : Promise.resolve(true),
      refreshNotificationSurface(),
      refreshRecurrences(),
    ]);
    return results.every(Boolean);
  }, [
    agendaRange,
    paidHistoryStatus,
    refreshAgenda,
    refreshPaidHistory,
    refreshNotificationSurface,
    refreshRecurrences,
    refreshToday,
  ]);

  useEffect(() => {
    if (!profile?.timezone) return;
    const profileTimezone = profile.timezone;

    async function refreshAfterProfileDayChange() {
      if (rolloverRefreshBusyRef.current) return;
      const profileDate = getProfileDateOnly(new Date(), profileTimezone);
      if (!todayDateRef.current || profileDate === todayDateRef.current) return;

      rolloverRefreshBusyRef.current = true;
      try {
        await Promise.all([refreshDomainViews(), refreshDocuments()]);
      } finally {
        rolloverRefreshBusyRef.current = false;
      }
    }

    const interval = window.setInterval(() => void refreshAfterProfileDayChange(), 60_000);
    const handleVisibility = () => {
      if (document.visibilityState === "visible") void refreshAfterProfileDayChange();
    };
    window.addEventListener("focus", refreshAfterProfileDayChange);
    document.addEventListener("visibilitychange", handleVisibility);
    return () => {
      window.clearInterval(interval);
      window.removeEventListener("focus", refreshAfterProfileDayChange);
      document.removeEventListener("visibilitychange", handleVisibility);
    };
  }, [profile?.timezone, refreshDocuments, refreshDomainViews]);

  useEffect(() => {
    if (!profile?.timezone) return;
    let intervalId: number | null = null;
    let controller: AbortController | null = null;

    function poll() {
      if (document.visibilityState !== "visible") return;
      controller?.abort();
      controller = new AbortController();
      if (notificationCenterOpenRef.current) {
        void refreshNotifications(false, controller.signal, true);
      } else {
        void refreshUnreadCount(controller.signal);
      }
    }

    function stop() {
      if (intervalId !== null) window.clearInterval(intervalId);
      intervalId = null;
      controller?.abort();
      controller = null;
    }

    function start() {
      if (document.visibilityState !== "visible" || intervalId !== null) return;
      poll();
      intervalId = window.setInterval(poll, 60_000);
    }

    function handleVisibility() {
      if (document.visibilityState === "visible") start();
      else stop();
    }

    document.addEventListener("visibilitychange", handleVisibility);
    start();
    return () => {
      document.removeEventListener("visibilitychange", handleVisibility);
      stop();
    };
  }, [profile?.timezone, refreshNotifications, refreshUnreadCount]);

  async function saveTimezone(timezone: string) {
    const updateResult = await executeAuthenticatedMutation(() => updateProfile(token, timezone));
    if (!updateResult.ok) return;
    const updated = updateResult.value;
    const nextProfile = {
      ...updated,
      timezone: updated.timezone || timezone,
    };
    setProfile(nextProfile);
    const todayResult = await executeAuthenticatedMutation(() => getToday(token));
    if (!todayResult.ok) return;
    const nextToday = todayResult.value;
    todayDateRef.current = nextToday.date;
    setToday(nextToday);
    agendaRangeFollowsTodayRef.current = true;
    setAgendaRange(getDefaultAgendaRange(nextToday.date));
    setAgenda(null);
    setAgendaStatus("idle");
    setPaidBills([]);
    setPaidHistoryCursor(null);
    setPaidHistoryStatus("idle");
    await refreshDocuments();
    setSettingsOpen(false);
    setStatus("ready");
    setToast({ message: "Zona waktu berhasil diperbarui.", tone: "success" });
  }

  const handleLoadReminders = useCallback(async (
    sourceKind: ReminderSourceKind,
    sourceId: string,
    signal?: AbortSignal,
  ): Promise<Reminder[]> => {
    try {
      return await getReminders(token, sourceKind, sourceId, signal);
    } catch (reason) {
      if (reason instanceof DOMException && reason.name === "AbortError") throw reason;
      if (await handleAuthError(reason)) return [];
      throw reason;
    }
  }, [handleAuthError, token]);

  async function handleCreateReminder(input: CreateReminderInput): Promise<Reminder | null> {
    const result = await executeAuthenticatedMutation(() => createReminder(token, input));
    if (!result.ok) return null;
    await refreshNotificationSurface();
    setToast({ message: "Pengingat dijadwalkan.", tone: "success" });
    return result.value;
  }

  async function handleUpdateReminder(
    reminderId: string,
    input: UpdateReminderInput,
  ): Promise<Reminder | null> {
    const result = await executeAuthenticatedMutation(() => updateReminder(token, reminderId, input));
    if (!result.ok) return null;
    await refreshNotificationSurface();
    setToast({ message: "Pengingat diperbarui.", tone: "success" });
    return result.value;
  }

  async function handleDeleteReminder(reminderId: string): Promise<boolean> {
    const result = await executeAuthenticatedMutation(() => deleteReminder(token, reminderId));
    if (!result.ok) return false;
    await refreshNotificationSurface();
    setToast({ message: "Pengingat dihapus.", tone: "success" });
    return true;
  }

  async function handleUpdateRecurrence(seriesId: string, input: RecurrenceInput): Promise<boolean> {
    const result = await executeAuthenticatedMutation(() => updateRecurrenceSeries(token, seriesId, input));
    if (!result.ok) return false;
    setRecurrenceSeries((current) => current.map((item) => item.id === result.value.id ? result.value : item));
    const refreshed = await refreshDomainViews();
    setToast(refreshed
      ? { message: "Aturan pengulangan diperbarui.", tone: "success" }
      : { message: "Aturan tersimpan, tetapi tampilan belum sepenuhnya diperbarui.", tone: "error" });
    return true;
  }

  async function handleStopRecurrence(seriesId: string): Promise<boolean> {
    const result = await executeAuthenticatedMutation(() => stopRecurrenceSeries(token, seriesId));
    if (!result.ok) return false;
    const refreshed = await refreshDomainViews();
    setToast(refreshed
      ? { message: "Seri dihentikan. Riwayat yang sudah selesai atau lunas tetap tersimpan.", tone: "success" }
      : { message: "Seri dihentikan, tetapi tampilan belum sepenuhnya diperbarui.", tone: "error" });
    return true;
  }

  async function handleMarkNotificationRead(notificationId: string) {
    notificationRequestGateRef.current?.begin();
    unreadRequestGateRef.current?.begin();
    setMarkingNotificationId(notificationId);
    try {
      const result = await executeAuthenticatedMutation(() => markNotificationRead(token, notificationId));
      if (!result.ok) return;
      notificationRequestGateRef.current?.begin();
      unreadRequestGateRef.current?.begin();
      setNotifications((current) => current.map((notification) => (
        notification.id === result.value.item.id ? result.value.item : notification
      )));
      setNotificationUnreadCount(result.value.unreadCount);
    } catch (reason) {
      setToast({ message: errorMessage(reason), tone: "error" });
    } finally {
      setMarkingNotificationId(null);
    }
  }

  async function handleMarkAllNotificationsRead() {
    notificationRequestGateRef.current?.begin();
    unreadRequestGateRef.current?.begin();
    setMarkingAllNotifications(true);
    try {
      const result = await executeAuthenticatedMutation(() => markAllNotificationsRead(token));
      if (!result.ok) return;
      notificationRequestGateRef.current?.begin();
      unreadRequestGateRef.current?.begin();
      setNotificationUnreadCount(result.value.unreadCount);
      const readAt = new Date().toISOString();
      setNotifications((current) => current.map((notification) => (
        notification.readAt ? notification : { ...notification, readAt }
      )));
      const refreshed = await refreshNotifications(false, undefined, true);
      if (!refreshed) {
        setToast({
          message: "Status baca tersimpan, tetapi daftar belum dapat diperbarui.",
          tone: "error",
        });
      }
    } catch (reason) {
      setToast({ message: errorMessage(reason), tone: "error" });
    } finally {
      setMarkingAllNotifications(false);
    }
  }

  async function handleCreateTask(input: CreateTaskInput) {
    const result = await executeAuthenticatedMutation(() => createTask(token, input));
    if (!result.ok) return;
    const refreshed = await refreshDomainViews();
    setToast(
      refreshed
        ? { message: "Tugas disimpan.", tone: "success" }
        : {
            message: "Tugas tersimpan, tetapi Today belum dapat diperbarui. Gunakan tombol Perbarui.",
            tone: "error",
          },
    );
  }

  async function handleParseSmartCapture(text: string): Promise<SmartCaptureResult> {
    try {
      return await parseSmartCapture(token, text);
    } catch (reason) {
      await handleAuthError(reason);
      throw reason;
    }
  }

  async function handleCreateEvent(input: CreateEventInput) {
    const result = await executeAuthenticatedMutation(() => createEvent(token, input));
    if (!result.ok) return;
    const refreshed = await refreshDomainViews();
    setToast(
      refreshed
        ? { message: "Jadwal disimpan.", tone: "success" }
        : {
            message: "Jadwal tersimpan, tetapi Today belum dapat diperbarui. Gunakan tombol Perbarui.",
            tone: "error",
          },
    );
  }

  async function handleCreateBill(input: CreateBillInput) {
    const result = await executeAuthenticatedMutation(() => createBill(token, input));
    if (!result.ok) return;
    const refreshed = await refreshDomainViews();
    setToast(
      refreshed
        ? { message: "Tagihan disimpan.", tone: "success" }
        : {
            message: "Tagihan tersimpan, tetapi Today belum dapat diperbarui. Gunakan tombol Perbarui.",
            tone: "error",
          },
    );
  }

  async function handleCreateDocument(input: CreateDocumentInput) {
    const result = await executeAuthenticatedMutation(() => createDocument(token, input));
    if (!result.ok) return;
    const [viewsRefreshed, documentsRefreshed] = await Promise.all([
      refreshDomainViews(),
      refreshDocuments(),
    ]);
    setToast(
      viewsRefreshed && documentsRefreshed
        ? { message: "Metadata dokumen disimpan.", tone: "success" }
        : {
            message: "Metadata dokumen tersimpan, tetapi tampilan belum sepenuhnya diperbarui. Gunakan tombol Perbarui atau Coba lagi.",
            tone: "error",
          },
    );
  }

  async function handleUpdateDocument(documentId: string, input: UpdateDocumentInput) {
    const result = await executeAuthenticatedMutation(() => updateDocument(token, documentId, input));
    if (!result.ok) return;
    const [viewsRefreshed, documentsRefreshed] = await Promise.all([
      refreshDomainViews(),
      refreshDocuments(),
    ]);
    setToast(
      viewsRefreshed && documentsRefreshed
        ? { message: "Metadata dokumen diperbarui.", tone: "success" }
        : {
            message: "Perubahan tersimpan, tetapi tampilan belum sepenuhnya diperbarui.",
            tone: "error",
          },
    );
  }

  async function handleDeleteDocument(documentId: string) {
    const result = await executeAuthenticatedMutation(() => deleteDocument(token, documentId));
    if (!result.ok) return;
    const [viewsRefreshed, documentsRefreshed] = await Promise.all([
      refreshDomainViews(),
      refreshDocuments(),
    ]);
    setToast(
      viewsRefreshed && documentsRefreshed
        ? { message: "Metadata dokumen dihapus.", tone: "success" }
        : {
            message: "Metadata dihapus, tetapi tampilan belum sepenuhnya diperbarui.",
            tone: "error",
          },
    );
  }

  async function handleUpdateTask(taskId: string, input: UpdateTaskInput) {
    const result = await executeAuthenticatedMutation(() => updateTask(token, taskId, input));
    if (!result.ok) return;
    const refreshed = await refreshDomainViews();
    setToast(refreshed
      ? { message: "Tugas diperbarui.", tone: "success" }
      : { message: "Tugas tersimpan, tetapi tampilan belum sepenuhnya diperbarui.", tone: "error" });
  }

  async function handleUpdateEvent(eventId: string, input: UpdateEventInput) {
    const result = await executeAuthenticatedMutation(() => updateEvent(token, eventId, input));
    if (!result.ok) return;
    const refreshed = await refreshDomainViews();
    setToast(refreshed
      ? { message: "Jadwal diperbarui.", tone: "success" }
      : { message: "Jadwal tersimpan, tetapi tampilan belum sepenuhnya diperbarui.", tone: "error" });
  }

  async function handleUpdateBill(billId: string, input: UpdateBillInput) {
    const result = await executeAuthenticatedMutation(() => updateBill(token, billId, input));
    if (!result.ok) return;
    const refreshed = await refreshDomainViews();
    setToast(refreshed
      ? { message: "Tagihan diperbarui.", tone: "success" }
      : { message: "Tagihan tersimpan, tetapi tampilan belum sepenuhnya diperbarui.", tone: "error" });
  }

  async function handleDeleteCorrectable(kind: "task" | "event" | "bill", id: string) {
    const result = await executeAuthenticatedMutation(async () => {
      if (kind === "task") await deleteTask(token, id);
      else if (kind === "event") await deleteEvent(token, id);
      else await deleteBill(token, id);
    });
    if (!result.ok) return;
    const refreshed = await refreshDomainViews();
    setToast(refreshed
      ? { message: `${kind === "task" ? "Tugas" : kind === "event" ? "Jadwal" : "Tagihan"} dihapus.`, tone: "success" }
      : { message: "Item dihapus, tetapi tampilan belum sepenuhnya diperbarui.", tone: "error" });
  }

  async function handleUncompleteTask(taskId: string) {
    const result = await executeAuthenticatedMutation(() => uncompleteTask(token, taskId));
    if (!result.ok) return;
    const refreshed = await refreshDomainViews();
    setToast(refreshed
      ? { message: "Tugas dikembalikan ke status terbuka.", tone: "success" }
      : { message: "Status tugas tersimpan, tetapi tampilan belum sepenuhnya diperbarui.", tone: "error" });
  }

  async function handleMarkBillUnpaid(billId: string) {
    const result = await executeAuthenticatedMutation(() => markBillUnpaid(token, billId));
    if (!result.ok) return;
    const refreshed = await refreshDomainViews();
    setToast(refreshed
      ? { message: "Tagihan ditandai belum lunas.", tone: "success" }
      : { message: "Status tagihan tersimpan, tetapi tampilan belum sepenuhnya diperbarui.", tone: "error" });
  }

  async function handleCompleteTask(taskId: string) {
    setCompletingTaskId(taskId);
    try {
      await completeTask(token, taskId);
      const refreshed = await refreshDomainViews();
      setToast(
        refreshed
          ? {
              message: "Tugas selesai. Kerja bagus.",
              tone: "success",
              action: { label: "Batalkan", run: () => handleUncompleteTask(taskId) },
            }
          : {
              message: "Status selesai tersimpan, tetapi Today belum dapat diperbarui. Gunakan tombol Perbarui.",
              tone: "error",
            },
      );
    } catch (reason) {
      if (await handleAuthError(reason)) return;
      setToast({ message: errorMessage(reason), tone: "error" });
    } finally {
      setCompletingTaskId(null);
    }
  }

  async function handleMarkBillPaid(billId: string) {
    setMarkingBillId(billId);
    try {
      await markBillPaid(token, billId);
      const refreshed = await refreshDomainViews();
      setToast(
        refreshed
          ? {
              message: "Tagihan ditandai lunas.",
              tone: "success",
              action: { label: "Batalkan", run: () => handleMarkBillUnpaid(billId) },
            }
          : {
              message: "Status lunas tersimpan, tetapi Today belum dapat diperbarui. Gunakan tombol Perbarui.",
              tone: "error",
            },
      );
    } catch (reason) {
      if (await handleAuthError(reason)) return;
      setToast({ message: errorMessage(reason), tone: "error" });
    } finally {
      setMarkingBillId(null);
    }
  }

  async function handleDeleteProfileData(confirmation: string) {
    await deleteProfileData(token, confirmation);
    setSettingsOpen(false);
    await signOut();
  }

  if (status === "loading") return <AppLoadingScreen />;

  if (status === "error") {
    return (
      <WorkspaceError
        message={workspaceError}
        onRetry={() => setReloadKey((current) => current + 1)}
        onSignOut={() => void signOut()}
      />
    );
  }

  if (!profile?.timezone) {
    return (
      <TimezoneForm
        mode="onboarding"
        onSave={saveTimezone}
        onSignOut={() => void signOut()}
      />
    );
  }

  if (!today) return <AppLoadingScreen />;

  return (
    <>
      <AppShell
        activeView={activeView}
        email={email}
        notificationUnreadCount={notificationUnreadCount}
        onOpenNotifications={() => {
          setNotificationCenterOpen(true);
          void refreshNotifications(false);
        }}
        onOpenSettings={() => setSettingsOpen(true)}
        onSignOut={() => void signOut()}
        timezone={profile.timezone}
      >
        {activeView === "today" ? (
          <>
          <TodayView
            completingTaskId={completingTaskId}
            markingBillId={markingBillId}
            onComplete={handleCompleteTask}
            onCreateBill={handleCreateBill}
            onCreateDocument={handleCreateDocument}
            onCreateEvent={handleCreateEvent}
            onCreateTask={handleCreateTask}
            onParseSmartCapture={handleParseSmartCapture}
            onMarkBillPaid={handleMarkBillPaid}
            onEdit={setCorrectionItem}
            onRefresh={async () => {
              await Promise.all([refreshDomainViews(), refreshDocuments()]);
            }}
            refreshing={refreshing}
            timezone={profile.timezone}
            today={today}
          />
          <RecurrenceManager
            error={recurrenceError}
            items={recurrenceSeries}
            onRetry={refreshRecurrences}
            onStop={handleStopRecurrence}
            onUpdate={handleUpdateRecurrence}
            status={recurrenceStatus}
          />
          <DocumentsManager
            documents={documents}
            error={documentsError}
            onCreateReminder={handleCreateReminder}
            onDelete={handleDeleteDocument}
            onDeleteReminder={handleDeleteReminder}
            onLoadReminders={handleLoadReminders}
            onRetry={refreshDocuments}
            onUpdate={handleUpdateDocument}
            onUpdateReminder={handleUpdateReminder}
            status={documentsStatus}
            timezone={profile.timezone}
          />
          </>
        ) : (
          <AgendaView
            agenda={agenda}
            error={agendaError}
            loadingMorePaid={loadingMorePaid}
            onEdit={setCorrectionItem}
            onJumpDate={(dateOnly) => {
              agendaRangeFollowsTodayRef.current = false;
              setAgendaRange(getAgendaRangeFrom(dateOnly));
            }}
            onLoadMorePaid={() => refreshPaidHistory(true)}
            onLoadPaidHistory={() => refreshPaidHistory(false)}
            onRetry={() => refreshAgenda(agendaRange)}
            paidBills={paidBills}
            paidHistoryError={paidHistoryError}
            paidHistoryHasMore={Boolean(paidHistoryCursor)}
            paidHistoryStatus={paidHistoryStatus}
            range={agendaRange}
            status={agendaStatus}
            timezone={profile.timezone}
          />
        )}
      </AppShell>

      {correctionItem ? (
        <CorrectionSheet
          item={correctionItem}
          key={`${correctionItem.kind}-${correctionItem.id}`}
          onClose={() => setCorrectionItem(null)}
          onCreateReminder={handleCreateReminder}
          onDelete={handleDeleteCorrectable}
          onDeleteReminder={handleDeleteReminder}
          onLoadReminders={handleLoadReminders}
          onMarkUnpaid={handleMarkBillUnpaid}
          onSaveBill={handleUpdateBill}
          onSaveEvent={handleUpdateEvent}
          onSaveTask={handleUpdateTask}
          onUncomplete={handleUncompleteTask}
          onUpdateReminder={handleUpdateReminder}
          timezone={profile.timezone}
        />
      ) : null}

      {notificationCenterOpen ? (
        <NotificationCenter
          error={notificationError}
          hasMore={Boolean(notificationCursor)}
          items={notifications}
          loadingMore={loadingMoreNotifications}
          markingAll={markingAllNotifications}
          markingId={markingNotificationId}
          onClose={() => setNotificationCenterOpen(false)}
          onLoadMore={() => refreshNotifications(true)}
          onMarkAllRead={handleMarkAllNotificationsRead}
          onMarkRead={handleMarkNotificationRead}
          onRetry={() => refreshNotifications(false)}
          status={notificationStatus}
          timezone={profile.timezone}
          unreadCount={notificationUnreadCount}
        />
      ) : null}

      {settingsOpen ? (
        <TimezoneForm
          initialTimezone={profile.timezone}
          mode="dialog"
          onCancel={() => setSettingsOpen(false)}
          onDeleteData={handleDeleteProfileData}
          onSave={saveTimezone}
        />
      ) : null}

      {toast ? (
        <div className={`toast toast-${toast.tone}`} role={toast.tone === "error" ? "alert" : "status"}>
          {toast.tone === "success" ? <CheckCircle2 size={19} aria-hidden="true" /> : <AlertCircle size={19} aria-hidden="true" />}
          <span>{toast.message}</span>
          {toast.action ? (
            <button
              className="toast-action"
              disabled={toastActionBusy}
              onClick={async () => {
                setToastActionBusy(true);
                try {
                  await toast.action?.run();
                } catch (reason) {
                  if (!(await handleAuthError(reason))) {
                    setToast({ message: errorMessage(reason), tone: "error" });
                  }
                } finally {
                  setToastActionBusy(false);
                }
              }}
              type="button"
            >
              {toastActionBusy ? "Membatalkan…" : toast.action.label}
            </button>
          ) : null}
          <button className="toast-close" onClick={() => setToast(null)} type="button">
            <X size={17} aria-hidden="true" /><span className="sr-only">Tutup pemberitahuan</span>
          </button>
        </div>
      ) : null}
    </>
  );
}

export function LifeHubClient() {
  const { status, accessToken } = useAuth();

  if (status === "loading") return <AppLoadingScreen />;
  if (status === "signed-out" || !accessToken) return <LoginScreen />;
  return <AuthenticatedWorkspace token={accessToken} />;
}
