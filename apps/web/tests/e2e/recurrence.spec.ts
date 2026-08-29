import { expect, test, type Page } from "@playwright/test";

test.setTimeout(150_000);

async function enterWorkspace(page: Page, runId: string) {
  await page.goto("/");
  await page.getByLabel("Email").fill(`lifehub-recurrence-${runId}@local.test`);
  await page.getByRole("button", { name: "Masuk ke LifeHub" }).click();
  await page.getByRole("heading", { name: "Di zona waktu mana kamu berada?" }).waitFor();
  await page.getByLabel("Zona waktu IANA").fill("Asia/Jakarta");
  await page.getByRole("button", { name: "Lanjut ke Today" }).click();
  await expect(page.getByRole("heading", { name: "Today", exact: true })).toBeVisible();
}

async function profileDates(page: Page) {
  return page.evaluate(() => {
    const parts = new Intl.DateTimeFormat("en-CA", {
      timeZone: "Asia/Jakarta",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).formatToParts(new Date());
    const values = Object.fromEntries(parts.map((part) => [part.type, part.value]));
    const base = new Date(Date.UTC(Number(values.year), Number(values.month) - 1, Number(values.day), 12));
    const at = (offset: number) => {
      const date = new Date(base);
      date.setUTCDate(date.getUTCDate() + offset);
      return date.toISOString().slice(0, 10);
    };
    return { today: at(0), day1: at(1), day14: at(14) };
  });
}

async function closeToast(page: Page) {
  const close = page.getByRole("button", { name: "Tutup pemberitahuan" });
  if (await close.isVisible()) await close.click();
}

test("@mobile recurring bill occurrences remain independent and series rules can be changed or stopped", async ({ page }, testInfo) => {
  const runId = `${testInfo.project.name}-${Date.now()}`;
  const title = `Internet berulang ${runId}`;
  await enterWorkspace(page, runId);
  const dates = await profileDates(page);

  const quickAdd = page.locator("#quick-add");
  await quickAdd.getByText("Tagihan", { exact: true }).click();
  await quickAdd.getByLabel("Tagihan apa yang perlu dibayar?").fill(title);
  await quickAdd.getByLabel(/Nominal/).fill("450000");
  await quickAdd.getByLabel("Jatuh tempo lokal").fill(`${dates.today}T23:30`);
  await quickAdd.getByRole("checkbox", { name: /Jadikan berulang/ }).check();
  const recurrenceControls = quickAdd.locator(".recurrence-control");
  await recurrenceControls.getByRole("combobox").selectOption("daily");
  await recurrenceControls.getByRole("spinbutton").fill("1");
  await recurrenceControls.getByLabel(/Berakhir/).fill(dates.day14);
  await quickAdd.getByRole("button", { name: "Simpan tagihan" }).click();

  await expect(page.getByText("Tagihan disimpan.")).toBeVisible();
  const todayBill = page.locator(".timeline-card").getByRole("article").filter({ hasText: title });
  await expect(todayBill).toBeVisible();
  await todayBill.getByRole("button", { name: `Bayar ${title}` }).click();
  await expect(page.getByText("Tagihan ditandai lunas.")).toBeVisible();
  await closeToast(page);

  const manager = page.getByRole("region", { name: "Seri berulang" });
  await manager.locator(".recurrence-disclosure > summary").click();
  const seriesRow = manager.getByRole("article").filter({ hasText: title });
  await expect(seriesRow).toBeVisible();
  await expect(seriesRow.getByText(/harian/i)).toBeVisible();

  await page.locator(".mobile-nav").getByRole("link", { name: "Agenda" }).click();
  await expect(page.getByRole("heading", { name: "Agenda", exact: true })).toBeVisible();
  const day1Row = page.locator(".agenda-groups").getByRole("article").filter({ hasText: title }).first();
  await expect(day1Row).toBeVisible();
  await expect(day1Row).toContainText("Rp450.000");

  await page.locator(".mobile-nav").getByRole("link", { name: "Today" }).click();
  await manager.locator(".recurrence-disclosure > summary").click();
  await seriesRow.getByRole("button", { name: "Ubah aturan" }).click();
  await seriesRow.getByRole("combobox").selectOption("weekly");
  await seriesRow.getByRole("spinbutton").fill("2");
  await seriesRow.getByLabel(/Berakhir/).fill(dates.day14);
  await seriesRow.getByRole("button", { name: "Simpan aturan" }).click();
  await expect(page.getByText("Aturan pengulangan diperbarui.")).toBeVisible();
  await closeToast(page);
  await expect(seriesRow.getByText(/setiap 2 mingguan/i)).toBeVisible();

  await page.locator(".mobile-nav").getByRole("link", { name: "Agenda" }).click();
  const futureRows = page.locator(".agenda-groups").getByRole("article").filter({ hasText: title });
  await expect(futureRows).toHaveCount(1);
  await page.locator(".mobile-nav").getByRole("link", { name: "Today" }).click();

  await manager.locator(".recurrence-disclosure > summary").click();
  await seriesRow.getByRole("button", { name: "Hentikan" }).click();
  await expect(seriesRow.getByRole("group", { name: `Konfirmasi hentikan ${title}` })).toBeVisible();
  await seriesRow.getByRole("button", { name: "Ya, hentikan" }).click();
  await expect(page.getByText(/Seri dihentikan/)).toBeVisible();
  await expect(manager.getByRole("article").filter({ hasText: title })).toHaveCount(0);
  await closeToast(page);

  await page.locator(".mobile-nav").getByRole("link", { name: "Agenda" }).click();
  await expect(page.locator(".agenda-groups").getByRole("article").filter({ hasText: title })).toHaveCount(0);
  await page.locator(".mobile-nav").getByRole("link", { name: "Today" }).click();
  await page.locator(".completed-section > summary").click();
  const paidAnchor = page.locator(".timeline-card").getByRole("article").filter({ hasText: title });
  await expect(paidAnchor).toBeVisible();
  await expect(paidAnchor.getByText("Lunas", { exact: true })).toBeVisible();

  await page.reload();
  await expect(page.getByRole("heading", { name: "Today", exact: true })).toBeVisible();
  await page.getByRole("region", { name: "Seri berulang" }).locator(".recurrence-disclosure > summary").click();
  await expect(page.getByRole("region", { name: "Seri berulang" }).getByText("Belum ada seri aktif", { exact: true })).toBeVisible();
});
