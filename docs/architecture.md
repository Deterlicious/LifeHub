# LifeHub Architecture

Status: task/Today, event, bill, document expiry/management, Agenda & Corrections, durable reminders/notifications, recurrence, and draft-only Smart Capture are implemented and locally verified. Hosted auth and deployment remain unverified.

## Goals

- daily-use responsiveness;
- reliable reminders;
- correct timezone behavior;
- strong user isolation;
- simple operations;
- future mobile-client compatibility.

Advanced only where correctness requires it.

## System

Implemented topology:

```text
Browser / Next.js → Go API → PostgreSQL
        Bearer JWT ────▲
```

Target topology after reminder work:

```text
Browser / Next.js
       │
       │ HTTPS JSON + auth token
       ▼
     Go API
    /      \
   ▼        ▼
PostgreSQL  durable job insertion
   ▲        │
   │        ▼
   └──── Go Worker
             │
             ▼
        Notifications

Identity:
Browser → Supabase Auth → JWT → Go verifies JWKS
```

For local development, the browser instead requests a short-lived token from an explicitly non-production HMAC issuer. Production startup never registers that route and requires HTTPS Supabase issuer/JWKS URLs. This preserves the same Bearer-token and authenticated-subject boundary without shipping an auth bypass.

## Processes

### Web
Rendering, forms, PWA/browser interactions, session acquisition, calls Go API.

### API
JWT verification, authorization, validation, domain use-cases, Today aggregation, transaction orchestration, reminder schedule creation, REST contract, health/readiness.

### Worker
Durable reminder jobs, retry, notification delivery/materialization, reconciliation.

API and worker may share one Go image with different commands.

The worker is now implemented as a separate Go command with bounded concurrency, retry, timeout, reconciliation, and graceful shutdown. API and worker share domain/store code but run as separate processes.

## Domain packages

Suggested:

```text
internal/
  today/
  tasks/
  events/
  bills/
  documents/
  reminders/
  notifications/
  smartcapture/
```

Avoid generic `services`/`utils` dumping grounds.

## Dependency direction

```text
HTTP handler
   ↓
Application service
   ↓
Domain + repository interface
   ↓
pgx store adapter
```

The current small query surface uses explicit parameterized pgx SQL. sqlc is deferred until generation materially improves safety/maintenance; database types do not leak into HTTP DTOs.

## Authentication

`Browser → Supabase Auth → token → Go → JWKS verify → subject → owned query`

All authorization happens in Go. Every implemented task, event, bill, and document query binds the verified subject as `user_id`; cross-owner task completion, bill payment, and document CRUD return the same 404 as missing data. Inserts derive ownership only from the verified subject, and real-PostgreSQL tests prove Today and document-list isolation between users.

## Today aggregation

Dedicated Go application service:

1. obtain user timezone;
2. derive local day boundaries;
3. convert boundaries to instants;
4. query relevant tasks/events/bills/documents;
5. map to `TodayItem`;
6. calculate urgency;
7. stable-sort;
8. return DTO.

Avoid aggressive caching before correctness/measurement.

The implemented aggregator receives tasks, events, bills, and documents as a discriminated feed owned and ordered by Go. Its cross-domain order is:

1. overdue tasks and bills by due time;
2. expired documents by expiry date;
3. events happening now;
4. all-day events;
5. documents expiring today;
6. timed tasks/events and bills for today by effective instant;
7. anytime tasks;
8. tasks completed and bills paid today by closed time.

Document dates retain calendar semantics. Expired and expires-today documents are primary Today attention; tomorrow through local day 30 inclusive is returned separately as `upcoming`, so future awareness does not inflate primary open/completed counts. The full document manager queries every owned record, preserving reachability beyond that horizon.

After effective-time and task-priority comparisons, stable tie-breaks use kind, creation time, and UUID. Event and bill rows are not task-shaped; events have no completion action, while bills use an explicit idempotent payment action. Timed event selection uses overlap with the user-local Today window; all-day selection uses inclusive local date ranges. Unpaid bills remain visible when overdue and paid bills remain visible only when paid within the local day.

## Time

DB:
- `timestamptz` for moments;
- `date` for all-day event ranges and date-only expiry;
- IANA timezone in profile.

API:
RFC 3339.

Never strip timezone offsets and silently assume UTC.

### Implemented event time model

The create API accepts a strict union:

