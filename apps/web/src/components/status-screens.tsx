import { AlertTriangle, Leaf, LoaderCircle, RefreshCw } from "lucide-react";

export function AppLoadingScreen() {
  return (
    <main className="centered-status" aria-busy="true" aria-live="polite">
      <span className="brand"><span className="brand-mark" aria-hidden="true"><Leaf size={20} /></span> LifeHub</span>
      <div className="loading-orbit" aria-hidden="true"><LoaderCircle size={26} /></div>
      <h1>Menyiapkan harimu…</h1>
      <p>Kami sedang menyusun hal yang penting untuk Today.</p>
    </main>
  );
}

export function WorkspaceError({ message, onRetry, onSignOut }: {
  message: string;
  onRetry: () => void;
  onSignOut: () => void;
}) {
  return (
    <main className="centered-status">
      <span className="status-icon status-icon-error" aria-hidden="true"><AlertTriangle size={26} /></span>
      <h1>Today belum dapat dimuat</h1>
      <p>{message}</p>
      <div className="status-actions">
        <button className="button button-primary" onClick={onRetry} type="button">
          <RefreshCw size={17} aria-hidden="true" /> Coba lagi
        </button>
        <button className="quiet-button" onClick={onSignOut} type="button">Keluar</button>
      </div>
    </main>
  );
}
