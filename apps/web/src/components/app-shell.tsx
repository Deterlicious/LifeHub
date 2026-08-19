"use client";

import {
  CircleUserRound,
  Leaf,
  LogOut,
  Plus,
  Settings2,
  SunMedium,
} from "lucide-react";
import type { ReactNode } from "react";

interface AppShellProps {
  children: ReactNode;
  email: string | null;
  timezone: string;
  onOpenSettings: () => void;
  onSignOut: () => void;
}

export function AppShell({
  children,
  email,
  timezone,
  onOpenSettings,
  onSignOut,
}: AppShellProps) {
  return (
    <div className="app-layout">
      <a className="skip-link" href="#main-content">Lewati ke konten utama</a>

      <aside className="sidebar" aria-label="Navigasi utama">
        <a className="brand" href="#today" aria-label="LifeHub Today">
          <span className="brand-mark" aria-hidden="true"><Leaf size={21} strokeWidth={2.2} /></span>
          <span>LifeHub</span>
        </a>

        <nav className="desktop-nav" aria-label="Menu LifeHub">
          <a className="nav-item nav-item-active" href="#today" aria-current="page">
            <SunMedium size={19} aria-hidden="true" />
            <span>Today</span>
          </a>
        </nav>

        <a className="sidebar-add" href="#quick-add">
          <Plus size={18} aria-hidden="true" /> Tambah tugas
        </a>

        <div className="sidebar-account">
          <div className="account-avatar" aria-hidden="true">
            <CircleUserRound size={22} />
          </div>
          <div className="account-copy">
            <strong>{email?.split("@")[0] || "Pengguna"}</strong>
            <span>{timezone}</span>
          </div>
          <button className="icon-button" onClick={onOpenSettings} type="button">
            <Settings2 size={18} aria-hidden="true" />
            <span className="sr-only">Buka pengaturan zona waktu</span>
          </button>
        </div>
      </aside>

      <div className="app-column">
        <header className="mobile-header">
          <a className="brand" href="#today" aria-label="LifeHub Today">
            <span className="brand-mark" aria-hidden="true"><Leaf size={19} /></span>
            <span>LifeHub</span>
          </a>
          <button className="icon-button" onClick={onOpenSettings} type="button">
            <Settings2 size={19} aria-hidden="true" />
            <span className="sr-only">Buka pengaturan zona waktu</span>
          </button>
        </header>

        <div className="desktop-topbar">
          <span className="timezone-chip"><span aria-hidden="true">●</span> {timezone}</span>
          <button className="quiet-button" onClick={onSignOut} type="button">
            <LogOut size={16} aria-hidden="true" /> Keluar
          </button>
        </div>

        <main id="main-content" tabIndex={-1}>{children}</main>

        <nav className="mobile-nav" aria-label="Navigasi utama seluler">
          <a className="mobile-nav-item mobile-nav-item-active" href="#today" aria-current="page">
            <SunMedium size={20} aria-hidden="true" /><span>Today</span>
          </a>
          <a className="mobile-nav-item" href="#quick-add">
            <Plus size={20} aria-hidden="true" /><span>Tambah</span>
          </a>
        </nav>
      </div>
    </div>
  );
}
