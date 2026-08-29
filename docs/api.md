# LifeHub API Contract

Status: **implemented task/Today, event, bill, document expiry/management, Agenda & Corrections, reminder/notification, recurrence, and Smart Capture contracts**. If an OpenAPI document is introduced, it becomes the machine-readable source of truth and this file should link to it.

Base path:

```text
/api/v1
```

All private endpoints require a verified authenticated user.

## Conventions

### Content type

```text
application/json
```

### Time

- Instants use RFC 3339 / ISO 8601 with timezone information.
- Date-only values use `YYYY-MM-DD`.
- Server normalizes absolute instants internally.
- User-local Today semantics come from the user's stored IANA timezone.

### Money

Integer minor units.

For IDR:

```json
{
  "amount": 350000,
  "currency": "IDR"
}
```

means Rp350.000.

### Ownership

Do not accept `user_id` as authority from request bodies.

The server derives identity from verified authentication claims.

### Error envelope

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Periksa data yang belum valid.",
    "fields": {
      "due_local": "Waktu jatuh tempo tidak valid."
    },
    "request_id": "req_example"
  }
}
```

Implemented codes:

```text
VALIDATION_ERROR
UNAUTHENTICATED
ORIGIN_NOT_ALLOWED
NOT_FOUND
PROFILE_INCOMPLETE
PAYLOAD_TOO_LARGE
METHOD_NOT_ALLOWED
NOT_READY
INTERNAL_ERROR
RECURRENCE_INACTIVE
SMART_CAPTURE_UNAVAILABLE
RATE_LIMITED
```

Never expose SQL errors, stack traces, secrets, or provider internals.

## Implemented route set

```text
GET   /healthz
GET   /readyz
POST  /api/v1/auth/dev-session              # non-production only
GET   /api/v1/profile                       # Bearer token
PATCH /api/v1/profile                       # Bearer token
DELETE /api/v1/profile/data                 # Bearer token + exact confirmation
POST  /api/v1/smart-capture/parse           # Bearer token; draft only
POST  /api/v1/tasks                         # Bearer token
GET   /api/v1/tasks/{taskID}
PATCH /api/v1/tasks/{taskID}
DELETE /api/v1/tasks/{taskID}
POST  /api/v1/tasks/{taskID}/complete       # Bearer token
POST  /api/v1/tasks/{taskID}/uncomplete
POST  /api/v1/events                        # Bearer token
GET   /api/v1/events
GET   /api/v1/events/{eventID}
PATCH /api/v1/events/{eventID}
DELETE /api/v1/events/{eventID}
POST  /api/v1/bills                         # Bearer token
GET   /api/v1/bills
GET   /api/v1/bills/{billID}
PATCH /api/v1/bills/{billID}
DELETE /api/v1/bills/{billID}
POST  /api/v1/bills/{billID}/mark-paid      # Bearer token
POST  /api/v1/bills/{billID}/mark-unpaid
GET   /api/v1/documents
POST  /api/v1/documents
GET   /api/v1/documents/{documentID}
PATCH /api/v1/documents/{documentID}
DELETE /api/v1/documents/{documentID}
GET   /api/v1/today                         # Bearer token
GET   /api/v1/agenda
GET   /api/v1/reminders
POST  /api/v1/reminders
GET   /api/v1/reminders/{reminderID}
PATCH /api/v1/reminders/{reminderID}
DELETE /api/v1/reminders/{reminderID}
GET   /api/v1/notifications
GET   /api/v1/notifications/unread-count
POST  /api/v1/notifications/{notificationID}/mark-read
POST  /api/v1/notifications/mark-all-read
GET   /api/v1/recurrence-series
GET   /api/v1/recurrence-series/{seriesID}
PATCH /api/v1/recurrence-series/{seriesID}
DELETE /api/v1/recurrence-series/{seriesID}
```

`POST /api/v1/auth/dev-session` accepts `{"email":"user@example.com"}` and returns `{"access_token":"..."}` only when the API is running outside production with an explicit development secret. Non-production startup enforces a loopback listener. Production identity comes from Supabase Auth; the Go API verifies its Bearer JWT against configured JWKS.

## Profile

`GET /api/v1/profile` ensures and returns the authenticated user's profile. A first-time user receives `timezone: null`, `locale: "id-ID"`, and `currency: "IDR"`; null timezone is an onboarding state, not a server error.

```json
{
  "user_id": "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270",
  "timezone": null,
  "locale": "id-ID",
  "currency": "IDR"
}
```

`PATCH /api/v1/profile` currently accepts only:

```json
{
  "timezone": "Asia/Jakarta"
}
```

Unknown fields are rejected. The timezone must be a valid IANA name.

`DELETE /api/v1/profile/data` requires:

```json
{
  "confirmation": "HAPUS DATA LIFEHUB"
}
```

It derives the owner solely from the verified token, transactionally cancels that user's scheduled River reminder jobs, and deletes the LifeHub profile so PostgreSQL cascades through owned tasks, events, bills, document metadata, recurrence series/templates, reminders/schedules, and notifications. It returns 204 and does not delete the separate Supabase authentication identity. A later authenticated profile read therefore starts fresh onboarding.

---

# Today

## GET /api/v1/today

Returns the user's integrated Today feed.

Implemented response shape:

```json
{
  "date": "2026-08-19",
  "timezone": "Asia/Jakarta",
  "items": [
    {
      "kind": "task",
      "id": "uuid",
      "title": "Submit laporan",
      "notes": null,
      "priority": "high",
      "due_at": "2026-08-19T13:00:00Z",
      "completed_at": null,
      "urgency": "today",
      "bucket": "due_today",
      "status": "open"
    }
  ],
  "upcoming": [],
  "upcoming_horizon_days": 30,
  "summary": {
    "open": 3,
    "completed": 2,
    "upcoming": 0
  }
}
```

Go returns every incomplete overdue task, every incomplete task due before the end of the local day, every open task without a due time, and every task completed during the local day. The first slice deliberately does not silently cap a trusted daily view. Bucket order is `overdue`, `due_today`, `anytime`, then `completed_today`; ordering within a bucket is deterministic by effective time, priority, creation time, and UUID. Local day boundaries use the stored IANA timezone and DST-aware instants.

### Implemented unified task/event/bill/document Today feed

Event items use a distinct shape rather than inheriting task-only fields:

```json
{
  "kind": "event",
  "id": "uuid",
  "title": "Meeting proyek",
  "notes": null,
  "location": "Online",
  "all_day": false,
  "timezone": "Asia/Jakarta",
  "starts_at": "2026-08-20T07:00:00Z",
  "ends_at": "2026-08-20T08:00:00Z",
  "starts_on": null,
  "ends_on": null,
  "urgency": "today",
  "bucket": "event_today",
  "status": "scheduled"
}
```

The web contract models Today items as a discriminated union keyed by `kind`; event rows do not inherit task priority/completion fields and do not expose a task completion action.

An implemented bill item is:

```json
{
  "kind": "bill",
  "id": "uuid",
  "title": "Internet",
  "notes": null,
  "amount": 350000,
  "currency": "IDR",
  "due_at": "2026-08-20T16:59:00Z",
  "paid_at": null,
  "urgency": "today",
  "bucket": "due_today",
  "status": "unpaid"
}
```

Bill items do not inherit task priority/completion fields. Paid bills use `paid_today`, `completed`, and `paid` for bucket, urgency, and status respectively.

An implemented document item is:

```json
{
  "kind": "document",
  "id": "uuid",
  "title": "SIM A",
  "notes": null,
  "category": "license",
  "expires_on": "2026-08-20",
  "days_until_expiry": 0,
  "urgency": "today",
  "bucket": "expires_today",
  "status": "expiring"
}
```

Documents whose profile-local expiry date is before Today or exactly Today appear in primary `items`. The separate, uncapped `upcoming` array covers unresolved tasks, events, bills, and documents from local tomorrow through day 30 inclusive and contributes only to `summary.upcoming`. Future task/event/bill items use `bucket: "upcoming"`, retain their native open/scheduled/unpaid status, and use `urgency: "upcoming"`; documents retain `bucket: "expiring_soon"`. Day 31 and later remain reachable through Agenda or the owned-domain list rather than silently disappearing.

The implemented unified ordering is:

1. overdue tasks and bills by due time;
2. expired documents by expiry date;
3. events happening now;
4. all-day events;
5. documents expiring today;
6. timed tasks/events and bills for today ordered by effective instant;
7. anytime tasks;
8. tasks completed and bills paid today by closed time.

After the relevant effective-time and task-priority comparisons, deterministic tie-breaks use kind, creation time, and UUID. Timed events that overlap the user's local day are eligible for Today, including a ranged event that began earlier and remains in progress. An all-day event is eligible when the user's Today date falls within its inclusive date range.

---

# Agenda

## GET /api/v1/agenda

```text
GET /api/v1/agenda?from=2026-08-21&to=2026-09-19
```

`from` and `to` are optional only as a pair. The default is profile-local tomorrow through Today +30 inclusive. The inclusive range may contain at most 31 calendar dates; malformed, reversed, overlong, duplicate, or unknown query parameters are rejected. A profile without a timezone receives `409 PROFILE_INCOMPLETE`.

Response:

```json
{
  "from": "2026-08-21",
  "to": "2026-09-19",
  "timezone": "Asia/Jakarta",
  "items": [
    {
      "kind": "task",
      "display_on": "2026-08-22",
      "id": "uuid",
      "title": "Kirim laporan",
      "notes": null,
      "priority": "high",
      "due_at": "2026-08-22T03:00:00Z",
      "completed_at": null,
      "status": "open",
      "created_at": "2026-08-20T01:00:00Z",
      "updated_at": "2026-08-20T01:00:00Z"
    }
  ],
  "summary": {
    "total": 8,
    "tasks": 2,
    "events": 3,
    "bills": 2,
    "documents": 1
  }
}
```

Agenda is an uncapped discriminated task/event/bill/document union. It includes unresolved due tasks, overlapping events, unpaid bills, and document expiries in the requested profile-local range. Each entity appears once with `display_on`; ongoing events that began before the range use the first requested date. Go performs four fixed domain queries, computes the full summary, and applies deterministic date/presentation/time/priority/kind/creation/UUID ordering without N+1 queries.

---

# Tasks

Implemented:

```text
POST   /api/v1/tasks
GET    /api/v1/tasks/{id}
PATCH  /api/v1/tasks/{id}
DELETE /api/v1/tasks/{id}
POST   /api/v1/tasks/{id}/complete
POST   /api/v1/tasks/{id}/uncomplete
```

Create concept:

```json
{
  "title": "Submit laporan",
  "notes": null,
  "priority": "high",
  "due_local": "2026-08-19T20:00:00"
}
```

`priority` is `low`, `normal`, or `high` and defaults to `normal`. `due_local` is an optional wall-clock value interpreted only in the authenticated profile's timezone; the server rejects ambiguous/nonexistent DST wall times and persists the resulting instant as `timestamptz`. Task creation returns 409 `PROFILE_INCOMPLETE` until timezone onboarding is complete.

Create response:

```json
{
  "id": "9e75761a-2554-4abf-b065-a50e15d098b7",
  "title": "Submit laporan",
  "notes": null,
  "priority": "high",
  "due_at": "2026-08-19T13:00:00Z",
  "completed_at": null,
  "created_at": "2026-08-19T01:00:00Z",
  "updated_at": "2026-08-19T01:00:00Z"
}
```

Completion and uncomplete are ownership-scoped and idempotent: repeated calls preserve the first transition's timestamp/`updated_at`. PATCH distinguishes omitted, null, and value fields: `notes: null` clears notes, `due_local: null` removes the due time, while title and priority cannot be null. Empty/unknown-field bodies are rejected. DELETE returns 204. Invalid, missing, and cross-owner UUIDs are indistinguishable 404 responses.

Creation may include the recurrence object documented below. One-off reminders remain configured through the separate owned reminder contract.

---

# Events — implemented create, bounded list, and correction slice

Implemented routes:

```text
POST   /api/v1/events
GET    /api/v1/events?from=YYYY-MM-DD&to=YYYY-MM-DD
GET    /api/v1/events/{id}
PATCH  /api/v1/events/{id}
DELETE /api/v1/events/{id}
```

Authentication is mandatory. The server derives `user_id` from the verified Bearer token and never accepts it as input.

## Timed event input

```json
{
  "title": "Meeting proyek",
  "notes": null,
  "location": "Online",
  "all_day": false,
  "starts_local": "2026-08-20T14:00:00",
  "ends_local": "2026-08-20T15:00:00"
}
```

`starts_local` is required and `ends_local` is optional. Both are wall-clock values interpreted by Go in the authenticated profile's stored IANA timezone. The server rejects nonexistent or ambiguous local times and requires an optional end to be later than the start. The persisted values are absolute `timestamptz` instants.

`starts_on` and `ends_on` must be absent from a timed request.

## All-day event input

```json
{
  "title": "Workshop kampus",
  "notes": null,
  "location": "Aula utama",
  "all_day": true,
  "starts_on": "2026-08-20",
  "ends_on": "2026-08-21"
}
```

`starts_on` is required and `ends_on` is optional. Dates use strict `YYYY-MM-DD` semantics; `ends_on` is **inclusive** and must not precede `starts_on`. If omitted, the event lasts one day. All-day dates remain PostgreSQL `date` values rather than being converted to midnight UTC.

`starts_local` and `ends_local` must be absent from an all-day request.

## Shared validation and response

- unknown fields and mixed timed/all-day shapes are rejected;
- title is trimmed, required, and at most 200 characters;
- notes are optional and at most 5000 characters;
- location is optional and at most 500 characters;
- a profile without a timezone receives `409 PROFILE_INCOMPLETE`;
- validation uses the normal safe error envelope;
- the created response does not include `user_id`.

Timed response concept:

```json
{
  "id": "uuid",
  "title": "Meeting proyek",
  "notes": null,
  "location": "Online",
  "all_day": false,
  "timezone": "Asia/Jakarta",
  "starts_at": "2026-08-20T07:00:00Z",
  "ends_at": "2026-08-20T08:00:00Z",
  "starts_on": null,
  "ends_on": null,
  "created_at": "2026-08-20T01:00:00Z",
  "updated_at": "2026-08-20T01:00:00Z"
}
```

The bounded event list defaults to local Today through day 30 inclusive and uses timed/all-day overlap semantics rather than start-only matching. It returns `{from,to,timezone,items}` without truncation.

Metadata-only PATCH may update title, notes, or location without changing the schedule; `notes: null` and `location: null` clear those values. Supplying any schedule field requires `all_day` plus one complete strict timed or all-day replacement shape. This permits timed ↔ all-day conversion while rejecting mixed/partial shapes, DST gaps/folds, and invalid end ordering. DELETE returns 204.

The strict union, ownership rules, PostgreSQL persistence, Today/Agenda integration, list overlap, and correction behavior are covered by HTTP, real-database, ordering, browser-persistence, and race tests.

Events can opt into the implemented recurrence contract below and use the separate one-off reminder contract. Hosted Supabase validation and deployment remain deferred.

---

# Bills — implemented create, payment, list, and correction slice

```text
POST   /api/v1/bills
GET    /api/v1/bills?state=unpaid|paid&limit=50&cursor=...
GET    /api/v1/bills/{id}
PATCH  /api/v1/bills/{id}
DELETE /api/v1/bills/{id}
POST   /api/v1/bills/{id}/mark-paid
POST   /api/v1/bills/{id}/mark-unpaid
```

Create request:

```json
{
  "title": "Internet",
  "notes": null,
  "amount": 350000,
  "currency": "IDR",
  "due_local": "2026-08-20T23:59:00"
}
```

`amount` must be an integer JSON number from 1 through `9007199254740991` inclusive. The upper bound keeps the value exact in JavaScript and fits PostgreSQL `bigint`; floats, zero, negative, and larger values are rejected. Omitted currency defaults to `IDR`; a supplied currency is trimmed and must be exactly three uppercase ASCII letters. `due_local` is required, resolved by Go in the authenticated profile's stored IANA timezone, and persisted as `due_at timestamptz`. Creation returns 409 `PROFILE_INCOMPLETE` until timezone onboarding is complete.

Create response:

```json
{
  "id": "uuid",
  "title": "Internet",
  "notes": null,
  "amount": 350000,
  "currency": "IDR",
  "due_at": "2026-08-20T16:59:00Z",
  "paid_at": null,
  "created_at": "2026-08-20T01:00:00Z",
  "updated_at": "2026-08-20T01:00:00Z"
}
```

`mark-paid` and `mark-unpaid` are explicit domain actions rather than generic boolean updates. Both are ownership-scoped and idempotent: repeats preserve the first transition's `updated_at`; repeated payment preserves the first `paid_at`. Missing or cross-owner UUIDs return 404. Today returns all owned unpaid bills with `due_at` before the local day ends plus bills paid within `[local day start, local day end)`, without truncation.

The bill list defaults to `state=unpaid` and limit 50, accepts a maximum limit of 100, and returns `{items,next_cursor}`. Unpaid ordering is due time then UUID; paid history is payment time descending then UUID. The opaque base64url cursor is versioned, state-bound, strictly decoded as untrusted input, and produces stable pages without duplicates.

PATCH follows create validation, supports omitted/null/value semantics for optional notes, and does not allow due time or required fields to become null. State cannot be changed through PATCH. DELETE returns 204. Creation may include recurrence; one-off reminders use the separate reminder contract below.

---

# Documents — implemented metadata/expiry management slice

```text
GET    /api/v1/documents
POST   /api/v1/documents
GET    /api/v1/documents/{id}
PATCH  /api/v1/documents/{id}
DELETE /api/v1/documents/{id}
```

Create request:

```json
{
  "name": "SIM",
  "category": "license",
  "expires_on": "2027-11-06",
  "notes": null
}
```

`name` is trimmed and limited to 200 characters. `category` is one of `identity`, `license`, `insurance`, `education`, `work`, or `other`. Optional notes are limited to 5000 characters. Expiry is a strict PostgreSQL `date`; the browser and API do not convert it through UTC midnight.

Response and list item:

```json
{
  "id": "uuid",
  "name": "SIM",
  "category": "license",
  "notes": null,
  "expires_on": "2027-11-06",
  "status": "valid",
  "days_until_expiry": 443,
  "created_at": "2026-08-20T01:00:00Z",
  "updated_at": "2026-08-20T01:00:00Z"
}
```

Status is derived by Go from the authenticated profile's local calendar date: `expired` before Today, `expiring` from Today through day 30 inclusive, and `valid` after day 30. `GET /documents` returns every owned metadata record ordered by expiry, creation time, and UUID; there is no horizon cap.

PATCH uses strict omitted/null/value semantics. Omitted fields remain unchanged; `notes: null` clears notes; required fields cannot be null; an empty body and unknown fields are rejected. DELETE is ownership-scoped, explicitly confirmed in the UI, and returns 204. Missing, invalid, and cross-owner identifiers are indistinguishable 404 responses. All private responses use `Cache-Control: no-store`.

Document reminders are not embedded in document create/update bodies. They use the separate owned date-reminder contract below.

---

# Reminders and notifications — implemented

```text
GET    /api/v1/reminders?source_kind=task|event|bill|document&source_id={uuid}
POST   /api/v1/reminders
GET    /api/v1/reminders/{id}
PATCH  /api/v1/reminders/{id}
DELETE /api/v1/reminders/{id}

