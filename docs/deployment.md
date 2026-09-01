# LifeHub Deployment Guide

Status: **production images and a strict zero-charge deployment profile are implemented and locally verified; no hosted LifeHub resources have been provisioned and no production deployment is claimed**.

Current provider capabilities, account usage, and stable container bases were rechecked on 1 September 2026. The owner requires a deployment that cannot create a bill. Google Cloud is therefore excluded: its billing account is required even for Free Tier, and its alerts-only budget is not a global hard spending cap. Never claim a deployment before its public journeys pass.

## Selected zero-charge topology — pending provisioning

The selected target uses only free plans configured without a payment method:

```text
Supabase Free Auth + PostgreSQL
        │ TLS + Bearer JWT/JWKS
        ├── Render Free static site: lifehub-web
        └── Render Free web service: lifehub-api
                    ▲
                    └── public-repository GitHub Actions worker (every 15 minutes)
```

Zero-charge guardrails:

- do not add a payment method to Render; if free hours, bandwidth, or build allowances are exhausted, Render suspends the free service/build instead of charging it;
- select only the explicit Render `free` API plan and the free static-site runtime from `render.free.yaml`;
- keep Supabase on Free with no paid upgrade; its database becomes restricted when the free quota is exceeded rather than silently upgrading;
- use standard GitHub-hosted runners only in this public repository, where Actions usage is free;
- keep the scheduled worker disabled until the `LIFEHUB_FREE_DEPLOYMENT_ENABLED=true` repository variable is deliberately set after database verification;
- pin migration and worker runs to the already published API image commit instead of a mutable tag;
- create no LifeHub resource, secret, scheduler, image, service, or job in Google Cloud;
- Supabase's generally available asymmetric signing-key system exposes a rotatable public JWKS, matching the implemented Go verifier without exposing an auth secret to the API;
- the reminder worker remains durable because River jobs stay in PostgreSQL, but scheduled GitHub runs can be delayed.

Official references:

- [Render Free limitations](https://render.com/docs/free)
- [Render Blueprint reference](https://render.com/docs/blueprint-spec)
- [GitHub Actions billing](https://docs.github.com/en/billing/concepts/product-billing/github-actions)
- [GitHub Packages billing](https://docs.github.com/en/billing/concepts/product-billing/github-packages)
- [Supabase database size](https://supabase.com/docs/guides/platform/database-size)
- [Supabase JWT signing keys](https://supabase.com/docs/guides/auth/signing-keys)

The tradeoff is availability rather than money: the API sleeps after 15 idle minutes and can take about one minute to wake; reminders can arrive late when GitHub delays a scheduled run; Render can suspend services for the rest of a billing period after free allowances are exhausted. This profile is suitable for demonstration/hobby use, not a production SLA.

## Existing paid alternative

`render.yaml` remains a valid paid alternative for an always-on worker. It is not the selected profile. Do not create or sync it; only `render.free.yaml` is allowed while the zero-charge requirement remains active.

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

`web`, `api`, the River worker, the shared application/River migrator, and PostgreSQL exist locally. `apps/web/Dockerfile` and `services/api/Dockerfile` produce non-root production runtimes. `render.free.yaml` serves the Next static export and the pinned public API image without a paid worker or Render database.

`.github/workflows/publish-images.yml` publishes the API/worker/migrator image. `.github/workflows/free-migrate.yml` applies migrations manually, and `.github/workflows/free-worker.yml` runs the same pinned image for a bounded 45-second window every 15 minutes. The GHCR image was anonymously pullable before this profile was selected.

The checked-in Blueprint selects current Render `0.5c-512mb` plans for web/API/worker and `0.1c-256mb` PostgreSQL. These are paid resources. Services deploy only after linked GitHub checks pass. Blueprint creation or sync is prohibited until the owner explicitly approves the resulting charges.

On 1 September 2026, `render.free.yaml` passed the current official `https://render.com/schema/render.yaml.json` JSON Schema with zero errors. The paid `render.yaml` also remains schema-valid but is prohibited by the selected cost policy.

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
WORKER_PERIODIC_JOBS=true

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
- Web: multi-stage `node:24.19.0-alpine3.24`; exact pnpm 11.24.0 frozen install; Next standalone output by default; opt-in static export for the hard-free profile; non-root `lifehub` user in the container profile.
- Build contexts use dedicated `.dockerignore` files so secrets, caches, reports, and unrelated development artifacts are not copied.
- The API honors an explicit `HTTP_ADDR`; when it is absent in production, the platform `PORT` becomes `0.0.0.0:$PORT`. Non-production listeners remain restricted to loopback.

## Database migrations

The API image includes `/usr/local/bin/migrate`, which applies the embedded seven-version Goose schema and River schema target 7. The hard-free profile runs this binary through the manual `free-migrate.yml` workflow before creating the Render API. API startup itself never races to migrate. The same pinned image includes `/usr/local/bin/worker` for the scheduled workflow.

The migrator is safe to rerun when both schemas are current and refuses an unsupported newer River schema. Before production, run its clean-database smoke test and record a tested provider backup/restore procedure.

## Rollout sequence

Safe hard-free order:

1. create the Supabase Free project and enable asymmetric signing keys;
2. add only `LIFEHUB_DATABASE_URL` as a GitHub Actions secret and run the manual migration workflow;
3. create a Render account/workspace without adding a payment method;
4. deploy only `render.free.yaml`, confirm `lifehub-api` is explicitly Free, and configure exact public origins;
5. configure the static site's CSP after the final API and Supabase origins are known;
6. configure Supabase redirect URLs and smoke-test hosted authentication, Today, and a write/read cycle;
7. set `LIFEHUB_FREE_DEPLOYMENT_ENABLED=true`, run the worker manually once, and verify a scheduled reminder;
8. verify headers and account dashboards show no paid plan, payment method, or Google Cloud LifeHub resource.

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

CI repeats both production image builds after the web and Go gates, using non-secret public placeholders for the Supabase/web build boundary. The separate manual publish workflow emits immutable commit tags plus `latest`; its result is not deployment proof. GitHub Actions run `33513531173` passed the web, Go integration/race, container-build, and eight-case E2E jobs for commit `1a463e5`. The hard-free static export subsequently built locally and produced `apps/web/out/index.html`.

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
