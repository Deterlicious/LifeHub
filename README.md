# LifeHub

**LifeHub** is a personal life command center that brings tasks, events, recurring bills, document-expiry tracking, reminders, and notifications into one calm daily view.

> **Hal penting hari ini, ada di satu tempat.**

This project is deliberately designed as a real daily-use application rather than a portfolio-only CRUD demo. A Next.js frontend provides the product experience, while a Go backend owns domain logic, authorization, time semantics, Today aggregation, recurring schedules, durable reminder jobs, and persistence.

## Implemented vertical slices

The repository now contains eight locally verified product journeys plus a dedicated desktop-layout regression check:

```text
development sign-in
  → timezone onboarding
  → create a task due today
  → Go-owned Today aggregation
  → complete the task
  → reload and verify PostgreSQL persistence

authenticated user
  → create a timed or all-day event
  → Go validates the user's local-time/date input
  → PostgreSQL persists the owned event
  → Go merges it into Today
  → reload and verify event persistence

authenticated user
  → create an integer-IDR bill due today
  → Go merges it into Today
  → mark it paid idempotently
  → reload and verify the paid state

authenticated user
  → create date-only document metadata
  → Go separates Today attention from the 30-day Upcoming horizon
  → reach, edit, clear notes, and delete metadata outside that horizon
  → reload and verify PostgreSQL persistence

authenticated user
  → browse a profile-local, mixed-domain Agenda
  → edit or delete owned tasks, events, and bills in one shared sheet
  → undo task completion or bill payment idempotently
  → navigate with browser history and reload to verify persistence

authenticated user
  → configure an owned task/event/bill/document reminder
  → PostgreSQL and River retain the scheduled job
  → a restartable Go worker produces exactly one in-app notification
  → mark it read and reload to verify persisted state

authenticated user
  → create a daily/weekly/monthly/yearly recurring task, event, or bill
  → Go materializes a bounded PostgreSQL occurrence window
  → edit/exclude one occurrence or edit/stop future series work
  → reload and verify history plus future state remain correct

authenticated user
  → describe work in Indonesian through Smart Quick Add
  → receive a validated draft with explicit ambiguities
  → review/edit the ordinary structured form without any automatic write
  → save through the normal owned Go API
```

The responsive Next.js UI, Go API, restartable Go worker, seven Goose migrations, River schema pinned at version 7, PostgreSQL 18.6 development service, unit/integration tests, and nine mobile/desktop Playwright cases are present. Agenda and all correction APIs are uncapped or explicitly cursor-paginated, ownership-scoped, and tested across ordinary DST changes, midnight gaps, and skipped civil dates. A Supabase Free project is provisioned, its ES256 JWKS is verified, and the application plus River migrations passed against the hosted database. The Netlify production site is live at `https://rainbow-cocada-24a2a9.netlify.app`; hosted session recovery, Today/Agenda loading, and an authenticated task write/read cycle have been manually verified. Explicit hosted sign-out/re-login, backup/restore proof, and the scheduled reminder delivery smoke remain release work.

## Events → Today

The focused create-to-Today event slice is implemented and locally verified:

```text
authenticated user
  → create a timed or all-day event
  → Go validates the user's local-time/date input
  → PostgreSQL persists the owned event
  → Go merges it into Today
  → reload preserves the event
```

The agreed create contract uses `starts_local`/`ends_local` for timed events and `starts_on`/`ends_on` for all-day events. Timed values are interpreted in the authenticated profile's IANA timezone and stored as instants; all-day values retain date semantics, with `ends_on` inclusive. These are strict alternative input shapes, not interchangeable fields.

Agenda & Corrections subsequently added bounded event list/get/edit/delete and strict schedule replacement without turning LifeHub into a generic event admin page. Recurrence and reminders are implemented through the shared series and durable-job engines. Supabase JWKS, database migrations, public deployment, hosted session recovery, and an authenticated production task cycle are verified. No additional event-specific dependency was needed.

## Bills → Today

The focused bill create/payment slice is implemented and locally verified. Go accepts only positive integer JSON amounts up to JavaScript's safe-integer limit, defaults currency to `IDR`, resolves `due_local` in the stored IANA timezone, and stores the instant as `timestamptz`. Today includes every owned unpaid overdue/due-today bill plus bills paid during the local day. `mark-paid` is ownership-scoped and idempotently preserves the first payment time.

The responsive Quick Add and Today row display Indonesian rupiah, overdue/paid state, and a dedicated `Bayar` action. Agenda & Corrections added cursor-paginated bill history, get/edit/delete, and idempotent `mark-unpaid` without adding a separate Bills dashboard. Bills can now use the shared recurrence and durable-reminder engines.

## Documents → Today and management