GET  /api/v1/notifications?limit=50&cursor=...
GET  /api/v1/notifications/unread-count
POST /api/v1/notifications/{id}/mark-read
POST /api/v1/notifications/mark-all-read
```

Reminder creation is a strict union. A timed task, bill, or timed event uses:

```json
{
  "source_kind": "task",
  "source_id": "uuid",
  "schedule": {
    "kind": "before_moment",
    "minutes_before": 30
  }
}
```

`minutes_before` is an integer from 0 through 525600. A document or all-day event uses profile-local calendar semantics:

```json
{
  "source_kind": "document",
  "source_id": "uuid",
  "schedule": {
    "kind": "before_date",
    "days_before": 7,
    "time_local": "09:00"
  }
}
```

`days_before` is an integer from 0 through 3650 and `time_local` is strict `HH:MM`. Go rejects a schedule kind that does not match the source representation, inactive/completed/paid sources, past fire times, missing profile timezone, and ambiguous/nonexistent local wall times. List/get/PATCH/DELETE are ownership-scoped; missing and cross-owner resources share the same 404 response. PATCH replaces the complete schedule union. DELETE returns 204.

Response fields are `id`, `source_kind`, `source_id`, the discriminated `schedule`, `status` (`scheduled`, `delivered`, or `inactive`), nullable RFC3339 `next_fire_at`, and timestamps. Source anchor/timezone/state changes atomically cancel or replace the active immutable generation; metadata-only edits preserve a due schedule and delivery rehydrates the latest safe source title. River arguments contain only schedule ID and generation.

The worker retries failures and the database unique key `(schedule_id,generation)` prevents duplicate visible notifications across retry or restart. The notification list returns `{items,next_cursor,unread_count}` in stable descending `(created_at,id)` order with default limit 50 and maximum 100. Cursors are opaque/versioned. Mark-one and mark-all actions accept only an empty body or `{}`, are idempotent, and persist `read_at`. All routes require a verified Bearer identity and use `Cache-Control: no-store`.

---

# Recurrence — implemented

Task, event, and bill creation accepts an optional rule:

```json
{
  "recurrence": {
    "frequency": "monthly",
    "interval": 1,
    "ends_on": "2027-08-29"
  }
}
```

`frequency` is `daily`, `weekly`, `monthly`, or `yearly`; interval defaults to 1 and is limited to 1 through 365; `ends_on` is optional and inclusive. A recurring task requires `due_local`, a recurring timed event requires `starts_local`, an all-day event retains date semantics, and a recurring bill requires `due_local`.

Go stores an owned series plus a typed template, then materializes an idempotent bounded 90-day occurrence window in PostgreSQL. A durable River maintenance job extends all active series every 12 hours and at worker startup. Monthly recurrence clamps to the last calendar day without changing the original anchor; leap days, DST gap/fold resolution, and skipped civil dates follow the documented time rules.

```text
GET    /api/v1/recurrence-series
GET    /api/v1/recurrence-series/{id}
PATCH  /api/v1/recurrence-series/{id}
DELETE /api/v1/recurrence-series/{id}
```

Series responses contain `id`, `source_kind`, `title`, `frequency`, `interval`, `anchor_on`, nullable `ends_on`, `timezone`, `active`, and timestamps. PATCH replaces the frequency/interval/end rule and regenerates eligible future occurrences while preserving completed/paid items and individually edited exceptions. Deleting one occurrence records an exclusion so a sweep cannot recreate it. DELETE on the series accepts an empty body, stops future generation, removes future unresolved occurrences, cancels their reminders, and retains completed/paid history. Every route and materialization query enforces authenticated ownership.

---

# Future profile expansion

```text
GET   /api/v1/profile
PATCH /api/v1/profile
```

Timezone update concept:

```json
{
  "timezone": "Asia/Jakarta",
  "locale": "id-ID",
  "currency": "IDR"
}
```

Timezone is already implemented as described above. Client-editable locale and currency are future expansions; the first slice keeps server defaults `id-ID` and `IDR`.

---

# Smart Capture

## POST /api/v1/smart-capture/parse

Input:

```json
{
  "text": "Bayar internet 350 ribu tanggal 15 tiap bulan"
}
```

Output is a **draft**, never a mutation:

```json
{
  "draft": {
    "kind": "bill",
    "title": "Internet",
    "amount": 350000,
    "currency": "IDR",
    "recurrence": {
      "frequency": "monthly",
      "interval": 1
    }
  },
  "ambiguities": [
    "Jam jatuh tempo belum disebutkan."
  ],
  "provider": "rule"
}
```

The endpoint requires a verified Bearer token and a completed profile timezone, accepts 1 through 1,000 characters, applies a two-second provider timeout, validates the provider output as untrusted data, and is limited to 20 requests per authenticated user per minute. `RateLimit-Limit`, `RateLimit-Remaining`, and `Retry-After` are exposed where applicable.

The built-in deterministic Indonesian rule provider and deterministic mock provider require no AI credential. The frontend fills the normal editable form, displays ambiguities, and only later calls the ordinary task/event/bill/document creation endpoint after the user presses its explicit Save button. Parsing never performs a domain write.

---

# Pagination

For history/list endpoints that can grow, prefer cursor-based or well-defined page pagination once needed.

Do not add pagination complexity to tiny fixed Today responses.

---

# Idempotency

For operations where accidental duplicate submission is realistic, evaluate support for an `Idempotency-Key` request header.

Durable reminder delivery has its own server-generated idempotency identity independent of client requests.

---

# API evolution

Breaking API changes require:
- versioned route or explicit coordinated migration;
- updated tests;
- updated docs/OpenAPI;
- no silent client/server mismatch.
