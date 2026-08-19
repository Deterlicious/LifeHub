# LifeHub Architecture

Status: first task/Today slice implemented; worker and remaining domains are proposed.

## Goals

- daily-use responsiveness;
- reliable reminders;
- correct timezone behavior;
- strong user isolation;
- simple operations;
- future mobile-client compatibility.

Advanced only where correctness requires it.

## System

Implemented first-slice topology:

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

The worker is not implemented in the first slice.

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

All authorization happens in Go. Every implemented task query binds the verified subject as `user_id`; cross-owner completion returns the same 404 as a missing task.

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

## Time

DB:
- `timestamptz` for moments;
- `date` for date-only expiry;
- IANA timezone in profile.

API:
RFC 3339.

Never strip timezone offsets and silently assume UTC.

## Recurrence

Use explicit model:
- frequency;
- interval;
- start;
- optional end;
- optional weekdays later.

Choose and document occurrence strategy before implementation:
- lazily generate next occurrence; or
- materialize a bounded window.

Do not store arbitrary unvalidated recurrence JSON just for flexibility.

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

Expected tables:
- profiles;
- tasks;
- events;
- bills;
- documents;
- reminders;
- notifications;
- River job tables.

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

Provider is isolated and cannot directly access repositories.

`parse → draft → validate → response → user confirms via normal create API`

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
