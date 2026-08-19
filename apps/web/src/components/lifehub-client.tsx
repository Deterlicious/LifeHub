"use client";

import { AlertCircle, CheckCircle2, X } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import { AppShell } from "@/components/app-shell";
import { useAuth } from "@/components/auth/auth-provider";
import { LoginScreen } from "@/components/auth/login-screen";
import { AppLoadingScreen, WorkspaceError } from "@/components/status-screens";
import { TimezoneForm } from "@/components/timezone-form";
import { TodayView } from "@/components/today/today-view";
import {
  ApiError,
  completeTask,
  createTask,
  getProfile,
  getToday,
  updateProfile,
} from "@/lib/api/client";
import type { CreateTaskInput, Profile, Today } from "@/lib/api/types";

type WorkspaceStatus = "loading" | "ready" | "error";

interface ToastState {
  message: string;
  tone: "success" | "error";
}

function errorMessage(reason: unknown): string {
  return reason instanceof Error ? reason.message : "Terjadi kendala yang tidak terduga.";
}

function AuthenticatedWorkspace({ token }: { token: string }) {
  const { email, signOut } = useAuth();
  const [status, setStatus] = useState<WorkspaceStatus>("loading");
  const [profile, setProfile] = useState<Profile | null>(null);
  const [today, setToday] = useState<Today | null>(null);
  const [workspaceError, setWorkspaceError] = useState<string>("");
  const [refreshing, setRefreshing] = useState(false);
  const [completingTaskId, setCompletingTaskId] = useState<string | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [toast, setToast] = useState<ToastState | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

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

  useEffect(() => {
    let active = true;
    const controller = new AbortController();

    async function loadWorkspace() {
      setStatus("loading");
      setWorkspaceError("");

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
        setToday(nextToday);
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
      setToday(await getToday(token));
      return true;
    } catch (reason) {
      if (await handleAuthError(reason)) return false;
      setToast({ message: errorMessage(reason), tone: "error" });
      return false;
    } finally {
      setRefreshing(false);
    }
  }, [handleAuthError, token]);

  async function saveTimezone(timezone: string) {
    const updated = await updateProfile(token, timezone);
    const nextProfile = {
      ...updated,
      timezone: updated.timezone || timezone,
    };
    setProfile(nextProfile);
    setToday(await getToday(token));
    setSettingsOpen(false);
    setStatus("ready");
    setToast({ message: "Zona waktu berhasil diperbarui.", tone: "success" });
  }

  async function handleCreateTask(input: CreateTaskInput) {
    await createTask(token, input);
    const refreshed = await refreshToday();
    setToast(
      refreshed
        ? { message: "Tugas ditambahkan ke Today.", tone: "success" }
        : {
            message: "Tugas tersimpan, tetapi Today belum dapat diperbarui. Gunakan tombol Perbarui.",
            tone: "error",
          },
    );
  }

  async function handleCompleteTask(taskId: string) {
    setCompletingTaskId(taskId);
    try {
      await completeTask(token, taskId);
      const refreshed = await refreshToday();
      setToast(
        refreshed
          ? { message: "Tugas selesai. Kerja bagus.", tone: "success" }
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
        email={email}
        onOpenSettings={() => setSettingsOpen(true)}
        onSignOut={() => void signOut()}
        timezone={profile.timezone}
      >
        <TodayView
          completingTaskId={completingTaskId}
          onComplete={handleCompleteTask}
          onCreate={handleCreateTask}
          onRefresh={async () => {
            await refreshToday();
          }}
          refreshing={refreshing}
          timezone={profile.timezone}
          today={today}
        />
      </AppShell>

      {settingsOpen ? (
        <TimezoneForm
          initialTimezone={profile.timezone}
          mode="dialog"
          onCancel={() => setSettingsOpen(false)}
          onSave={saveTimezone}
        />
      ) : null}

      {toast ? (
        <div className={`toast toast-${toast.tone}`} role={toast.tone === "error" ? "alert" : "status"}>
          {toast.tone === "success" ? <CheckCircle2 size={19} aria-hidden="true" /> : <AlertCircle size={19} aria-hidden="true" />}
          <span>{toast.message}</span>
          <button onClick={() => setToast(null)} type="button">
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
