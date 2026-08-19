# LifeHub API Contract

Status: **implemented first-slice contract plus explicitly labelled future design**. If an OpenAPI document is introduced, it becomes the machine-readable source of truth and this file should link to it.

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
```

Never expose SQL errors, stack traces, secrets, or provider internals.

## Implemented route set

```text
GET   /healthz
GET   /readyz
POST  /api/v1/auth/dev-session              # non-production only
GET   /api/v1/profile                       # Bearer token
PATCH /api/v1/profile                       # Bearer token
POST  /api/v1/tasks                         # Bearer token
POST  /api/v1/tasks/{taskID}/complete       # Bearer token
GET   /api/v1/today                         # Bearer token
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
  "summary": {
    "open": 3,
    "completed": 2
  }
}
```

Go returns every incomplete overdue task, every incomplete task due before the end of the local day, every open task without a due time, and every task completed during the local day. The first slice deliberately does not silently cap a trusted daily view. Bucket order is `overdue`, `due_today`, `anytime`, then `completed_today`; ordering within a bucket is deterministic by effective time, priority, creation time, and UUID. Local day boundaries use the stored IANA timezone and DST-aware instants.

---

# Tasks

Implemented:

```text
POST /api/v1/tasks
POST /api/v1/tasks/{id}/complete
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

Completion is ownership-scoped and idempotent: repeated calls preserve the first `completed_at`. A missing or cross-owner UUID returns 404.

Planned task list/get/edit/delete/uncomplete, recurrence, and reminders are not implemented.

---

# Future contracts — not implemented

---

# Events

```text
GET    /api/v1/events
POST   /api/v1/events
GET    /api/v1/events/{id}
PATCH  /api/v1/events/{id}
DELETE /api/v1/events/{id}
```

Create concept:

```json
{
  "title": "Meeting proyek",
  "location": "Online",
  "starts_at": "2026-08-20T14:00:00+07:00",
  "ends_at": "2026-08-20T15:00:00+07:00",
  "all_day": false,
  "recurrence": null,
  "reminders": [
    { "offset_seconds": 1800, "channel": "in_app" }
  ]
}
```

---

# Bills

```text
GET    /api/v1/bills
POST   /api/v1/bills
GET    /api/v1/bills/{id}
PATCH  /api/v1/bills/{id}
DELETE /api/v1/bills/{id}
POST   /api/v1/bills/{id}/mark-paid
POST   /api/v1/bills/{id}/mark-unpaid
```

Create concept:

```json
{
  "title": "Internet",
  "amount": 350000,
  "currency": "IDR",
  "due_at": "2026-09-15T23:59:00+07:00",
  "recurrence": {
    "frequency": "monthly",
    "interval": 1,
    "ends_on": null
  },
  "reminders": [
    { "offset_seconds": 259200, "channel": "in_app" }
  ]
}
```

`mark-paid` is an explicit domain action rather than a generic boolean update.

---

# Documents

```text
GET    /api/v1/documents
POST   /api/v1/documents
GET    /api/v1/documents/{id}
PATCH  /api/v1/documents/{id}
DELETE /api/v1/documents/{id}
```

Create concept:

```json
{
  "name": "SIM",
  "category": "license",
  "expires_on": "2027-11-06",
  "notes": null,
  "reminders": [
    { "offset_seconds": 2592000, "channel": "in_app" }
  ]
}
```

Expiry is date-only.

---

# Notifications

```text
GET  /api/v1/notifications
POST /api/v1/notifications/{id}/read
POST /api/v1/notifications/read-all
```

The notifications list is derived from durable reminder processing.

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
  "ambiguities": [],
  "provider": "rule"
}
```

The frontend shows review UI and later calls the normal Bill create endpoint.

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
