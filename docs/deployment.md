# LifeHub Deployment Guide

Status: **local first-slice runtime documented; production remains provider-neutral and undeployed**.

Do not choose hosting based on outdated free-tier assumptions. At deployment time, research current official pricing/runtime limits and update this document with the actual provider and commands.

## Deployable units

The complete product has three logical runtime units:

```text
web
api
worker
```

and one managed dependency:

```text
PostgreSQL
```

The Go API and worker may use the same container image with different commands.

Only `web`, `api`, and PostgreSQL exist today. The worker is deferred until durable reminders are implemented.

## Production requirements

### Web
- HTTPS;
- configured Go API origin;
- Supabase public auth configuration only;
- no server secret leaked to browser.

### Go API
- HTTPS behind trusted ingress/proxy;
- database connection;
- Supabase issuer/JWKS configuration;
- CORS allowlist;
- structured logs;
- health/readiness;
- graceful shutdown;
- production server timeouts.

### Worker
- same DB;
- River queue access;
- durable lifecycle;
- graceful shutdown/drain;
- restart policy.

### PostgreSQL
- managed service preferred;
- TLS;
- backups;
- restore procedure;
- least-privileged runtime credentials.

## Environment variables

Implemented names:

```text
# Web - public/safe
NEXT_PUBLIC_API_URL=
NEXT_PUBLIC_AUTH_MODE=supabase
NEXT_PUBLIC_SUPABASE_URL=
NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY=

# Go API - server only
APP_ENV=
HTTP_ADDR=
DATABASE_URL=
WEB_ORIGIN=
SUPABASE_ISSUER=
SUPABASE_JWKS_URL=
SUPABASE_AUDIENCE=authenticated

# Local development only; omit in production
DEV_AUTH_SECRET=

# Optional future push
VAPID_PUBLIC_KEY=
VAPID_PRIVATE_KEY=

# Optional future AI
SMART_CAPTURE_PROVIDER=
SMART_CAPTURE_API_KEY=
```

Never put secret values in this document or `.env.example`.

`APP_ENV=production` requires HTTPS for the configured web origin, issuer, and JWKS URL, ignores the development secret, and does not expose the development-session route.

## Container strategy

Use multi-stage builds.

Possible Go image:

```text
builder → compile static/minimal binary
runtime → non-root user + binary + required CA certificates/tzdata
```

If the application relies on IANA timezone data, ensure runtime timezone data is available or use an intentional Go tzdata strategy.

## Database migrations

Deployment must define who runs migrations.

Preferred options:
- explicit release/migration job before app rollout; or
- a controlled deployment step.

Avoid every API replica racing to run migrations on startup unless migration tooling/locking was intentionally designed for it.

## Rollout sequence

Typical safe order:

1. backup/verify DB recovery capability;
2. run compatible migrations;
3. deploy API;
4. deploy worker;
5. deploy web;
6. smoke-test auth and Today;
7. verify worker health;
8. verify logs/errors.

For breaking schema changes, use expand-and-contract migrations.

## Health

Expected:

```text
GET /healthz
GET /readyz
```

`healthz` checks process health.

`readyz` checks dependencies required to serve traffic, especially PostgreSQL.

Do not expose secrets or detailed topology in health responses.

## Current first-slice smoke test

1. load public/auth page;
2. sign in;
3. create task;
4. verify Today;
5. complete task;
6. refresh/re-login and verify persistence;
7. check logs for errors without private payload leakage.

## Future worker smoke test

After the reminder slice exists:

1. create a near-future reminder with a controlled test user;
2. verify one durable job produces one notification;
3. retry/restart the worker and confirm no duplicate visible notification;
4. edit/delete the source and confirm stale jobs cannot notify.

## Production checklist

- [ ] HTTPS
- [ ] allowed CORS origins
- [ ] Supabase JWT/JWKS configured
- [ ] no secret in client bundle
- [ ] DB TLS
- [ ] backups enabled
- [ ] restore procedure documented/tested
- [ ] migrations tested
- [ ] API health/readiness
- [ ] worker restart policy
- [ ] graceful shutdown
- [ ] log redaction
- [ ] dependency audit
- [ ] `govulncheck ./...`
- [ ] Playwright critical journey
- [ ] data deletion procedure
- [ ] monitoring/alerting proportional to real operational needs

## Deployment record

Fill this after actual deployment:

```text
Web provider:
Web URL:
API provider:
API URL:
Worker provider:
PostgreSQL provider:
Region:
Deployment date:
Web commit:
API commit:
Migration version:
Go version:
Node version:
Known limitations:
```
