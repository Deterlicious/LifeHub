# LifeHub — START HERE

Dokumen ini adalah entry point ketika project dipindahkan ke Codex / ChatGPT Desktop.

## File yang harus tersedia

### Core context

1. `LIFEHUB_CODEX_MASTER_PROMPT.md`
2. `SKILL.md`
3. `AGENTS.md`
4. `coldstart.md`
5. `README.md`

### Engineering context

6. `docs/implementation-plan.md`
7. `docs/architecture.md`
8. `docs/stack-versions.md`
9. `docs/security.md`
10. `docs/ai-usage.md`
11. `docs/api.md`
12. `docs/deployment.md`

## Urutan penggunaan

1. Letakkan seluruh file ini di repository LifeHub dengan struktur folder yang sama.
2. Buka repository tersebut di Codex / ChatGPT Desktop.
3. Pastikan agent dapat membaca root repository dan folder `docs/`.
4. Kirim prompt bootstrap di bawah.
5. Biarkan agent melakukan **repository inspection terlebih dahulu**.
6. Agent harus memperbarui `docs/implementation-plan.md` dan `docs/stack-versions.md` berdasarkan kondisi repo yang sebenarnya.
7. Setelah itu agent mulai vertical slice pertama. Jangan berhenti hanya pada planning.

## Prompt bootstrap yang direkomendasikan

```text
Read AGENTS.md, coldstart.md, README.md, SKILL.md, LIFEHUB_CODEX_MASTER_PROMPT.md, and every relevant file under docs/ before changing code.

Use the $lifehub-senior-fullstack-go-builder skill and treat coldstart.md as the product source of truth.

First inspect the current repository, package manifests, Go modules, migrations, scripts, environment, and existing UI. Preserve good existing work.

Then:
1. update docs/implementation-plan.md so it reflects the actual repository,
2. verify the current latest stable and mutually compatible dependency versions from official documentation/package registries,
3. update docs/stack-versions.md with the versions actually selected,
4. identify the smallest production-worthy vertical slice,
5. implement that slice end-to-end,
6. run its tests and relevant quality gates,
7. continue until the slice genuinely works.

Do not use prerelease packages unless explicitly approved.
Do not add infrastructure merely to make the architecture look advanced.
Do not let Smart Quick Add or AI write production data autonomously.
Do not trust client-supplied user IDs.
Do not use in-memory timers as the source of truth for reminders.
Do not claim anything works unless you actually ran or tested it.

If a decision can be resolved from the project documents, make the decision and proceed. Ask me only when a real product decision blocks implementation.
```

## Vertical slice pertama

Target awal yang harus benar-benar selesai:

```text
Authentication
    ↓
User timezone
    ↓
Create Task
    ↓
Today menampilkan task
    ↓
Complete task
    ↓
Refresh / persistence tetap benar
    ↓
Automated tests
```

Jangan langsung membangun semua halaman.

Setelah vertical slice ini stabil:

```text
Events
  ↓
Bills
  ↓
Documents
  ↓
Reminder Engine
  ↓
Recurrence hardening
  ↓
Smart Quick Add
  ↓
PWA / Web Push
```

## Catatan versi

Snapshot versi di `docs/stack-versions.md` dibuat berdasarkan riset **19 Agustus 2026**.

Snapshot tersebut **bukan perintah untuk blind pin**.

Coding agent tetap wajib mengecek versi stabil terbaru dan kompatibilitas pada waktu implementasi. Bila versi terbaru tidak kompatibel atau punya masalah keamanan/runtime, pilih stable version terbaru yang kompatibel dan dokumentasikan alasannya.

## Prinsip penting

LifeHub tidak boleh berubah menjadi:

- CRUD demo;
- generic admin dashboard;
- kumpulan card yang tidak terintegrasi;
- AI demo tanpa manual fallback;
- architecture showcase yang overengineered.

LifeHub harus menjadi aplikasi yang bisa dipakai setiap hari dan backend Go harus mempunyai alasan teknis yang nyata: recurrence, durable reminders, workers, authorization, time handling, Today aggregation, idempotency, dan reliability.
