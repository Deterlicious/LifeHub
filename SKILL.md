---
name: lifehub-senior-fullstack-go-builder
description: Build, continue, review, test, secure, and deploy LifeHub, a daily-use personal life command center with a Next.js frontend and Go/PostgreSQL backend. Use this skill for Today aggregation, tasks, schedules, recurring bills, document-expiry tracking, reminders, durable jobs, notifications, smart quick-add, Supabase Auth integration, responsive UI/UX, testing, observability, and deployment. Do not use it for unrelated applications.
---

# LifeHub Senior Full-Stack Go Builder

## Mission

Act as the senior engineer accountable for delivering a trustworthy, polished, daily-use LifeHub product.

Do not maximize feature count. Optimize for:
1. correct time behavior;
2. durable reminders;
3. a clear Today experience;
4. privacy;
5. maintainable Go;
6. reliable PostgreSQL transactions;
7. accessible mobile-first UX;
8. testability;
9. explicit failure states;
10. production-quality handoff.

Product copy is Indonesian. Code, API fields, database names, and technical documentation are English unless they are user-facing strings.

## Product source of truth

LifeHub combines:
- Tasks
- Events
- Bills
- Document expiry
- Reminders
- Notifications
- Today dashboard

Promise:

> **Hal penting hari ini, ada di satu tempat.**

The user should be able to open LifeHub and quickly understand:
- what is overdue;
- what must happen today;
- what happens next;
- what bill is approaching;
- what document will expire;
- what has been finished.

## Users

Primary users:
- students;
- workers;
- freelancers;
- people managing study + work + personal obligations;
- users switching between phone and laptop.

Assume the user often opens LifeHub for less than one minute. Optimize scan speed.

## Core entities

### UserProfile

```go
type UserProfile struct {
    UserID   uuid.UUID
    Timezone string
    Locale   string
    Currency string
}
```

Defaults:
- locale: `id-ID`
- currency: `IDR`

Timezone must be explicit and editable.

### Task

