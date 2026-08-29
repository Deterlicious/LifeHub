export type AppView = "today" | "agenda";

export function appViewFromHash(hash: string): AppView {
  return hash === "#agenda" ? "agenda" : "today";
}

export function isKnownAppHash(hash: string): boolean {
  return hash === "#today" || hash === "#quick-add" || hash === "#agenda";
}
