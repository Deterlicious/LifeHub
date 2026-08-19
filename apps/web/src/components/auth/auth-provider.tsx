"use client";

import { createClient, type Session, type SupabaseClient } from "@supabase/supabase-js";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import { createDevSession } from "@/lib/api/client";
import { getAuthMode, getSupabaseConfig, type AuthMode } from "@/lib/config";

const DEV_TOKEN_KEY = "lifehub.dev.access-token";
const DEV_EMAIL_KEY = "lifehub.dev.email";

type AuthStatus = "loading" | "signed-out" | "authenticated";

interface AuthContextValue {
  mode: AuthMode;
  status: AuthStatus;
  accessToken: string | null;
  email: string | null;
  configurationError: string | null;
  signIn: (email: string, password?: string) => Promise<void>;
  signUp: (email: string, password: string) => Promise<string | null>;
  signOut: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

let supabaseClient: SupabaseClient | null = null;

function getBrowserSupabaseClient(): SupabaseClient | null {
  const config = getSupabaseConfig();
  if (!config) return null;

  supabaseClient ??= createClient(config.url, config.publishableKey, {
    auth: {
      persistSession: true,
      autoRefreshToken: true,
      detectSessionInUrl: true,
      flowType: "pkce",
    },
  });

  return supabaseClient;
}

function sessionEmail(session: Session | null): string | null {
  return session?.user.email ?? null;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const mode = getAuthMode();
  const [status, setStatus] = useState<AuthStatus>("loading");
  const [accessToken, setAccessToken] = useState<string | null>(null);
  const [email, setEmail] = useState<string | null>(null);
  const configurationError =
    mode === "supabase" && !getSupabaseConfig()
      ? "Supabase belum dikonfigurasi. Isi URL dan publishable key publik untuk masuk."
      : null;

  const applySession = useCallback((session: Session | null) => {
    setAccessToken(session?.access_token ?? null);
    setEmail(sessionEmail(session));
    setStatus(session ? "authenticated" : "signed-out");
  }, []);

  useEffect(() => {
    if (mode === "development") {
      let active = true;
      window.queueMicrotask(() => {
        if (!active) return;
        const storedToken = window.localStorage.getItem(DEV_TOKEN_KEY);
        const storedEmail = window.localStorage.getItem(DEV_EMAIL_KEY);
        setAccessToken(storedToken);
        setEmail(storedEmail);
        setStatus(storedToken ? "authenticated" : "signed-out");
      });
      return () => {
        active = false;
      };
    }

    const client = getBrowserSupabaseClient();
    if (!client) {
      let active = true;
      window.queueMicrotask(() => {
        if (active) setStatus("signed-out");
      });
      return () => {
        active = false;
      };
    }

    let active = true;
    void client.auth.getSession().then(({ data, error }) => {
      if (!active) return;
      applySession(error ? null : data.session);
    });

    const { data: subscription } = client.auth.onAuthStateChange((_event, session) => {
      if (active) applySession(session);
    });

    return () => {
      active = false;
      subscription.subscription.unsubscribe();
    };
  }, [applySession, mode]);

  const signIn = useCallback(
    async (nextEmail: string, password?: string) => {
      if (mode === "development") {
        const session = await createDevSession(nextEmail);
        window.localStorage.setItem(DEV_TOKEN_KEY, session.accessToken);
        window.localStorage.setItem(DEV_EMAIL_KEY, nextEmail);
        setAccessToken(session.accessToken);
        setEmail(nextEmail);
        setStatus("authenticated");
        return;
      }

      const client = getBrowserSupabaseClient();
      if (!client) throw new Error("Supabase belum dikonfigurasi.");
      if (!password) throw new Error("Kata sandi wajib diisi.");

      const { data, error } = await client.auth.signInWithPassword({
        email: nextEmail,
        password,
      });

      if (error) throw new Error(error.message);
      applySession(data.session);
    },
    [applySession, mode],
  );

  const signUp = useCallback(
    async (nextEmail: string, password: string) => {
      if (mode === "development") {
        await signIn(nextEmail);
        return null;
      }

      const client = getBrowserSupabaseClient();
      if (!client) throw new Error("Supabase belum dikonfigurasi.");

      const { data, error } = await client.auth.signUp({
        email: nextEmail,
        password,
      });

      if (error) throw new Error(error.message);
      applySession(data.session);

      return data.session
        ? null
        : "Periksa email untuk mengonfirmasi akun, lalu kembali ke halaman ini.";
    },
    [applySession, mode, signIn],
  );

  const signOut = useCallback(async () => {
    window.localStorage.removeItem(DEV_TOKEN_KEY);
    window.localStorage.removeItem(DEV_EMAIL_KEY);

    if (mode === "supabase") {
      const client = getBrowserSupabaseClient();
      if (client) await client.auth.signOut();
    }

    setAccessToken(null);
    setEmail(null);
    setStatus("signed-out");
  }, [mode]);

  const value = useMemo<AuthContextValue>(
    () => ({
      mode,
      status,
      accessToken,
      email,
      configurationError,
      signIn,
      signUp,
      signOut,
    }),
    [accessToken, configurationError, email, mode, signIn, signOut, signUp, status],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) throw new Error("useAuth harus digunakan di dalam AuthProvider.");
  return context;
}