The metadata-only document slice is implemented and locally verified. Go treats expiry as a profile-local calendar date: expired and expiring-today records appear in primary Today, tomorrow through day 30 inclusive appears in a separate Upcoming section, and later records remain reachable in the full owned-document manager. The UI can create, list, edit, clear notes, and explicitly delete metadata without accepting scans or sensitive document numbers.

Document queries and mutations are ownership-scoped, private responses use `Cache-Control: no-store`, and date/status boundaries are covered by unit, HTTP, real-PostgreSQL, Linux race, and mobile reload-persistence tests. Documents now support date-based reminders through the same durable engine used by the other domains.

## Durable reminders and notifications

Owned one-off reminders are implemented for tasks, timed/all-day events, bills, and documents. Moment reminders use a relative minute offset; date-only reminders use a day offset plus an explicit local wall-clock time. Go validates the schedule against the source shape and authenticated profile timezone, then atomically persists an immutable schedule generation and River job in PostgreSQL.

Source schedule changes, completion/payment state, deletion, and timezone changes cancel or replace stale generations transactionally. The separate bounded worker rehydrates private source data only at delivery time, retries failures, reconciles discarded jobs, and relies on a database uniqueness constraint so retry/restart cannot create duplicate visible notifications. The web notification center supports a persisted unread count, stable cursor pagination, mark-one/all-read, and visible-page polling without Web Push.

## Recurrence

Tasks, timed/all-day events, and bills support validated daily, weekly, monthly, and yearly series with an interval and optional inclusive end. Go materializes a bounded 90-day window and a durable twice-daily River sweep extends active series. The original local anchor prevents month-end, leap-year, and DST drift; individual edits become exceptions, deletion becomes an exclusion, series edits affect eligible future work, and stop preserves completed/paid history while cancelling future work and reminders.

## Smart Quick Add

Smart Quick Add is a draft accelerator, not an autonomous writer. Its authenticated endpoint accepts a maximum of 1,000 characters, uses the stored IANA timezone, enforces a two-second provider timeout and a 20-request/minute per-user limit, validates the returned draft, and performs no domain mutation. The built-in deterministic Indonesian rule provider and mock provider need no AI credentials. The UI fills the existing editable form, surfaces ambiguities, and saves only after the user presses the ordinary Save action.

## Product scope

The full LifeHub direction includes the domains below. The task, event, bill, document expiry/management, Agenda & Corrections, durable reminder, recurrence, and draft-only Smart Quick Add journeys described above are implemented and locally verified. Supabase Auth/JWKS, hosted database migrations, the public Netlify site, hosted session recovery, and an authenticated production task cycle are verified; scheduled reminder delivery and the remaining operational gates are still pending.

### Today
One ordered daily feed:
- overdue;
- tasks;
- events;
- bills due soon;
- document expiry;
- quick actions;
- upcoming preview.

### Tasks
Priority, due date/time, completion, recurrence, reminders.

### Events
Start/end, all-day, optional location/notes, recurrence, reminders, timezone-safe display.

### Bills
Integer amount, currency, due date, recurrence, paid state, reminders.

### Documents
Metadata + expiry tracking. MVP intentionally does **not** store sensitive document scans.

### Reminder engine
Persisted reminders handled by a Go worker with PostgreSQL-backed durable jobs.

### Smart Quick Add
Optional:
`Bayar internet 350 ribu tanggal 15 tiap bulan`
→ editable structured draft.
AI never saves automatically.

## Architecture

Implemented slices:

```text
Browser → Next.js UI → Go API → PostgreSQL
             │             ▲
             └─ Bearer JWT ┘
```

Target after reminder work:

```text
Next.js Web
    │ HTTPS JSON + auth token
    ▼
Go API ───────────────► PostgreSQL
    │                      ▲
    │ durable jobs         │
    └──────────────► Go Worker
                        │
                        ▼
                  Notifications

Auth:
Browser → Supabase Auth → JWT → Go verifies JWKS
```

## Current first-slice stack

### Web
- Next.js
- React
- TypeScript strict
- Tailwind CSS
- Lucide
- Supabase JS for auth/session
- Vitest
- Playwright

### Backend
- Go
- `net/http`
- chi
- pgx/v5
- Goose
- PostgreSQL
- `log/slog`

Exact installed versions and compatibility pins are in `docs/stack-versions.md`. River is now pinned for the durable reminder worker; sqlc, shadcn/ui, Motion, Zod, and React Hook Form remain deferred until their corresponding query or UI complexity exists.

## Run locally

Prerequisites: Docker Desktop, Node.js matching the root engine range, Corepack/pnpm, and Go with `GOTOOLCHAIN=auto`.

