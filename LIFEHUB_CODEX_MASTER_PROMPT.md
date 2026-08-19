# Master Prompt untuk Codex / ChatGPT Desktop — LifeHub

Gunakan skill **`$lifehub-senior-fullstack-go-builder`** dan ikuti seluruh instruksinya.

Bangun **LifeHub** di repository/folder yang sedang terbuka. Bertindak sebagai senior full-stack engineer, senior Go backend engineer, product engineer, security-minded engineer, dan design engineer. Jangan berhenti pada mockup, landing page, atau CRUD sederhana. Bangun alur produk utama sampai benar-benar dapat dipakai setiap hari, diuji, dan dijalankan secara production-like.

## Product promise

> **Hal penting hari ini, ada di satu tempat.**

LifeHub menyatukan:
- tasks;
- events/jadwal;
- recurring bills;
- document-expiry tracking;
- reminders;
- notifications;
- satu halaman **Today** yang mengurutkan semuanya berdasarkan waktu dan urgensi.

## Prinsip utama

1. **Today-first** — halaman utama menjawab “Apa yang harus saya perhatikan hari ini?”
2. **Fast capture** — menambah sesuatu harus cepat.
3. **Durable reminders** — reminder tidak boleh hilang karena restart/deploy.
4. **Timezone-correct** — simpan instant sebagai `timestamptz`, simpan timezone pengguna sebagai IANA timezone, gunakan date-only untuk expiry bila tepat.
5. **Go backend authority** — business rules, recurrence, reminders, authorization, Today aggregation, dan persistence utama berada di Go.
6. **Progressive intelligence** — Smart Quick Add hanya accelerator; manual flow wajib selalu berfungsi.
7. **Privacy by design** — data LifeHub bersifat pribadi.

## Core user journey

### Daily
1. Login.
2. Buka **Today**.
3. Lihat overdue, task, event, bill, dan expiry dalam satu urutan.
4. Complete task / mark bill paid / buka detail.
5. Gunakan Quick Add bila perlu.
6. Reminder engine menyiapkan notifikasi secara durable.

### Create
`+ Tambah → Task | Event | Bill | Document → isi → recurrence/reminder → save → tampil di Today/upcoming`

### Smart Quick Add
Contoh:
- `Bayar internet 350 ribu tanggal 15 tiap bulan`
- `Meeting besok jam 2 siang`
- `SIM habis 6 November 2026 ingatkan 30 hari sebelumnya`

Flow:
`text → parser/provider → structured draft → validation → user review → explicit save`

AI/provider **tidak pernah** menulis data otomatis.

## Scope MVP

### Today Dashboard
Tampilkan:
- overdue;
- task hari ini;
- event berikutnya;
- bill yang mendekati jatuh tempo;
- dokumen yang mendekati expiry;
- quick add;
- upcoming beberapa hari ke depan.

Jangan tampilkan sebagai empat dashboard card terpisah bila timeline lebih jelas.

### Tasks
- title;
- notes opsional;
- priority;
- due date/time;
- complete/uncomplete;
- simple recurrence;
- reminder.

### Events
- title;
- notes/location opsional;
- start/end;
- all-day;
- simple recurrence;
- reminder;
- timezone-correct.

### Bills
- title;
- amount integer;
- currency default IDR;
- due;
- recurrence;
- paid/unpaid;
- paid_at;
- reminder.

### Documents
MVP fokus **metadata + expiry**, bukan file sensitif:
- name;
- category;
- notes;
- expiry date;
- reminders;
- valid/expiring/expired.

### Reminder + Notification Engine
Wajib:
- persisted reminder definitions;
- PostgreSQL-backed durable jobs;
- retries;
- idempotency;
- deduplication;
- cancellation/replacement after entity edit;
- restart safety;
- in-app notification center.

Web Push boleh setelah in-app notification stabil.

## Out of scope MVP

Jangan tambah tanpa permintaan eksplisit:
- native Android/iOS;
- payment gateway;
- bank sync;
- accounting/investment;
- family/team collaboration;
- chat/social;
- Google Calendar two-way sync;
- email scraping;
- location tracking;
- permanent KTP/passport scan storage;
- autonomous AI actions;
- voice assistant;
- microservices/Kubernetes/Kafka/Redis hanya untuk terlihat advanced.

## Preferred architecture

```text
lifehub/
├── apps/
│   └── web/                 # Next.js frontend
├── services/
│   └── api/
│       ├── cmd/api/
│       ├── cmd/worker/
│       ├── internal/
│       └── db/
├── docs/
├── compose.yaml
├── .env.example
├── AGENTS.md
├── SKILL.md
├── coldstart.md
└── README.md
```

