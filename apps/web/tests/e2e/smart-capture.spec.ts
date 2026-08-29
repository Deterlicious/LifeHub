import { expect, test, type Page } from "@playwright/test";

test.setTimeout(120_000);

async function enterWorkspace(page: Page, runId: string) {
  await page.goto("/");
  await page.getByLabel("Email").fill(`lifehub-smart-capture-${runId}@local.test`);
  await page.getByRole("button", { name: "Masuk ke LifeHub" }).click();
  await page.getByRole("heading", { name: "Di zona waktu mana kamu berada?" }).waitFor();
  await page.getByLabel("Zona waktu IANA").fill("Asia/Jakarta");
  await page.getByRole("button", { name: "Lanjut ke Today" }).click();
  await expect(page.getByRole("heading", { name: "Today", exact: true })).toBeVisible();
}

async function jakartaDate(page: Page): Promise<string> {
  return page.evaluate(() => {
    const parts = new Intl.DateTimeFormat("en-CA", {
      timeZone: "Asia/Jakarta",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).formatToParts(new Date());
    const values = Object.fromEntries(parts.map((part) => [part.type, part.value]));
    return `${values.year}-${values.month}-${values.day}`;
  });
}

test("@mobile smart capture prepares an editable recurring bill draft and never auto-saves", async ({ page }, testInfo) => {
  const runId = `${testInfo.project.name}-${Date.now()}`;
  const savedTitle = `Internet rumah ${runId}`;
  await enterWorkspace(page, runId);

  const quickAdd = page.locator("#quick-add");
  await quickAdd.getByLabel("Ceritakan yang ingin ditambahkan").fill(
    "Bayar internet 350 ribu tanggal 15 tiap bulan",
  );
  await quickAdd.getByRole("button", { name: "Buat draf", exact: true }).click();

  await expect(quickAdd.getByText("Draf siap diperiksa. Belum disimpan.")).toBeVisible();
  await expect(quickAdd.getByText("Jam jatuh tempo belum disebutkan.")).toBeVisible();
  await expect(quickAdd.getByLabel("Tagihan apa yang perlu dibayar?")).toHaveValue("Internet");
  await expect(quickAdd.getByLabel(/Nominal/)).toHaveValue("350000");
  await expect(quickAdd.getByRole("checkbox", { name: /Jadikan berulang/ })).toBeChecked();
  await expect(quickAdd.locator(".recurrence-control").getByRole("combobox")).toHaveValue("monthly");
  await expect(page.locator(".timeline-card").getByRole("article").filter({ hasText: "Internet" })).toHaveCount(0);

  await quickAdd.getByLabel("Tagihan apa yang perlu dibayar?").fill(savedTitle);
  await quickAdd.getByLabel("Jatuh tempo lokal").fill(`${await jakartaDate(page)}T23:40`);
  await quickAdd.getByRole("button", { name: "Simpan tagihan" }).click();

  await expect(page.getByText("Tagihan disimpan.")).toBeVisible();
  await expect(page.locator(".timeline-card").getByRole("article").filter({ hasText: savedTitle })).toBeVisible();
  await expect(quickAdd.getByLabel("Ceritakan yang ingin ditambahkan")).toHaveValue("");

  const manager = page.getByRole("region", { name: "Seri berulang" });
  await manager.locator(".recurrence-disclosure > summary").click();
  await expect(manager.getByRole("article").filter({ hasText: savedTitle }).getByText(/bulanan/i)).toBeVisible();
});