```powershell
corepack enable
pnpm install --frozen-lockfile
docker compose up -d postgres

$env:APP_ENV = 'development'
$env:HTTP_ADDR = '127.0.0.1:8080'
$env:DATABASE_URL = 'postgres://lifehub:lifehub@localhost:55432/lifehub?sslmode=disable'
$env:WEB_ORIGIN = 'http://127.0.0.1:3000'
$env:DEV_AUTH_SECRET = 'replace-this-local-secret-with-32-bytes'

pnpm migrate:api
pnpm dev:api
```

In a second PowerShell terminal:

```powershell
Copy-Item apps/web/.env.example apps/web/.env.local
pnpm dev:web
```

Open `http://127.0.0.1:3000`. Development authentication is visibly labelled and cannot be enabled when `APP_ENV=production`.

## Why Go exists here

Go is not a CRUD wrapper. It owns:
- verified identity/authorization;
- Today aggregation;
- recurrence;
- reminder scheduling;
- durable workers;
- retries/idempotency;
- notifications;
- transaction boundaries;
- graceful shutdown;
- health/readiness.

## Time rules

- Absolute moments: PostgreSQL `timestamptz`
- User timezone: IANA, e.g. `Asia/Jakarta`
- All-day events and document expiry: date semantics
- API: RFC 3339 / ISO 8601
- Today: calculated in user timezone

## Money

For IDR:
`Rp350.000 → 350000`

Never store bill amounts as float.

## Repository direction

```text
.
├── apps/web/
├── services/api/
│   ├── cmd/api/
│   ├── cmd/worker/
│   ├── internal/
│   └── db/
├── docs/
├── compose.yaml
├── .env.example
├── AGENTS.md
├── SKILL.md
├── coldstart.md
└── README.md
```

## Development philosophy

1. Build small vertical slices.
2. Keep the app runnable.
3. Correctness before unnecessary features.
4. Manual flow works without AI.
5. Important background work is durable.
6. Explicit SQL/transactions over hidden magic.
7. Document decisions.
8. Never claim untested work is complete.

## Quality gates

Web:
```bash
pnpm typecheck
pnpm lint
pnpm test
pnpm build
pnpm test:e2e
```

Go:
```bash
go -C services/api test ./...
go -C services/api vet ./...
go -C services/api tool govulncheck ./...
```

Set `TEST_DATABASE_URL` to the local PostgreSQL URL to include the real-database tests. The Windows host has no race-capable C toolchain, so the final `go test -race` gate runs in a Go 1.27.0 Linux environment (WSL locally or the CI job); see `docs/implementation-plan.md` for the latest recorded result.

To make the database requirement explicit rather than silently skipping repository tests:

```powershell
$env:TEST_DATABASE_URL = 'postgres://lifehub:lifehub@localhost:55432/lifehub?sslmode=disable'
pnpm test:go:integration
```

## Current E2E

The nine Playwright cases cover the four-domain first journey, explicit whole-LifeHub-data deletion, mobile/desktop Agenda corrections, mobile/desktop reminder persistence, mobile recurrence, mobile Smart Quick Add review-before-save, and a 1920px Today-layout regression for the Upcoming placement plus non-overlapping due-date/priority controls. The command expects PostgreSQL on the documented loopback URL, applies both application and River migrations, and starts/stops fresh API, worker where required, and web processes through Playwright:

```bash
pnpm test:e2e
```

## Full-product E2E target

1. Sign in.
2. Open Today.
3. Create task due today.
4. Create recurring bill.
5. Add document expiry.
6. Verify views.
7. Complete task.
8. Mark bill paid.
9. Edit and refresh.
10. Sign out and verify private route protection.

## Privacy

LifeHub may contain private data:
- minimize collection;
- do not store document scans in MVP;
- enforce ownership in Go;
- never log tokens/private notes;
- keep secrets server-side;
- TLS in production.

The settings dialog includes an exact typed-confirmation action to remove every LifeHub application row owned by the verified user and cancel queued reminder jobs before signing out. It intentionally does not claim to delete the separate Supabase authentication identity.

## Documentation

- `coldstart.md` — product source of truth
- `docs/implementation-plan.md`
- `docs/architecture.md`
- `docs/stack-versions.md`
- `docs/security.md`
- `docs/ai-usage.md`
- `docs/deployment.md`
- `docs/api.md`

## Status

**Task/Today, event, bill, document expiry/management, Agenda & Corrections, durable reminders/notifications, recurrence, and draft-only Smart Quick Add are implemented and locally verified.** Supabase Free, ES256 JWKS, hosted migrations, and the public Netlify static-web plus Go Function deployment are verified. The production browser recovered an existing Supabase session, loaded Today/Agenda, and persisted an authenticated task create/complete/uncomplete cycle. The scheduled worker remains deliberately disabled until a live reminder smoke is completed. Both Render profiles are retained as unselected references and must not be provisioned while the strict zero-charge policy remains active.