### Web
Prefer latest stable mutually compatible:
- Next.js App Router;
- React;
- TypeScript strict;
- Tailwind CSS;
- shadcn/ui, Base UI default unless current research justifies otherwise;
- Lucide;
- Motion sparingly;
- Zod;
- React Hook Form only for forms that benefit;
- Supabase JS only for auth/session boundary.

### Go backend
Prefer:
- current patched stable Go;
- `net/http` + `chi`;
- `pgx/v5`;
- PostgreSQL;
- `sqlc`;
- Goose;
- River PostgreSQL-backed jobs;
- `log/slog`;
- explicit transactions;
- no ORM unless there is a concrete reason.

### Authentication
Preferred:
`Browser → Supabase Auth → access token → Go API → verify JWKS → ownership checks`

Rules:
- never trust `user_id` from request body;
- frontend private writes go through Go;
- never expose DB service credentials;
- verify JWT cryptographically, not just decode it.

## Go must be meaningful

Use Go for:
- auth/authorization;
- domain validation;
- Today aggregation;
- recurrence generation;
- reminder scheduling;
- durable workers;
- notification dispatch;
- idempotency;
- bounded concurrency;
- retry/backoff;
- graceful shutdown;
- health/readiness;
- transaction boundaries;
- observability.

Do not spawn uncontrolled goroutines. Important work cannot be fire-and-forget.

## Time rules

- Persist instants as PostgreSQL `timestamptz`.
- Persist user timezone as IANA value, e.g. `Asia/Jakarta`.
- Date-only document expiry should use date semantics.
- API uses ISO 8601 / RFC 3339.
- Today is calculated from local midnight to next local midnight in the **user timezone**, not server timezone.

## Recurrence scope

MVP:
- none;
- daily;
- weekly;
- monthly;
- yearly;
- interval >= 1;
- optional end date.

Do not implement full RFC 5545 unless actually needed.

## Reminder idempotency

Stable idempotency identity should derive from:
`user + entity + occurrence + reminder offset + channel`

Duplicate worker execution must not create duplicate visible notifications.

## API rules

- REST JSON under `/api/v1`.
- Consistent error envelopes.
- Validate input server-side.
- Never trust client ownership fields.
- Request IDs.
- Pagination for growing collections.
- Maintain `docs/openapi.yaml` or explicit API docs.
- No GraphQL unless proven useful.

## Smart Quick Add boundary

Interface concept:

```go
type SmartCaptureProvider interface {
    Parse(ctx context.Context, input string, now time.Time, timezone string) (Draft, error)
}
```

Implement:
1. deterministic/rule provider;
2. mock provider;
3. optional remote AI provider.

Provider:
- never writes DB;
- returns uncertainty;
- never silently resolves important ambiguity;
- always passes through normal domain validation.

## Design — “Calm Command Center”

Personality:
- calm;
- capable;
- trustworthy;
- focused;
- personal;
- slightly warm.

Starting tokens (verify contrast):

```css
:root {
  --canvas: #f5f6f2;
  --surface: #ffffff;
  --surface-soft: #eef1ec;
  --ink: #17201c;
  --muted: #66706a;
  --brand: #285f52;
  --brand-strong: #17483d;
  --accent: #d97745;
  --accent-soft: #fae9de;
  --line: #dce1db;
  --success: #2f7458;
  --warning: #9a6417;
  --danger: #b44343;
}
```

Typography candidate: **DM Sans** via `next/font` after verifying current availability.

Mobile:
- usable at 360px;
- QA at ~390×844;
- quick add thumb-reachable;
- safe-area padding;
- touch targets ~44–48px.

Desktop:
- compact navigation;
- working shell around 1120–1240px;
- do not stretch every control.

Do not use:
- purple/blue neon AI gradients;
- glassmorphism everywhere;
- glowing blobs;
- fake charts;
- giant empty hero;
- meaningless three-card feature grid;
- every element as a pill;
- stock illustrations;
- emoji as primary icons;
- generic “boost your productivity” copy.

## UX

- One dominant action per screen.
- Clear empty states.
- Critical validation inline.
- Destructive actions confirmed.
- Loading states preserve layout.
- Overdue is visible without becoming alarming.
- Do not hide key actions in unlabeled menus.
- Preserve form state where reasonable.

## Accessibility

Target WCAG 2.2 AA:
- semantic landmarks;
- logical headings;
- keyboard support;
- visible focus;
- persistent labels;
- live regions for important state changes;
- no color-only meaning;
- reduced-motion;
- 200% zoom;
- accessible date/time controls.

## Security/privacy

