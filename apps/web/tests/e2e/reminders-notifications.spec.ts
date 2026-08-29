import { expect, test, type Page } from "@playwright/test";

test.setTimeout(180_000);

async function enterWorkspace(page: Page, runId: string) {
  await page.goto("/");
  await page.getByLabel("Email").fill(`lifehub-reminders-${runId}@local.test`);
  await page.getByRole("button", { name: "Masuk ke LifeHub" }).click();
  await page.getByRole("heading", { name: "Di zona waktu mana kamu berada?" }).waitFor();
  await page.getByLabel("Zona waktu IANA").fill("Asia/Jakarta");
  await page.getByRole("button", { name: "Lanjut ke Today" }).click();
  await expect(page.getByRole("heading", { name: "Today", exact: true })).toBeVisible();
}

async function jakartaDateTime(page: Page, minutesFromNow: number) {
  return page.evaluate((offset) => {
    const target = new Date(Date.now() + offset * 60_000);
    target.setSeconds(0, 0);
    const parts = new Intl.DateTimeFormat("en-CA", {
      timeZone: "Asia/Jakarta",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      hourCycle: "h23",
    }).formatToParts(target);
    const values = Object.fromEntries(parts.map((part) => [part.type, part.value]));
    return {
      local: `${values.year}-${values.month}-${values.day}T${values.hour}:${values.minute}`,
      fireAtMilliseconds: target.getTime(),
      dateOnly: `${values.year}-${values.month}-${values.day}`,
    };
  }, minutesFromNow);
}

