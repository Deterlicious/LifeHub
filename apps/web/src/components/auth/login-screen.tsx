"use client";

import { ArrowRight, Check, Leaf, LockKeyhole, ShieldCheck, Sparkles } from "lucide-react";
import { useState, type FormEvent } from "react";

import { useAuth } from "@/components/auth/auth-provider";

export function LoginScreen() {
  const { mode, configurationError, signIn, signUp } = useAuth();
  const [intent, setIntent] = useState<"sign-in" | "sign-up">("sign-in");
  const [email, setEmail] = useState(
    mode === "development" ? "demo@lifehub.local" : "",
  );
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError(null);
    setNotice(null);

    try {
      if (intent === "sign-up" && mode === "supabase") {
        setNotice(await signUp(email.trim(), password));
      } else {
        await signIn(email.trim(), password || undefined);
      }
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Belum dapat masuk ke LifeHub.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="auth-layout">
      <section className="auth-story" aria-labelledby="auth-heading">
        <a className="brand brand-on-dark" href="#auth-heading" aria-label="LifeHub">
          <span className="brand-mark" aria-hidden="true">
            <Leaf size={21} strokeWidth={2.2} />
          </span>
          <span>LifeHub</span>
        </a>

        <div className="auth-story-copy">
          <p className="eyebrow eyebrow-light">Calm command center</p>
          <h1 id="auth-heading">Jangan simpan semuanya di kepala.</h1>
          <p>
            Tugas, jadwal, tagihan, dan dokumen penting—dirangkum menjadi satu
            pandangan yang tenang untuk hari ini.
          </p>
        </div>

        <ul className="auth-benefits" aria-label="Manfaat LifeHub">
          <li><Check size={17} aria-hidden="true" /> Lihat yang paling penting lebih dulu</li>
          <li><Check size={17} aria-hidden="true" /> Tangkap tugas tanpa kehilangan fokus</li>
          <li><Check size={17} aria-hidden="true" /> Waktu mengikuti zona waktumu</li>
        </ul>
      </section>

      <section className="auth-panel" aria-label="Masuk ke LifeHub">
        <div className="auth-card">
          {mode === "development" ? (
            <div className="dev-mode-banner" role="note">
              <Sparkles size={16} aria-hidden="true" />
              <span><strong>Mode dev lokal</strong> · sesi uji, bukan login produksi</span>
            </div>
          ) : null}

          <div className="auth-card-heading">
            <span className="auth-icon" aria-hidden="true"><LockKeyhole size={22} /></span>
            <div>
              <p className="eyebrow">Mulai hari dengan jernih</p>
              <h2>{intent === "sign-up" ? "Buat akunmu" : "Selamat datang kembali"}</h2>
              <p>
                {mode === "development"
                  ? "Gunakan email uji untuk membuka slice lokal LifeHub."
                  : "Masuk aman dengan akun Supabase-mu."}
              </p>
            </div>
          </div>

          {configurationError ? (
            <div className="inline-alert inline-alert-error" role="alert">{configurationError}</div>
          ) : null}
          {error ? <div className="inline-alert inline-alert-error" role="alert">{error}</div> : null}
          {notice ? <div className="inline-alert inline-alert-success" role="status">{notice}</div> : null}

          <form className="auth-form" onSubmit={handleSubmit}>
            <label className="field">
              <span>Email</span>
              <input
                autoComplete="email"
                disabled={submitting || Boolean(configurationError)}
                inputMode="email"
                onChange={(event) => setEmail(event.target.value)}
                placeholder="nama@email.com"
                required
                type="email"
                value={email}
              />
            </label>

            {mode === "supabase" ? (
              <label className="field">
                <span>Kata sandi</span>
                <input
                  autoComplete={intent === "sign-up" ? "new-password" : "current-password"}
                  disabled={submitting || Boolean(configurationError)}
                  minLength={8}
                  onChange={(event) => setPassword(event.target.value)}
                  required
                  type="password"
                  value={password}
                />
              </label>
            ) : null}

            <button
              className="button button-primary button-full"
              disabled={submitting || Boolean(configurationError)}
              type="submit"
            >
              {submitting
                ? "Menyiapkan harimu…"
                : intent === "sign-up"
                  ? "Buat akun"
                  : "Masuk ke LifeHub"}
              {!submitting ? <ArrowRight size={18} aria-hidden="true" /> : null}
            </button>
          </form>

          {mode === "supabase" ? (
            <button
              className="text-button auth-switch"
              onClick={() => {
                setIntent((current) => (current === "sign-in" ? "sign-up" : "sign-in"));
                setError(null);
                setNotice(null);
              }}
              type="button"
            >
              {intent === "sign-in" ? "Belum punya akun? Buat akun" : "Sudah punya akun? Masuk"}
            </button>
          ) : null}

          <p className="auth-security-note">
            <ShieldCheck size={16} aria-hidden="true" />
            Data pribadimu diteruskan ke API Go dengan access token terverifikasi.
          </p>
        </div>
      </section>
    </main>
  );
}
