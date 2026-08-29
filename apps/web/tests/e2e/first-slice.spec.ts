import { expect, test } from "@playwright/test";

test.setTimeout(90_000);

test("@mobile baseline dev login → use Today and owned document metadata → resolve → reload", async ({ page }) => {
  const runId = Date.now();
  const taskTitle = `Tugas E2E ${runId}`;
  const eventTitle = `Jadwal E2E ${runId}`;
  const billTitle = `Tagihan E2E ${runId}`;
  const todayDocumentName = `SIM E2E ${runId}`;
  const upcomingDocumentName = `Polis E2E ${runId}`;
  const farDocumentName = `Arsip E2E ${runId}`;
  const editedFarDocumentName = `Arsip kerja E2E ${runId}`;
  const farDocumentNote = "Periksa syarat administrasi kantor";

  await page.goto("/");
  await expect(page.getByText("Mode dev lokal", { exact: false })).toBeVisible();

  await page.getByLabel("Email").fill(`lifehub-e2e-${runId}@local.test`);
  await page.getByRole("button", { name: "Masuk ke LifeHub" }).click();

  const timezoneHeading = page.getByRole("heading", {
    name: "Di zona waktu mana kamu berada?",
  });
  const todayHeading = page.getByRole("heading", { name: "Today", exact: true });
  await expect(timezoneHeading.or(todayHeading)).toBeVisible();
  await expect(timezoneHeading).toBeVisible();
  await page.getByLabel("Zona waktu IANA").fill("Asia/Jakarta");
  await page.getByRole("button", { name: "Lanjut ke Today" }).click();

  await expect(page.getByRole("heading", { name: "Today", exact: true })).toBeVisible();
  await page.getByLabel("Apa yang perlu dilakukan?").fill(taskTitle);
  await page.getByRole("button", { name: "Simpan tugas" }).click();

  await expect(page.getByText("Tugas disimpan.")).toBeVisible();
  await expect(page.getByRole("heading", { name: taskTitle, exact: true })).toBeVisible();

  const { todayDate, upcomingDate, farFutureDate } = await page.evaluate(() => {
    const parts = new Intl.DateTimeFormat("en-CA", {
      timeZone: "Asia/Jakarta",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).formatToParts(new Date());
    const values = Object.fromEntries(parts.map((part) => [part.type, part.value]));
    const today = `${values.year}-${values.month}-${values.day}`;
    const farFuture = new Date(Date.UTC(
      Number(values.year),
      Number(values.month) - 1,
      Number(values.day) + 60,
      12,
    ));
    const upcoming = new Date(Date.UTC(
      Number(values.year),
      Number(values.month) - 1,
      Number(values.day) + 10,
      12,
    ));
    return {
      todayDate: today,
      upcomingDate: upcoming.toISOString().slice(0, 10),
      farFutureDate: farFuture.toISOString().slice(0, 10),
    };
  });

  await page.getByText("Jadwal", { exact: true }).click();
  await expect(page.getByRole("radio", { name: "Jadwal" })).toBeChecked();
  await page.getByLabel("Apa jadwalnya?").fill(eventTitle);
  await page.getByRole("checkbox", { name: /Sepanjang hari/ }).check();
  await page.getByLabel("Tanggal mulai").fill(todayDate);
  await page.getByLabel("Tanggal selesai").fill(todayDate);
  await page.getByLabel(/Lokasi/).fill("Ruang E2E");
  await page.getByRole("button", { name: "Simpan jadwal" }).click();

  await expect(page.getByText("Jadwal disimpan.")).toBeVisible();
  const eventRow = page.getByRole("article").filter({ hasText: eventTitle });
  await expect(eventRow).toBeVisible();
  await expect(eventRow.getByText("Jadwal", { exact: true })).toBeVisible();
  await expect(eventRow.getByText("Ruang E2E", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Tutup pemberitahuan" }).click();

  await page.getByText("Dokumen", { exact: true }).click();
  await expect(page.getByRole("radio", { name: "Dokumen" })).toBeChecked();
  await expect(page.locator("#quick-add").getByText(
    "Simpan metadata saja. Jangan masukkan nomor dokumen atau unggah scan.",
    { exact: true },
  )).toBeVisible();
  await page.getByLabel("Nama dokumen").fill(todayDocumentName);
  await page.getByLabel("Kategori").selectOption("license");
  await page.getByLabel("Tanggal kedaluwarsa").fill(todayDate);
  await page.getByRole("button", { name: "Simpan dokumen" }).click();

  await expect(page.getByText("Metadata dokumen disimpan.")).toBeVisible();
  const todayDocumentRow = page.locator(".timeline-card").getByRole("article").filter({
    hasText: todayDocumentName,
  });
  await expect(todayDocumentRow).toBeVisible();
  await expect(todayDocumentRow.getByText("Lisensi", { exact: true })).toBeVisible();
  await expect(todayDocumentRow.getByText("Segera kedaluwarsa", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Tutup pemberitahuan" }).click();

  await page.getByLabel("Nama dokumen").fill(upcomingDocumentName);
  await page.getByLabel("Kategori").selectOption("insurance");
  await page.getByLabel("Tanggal kedaluwarsa").fill(upcomingDate);
  await page.getByRole("button", { name: "Simpan dokumen" }).click();

  await expect(page.getByText("Metadata dokumen disimpan.")).toBeVisible();
  await expect(page.locator(".timeline-card").getByRole("article").filter({
    hasText: upcomingDocumentName,
  })).toHaveCount(0);
  const upcomingDocumentRow = page.locator(".upcoming-documents").getByRole("article").filter({
    hasText: upcomingDocumentName,
  });
  await expect(upcomingDocumentRow).toBeVisible();
  await expect(upcomingDocumentRow.getByText("Asuransi", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Tutup pemberitahuan" }).click();

  await page.getByLabel("Nama dokumen").fill(farDocumentName);
  await page.getByLabel("Kategori").selectOption("other");
  await page.getByLabel("Tanggal kedaluwarsa").fill(farFutureDate);
  await page.locator("#quick-add details.notes-disclosure > summary").click();
  await page.getByLabel("Catatan dokumen").fill(farDocumentNote);
  await page.getByRole("button", { name: "Simpan dokumen" }).click();

  await expect(page.getByText("Metadata dokumen disimpan.")).toBeVisible();
  await expect(page.locator(".timeline-card").getByRole("article").filter({
    hasText: farDocumentName,
  })).toHaveCount(0);
  const documentsManager = page.getByRole("region", { name: "Kelola metadata dokumen" });
  await documentsManager.getByText("Dokumen saya", { exact: true }).click();
  await expect(documentsManager.getByText("3 metadata tersimpan", { exact: true })).toBeVisible();
  const farDocumentRecord = documentsManager.getByRole("article").filter({
    hasText: farDocumentName,
  });
  await expect(farDocumentRecord).toBeVisible();
  await expect(farDocumentRecord.getByText(farDocumentNote, { exact: true })).toBeVisible();
  await farDocumentRecord.getByRole("button", { name: `Ubah ${farDocumentName}` }).click();
  await farDocumentRecord.getByLabel("Nama dokumen").fill(editedFarDocumentName);
  await farDocumentRecord.getByLabel("Kategori").selectOption("work");
  await farDocumentRecord.getByLabel("Catatan").fill("");
  await farDocumentRecord.getByRole("button", { name: "Simpan perubahan" }).click();

  await expect(page.getByText("Metadata dokumen diperbarui.")).toBeVisible();
  const editedFarDocumentRecord = documentsManager.getByRole("article").filter({
    hasText: editedFarDocumentName,
  });
  await expect(editedFarDocumentRecord).toBeVisible();
  await expect(editedFarDocumentRecord.getByText("Pekerjaan", { exact: true })).toBeVisible();
  await expect(documentsManager.getByText(farDocumentNote, { exact: true })).toHaveCount(0);

  await page.reload();
  await expect(page.getByRole("heading", { name: "Today", exact: true })).toBeVisible();
  await expect(page.locator(".timeline-card").getByRole("article").filter({
    hasText: todayDocumentName,
  })).toBeVisible();
  await documentsManager.getByText("Dokumen saya", { exact: true }).click();
  await expect(editedFarDocumentRecord).toBeVisible();
  await expect(documentsManager.getByText(farDocumentNote, { exact: true })).toHaveCount(0);
  await editedFarDocumentRecord.getByRole("button", {
    name: `Hapus ${editedFarDocumentName}`,
  }).click();
  await expect(documentsManager.getByRole("group", {
    name: `Konfirmasi hapus ${editedFarDocumentName}`,
  })).toBeVisible();
  await editedFarDocumentRecord.getByRole("button", { name: "Ya, hapus" }).click();
  await expect(page.getByText("Metadata dokumen dihapus.")).toBeVisible();
  await expect(documentsManager.getByRole("article").filter({
    hasText: editedFarDocumentName,
  })).toHaveCount(0);

  await page.getByText("Tagihan", { exact: true }).click();
  await expect(page.getByRole("radio", { name: "Tagihan" })).toBeChecked();
  await page.getByLabel("Tagihan apa yang perlu dibayar?").fill(billTitle);
  await page.getByLabel(/Nominal/).fill("350000");
  await page.getByLabel("Jatuh tempo lokal").fill(`${todayDate}T23:59`);
  await page.getByRole("button", { name: "Simpan tagihan" }).click();

  await expect(page.getByText("Tagihan disimpan.")).toBeVisible();
  const billRow = page.getByRole("article").filter({ hasText: billTitle });
  await expect(billRow).toBeVisible();
  await expect(billRow.getByText("Rp350.000", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: `Bayar ${billTitle}` }).click();
  await expect(page.getByText("Tagihan ditandai lunas.")).toBeVisible();
  await expect(page.getByRole("button", { name: `Bayar ${billTitle}` })).toHaveCount(0);

  await page.getByRole("button", { name: `Selesaikan ${taskTitle}` }).click();
  await expect(page.getByText("Tugas selesai. Kerja bagus.")).toBeVisible();
  await expect(page.getByRole("button", { name: `Selesaikan ${taskTitle}` })).toHaveCount(0);
  await expect(page.getByRole("article").filter({ hasText: eventTitle })).toBeVisible();
  await page.getByText(/Tuntas hari ini \(2\)/).click();
  await expect(page.getByRole("article", { name: `${taskTitle}, selesai` })).toBeVisible();
  const paidBillRow = page.getByRole("article").filter({ hasText: billTitle });
  await expect(paidBillRow).toBeVisible();
  await expect(paidBillRow.getByText("Lunas", { exact: true })).toBeVisible();

  await page.reload();
  await expect(page.getByRole("heading", { name: "Today", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: `Selesaikan ${taskTitle}` })).toHaveCount(0);
  const reloadedEventRow = page.getByRole("article").filter({ hasText: eventTitle });
  await expect(reloadedEventRow).toBeVisible();
  await expect(reloadedEventRow.getByText("Ruang E2E", { exact: true })).toBeVisible();
  const reloadedDocumentRow = page.locator(".timeline-card").getByRole("article").filter({
    hasText: todayDocumentName,
  });
  await expect(reloadedDocumentRow).toBeVisible();
  await expect(reloadedDocumentRow.getByText("Lisensi", { exact: true })).toBeVisible();
  await expect(page.locator(".upcoming-documents").getByRole("article").filter({
    hasText: upcomingDocumentName,
  })).toBeVisible();
  await page.getByText(/Tuntas hari ini \(2\)/).click();
  await expect(page.getByRole("article", { name: `${taskTitle}, selesai` })).toBeVisible();
  const reloadedBillRow = page.getByRole("article").filter({ hasText: billTitle });
  await expect(reloadedBillRow).toBeVisible();
  await expect(reloadedBillRow.getByText("Rp350.000", { exact: true })).toBeVisible();
  await expect(reloadedBillRow.getByText("Lunas", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: `Bayar ${billTitle}` })).toHaveCount(0);
  await documentsManager.getByText("Dokumen saya", { exact: true }).click();
  await expect(documentsManager.getByText("2 metadata tersimpan", { exact: true })).toBeVisible();
  await expect(documentsManager.getByRole("article").filter({
    hasText: editedFarDocumentName,
  })).toHaveCount(0);
});

test("@mobile user can explicitly delete all LifeHub data without deleting the login identity", async ({ page }) => {
  const runId = Date.now();
  const email = `lifehub-delete-${runId}@local.test`;
  const taskTitle = `Data sementara ${runId}`;

  await page.goto("/");
  await page.getByLabel("Email").fill(email);
  await page.getByRole("button", { name: "Masuk ke LifeHub" }).click();
  await page.getByLabel("Zona waktu IANA").fill("Asia/Jakarta");
  await page.getByRole("button", { name: "Lanjut ke Today" }).click();
  await page.getByLabel("Apa yang perlu dilakukan?").fill(taskTitle);
  await page.getByRole("button", { name: "Simpan tugas" }).click();
  await expect(page.getByRole("heading", { name: taskTitle, exact: true })).toBeVisible();

  await page.getByRole("button", { name: "Buka pengaturan LifeHub" }).click();
  const settings = page.getByRole("dialog", { name: "Pengaturan LifeHub" });
  await expect(settings).toBeVisible();
  await settings.getByRole("button", { name: "Hapus seluruh data" }).click();
  const permanentDelete = settings.getByRole("button", { name: "Hapus permanen" });
  await expect(permanentDelete).toBeDisabled();
  await settings.getByLabel(/Ketik HAPUS DATA LIFEHUB/).fill("HAPUS DATA LIFEHUB");
  await permanentDelete.click();

  await expect(page.getByRole("button", { name: "Masuk ke LifeHub" })).toBeVisible();
  await page.getByLabel("Email").fill(email);
  await page.getByRole("button", { name: "Masuk ke LifeHub" }).click();
  await expect(page.getByRole("heading", {
    name: "Di zona waktu mana kamu berada?",
  })).toBeVisible();
  await expect(page.getByText(taskTitle, { exact: true })).toHaveCount(0);
});
