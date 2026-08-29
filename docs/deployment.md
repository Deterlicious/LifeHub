# LifeHub Deployment Guide

Status: **production images and a Render Blueprint are implemented and smoke-tested locally; no hosted resources have been provisioned and no production deployment is claimed**.

Current provider capabilities and stable container bases were rechecked against official documentation/registries on 29 August 2026. Provisioning still requires the owner's account, billing approval, Supabase project values, and final public origins; never create paid resources or claim a deployment before those choices are confirmed.

## Selected production topology — pending provisioning

The current recommended target is:

```text
Supabase Auth (asymmetric signing key, Singapore)
        │ Bearer JWT / JWKS
        ▼
Render Singapore
├── Next.js web service
├── Go API web service
├── Go River background worker
└── managed PostgreSQL 18 on the private network
```

Reasons:

- Render supports native Next.js web services, Go services, continuous background workers, managed PostgreSQL, Blueprint configuration, pre-deploy commands, and a Singapore region;
- keeping API, worker, and PostgreSQL in one region/private network gives River a direct session-capable database connection without Redis;
- Supabase's generally available asymmetric signing-key system exposes a rotatable public JWKS, matching the implemented Go verifier without exposing an auth secret to the API;
- the worker is continuous rather than request-only/serverless, so persisted due jobs can execute after restarts.

Official references:

- [Render Next.js deployment](https://render.com/docs/deploy-nextjs-app)
- [Render background workers](https://render.com/docs/background-workers)
- [Render Blueprint reference](https://render.com/docs/blueprint-spec)
- [Render regions](https://render.com/docs/regions)
- [Render PostgreSQL](https://render.com/docs/postgresql)
- [Supabase JWT signing keys](https://supabase.com/docs/guides/auth/signing-keys)

Render does not offer the `free` instance type for background workers, and its recommended pre-deploy command is a paid-service feature. Therefore a trustworthy always-on reminder deployment has a real hosting cost. Exact Render plans and Supabase Free/Pro choice must be approved at provisioning time; free-tier assumptions are not embedded in the code.

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

`web`, `api`, the River worker, the shared application/River migrator, and PostgreSQL exist locally. `apps/web/Dockerfile` and `services/api/Dockerfile` produce non-root production runtimes, and `render.yaml` connects three Singapore services to a private PostgreSQL 18 database. Hosted provisioning and credentials remain deployment work.

The checked-in Blueprint selects current Render `0.5c-512mb` plans for web/API/worker and `0.1c-256mb` PostgreSQL. These are paid resources. Services deploy only after linked GitHub checks pass. Blueprint creation or sync is prohibited until the owner explicitly approves the resulting charges.

On 29 August 2026, `render.yaml` passed the current official `https://render.com/schema/render.yaml.json` JSON Schema. The official Render CLI could not perform its account-backed validation without an authenticated workspace, so that platform check remains part of provisioning rather than being claimed locally.

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
PORT=
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

```

Never put secret values in this document or `.env.example`.

`APP_ENV=production` requires HTTPS for the configured web origin, issuer, and JWKS URL, ignores the development secret, and does not expose the development-session route.

## Container strategy — implemented locally

- API/worker/migrator: multi-stage `golang:1.27.0-alpine3.24` build; static CGO-disabled binaries; `alpine:3.24.1` runtime; CA certificates and IANA timezone data; non-root `lifehub` user.
- Web: multi-stage `node:24.19.0-alpine3.24`; exact pnpm 11.24.0 frozen install; Next standalone output; non-root `lifehub` user.
- Build contexts use dedicated `.dockerignore` files so secrets, caches, reports, and unrelated development artifacts are not copied.
- The API honors an explicit `HTTP_ADDR`; when it is absent in production, the platform `PORT` becomes `0.0.0.0:$PORT`. Non-production listeners remain restricted to loopback.

## Database migrations

The API image includes `/usr/local/bin/migrate`, which applies the embedded seven-version Goose schema and River schema target 7. The Render API uses it as `preDeployCommand`; API startup itself never races to migrate. The same image also includes `/usr/local/bin/worker`.

The migrator is safe to rerun when both schemas are current and refuses an unsupported newer River schema. Before production, run its clean-database smoke test and record a tested provider backup/restore procedure.

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

`readyz` checks PostgreSQL connectivity and verifies that both the application and River schemas are at the exact supported versions (7 and 7). A reachable but stale/newer database is not ready.

Do not expose secrets or detailed topology in health responses.

## Current local smoke proof

On 27 August 2026, both production images built and ran locally:

- the migrator reached application migration 7 and River migration 7, and a second run was a no-op;
- `/healthz` and `/readyz` returned healthy/ready against PostgreSQL;
- the API container emitted production security headers and shut down gracefully;
- the worker connected, started River, and drained cleanly on shutdown;
- the standalone web container returned HTTP 200 with its production CSP/HSTS headers;
- the full product journeys had already exercised durable reminder retry/restart idempotency, recurrence, and reviewed Smart Capture against PostgreSQL.

CI now repeats both production image builds after the web and Go gates, using non-secret public placeholders for the Supabase/web build boundary. The workflow passes `actionlint` 1.7.12 locally, and GitHub Actions run `33265715718` passed the web, Go, container-build, and eight-case E2E jobs for commit `542ef4c`.

This is local container evidence only. The fresh eight-case browser suite and clean-database application/River migration proof now pass against PostgreSQL 18.6. The final release still requires a rebuild after the 18.6 image pin, hosted Supabase journey, real ingress/header checks, and a live worker delivery smoke.

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
- [x] LifeHub application-data deletion procedure (typed confirmation, owned cascade, queued reminder cancellation)
- [ ] Supabase authentication-identity deletion procedure, if required by the production policy
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
