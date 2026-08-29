export function parseIntegerAmount(value: string): number | null {
  if (!/^\d+$/.test(value)) return null;

  const amount = Number(value);
  return Number.isSafeInteger(amount) && amount > 0 ? amount : null;
}

export function formatMoney(amount: number, currency = "IDR"): string {
  if (!Number.isSafeInteger(amount)) return "Nominal belum tersedia";

  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency,
    maximumFractionDigits: 0,
    minimumFractionDigits: 0,
  }).format(amount).replace(/\s/g, "");
}
