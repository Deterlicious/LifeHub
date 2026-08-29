# LifeHub — Coldstart / Product Source of Truth

Last planning update: **29 August 2026**

## 1. Product

**LifeHub** — Personal Productivity / Life Management / Helper Tool.

One-line:
LifeHub unifies tasks, schedules, recurring bills, document-expiry reminders, notifications, and one **Today** view.

Promise:

> **Hal penting hari ini, ada di satu tempat.**

## 2. Problem

Important obligations are fragmented across notes, calendars, chats, screenshots, browser tabs, reminders, and memory.

Consequences:
- forgotten tasks;
- late bills;
- missed events;
- expired documents;
- mental overhead.

LifeHub must reduce the effort of remembering, not become another app to maintain.

## 3. Primary user

A student/worker/freelancer who:
- manages study, work, and personal obligations;
- uses phone and laptop;
- wants a quick daily overview;
- needs recurring reminders to be trustworthy;
- does not need a heavy project-management tool.

## 4. Jobs to be done

- “When I start my day, show me everything important in one order.”
- “When I remember something, let me capture it quickly.”
- “When a recurring bill or expiry approaches, remind me reliably.”
- “When I finish/pay something, reflect the state correctly.”

## 5. MVP

### Today
Unified feed of overdue, tasks, events, bills, expiries, quick add, upcoming.

### Tasks
Title, notes, priority, due, completion, recurrence, reminders.

### Events
Title, notes/location, start/end, all-day, recurrence, reminders.

### Bills
Title, integer amount, due, recurrence, paid state, reminders.

### Documents
Name, category, notes, expiry, reminders, expiry state.

### Cross-cutting
Durable reminder + notification engine.

### Progressive enhancement
Smart Quick Add: natural language → editable draft.

## 6. Out of scope

- native mobile;
- bank sync/payment gateway;
- accounting/investing;
- collaboration/chat;
- full Google Calendar sync;
- email scraping;
- location tracking;
- sensitive document file storage;
- autonomous AI;
- voice assistant;
- architecture theater.

## 7. Core flows

First-time:
`Auth → timezone → Today empty state → add first item`

Daily:
`Open → Today → review → complete/pay/open → Quick Add`

Create:
`+ Tambah → type → structured form → reminder/recurrence → save`

Smart:
`text → parse → draft → review → explicit save`

## 8. Pages

- Today
- Tasks
- Calendar
- Bills
- Documents
- Notifications
- Settings

Mobile primary navigation:
- Today
- Tasks
- Calendar
- More

Quick Add is a primary action.

## 9. Today hierarchy

1. date/greeting;
2. overdue/urgent when present;
3. ordered Today timeline;
4. quick add;
5. upcoming;
6. small progress summary.

Avoid disconnected analytics cards.

## 10. Visual direction

**Calm Command Center**

Personality:
calm, capable, trustworthy, clean, personal, modern, slightly warm.

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

Typography candidate: DM Sans via `next/font`, verify at implementation.

Do not use neon AI gradients, glassmorphism everywhere, fake charts, stock illustrations, random emoji, giant empty hero, or everything-as-pill.

## 11. Technical decision

Frontend:
Next.js + React + TypeScript + Tailwind + shadcn/Base UI.

Backend:
Go + chi + pgx + PostgreSQL + sqlc + Goose + River + slog.

Auth preferred:
Supabase Auth → JWT → Go verifies JWKS → Go enforces ownership.

Private data writes go through Go.

## 12. Why Go

Go owns:
- auth/authorization;
- Today aggregation;
- recurrence;
- durable reminder jobs;
- worker concurrency;
- retry/idempotency;
- notifications;
- transactions;
- graceful shutdown;
- health/readiness.

## 13. Time

- user timezone is explicit IANA;
- moments stored as `timestamptz`;
- all-day event ranges use date semantics with an inclusive end date;
- document expiry is date-only where appropriate;
- Today uses user-local boundaries;
- server timezone must not define user Today.

## 14. Money

Integer only. IDR default. Never float.

## 15. Recurrence MVP

`none | daily | weekly | monthly | yearly`, interval >= 1, optional end.

No full RFC 5545 initially.

## 16. Reminder architecture

`Entity → Reminder Definition → Durable PostgreSQL Job → Go Worker → Notification`

Requirements:
persistent, retry-safe, deduplicated, idempotent, edit/delete aware, restart-safe.

## 17. Smart Quick Add

Provider output is untrusted and creates draft only.

Examples:
- `Bayar internet 350 ribu tanggal 15 tiap bulan`
- `Meeting besok jam 2`
- `SIM habis 6 November, ingatkan sebulan sebelumnya`

Manual structured fallback always exists.

## 18. Privacy

MVP does not need document files.
Avoid full sensitive identifiers and private payload logging.
All private rows are user-owned and server-authorized.