- timed events: `all_day=false`, required `starts_local`, optional `ends_local`;
- all-day events: `all_day=true`, required `starts_on`, optional inclusive `ends_on`.

Go interprets timed wall-clock values in the authenticated profile timezone, rejects DST gaps/folds, and stores the resolved instants in `starts_at`/`ends_at` `timestamptz` columns. All-day values remain `starts_on`/`ends_on` PostgreSQL `date` columns so changing offsets cannot move them to a different calendar date. Database checks enforce exactly one representation and valid end ordering.

## Recurrence

The implemented model is explicit:
- frequency;
- interval;
- an immutable local anchor;
- optional inclusive end date;
- optional weekday set for weekly series.

Go validates daily, weekly, monthly, and yearly rules, then materializes a bounded 90-day occurrence window in PostgreSQL. A durable River job extends every active series twice daily. The anchor is retained so month-end clamping and leap-year handling never cause schedule drift.

Occurrence edits become explicit exceptions; occurrence deletion becomes an exclusion. Series edits regenerate only eligible future work, while stop preserves completed/paid history and cancels future reminders. Rules are represented with typed columns rather than arbitrary unvalidated recurrence JSON.

## Reminder scheduling

```text
Source entity
    ↓
Reminder definition
    ↓
Durable River job
    ↓
Worker
    ↓
Notification record
```

Job identity must support deduplication without containing sensitive content.

## Idempotency

Stable key concept:

```text
hash(user_id + entity_type + entity_id + occurrence_at + offset + channel)
```

DB enforces unique user-visible notification identity.

Retries are expected.

## Editing scheduled entities

When due/recurrence changes:
1. update source;
2. update reminder definitions;
3. cancel/replace stale future schedules;
4. insert new durable jobs;
5. commit atomically where feasible.

## Persistence

Implemented tables:
- profiles;
- tasks;
- events;
- bills;
- documents.

Also implemented:
- reminder definitions and immutable schedule generations;
- notifications;
- River job tables pinned at migration target 7.

Possible future:
- push_subscriptions;
- minimal audit events.

Avoid persistent AI raw prompts/outputs unless needed.

## SQL

Use Goose migrations + handwritten parameterized SQL + pgx. Introduce sqlc when the query surface warrants it.

No ORM by default.

## Errors

Internal typed errors map to public safe envelopes.

Example:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Periksa data yang belum valid.",
    "request_id": "req_..."
  }
}
```

Never expose SQL/stack/token/provider internals.

## Observability

Request logs:
- request_id;
- method;
- route;
- status;
- duration.

Worker logs:
- job type;
- attempt;
- duration;
- result.

Do not log private titles/notes by default.

## Reliability

API:
- graceful shutdown;
- request deadlines/timeouts;
- bounded DB pool;
- readiness.

Worker:
- bounded concurrency;
- graceful drain;
- durable queue;
- retry;
- idempotency.

## Caching

Default: no Redis.

Indexes and query design first. Add cache only after measured benefit and clear invalidation rules.

## Smart capture

The provider is isolated and cannot directly access repositories. The default rule provider deterministically parses a bounded set of Indonesian task, event, bill, document, priority, money, and recurrence phrases. A mock provider supports deterministic failure and timeout tests. No remote provider or AI credential is required.

`parse → draft → validate → response → user confirms via normal create API`

The authenticated API accepts at most 1,000 characters, resolves dates in the stored profile timezone, applies a two-second provider timeout and a 20-request/minute per-user rate limit, and validates the provider's output. It never writes domain data. The web copies the result into the existing editable form; only the normal explicit Save action can mutate data.

## Future mobile

Because semantics live in Go API, a future Flutter/native client can reuse the same domain behavior.

## Explicit non-goals

No default requirement for:
- Kafka;
- Redis;
- Kubernetes;
- service mesh;
- GraphQL;
- event sourcing;
- CQRS;
- microservices.

## Current slice deferrals

Agenda & Corrections supplies bounded event management, task/bill corrections and inverse actions, paid-history cursor pagination, and one mixed-domain chronological view while Today remains primary. The Documents slice includes metadata management but intentionally excludes file storage. Durable reminders use exact-pinned River v0.44.0 and PostgreSQL for transactionally scheduled jobs and duplicate-safe notifications. Recurrence and draft-only Smart Capture are active. Hosted Supabase validation and public deployment remain explicit later work.
