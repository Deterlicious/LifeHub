import { expect, test } from "@playwright/test";

test.setTimeout(90_000);

test("@desktop Today keeps quick-add controls clear and places upcoming beside the form", async ({ page }) => {
  const runId = Date.now();
  const taskTitle = `Tata letak E2E ${runId}`;

  await page.setViewportSize({ width: 1920, height: 1080 });
  await page.goto("/");
  await page.getByLabel("Email").fill(`lifehub-layout-${runId}@local.test`);
  await page.getByRole("button", { name: "Masuk ke LifeHub" }).click();
  await page.getByLabel("Zona waktu IANA").fill("Asia/Jakarta");
  await page.getByRole("button", { name: "Lanjut ke Today" }).click();

  const tomorrow = await page.evaluate(() => {
    const parts = new Intl.DateTimeFormat("en-CA", {
      day: "2-digit",
      month: "2-digit",
      timeZone: "Asia/Jakarta",
      year: "numeric",
    }).formatToParts(new Date());
    const values = Object.fromEntries(parts.map((part) => [part.type, part.value]));
    return new Date(Date.UTC(
      Number(values.year),
      Number(values.month) - 1,
      Number(values.day) + 1,
      12,
    )).toISOString().slice(0, 10);
  });

  const quickAdd = page.locator("#quick-add");
  await quickAdd.getByLabel("Apa yang perlu dilakukan?").fill(taskTitle);
  await quickAdd.getByLabel("Tenggat lokal").fill(`${tomorrow}T09:00`);
  await quickAdd.getByRole("button", { name: "Simpan tugas" }).click();

  const upcoming = page.locator(".today-primary-column > .upcoming-documents");
  await expect(upcoming.getByRole("heading", { name: taskTitle, exact: true })).toBeVisible();
  await expect(upcoming.getByText("Prioritas normal", { exact: true })).toBeVisible();

  const dueBox = await quickAdd.getByLabel("Tenggat lokal").boundingBox();
  const priorityBox = await quickAdd.getByLabel("Prioritas").boundingBox();
  expect(dueBox).not.toBeNull();
  expect(priorityBox).not.toBeNull();
  expect(dueBox!.x + dueBox!.width + 8).toBeLessThanOrEqual(priorityBox!.x);

  const upcomingBox = await upcoming.boundingBox();
  const quickAddBox = await quickAdd.boundingBox();
  expect(upcomingBox).not.toBeNull();
  expect(quickAddBox).not.toBeNull();
  expect(upcomingBox!.y).toBeLessThan(quickAddBox!.y + quickAddBox!.height);
});