At minimum:
- verified JWT;
- ownership on every private entity;
- parameterized SQL;
- strict body limits;
- CORS allowlist;
- CSRF analysis based on token transport;
- secrets server-side;
- no private payloads/tokens in logs;
- rate-limit expensive/sensitive endpoints;
- dependency audit;
- `govulncheck ./...`;
- document data minimization.

## Observability/reliability

Use:
- `log/slog` structured production logs;
- request IDs;
- status + duration;
- job type/attempt/duration/result;
- `/healthz`;
- `/readyz`;
- graceful shutdown;
- DB health;
- HTTP timeouts;
- bounded worker concurrency;
- panic recovery.

Do not build a giant observability stack for appearance.

## Testing

### Go
```bash
go test ./...
go test -race ./...
go vet ./...
govulncheck ./...
```

Test:
- domain rules;
- timezone boundaries;
- recurrence;
- idempotency;
- repository integration with real PostgreSQL;
- handlers;
- workers;
- authorization.

### Web
```bash
pnpm typecheck
pnpm lint
pnpm test
pnpm build
pnpm test:e2e
```

Use Vitest + Playwright.

Critical E2E:
1. register/login;
2. open Today;
3. create task due today;
4. create recurring bill;
5. create document expiry;
6. verify Today/upcoming;
7. complete task;
8. mark bill paid;
9. edit and refresh;
10. logout and verify protection.

Test mobile ~390×844 and desktop.

## Package policy

Before installing, re-check official docs/package registries.

Never use prerelease/abandoned packages unless explicitly approved and documented.

Web verification:

```bash
pnpm view next version
pnpm view react version
pnpm view react-dom version
pnpm view typescript version
pnpm view tailwindcss version
pnpm view zod version
pnpm view motion version
pnpm view lucide-react version
pnpm view react-hook-form version
pnpm view vitest version
pnpm view @playwright/test version
pnpm view @supabase/supabase-js version
pnpm outdated
```

Go:

```bash
go version
go list -m -u all
govulncheck ./...
```

Record final installed versions in `docs/stack-versions.md`.

## Research snapshot — 19 August 2026

Context only, not permanent pins:
- Next.js 16.3.1;
- React 19.2 line, official page lists 19.2.7;
- TypeScript 7.0;
- Tailwind CSS 4.3;
- Go 1.26.6;
- Node.js 24 LTS;
- pnpm 11.x stable (pnpm 12 is RC at this snapshot);
- Zod 4.4.3;
- Motion 13.1.0;
- Vitest 4.1.10;
- Playwright 1.62.1;
- Lucide React 1.31.0;
- React Hook Form 7.85.0;
- Supabase JS 2.112.3;
- `@supabase/ssr` 0.12.4;
- chi v5.3.x;
- pgx v5.10.x;
- River v0.43.x;
- Goose v3.27.x;
- sqlc v1.31.1;
- golang-jwt v5.3.x.

Compatibility wins over novelty.

## Required documentation

Maintain:
- `README.md`
- `AGENTS.md`
- `.env.example`
- `coldstart.md`
- `docs/implementation-plan.md`
- `docs/architecture.md`
- `docs/stack-versions.md`
- `docs/security.md`
- `docs/ai-usage.md`
- `docs/deployment.md`
- API contract/docs.

## Work order

### Phase 0 — Inspect
Inspect repo, read all project docs, preserve good work, update plan and version research.

### Phase 1 — Foundation
Web + Go API + PostgreSQL + migrations + auth boundary + design system + CI.

### Phase 2 — First useful vertical slice
`Auth → timezone → create task → Today shows task → complete → persistence → tests`

### Phase 3
Events, then Bills, then Documents.

### Phase 4
Reminder engine + River worker + idempotency + notification center.

### Phase 5
Recurrence hardening.

### Phase 6
Smart Quick Add.

### Phase 7
PWA/web push only after in-app reliability.

### Phase 8
Accessibility, security, performance, deployment.

Do not generate 40 screens before the first vertical slice works.

## Completion report

At each meaningful handoff report:
1. implemented features;
2. architecture decisions;
3. files changed;
4. migrations;
5. package/module versions;
6. commands + exact results;
7. tests/manual QA;
8. security/privacy notes;
9. limitations/mocks;
10. one highest-priority next task.

Never claim something works if it was not run or tested.

## Start

1. Inspect repository.
2. Read `AGENTS.md`, `coldstart.md`, `README.md`, `SKILL.md`, and all docs.
3. Update `docs/implementation-plan.md`.
4. Verify current stable dependency versions.
5. Implement the highest-value vertical slice.
6. Continue until that slice passes relevant quality gates.
