# LifeHub

**LifeHub** is a personal life command center that brings tasks, events, recurring bills, document-expiry tracking, reminders, and notifications into one calm daily view.

> **Hal penting hari ini, ada di satu tempat.**

This project is deliberately designed as a real daily-use application rather than a portfolio-only CRUD demo. A Next.js frontend provides the product experience, while a Go backend owns domain logic, authorization, time semantics, Today aggregation, recurring schedules, durable reminder jobs, and persistence.

## Implemented vertical slice

The repository now contains one locally verified end-to-end journey:

```text
development sign-in
  → timezone onboarding
  → create a task due today
  → Go-owned Today aggregation
  → complete the task
  → reload and verify PostgreSQL persistence
```

The responsive Next.js UI, Go API, two Goose migrations, PostgreSQL 18.4 development service, unit/integration tests, and 390×844 Playwright journey are present. The production Supabase JWKS verifier is automated against an asymmetric test JWKS; hosted Supabase login and any deployment remain unverified because no external credentials were supplied.

## Product scope

The full LifeHub direction includes the domains below. Only the task/Today slice described in the status section is implemented today.

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

Implemented first slice:

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

Exact installed versions and compatibility pins are in `docs/stack-versions.md`. sqlc, River, shadcn/ui, Motion, Zod, and React Hook Form are deliberately deferred until their corresponding query, reminder, or UI complexity exists.

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
- Date-only expiry: date semantics
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

Set `TEST_DATABASE_URL` to the local PostgreSQL URL to include the real-database tests. The Windows host has no race-capable C toolchain, so `go test -race` is verified in a Go 1.26.6 Linux container; see `docs/implementation-plan.md` for the recorded result.

To make the database requirement explicit rather than silently skipping repository tests:

```powershell
$env:TEST_DATABASE_URL = 'postgres://lifehub:lifehub@localhost:55432/lifehub?sslmode=disable'
pnpm test:go:integration
```

## Current E2E

The automated mobile journey signs in with the development issuer, confirms `Asia/Jakarta`, creates and completes a task, expands completed work, reloads, and proves the state persisted. The command reconciles the loopback-only PostgreSQL service, applies migrations, and starts/stops fresh API and web processes through Playwright:

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

**First task/Today slice implemented and locally verified.** Events, bills, documents, reminders, recurrence, hosted auth, and deployment remain planned work, not completed features.
