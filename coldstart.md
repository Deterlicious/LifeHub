# LifeHub — Coldstart / Product Source of Truth

Last planning update: **19 August 2026**

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

Planning/bootstrap. Repository implementation is unknown until inspected.

## 21. First vertical slice

`Auth → timezone → create task → Today shows task → complete → persistence → tests`

Then Events → Bills → Documents → Reminder Engine.
