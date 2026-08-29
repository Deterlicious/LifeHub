import { expect, test, type Page } from "@playwright/test";

test.setTimeout(120_000);

async function enterWorkspace(page: Page, runId: string) {
  await page.goto("/");
  await page.getByLabel("Email").fill(`lifehub-agenda-${runId}@local.test`);
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
    return {
      today: at(0),
      day5: at(5),
      day6: at(6),
      day7: at(7),
      day30: at(30),
      day31: at(31),
    };
  });
}

async function closeToast(page: Page) {
  const close = page.getByRole("button", { name: "Tutup pemberitahuan" });
  if (await close.isVisible()) await close.click();
}

test("@mobile Agenda corrections persist across state, date, history, and browser navigation", async ({ page }, testInfo) => {
  const runId = `${testInfo.project.name}-${Date.now()}`;
  const taskTitle = `Koreksi tugas ${runId}`;
  const movedTaskTitle = `Tugas pekan depan ${runId}`;
  const eventTitle = `Jadwal waktu ${runId}`;
  const billTitle = `Tagihan agenda ${runId}`;
  const editedBillTitle = `Tagihan diperbarui ${runId}`;
  const day30Document = `Dokumen hari 30 ${runId}`;
  const day31Document = `Dokumen hari 31 ${runId}`;

  await enterWorkspace(page, runId);
  const dates = await profileDates(page);

  await page.getByLabel("Apa yang perlu dilakukan?").fill(taskTitle);
  await page.getByLabel("Tenggat lokal").fill(`${dates.today}T23:45`);
  await page.getByRole("button", { name: "Simpan tugas" }).click();
  await expect(page.getByText("Tugas disimpan.")).toBeVisible();
  await page.getByRole("button", { name: `Selesaikan ${taskTitle}` }).click();
  await expect(page.getByText("Tugas selesai. Kerja bagus.")).toBeVisible();
  await page.getByRole("button", { name: "Batalkan" }).click();
  await expect(page.getByText("Tugas dikembalikan ke status terbuka.")).toBeVisible();
  await expect(page.getByRole("button", { name: `Selesaikan ${taskTitle}` })).toBeVisible();
  await closeToast(page);

  const currentTaskRow = page.locator(".timeline-card").getByRole("article").filter({ hasText: taskTitle });
  const taskMoreButton = currentTaskRow.getByRole("button", { name: `Ubah atau hapus ${taskTitle}` });
  await taskMoreButton.click();
  const taskDialog = page.getByRole("dialog", { name: "Ubah tugas" });
  await taskDialog.getByLabel("Nama tugas").fill(movedTaskTitle);
  await taskDialog.getByLabel(/Tenggat lokal/).fill(`${dates.day5}T09:00`);
  await taskDialog.getByRole("button", { name: "Simpan perubahan" }).click();
  await expect(page.getByText("Tugas diperbarui.")).toBeVisible();
  await expect(page.locator("#main-content")).toBeFocused();
  await expect(page.locator(".timeline-card").getByRole("article").filter({ hasText: movedTaskTitle })).toHaveCount(0);
  await expect(page.locator(".upcoming-documents").getByRole("article").filter({ hasText: movedTaskTitle })).toBeVisible();
  await closeToast(page);

  await page.getByText("Jadwal", { exact: true }).click();
  await page.getByLabel("Apa jadwalnya?").fill(eventTitle);
  await page.getByRole("textbox", { name: /^Mulai/ }).fill(`${dates.day6}T09:00`);
  await page.getByRole("textbox", { name: /^Selesai/ }).fill(`${dates.day6}T10:00`);
  await page.getByLabel(/Lokasi/).fill("Ruang koreksi");
  await page.getByRole("button", { name: "Simpan jadwal" }).click();
  await expect(page.getByText("Jadwal disimpan.")).toBeVisible();
  await closeToast(page);

  await page.getByText("Tagihan", { exact: true }).click();
  await page.getByLabel("Tagihan apa yang perlu dibayar?").fill(billTitle);
  await page.getByLabel(/Nominal/).fill("375000");
  await page.getByLabel("Jatuh tempo lokal").fill(`${dates.day7}T23:00`);
  await page.getByRole("button", { name: "Simpan tagihan" }).click();
  await expect(page.getByText("Tagihan disimpan.")).toBeVisible();
  await closeToast(page);

  await page.getByText("Dokumen", { exact: true }).click();
  await page.getByLabel("Nama dokumen").fill(day30Document);
  await page.getByLabel("Kategori").selectOption("work");
  await page.getByLabel("Tanggal kedaluwarsa").fill(dates.day30);
  await page.getByRole("button", { name: "Simpan dokumen" }).click();
  await expect(page.getByText("Metadata dokumen disimpan.")).toBeVisible();
  await closeToast(page);
  await page.getByLabel("Nama dokumen").fill(day31Document);
  await page.getByLabel("Kategori").selectOption("education");
  await page.getByLabel("Tanggal kedaluwarsa").fill(dates.day31);
  await page.getByRole("button", { name: "Simpan dokumen" }).click();
  await expect(page.getByText("Metadata dokumen disimpan.")).toBeVisible();

  const upcoming = page.locator(".upcoming-documents");
  await expect(upcoming.getByRole("article").filter({ hasText: movedTaskTitle })).toBeVisible();
  await expect(upcoming.getByRole("article").filter({ hasText: eventTitle })).toBeVisible();
  await expect(upcoming.getByRole("article").filter({ hasText: billTitle })).toBeVisible();
  await expect(upcoming.getByRole("article").filter({ hasText: day30Document })).toBeVisible();
  await expect(upcoming.getByRole("article").filter({ hasText: day31Document })).toHaveCount(0);
  await expect(upcoming.getByRole("link", { name: "Lihat semua 4 di Agenda" })).toBeVisible();
  await closeToast(page);

  await page.locator(".mobile-nav").getByRole("link", { name: "Agenda" }).click();
  await expect(page).toHaveURL(/#agenda$/);
  await expect(page.getByRole("heading", { name: "Agenda", exact: true })).toBeVisible();
  await expect(page.locator(".mobile-nav").getByRole("link", { name: "Agenda" })).toHaveAttribute("aria-current", "page");
  await expect(page.locator(".agenda-groups").getByRole("article").filter({ hasText: movedTaskTitle })).toBeVisible();
  await page.goBack();
  await expect(page.getByRole("heading", { name: "Today", exact: true })).toBeVisible();
  await page.goForward();
  await expect(page.getByRole("heading", { name: "Agenda", exact: true })).toBeVisible();

  const agendaTaskRow = page.locator(".agenda-groups").getByRole("article").filter({ hasText: movedTaskTitle });
  await agendaTaskRow.getByRole("button", { name: `Ubah atau hapus ${movedTaskTitle}` }).click();
  const taskDeleteDialog = page.getByRole("dialog", { name: "Ubah tugas" });
  await taskDeleteDialog.getByRole("button", { name: "Hapus tugas" }).click();
  const cancelTaskDelete = taskDeleteDialog.getByRole("button", { name: "Batal" });
  await expect(cancelTaskDelete).toBeFocused();
  await cancelTaskDelete.click();
  await taskDeleteDialog.getByRole("button", { name: "Hapus tugas" }).click();
  await taskDeleteDialog.getByRole("button", { name: "Ya, hapus" }).click();
  await expect(page.getByText("Tugas dihapus.")).toBeVisible();
  await expect(page.locator("#main-content")).toBeFocused();
  await expect(page.locator(".agenda-groups").getByRole("article").filter({ hasText: movedTaskTitle })).toHaveCount(0);
  await closeToast(page);

  const agendaEventRow = page.locator(".agenda-groups").getByRole("article").filter({ hasText: eventTitle });
  await agendaEventRow.getByRole("button", { name: `Ubah atau hapus ${eventTitle}` }).click();
  const eventDialog = page.getByRole("dialog", { name: "Ubah jadwal" });
  await eventDialog.getByRole("checkbox", { name: /Sepanjang hari/ }).check();
  await eventDialog.getByLabel("Tanggal mulai").fill(dates.day6);
  await eventDialog.getByLabel(/Tanggal selesai/).fill(dates.day6);
  await eventDialog.getByRole("button", { name: "Simpan perubahan" }).click();
  await expect(page.getByText("Jadwal diperbarui.")).toBeVisible();
  await expect(agendaEventRow.getByText("Seharian", { exact: true })).toBeVisible();
  await closeToast(page);
  await page.reload();
  await expect(page.getByRole("heading", { name: "Agenda", exact: true })).toBeVisible();
  const reloadedEventRow = page.locator(".agenda-groups").getByRole("article").filter({ hasText: eventTitle });
  await expect(reloadedEventRow.getByText("Seharian", { exact: true })).toBeVisible();
  await reloadedEventRow.getByRole("button", { name: `Ubah atau hapus ${eventTitle}` }).click();
  const reloadedEventDialog = page.getByRole("dialog", { name: "Ubah jadwal" });
  await reloadedEventDialog.getByRole("button", { name: "Hapus jadwal" }).click();
  await reloadedEventDialog.getByRole("button", { name: "Ya, hapus" }).click();
  await expect(page.getByText("Jadwal dihapus.")).toBeVisible();
  await closeToast(page);

  const agendaBillRow = page.locator(".agenda-groups").getByRole("article").filter({ hasText: billTitle });
  await agendaBillRow.getByRole("button", { name: `Ubah atau hapus ${billTitle}` }).click();
  const billDialog = page.getByRole("dialog", { name: "Ubah tagihan" });
  await billDialog.getByLabel("Nama tagihan").fill(editedBillTitle);
  await billDialog.getByLabel(/Nominal/).fill("425000");
  await billDialog.getByRole("button", { name: "Simpan perubahan" }).click();
  await expect(page.getByText("Tagihan diperbarui.")).toBeVisible();
  await closeToast(page);

  await page.locator(".mobile-nav").getByRole("link", { name: "Today" }).click();
  const futureBillRow = page.locator(".upcoming-documents").getByRole("article").filter({ hasText: editedBillTitle });
  await expect(futureBillRow.getByText("Rp425.000", { exact: true })).toBeVisible();
  await futureBillRow.getByRole("button", { name: `Bayar ${editedBillTitle}` }).click();
  await expect(page.getByText("Tagihan ditandai lunas.")).toBeVisible();
  await closeToast(page);

  await page.locator(".mobile-nav").getByRole("link", { name: "Agenda" }).click();
  await page.locator(".agenda-filter-chips").getByRole("button", { name: "Tagihan" }).click();
  await page.locator(".agenda-bill-mode").getByRole("button", { name: "Riwayat lunas" }).click();
  const paidBillRow = page.locator(".agenda-groups").getByRole("article").filter({ hasText: editedBillTitle });
  await expect(paidBillRow).toBeVisible();
  await paidBillRow.getByRole("button", { name: `Ubah atau hapus ${editedBillTitle}` }).click();
  const paidBillDialog = page.getByRole("dialog", { name: "Ubah tagihan" });
  await paidBillDialog.getByRole("button", { name: "Tandai belum lunas" }).click();
  await expect(page.getByText("Tagihan ditandai belum lunas.")).toBeVisible();
  await closeToast(page);

  await page.reload();
  await expect(page.getByRole("heading", { name: "Agenda", exact: true })).toBeVisible();
  const unpaidBillRow = page.locator(".agenda-groups").getByRole("article").filter({ hasText: editedBillTitle });
  await expect(unpaidBillRow).toBeVisible();
  await unpaidBillRow.getByRole("button", { name: `Ubah atau hapus ${editedBillTitle}` }).click();
  const unpaidBillDialog = page.getByRole("dialog", { name: "Ubah tagihan" });
  await unpaidBillDialog.getByRole("button", { name: "Hapus tagihan" }).click();
  await unpaidBillDialog.getByRole("button", { name: "Ya, hapus" }).click();
  await expect(page.getByText("Tagihan dihapus.")).toBeVisible();
  await closeToast(page);

  let failDay31Once = true;
  await page.route("**/api/v1/agenda?*", async (route) => {
    const requestUrl = new URL(route.request().url());
    if (failDay31Once && requestUrl.searchParams.get("from") === dates.day31) {
      failDay31Once = false;
      await route.fulfill({
        body: JSON.stringify({
          error: { code: "AGENDA_TEST_FAILURE", message: "Rentang Agenda belum dapat dimuat." },
        }),
        contentType: "application/json",
        status: 503,
      });
      return;
    }
    await route.continue();
  });
  const agendaDateInput = page.getByLabel("Lompat ke tanggal");
  await agendaDateInput.fill(dates.day31);
  await expect(agendaDateInput).toHaveValue(dates.day31);
  const agendaError = page.getByRole("alert").filter({ hasText: "Rentang Agenda belum dapat dimuat." });
  await expect(agendaError).toBeVisible();
  await expect(agendaDateInput).toHaveValue(dates.day31);
  await agendaError.getByRole("button", { name: "Coba lagi" }).click();
  const day31AgendaRow = page.locator(".agenda-groups").getByRole("article").filter({ hasText: day31Document });
  await expect(day31AgendaRow).toBeVisible();
  await expect(day31AgendaRow.getByText("Pendidikan", { exact: false })).toBeVisible();
  await page.locator(".mobile-header").getByRole("button", { name: "Keluar dari LifeHub" }).click();
  await expect(page.getByRole("button", { name: "Masuk ke LifeHub" })).toBeVisible();
});

test("@desktop Agenda uses real navigation and an accessible side sheet", async ({ page }, testInfo) => {
  const runId = `${testInfo.project.name}-${Date.now()}`;
  const title = `Tugas desktop ${runId}`;
  await enterWorkspace(page, runId);
  const dates = await profileDates(page);

  const todayLink = page.locator(".desktop-nav").getByRole("link", { name: "Today" });
  const agendaLink = page.locator(".desktop-nav").getByRole("link", { name: "Agenda" });
  await expect(todayLink).toHaveAttribute("aria-current", "page");
  await page.getByLabel("Apa yang perlu dilakukan?").fill(title);
  await page.getByLabel("Tenggat lokal").fill(`${dates.day5}T09:00`);
  await page.getByRole("button", { name: "Simpan tugas" }).click();
  await expect(page.getByText("Tugas disimpan.")).toBeVisible();
  await closeToast(page);

  const upcomingRow = page.locator(".upcoming-documents").getByRole("article").filter({ hasText: title });
  const moreButton = upcomingRow.getByRole("button", { name: `Ubah atau hapus ${title}` });
  await moreButton.click();
  const dialog = page.getByRole("dialog", { name: "Ubah tugas" });
  await expect(dialog).toBeVisible();
  const box = await dialog.boundingBox();
  expect(box?.x).toBeGreaterThan(700);
  expect(box?.height).toBeGreaterThan(800);
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  await expect(moreButton).toBeFocused();

  await moreButton.click();
  const deleteDialog = page.getByRole("dialog", { name: "Ubah tugas" });
  const deleteTrigger = deleteDialog.getByRole("button", { name: "Hapus tugas" });
  await deleteTrigger.click();
  const cancelDelete = deleteDialog.getByRole("button", { name: "Batal" });
  await expect(cancelDelete).toBeFocused();
  await cancelDelete.click();
  await expect(deleteTrigger).toBeFocused();
  await page.route("**/api/v1/tasks/*", async (route) => {
    if (route.request().method() === "DELETE") {
      await new Promise((resolve) => setTimeout(resolve, 350));
    }
    await route.continue();
  });
  await deleteTrigger.click();
  await deleteDialog.getByRole("button", { name: "Ya, hapus" }).click();
  await expect(deleteDialog.getByRole("button", { name: "Menghapus…" })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(deleteDialog).toBeVisible();
  await expect(page.getByText("Tugas dihapus.")).toBeVisible();
  await expect(deleteDialog).toHaveCount(0);
  await expect(page.locator("#main-content")).toBeFocused();
  await page.unroute("**/api/v1/tasks/*");
  await closeToast(page);

  await agendaLink.click();
  await expect(page).toHaveURL(/#agenda$/);
  await expect(agendaLink).toHaveAttribute("aria-current", "page");
  await expect(todayLink).not.toHaveAttribute("aria-current", "page");
  const firstDateButton = page.locator(".agenda-date-strip button").first();
  const dateBox = await firstDateButton.boundingBox();
  expect(dateBox?.height).toBeGreaterThanOrEqual(44);
  await page.goBack();
  await expect(todayLink).toHaveAttribute("aria-current", "page");

  await page.route("**/api/v1/tasks", async (route) => {
    if (route.request().method() === "POST") {
      await route.fulfill({
        body: JSON.stringify({ error: { code: "UNAUTHORIZED", message: "Sesi sudah berakhir." } }),
        contentType: "application/json",
        status: 401,
      });
      return;
    }
    await route.continue();
  });
  await page.getByLabel("Apa yang perlu dilakukan?").fill(`Sesi kedaluwarsa ${runId}`);
  await page.getByRole("button", { name: "Simpan tugas" }).click();
  await expect(page.getByRole("button", { name: "Masuk ke LifeHub" })).toBeVisible();
});