```go
type Task struct {
    ID          uuid.UUID
    UserID      uuid.UUID
    Title       string
    Notes       *string
    Priority    Priority
    DueAt       *time.Time
    CompletedAt *time.Time
    Recurrence  *RecurrenceRule
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### Event

```go
type Event struct {
    ID         uuid.UUID
    UserID     uuid.UUID
    Title      string
    Notes      *string
    Location   *string
    StartsAt   time.Time
    EndsAt     *time.Time
    AllDay     bool
    Recurrence *RecurrenceRule
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

### Bill

```go
type Bill struct {
    ID         uuid.UUID
    UserID     uuid.UUID
    Title      string
    Amount     int64
    Currency   string
    DueAt      time.Time
    PaidAt     *time.Time
    Recurrence *RecurrenceRule
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

Money uses integer minor units. For IDR, one unit is one rupiah.

### Document

MVP tracks metadata/expiry, not sensitive file scans.

```go
type Document struct {
    ID        uuid.UUID
    UserID    uuid.UUID
    Name      string
    Category  string
    Notes     *string
    ExpiresOn *DateOnly
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### Reminder

```go
type Reminder struct {
    ID         uuid.UUID
    UserID     uuid.UUID
    EntityType EntityType
    EntityID   uuid.UUID
    OffsetSec  int64
    Channel    ReminderChannel
    Enabled    bool
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

### Notification

Notification is a delivery artifact, not reminder truth.

Support:
- idempotency key;
- source entity;
- occurrence;
- channel;
- state;
- read_at;
- created_at.

## Required capabilities

### Today

Today combines all domains into one ordered feed.

Do not render four unrelated widgets when a timeline is clearer.

Concept:

```go
type TodayItem struct {
    Kind      TodayItemKind
    EntityID  uuid.UUID
    Title     string
    StartsAt  *time.Time
    DueAt     *time.Time
    Amount    *int64
    Priority  *Priority
    Status    string
    Urgency   Urgency
}
```

Ordering must be deterministic.

Recommended initial priority:
1. overdue;
2. happening now;
3. due soon today;
4. scheduled later today;
5. low-priority anytime items.

Document and test the final algorithm.

### Tasks
Create, edit, delete, complete/uncomplete, due, priority, simple recurrence, reminders.

### Events
Create, edit, delete, start/end, all-day, simple recurrence, reminders.

### Bills
Create, edit, delete, integer amount, due, recurrence, paid/unpaid, next occurrence, reminders.

Do not turn bills into accounting.

### Documents
Create, edit, delete, category, expiry date, reminder, status.

### Reminder engine
Persistent and restart-safe.

## Operating procedure

1. Read `AGENTS.md`.
2. Read `coldstart.md`.
3. Read project docs.
4. Inspect repo, package manifests, Go modules, migrations, scripts, existing UI.
5. Preserve good existing work.
6. Update `docs/implementation-plan.md`.
7. Verify dependency versions.
8. Build the smallest production-worthy vertical slice.
9. Test domain behavior.
10. Implement UI against explicit API behavior.
11. Run relevant checks after each slice.
12. Run full gates before handoff.
13. Update docs when implementation reality changes.
14. Report failures; do not hide them.
15. Never claim untested work is complete.

## Dependency policy

Before frontend install/update:

```bash
pnpm view <package> version
pnpm view <package> peerDependencies
pnpm outdated
```

Before Go update:

```bash
go list -m -u all
go mod tidy
govulncheck ./...
```

Do not add a dependency when:
- standard library is sufficient;
- current dependency already solves the need;
- package is abandoned;
- package only saves a tiny helper;
- it duplicates another abstraction.

Prefer fewer high-quality dependencies.

## Research snapshot — 19 August 2026

Re-check at implementation time.

Frontend:
- Next.js 16.3.1
- React 19.2 line; official page lists 19.2.7
- TypeScript 7.0
- Tailwind CSS 4.3
- Zod 4.4.3
- Motion 13.1.0
- Vitest 4.1.10
- Playwright 1.62.1
- Lucide React 1.31.0
- React Hook Form 7.85.0
- Supabase JS 2.112.3
- @supabase/ssr 0.12.4
- pnpm 11.x stable
- Node.js 24 LTS

Backend:
- Go 1.26.6
- chi v5.3.x
- pgx v5.10.x
- River v0.43.x
- Goose v3.27.x
- sqlc v1.31.1
- golang-jwt v5.3.x

Compatibility beats novelty. Final installed versions belong in `docs/stack-versions.md`.

## Preferred repository architecture

```text
apps/
  web/
    src/
      app/
        (auth)/
        (app)/
          today/
          tasks/
          calendar/
          bills/
          documents/
          notifications/
      components/
        app-shell/
        today/
        tasks/
        events/
        bills/
        documents/
        quick-add/
        ui/
      lib/
        api/
        auth/
        dates/
        currency/
    tests/

services/
  api/
    cmd/
      api/
      worker/
    internal/
      auth/
      config/
      httpx/
      today/
      tasks/
      events/
      bills/
      documents/
      reminders/
      notifications/
      smartcapture/
      jobs/
      store/
      observability/
    db/
      migrations/
      queries/
      generated/
    integration/

docs/
```

Adapt to the repository; do not force this tree blindly.

## Go backend principles

### HTTP
Prefer standard `net/http` with chi.

Configure:
- ReadHeaderTimeout;
- sensible Read/Write/Idle timeout behavior;
- MaxHeaderBytes;
- graceful shutdown.

### Context
Every DB query/external call receives context.
Never store `context.Context` in a struct.

### Errors
Use typed internal errors and map them to safe API errors.
Never leak SQL/stack/token/provider errors.

### Concurrency
Good:
- bounded worker pools;
- limited notification fanout;
- graceful shutdown coordination.

Bad:
```go
go func() {
    // fire-and-forget important work
}()
```

Important work must be durable.

### Transactions
Entity update + reminder rescheduling should be atomic where feasible.

### Idempotency
Worker handlers assume at-least-once execution. Side effects need stable keys.

## PostgreSQL rules

Use DB constraints as a second validation layer:
- ownership foreign keys;
- check constraints;
- unique idempotency keys;
- indexes such as `(user_id, due_at)` when query patterns justify them;
- partial indexes for incomplete/unpaid records where useful.

Do not create indexes without a query reason.

## SQL strategy

Prefer explicit SQL + sqlc + pgx.

Avoid generic CRUD repositories.

Example:

```go
type TaskRepository interface {
    Create(ctx context.Context, params CreateTaskParams) (Task, error)
    GetOwned(ctx context.Context, userID, taskID uuid.UUID) (Task, error)
    ListToday(ctx context.Context, userID uuid.UUID, window TimeWindow) ([]Task, error)
}
```

## Authentication

Preferred:

```text
Browser
  ↓ Supabase Auth
Access token
  ↓
Go API
  ├ verify JWKS signature
  ├ validate issuer/time claims
  ├ extract subject
  └ enforce ownership
```

Rules:
- never trust decoded-but-unverified tokens;
- cache JWKS safely;
- fail closed;
- never trust request-body user ID;
- no service key in browser;
- document cookie vs bearer transport and CSRF implications.

## Time model

This is a critical subsystem.

### Moments
Use PostgreSQL `timestamptz`.

### Date-only
Use `date` semantics for expiry.

Do not turn a date-only expiry into midnight UTC and accidentally shift it in Indonesia.

### User timezone
Store IANA timezone.

### Today window
`00:00 user timezone → next local midnight`, then convert boundaries for querying instants.

Never use server local date as user Today.

## Recurrence

MVP:
```text
frequency: daily | weekly | monthly | yearly
interval: integer >= 1
starts_at
optional ends_on
```

Optional weekdays can be added when required.

Test:
- month-end;
- leap year;
- DST-capable zones even if initial users are Indonesia-based;
- edits/deletes;
- bill payment behavior.

Do not implement full RRULE prematurely.

## Reminder engine

Recommended:

```text
Entity saved
    ↓
Reminder definitions persisted
    ↓
Jobs inserted durably
    ↓
River PostgreSQL queue
    ↓
Go Worker
    ↓
Notification service
    ↓
notifications table
    ↓
in-app / optional web push
```

Required:
- durable;
- retry-safe;
- bounded;
- idempotent;
- observable;
- cancellable/replaceable after source edits.

In-memory cron/timers are not the source of truth.

## Notification state

Suggested:
```text
scheduled
processing
delivered
failed
cancelled
```

Browser push success does not mean a human saw the reminder.

## Today aggregation

Prefer a Go application service that:
1. calculates user-local day boundaries;
2. loads relevant domain items;
3. maps to common Today items;
4. calculates urgency;
5. stable-sorts;
6. returns DTO.

This keeps future mobile clients consistent.

## Smart Capture

Concept:

```go
type SmartCaptureProvider interface {
    Parse(ctx context.Context, input string, now time.Time, timezone string) (Draft, error)
}
```

Implement:
- rule provider;
- mock provider;
- optional remote AI provider.

Provider cannot call repositories directly.

Flow:
`parse → draft → validate → response → user confirms through normal create API`

## API

Base `/api/v1`.

Examples:
```text
GET    /api/v1/today
GET    /api/v1/tasks
POST   /api/v1/tasks
PATCH  /api/v1/tasks/{id}
DELETE /api/v1/tasks/{id}

GET    /api/v1/events
POST   /api/v1/events

GET    /api/v1/bills
POST   /api/v1/bills
POST   /api/v1/bills/{id}/mark-paid

GET    /api/v1/documents
POST   /api/v1/documents

GET    /api/v1/notifications
POST   /api/v1/notifications/{id}/read

POST   /api/v1/smart-capture/parse
```

Use action endpoints when an operation is domain-significant.

## Frontend

Use Server Components by default.
Client Components only where browser interaction/state requires them.

Do not add TanStack Query automatically. Evaluate only if server-state complexity genuinely appears.

## Forms

Use native semantics for simple forms.
Use React Hook Form when recurrence/reminder/conditional complexity makes it objectively simpler.
Zod is useful on the frontend; Go remains authoritative.

## Design — Calm Command Center

Personality:
- calm;
- warm-neutral;
- high signal;
- low noise;
- modern daily utility.

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

Use Lucide icons. Use borders before heavy shadows. Pills only for statuses/compact filters.

## Today UI

Preferred mobile concept:

```text
Wed, 19 Aug
Selamat malam

Today
────────────────
08:00  Submit laporan
14:00  Meeting proyek
18:00  Internet · Rp350.000

Anytime
       Backup laptop

[ + Tambah ]
```

Avoid a fake analytics dashboard.

## Navigation

Mobile:
- Today
- Tasks
- Calendar
- More

More:
- Bills
- Documents
- Notifications
- Settings

Quick Add is a distinct primary action.

## Public/auth copy

If a concise landing page exists:

Title:
**Jangan simpan semuanya di kepala.**

Subtitle:
**LifeHub menyatukan tugas, jadwal, tagihan, dokumen penting, dan pengingat dalam satu tempat.**

Primary CTA:
**Mulai atur harimu**

Do not make marketing larger than the product.

## Accessibility

Target WCAG 2.2 AA.
Test keyboard, focus, labels, live regions, 200% zoom, reduced motion, mobile targets, form errors, date/time semantics.

## Security

Before production:
- JWT verified;
- ownership server-side;
- CORS configured;
- no secrets in client;
- no service-role key in browser;
- DB TLS;
- parameterized SQL;
- body limits;
- secure headers;
- logs scrubbed;
- dependency auditing;
- `govulncheck`;
- account/data deletion path documented;
- backup/restore plan documented.

## Testing matrix

Domain:
- task due/no due, complete/uncomplete, recurrence;
- bill integer amount, recurrence, mark paid;
- document valid/expiring/expired, date-only correctness;
- Today ordering, midnight, timezone;
- reminder duplicate/retry/edit/delete/restart;
- smart capture clear/ambiguous/timeout/invalid/manual fallback.

## DB integration

Do not mock PostgreSQL for everything.
Use a real ephemeral/test PostgreSQL for repositories and transaction behavior.

Migration test:
1. empty DB;
2. migrate up;
3. integration test;
4. optional safe down test.

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
go test ./...
go test -race ./...
go vet ./...
govulncheck ./...
```

Repo:
```bash
git diff --check
```

## Performance

- no Today N+1;
- bounded DB pool;
- query timeouts;
- paginate history;
- avoid giant client bundle;
- no Redis until a measured reason exists.

## PWA/offline

PWA is polish.
Safe offline:
- app shell;
- static assets;
- clear offline state.

Do not pretend writes succeeded offline without a real sync queue.

## Deployment

Keep container-first portability.

Deployable units:
```text
web
api
worker
managed PostgreSQL
```

API and worker may share one image with different commands.

Check current hosting documentation/costs at deployment time; do not assume old free tiers.

## Required docs

Maintain README, AGENTS, coldstart, implementation plan, architecture, stack versions, security, AI usage, deployment, and API contract.

Documentation describes reality, not aspiration.

## Completion report

Report:
1. what was implemented;
2. architecture decisions;
3. files/migrations;
4. commands/results;
5. versions;
6. tests/manual QA;
7. limitations;
8. mock/fallback status;
9. exact next priority.

Never silently skip failed checks.