async function dateDaysFromNow(page: Page, days: number) {
  return page.evaluate((offset) => {
    const parts = new Intl.DateTimeFormat("en-CA", {
      timeZone: "Asia/Jakarta",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).formatToParts(new Date());
    const values = Object.fromEntries(parts.map((part) => [part.type, part.value]));
    const date = new Date(Date.UTC(
      Number(values.year),
      Number(values.month) - 1,
      Number(values.day) + offset,
      12,
    ));
    return date.toISOString().slice(0, 10);
  }, days);
}

async function closeToast(page: Page) {
  const close = page.getByRole("button", { name: "Tutup pemberitahuan" });
  if (await close.isVisible()) await close.click();
}

async function addTask(page: Page, title: string, dueLocal: string) {
  const quickAdd = page.locator("#quick-add");
  await quickAdd.getByText("Tugas", { exact: true }).click();
  await quickAdd.getByLabel("Apa yang perlu dilakukan?").fill(title);
  await quickAdd.getByLabel("Tenggat lokal").fill(dueLocal);
  await quickAdd.getByRole("button", { name: "Simpan tugas" }).click();
  await expect(page.getByText("Tugas disimpan.")).toBeVisible();
  await closeToast(page);
}

async function openTaskEditor(page: Page, title: string) {
  const row = page.locator(".today-page").getByRole("article").filter({ hasText: title });
  await expect(row).toBeVisible();
  await row.getByRole("button", { name: `Ubah atau hapus ${title}` }).click();
  return page.getByRole("dialog", { name: "Ubah tugas" });
}

async function addMomentReminder(page: Page, dialog: ReturnType<Page["getByRole"]>) {
  const controls = dialog.getByRole("region", { name: "Pengingat" });
  await controls.getByRole("button", { name: "Tambah" }).click();
  const minutes = controls.getByLabel(/Menit sebelum/);
  await expect(minutes).toBeFocused();
  await minutes.fill("0");
  await controls.getByRole("button", { name: "Simpan pengingat" }).click();
  await expect(controls.getByText("Terjadwal", { exact: true })).toBeVisible();
  await expect(controls.getByRole("button", { name: "Tambah" })).toBeFocused();
  await closeToast(page);
}

test("@mobile one-off reminder survives reload, delivers once, and read/state invalidation persists", async ({ page }, testInfo) => {
  const runId = `${testInfo.project.name}-${Date.now()}`;
  const firingTitle = `Pengingat segera ${runId}`;
  const invalidationTitle = `Pengingat dikoreksi ${runId}`;
  await enterWorkspace(page, runId);

  const firing = await jakartaDateTime(page, 1);
  await addTask(page, firingTitle, firing.local);
  let dialog = await openTaskEditor(page, firingTitle);
  await addMomentReminder(page, dialog);
  await dialog.getByRole("button", { name: "Tutup editor" }).click();

  await page.reload();
  await expect(page.getByRole("heading", { name: "Today", exact: true })).toBeVisible();
  const waitForWorker = Math.max(0, firing.fireAtMilliseconds + 7_000 - Date.now());
  if (waitForWorker > 0) await page.waitForTimeout(waitForWorker);
  await page.reload();

  const bell = page.locator(".mobile-header").getByRole("button", { name: /Buka notifikasi/ });
  const notificationDialog = page.getByRole("dialog", { name: "Notifikasi" });
  const deliveredRow = notificationDialog.getByRole("article").filter({ hasText: firingTitle });
  await expect.poll(async () => {
    if (await notificationDialog.isVisible()) {
      await notificationDialog.getByRole("button", { name: "Tutup pusat notifikasi" }).click();
    }
    await bell.click();
    return deliveredRow.count();
  }, { timeout: 30_000, intervals: [1_000, 2_000, 3_000] }).toBe(1);
  await expect(deliveredRow).toHaveCount(1);
  const centerBox = await notificationDialog.boundingBox();
  expect(centerBox?.width).toBeLessThanOrEqual(390);
  expect(centerBox?.height).toBeLessThan(844);

  await deliveredRow.getByRole("button", { name: /Tandai .* sebagai dibaca/ }).click();
  await expect(notificationDialog.getByText("0 belum dibaca", { exact: true })).toBeVisible();
  await expect(notificationDialog.getByRole("button", { name: "Tutup pusat notifikasi" })).toBeFocused();
  await notificationDialog.getByRole("button", { name: "Tutup pusat notifikasi" }).click();
  await page.reload();
  await bell.click();
  await expect(deliveredRow).toHaveCount(1);
  await expect(deliveredRow.getByText("Dibaca", { exact: true })).toBeVisible();
  await expect(deliveredRow.getByRole("button", { name: /Tandai .* sebagai dibaca/ })).toHaveCount(0);
  await notificationDialog.getByRole("button", { name: "Tutup pusat notifikasi" }).click();

  const future = await jakartaDateTime(page, 15);
  await addTask(page, invalidationTitle, future.local);
  dialog = await openTaskEditor(page, invalidationTitle);
  await addMomentReminder(page, dialog);
  await dialog.getByRole("button", { name: "Tutup editor" }).click();
  const futureRow = page.locator(".today-page").getByRole("article").filter({ hasText: invalidationTitle });
  await futureRow.getByRole("button", { name: `Selesaikan ${invalidationTitle}` }).click();
  await expect(page.getByText("Tugas selesai. Kerja bagus.")).toBeVisible();
  await closeToast(page);
  await page.locator(".completed-section > summary").click();
  dialog = await openTaskEditor(page, invalidationTitle);
  await expect(dialog.getByRole("region", { name: "Pengingat" }).getByText("Tidak aktif", { exact: true })).toBeVisible();
  await dialog.getByRole("button", { name: "Batalkan selesai" }).click();
  await expect(page.getByText("Tugas dikembalikan ke status terbuka.")).toBeVisible();
  await closeToast(page);

  dialog = await openTaskEditor(page, invalidationTitle);
  await expect(dialog.getByRole("region", { name: "Pengingat" }).getByText("Terjadwal", { exact: true })).toBeVisible();
  const moved = await jakartaDateTime(page, 20);
  await dialog.getByLabel(/Tenggat lokal/).fill(moved.local);
  await dialog.getByRole("button", { name: "Simpan perubahan" }).click();
  await expect(page.getByText("Tugas diperbarui.")).toBeVisible();
  await closeToast(page);
  dialog = await openTaskEditor(page, invalidationTitle);
  await expect(dialog.getByRole("region", { name: "Pengingat" }).getByText("Terjadwal", { exact: true })).toBeVisible();
  await dialog.getByRole("button", { name: "Hapus tugas" }).click();
  await dialog.getByRole("button", { name: "Ya, hapus" }).click();
  await expect(page.locator(".today-page").getByRole("article").filter({ hasText: invalidationTitle })).toHaveCount(0);
});

test("@desktop document reminders and notification focus stay accessible", async ({ page }, testInfo) => {
  const runId = `${testInfo.project.name}-${Date.now()}`;
  const taskTitle = `Tugas aksesibel ${runId}`;
  const documentName = `Dokumen pengingat ${runId}`;
  await enterWorkspace(page, runId);
  const futureTask = await jakartaDateTime(page, 20);
  await addTask(page, taskTitle, futureTask.local);

  const quickAdd = page.locator("#quick-add");
  await quickAdd.getByText("Dokumen", { exact: true }).click();
  await quickAdd.getByLabel("Nama dokumen").fill(documentName);
  await quickAdd.getByLabel("Kategori").selectOption("license");
  await quickAdd.getByLabel("Tanggal kedaluwarsa").fill(await dateDaysFromNow(page, 30));
  await quickAdd.getByRole("button", { name: "Simpan dokumen" }).click();
  await expect(page.getByText("Metadata dokumen disimpan.")).toBeVisible();
  await closeToast(page);

  const manager = page.getByRole("region", { name: "Kelola metadata dokumen" });
  await manager.getByText("Dokumen saya", { exact: true }).click();
  const documentRecord = manager.getByRole("article").filter({ hasText: documentName });
  await documentRecord.getByRole("button", { name: `Ubah ${documentName}` }).click();
  const documentReminders = documentRecord.getByRole("region", { name: "Pengingat" });
  await documentReminders.getByRole("button", { name: "Tambah" }).click();
  await expect(documentReminders.getByLabel(/Hari sebelum/)).toBeFocused();

  const taskDialog = await openTaskEditor(page, taskTitle);
  const taskReminders = taskDialog.getByRole("region", { name: "Pengingat" });
  await taskReminders.getByRole("button", { name: "Tambah" }).click();
  const describedByIds = await page.locator(".reminder-controls input[aria-describedby]").evaluateAll((inputs) => (
    inputs.map((input) => input.getAttribute("aria-describedby") ?? "")
  ));
  expect(new Set(describedByIds).size).toBe(describedByIds.length);
  for (const describedBy of describedByIds) {
    expect(await page.locator(`[id="${describedBy}"]`).count()).toBe(1);
  }
  await taskReminders.getByRole("button", { name: "Batal mengubah pengingat" }).click();
  await taskDialog.getByRole("button", { name: "Tutup editor" }).click();

  await documentReminders.getByLabel(/Hari sebelum/).fill("29");
  await documentReminders.getByLabel("Waktu lokal").fill("09:00");
  await documentReminders.getByRole("button", { name: "Simpan pengingat" }).click();
  await expect(documentReminders.getByText("Terjadwal", { exact: true })).toBeVisible();
  await closeToast(page);

  const fakeNotification = {
    id: `notification-${runId}`,
    source_kind: "document",
    source_id: `document-${runId}`,
    title: "Dokumen perlu diperhatikan",
    body: documentName,
    created_at: new Date().toISOString(),
    read_at: null,
  };
  await page.route("**/api/v1/notifications?*", async (route) => {
    await route.fulfill({
      body: JSON.stringify({ items: [fakeNotification], next_cursor: null, unread_count: 1 }),
      contentType: "application/json",
      status: 200,
    });
  });
  await page.route("**/api/v1/notifications/*/mark-read", async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 400));
    await route.fulfill({
      body: JSON.stringify({
        item: { ...fakeNotification, read_at: new Date().toISOString() },
        unread_count: 0,
      }),
      contentType: "application/json",
      status: 200,
    });
  });

  const bell = page.locator(".desktop-topbar").getByRole("button", { name: /Buka notifikasi/ });
  await bell.click();
  const notificationDialog = page.getByRole("dialog", { name: "Notifikasi" });
  const box = await notificationDialog.boundingBox();
  expect(box?.x).toBeGreaterThan(750);
  const row = notificationDialog.getByRole("article").filter({ hasText: documentName });
  const markRead = row.getByRole("button", { name: /Tandai .* sebagai dibaca/ });
  await markRead.click();
  await page.keyboard.press("Tab");
  await expect(notificationDialog).toBeFocused();
  await expect(notificationDialog.getByRole("button", { name: "Tutup pusat notifikasi" })).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(notificationDialog).toHaveCount(0);
  await expect(bell).toBeFocused();
});