## 19. Success criteria

- authentication works;
- timezone works;
- Today aggregates correctly;
- task/event/bill/document flows work;
- reminders survive restarts;
- duplicate jobs do not duplicate visible notifications;
- responsive mobile UX;
- persistence works;
- quality gates pass;
- public deployment works.

## 20. Current status

The repository has been inspected and the task/Today vertical slice is implemented and locally verified:

`development auth → timezone → create task → Today → complete → reload persistence`

The focused **Events → Today** create slice is also implemented and locally verified:

`authenticated user → create timed/all-day event → PostgreSQL → unified Today → reload persistence`

The focused **Bills → Today** create/payment slice is implemented and locally verified:

`authenticated user → create integer bill → unified Today → mark paid → reload persistence`

The focused **Documents → Today and management** slice is also implemented and locally verified:

`authenticated user → create date-only metadata → Today/Upcoming → full manager edit/clear/delete → reload persistence`

Their API, ownership, time/money/date semantics, mixed Today ordering, mobile browser flow, build, database integration, vulnerability, and Linux race checks have run successfully. Agenda & Corrections is also complete locally: the unified Agenda, strict task/event/bill corrections, inverse actions, cursor history, browser navigation, and mobile/desktop persistence journeys are verified. Durable one-off reminders and in-app notifications are complete locally across all four source domains. Daily/weekly/monthly/yearly recurrence is complete locally for tasks, events, and bills, including durable 90-day materialization, exceptions/exclusions, series edit/stop, and calendar-edge coverage. Draft-only Smart Quick Add is also complete locally with deterministic Indonesian parsing, bounded validation, ambiguity review, rate limiting, timeout handling, and explicit-save browser proof.

## 21. First vertical slice

`Auth → timezone → create task → Today shows task → complete → persistence → tests`

This slice is complete for the local development provider. Hosted Supabase sign-in remains unverified because project credentials were not supplied.

## 22. Implemented Events → Today contract

Timed event input uses:

```text
all_day = false
starts_local = local wall-clock in the profile timezone
ends_local = optional local wall-clock in the profile timezone
```

All-day event input uses:

```text
all_day = true
starts_on = YYYY-MM-DD
ends_on = optional inclusive YYYY-MM-DD
```

The two shapes are strict alternatives. Go interprets timed values in the authenticated profile's IANA timezone and persists the resulting instants as `timestamptz`. All-day events preserve `date` semantics. Ownership comes only from the verified subject, never browser input.

The implemented unified Today order is:

1. overdue tasks and bills;
2. expired documents;
3. events happening now;
4. all-day events;
5. documents expiring today;
6. timed tasks/events and bills for today by effective instant;
7. anytime tasks;
8. tasks completed and bills paid today.

Stable tie-breaks use kind, creation time, and UUID after the relevant effective-time/priority comparison.

Agenda & Corrections subsequently implemented bounded event list/get/edit/delete, strict timed/all-day schedule replacement, and the mixed-domain Agenda. The durable reminder phase added moment/date reminders and restart-safe notifications. Recurrence now uses typed rules plus bounded materialization rather than arbitrary recurrence JSON, and Smart Quick Add remains draft-only. Hosted Supabase validation and public deployment remain explicit follow-up work.

## 23. Implemented Bills → Today contract

Bill creation uses a positive integer `amount`, an optional three-letter uppercase `currency` defaulting to `IDR`, and required `due_local`. Go resolves the wall-clock due time in the authenticated profile's stored IANA timezone and persists `due_at` as `timestamptz`. Floating-point and out-of-safe-range amounts are rejected.

`mark-paid` is an explicit, ownership-scoped, idempotent action that preserves the first `paid_at`. Today returns every owned unpaid bill due before the local day ends plus bills paid within that local day; it does not silently cap the feed.

Agenda & Corrections subsequently implemented cursor-paginated bill list/history, get/edit/delete, and idempotent `mark-unpaid` through one chronological experience rather than separate admin dashboards. One-off reminders and recurring bill series are implemented through their dedicated shared engines.

## 24. Implemented Documents → Today and management contract

Document storage is metadata-only: owned name, category, optional notes, and `expires_on date`. The API and UI do not accept scans, uploaded files, or sensitive document numbers.

Go derives status using the authenticated profile's local calendar date:

- before Today: `expired` and included in primary Today;
- Today: `expiring` and included in primary Today;
- tomorrow through day 30 inclusive: `expiring` and included only in the separate Upcoming feed;
- after day 30: `valid` and still reachable through the uncapped owned-document manager.

Create/list/get/PATCH/DELETE are implemented with ownership predicates. PATCH distinguishes omitted fields from `notes: null`; the latter clears notes. The responsive manager supports edit, explicit delete confirmation, reload persistence, and an owned date-based reminder control backed by River.
