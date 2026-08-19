import { expect, test } from "@playwright/test";

test("dev login → timezone → create Today task → complete → reload", async ({ page }) => {
  const runId = Date.now();
  const taskTitle = `Tugas E2E ${runId}`;

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
  await page.getByRole("button", { name: "Tambah ke Today" }).click();

  await expect(page.getByText("Tugas ditambahkan ke Today.")).toBeVisible();
  await expect(page.getByRole("heading", { name: taskTitle, exact: true })).toBeVisible();

  await page.getByRole("button", { name: `Selesaikan ${taskTitle}` }).click();
  await expect(page.getByText("Tugas selesai. Kerja bagus.")).toBeVisible();
  await expect(page.getByRole("button", { name: `Selesaikan ${taskTitle}` })).toHaveCount(0);
  await page.getByText(/Selesai hari ini \(1\)/).click();
  await expect(page.getByRole("article", { name: `${taskTitle}, selesai` })).toBeVisible();

  await page.reload();
  await expect(page.getByRole("heading", { name: "Today", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: `Selesaikan ${taskTitle}` })).toHaveCount(0);
  await page.getByText(/Selesai hari ini \(1\)/).click();
  await expect(page.getByRole("article", { name: `${taskTitle}, selesai` })).toBeVisible();
});
