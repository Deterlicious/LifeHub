# LifeHub Deployment Guide

Status: **production images, a paid Render Blueprint, and a Google Cloud free-tier deployment profile are implemented; no hosted LifeHub resources have been provisioned and no production deployment is claimed**.

Current provider capabilities, account usage, and stable container bases were rechecked on 1 September 2026. Provisioning still requires a Supabase Free project, the owner's acknowledgement that free tiers are usage limits rather than a hard spending cap, and final public origins. Never claim a deployment before its public journeys pass.

## Selected Google Cloud free-tier topology — pending provisioning

The owner selected Google Cloud project `project-592af0e7-ebee-4483-88e`. LifeHub resources use the `lifehub-` prefix and must not modify the existing BagiYuk service or images in that project.

The selected target is:

```text
Supabase Free Auth + PostgreSQL
        │ TLS + Bearer JWT/JWKS
        ├── Cloud Run lifehub-web (min 0, max 1)
        ├── Cloud Run lifehub-api (min 0, max 1)
        ├── Cloud Run Job lifehub-reminders (every 15 minutes)
        └── Cloud Run Job lifehub-maintenance (twice daily)
                     ▲
                     └── two Cloud Scheduler triggers
```

Reasons:

- Cloud Run services can scale to zero and are capped at one instance for this low-traffic deployment;
- two bounded Cloud Run Jobs keep the durable River/PostgreSQL design without paying for an always-on worker;
- two Scheduler jobs fit inside the current three-job billing-account allowance when no unrelated jobs are added;
- public GHCR images avoid adding LifeHub images to the Google Artifact Registry account usage, which already exceeds its 0.5 GiB free allowance;
- Supabase's generally available asymmetric signing-key system exposes a rotatable public JWKS, matching the implemented Go verifier without exposing an auth secret to the API;
- Supabase Free keeps PostgreSQL as the source of truth; Cloud SQL is excluded because its free use is trial-based rather than a permanent PostgreSQL free tier.

Official references:

- [Cloud Run pricing](https://cloud.google.com/run/pricing)
- [Cloud Scheduler pricing](https://cloud.google.com/scheduler/pricing)
- [Artifact Registry pricing](https://cloud.google.com/artifact-registry/pricing)
- [Secret Manager pricing](https://cloud.google.com/secret-manager/pricing)
- [GitHub Packages billing](https://docs.github.com/en/billing/concepts/product-billing/github-packages)
- [Supabase database size](https://supabase.com/docs/guides/platform/database-size)
- [Supabase JWT signing keys](https://supabase.com/docs/guides/auth/signing-keys)

The reminder job's 15-minute schedule means a due reminder can be delivered roughly 15 minutes late. A 45-second bounded run at that interval plus a twice-daily maintenance run is designed to remain below the current Cloud Run monthly CPU free allowance, but free allowances are shared usage limits and can change. Network egress, database growth, secrets, or unexpected traffic can still create charges. The existing IDR 25,000 budget sends alerts only; it is not a hard cap.

## Existing paid alternative

`render.yaml` remains a valid paid alternative for an always-on worker. Render does not offer a free background-worker instance, and its recommended pre-deploy command is a paid-service feature. Do not create or sync the Blueprint unless the owner explicitly chooses that paid topology.

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

`.github/workflows/publish-images.yml` publishes the API/worker/migrator image and, once its public build-time endpoints are known, the web image to GHCR. The image package must be verified as publicly pullable before Cloud Run provisioning. Cloud Run uses the same API image with `/usr/local/bin/api`, `/usr/local/bin/worker`, or `/usr/local/bin/migrate` as the command.

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

# Worker only; omit for the normal continuous mode
WORKER_RUN_DURATION=45s
WORKER_PERIODIC_JOBS=false

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

For the Google Cloud profile, a one-off `lifehub-migrate` Cloud Run Job must succeed before API traffic is enabled. `lifehub-reminders` runs the worker for 45 seconds with periodic jobs disabled. `lifehub-maintenance` runs the same bounded worker with periodic jobs enabled so River's run-on-start recurrence sweep is inserted durably.

The migrator is safe to rerun when both schemas are current and refuses an unsupported newer River schema. Before production, run its clean-database smoke test and record a tested provider backup/restore procedure.

## Rollout sequence

Safe Google Cloud order:

1. create the Supabase Free project and enable asymmetric signing keys;
2. publish the API image and verify anonymous GHCR pull access;
3. store the database URL in Secret Manager and create least-privileged service accounts;
4. run the migration job and verify schema versions 7 and 7;
5. deploy the API, then publish/deploy the web image using the public API and Supabase values;
6. deploy bounded reminder and maintenance jobs, then create their Scheduler triggers;
7. smoke-test hosted authentication, Today, a write/read cycle, and a scheduled reminder;
8. verify TLS/CORS/security headers, logs, quotas, and budget alerts.

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

CI repeats both production image builds after the web and Go gates, using non-secret public placeholders for the Supabase/web build boundary. The separate manual publish workflow emits immutable commit tags plus `latest`; its result is not deployment proof. GitHub Actions run `33265929162` attempt 2 passed the web, Go integration/race, container-build, and eight-case E2E jobs for commit `0c3ecb2` during the Google Cloud verification.

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
