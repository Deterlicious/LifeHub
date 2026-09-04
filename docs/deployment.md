# LifeHub Deployment Guide

Status: **Supabase Free Auth/PostgreSQL and the public strict zero-charge Netlify app are verified; the scheduled worker remains deliberately disabled pending a live reminder smoke**.

Current provider capabilities, account usage, and stable runtime versions were rechecked on 2 September 2026. The owner requires a deployment that cannot create a bill. Google Cloud is therefore excluded: its billing account is required even for Free Tier, and its alerts-only budget is not a global hard spending cap. Render was also rejected when it required a credit card before Blueprint creation. Never claim a deployment before its public journeys pass.

## Selected zero-charge topology — web/API provisioned

The selected target uses only free plans configured without a payment method:

```text
Supabase Free Auth + PostgreSQL
        │ TLS + Bearer JWT/JWKS
        ├── Netlify Free static Next export
        ├── Netlify Go Function: existing LifeHub API handler
        └── public-repository GitHub Actions worker (every 15 minutes)
```

Zero-charge guardrails:

- keep Netlify on Free without a payment method or team upgrade; the 300-credit monthly limit is hard, auto-recharge is unavailable on Free, and projects pause when the limit is reached;
- deploy only through `netlify.toml`, which produces the static web build and the Go Function on one same-origin site;
- keep Supabase on Free with no paid upgrade; its database becomes restricted when the free quota is exceeded rather than silently upgrading;
- use standard GitHub-hosted runners only in this public repository, where Actions usage is free;
- keep the scheduled worker disabled until the `LIFEHUB_FREE_DEPLOYMENT_ENABLED=true` repository variable is deliberately set after database verification;
- pin migration and worker runs to the already published API image commit instead of a mutable tag;
- create no LifeHub resource, secret, scheduler, image, service, or job in Google Cloud;
- Supabase's generally available asymmetric signing-key system exposes a rotatable public JWKS, matching the implemented Go verifier without exposing an auth secret to the API;
- the reminder worker remains durable because River jobs stay in PostgreSQL, but scheduled GitHub runs can be delayed.

Official references:

- [Netlify pricing](https://www.netlify.com/pricing/)
- [Netlify credit-based plan behavior](https://docs.netlify.com/manage/accounts-and-billing/billing/billing-for-credit-based-plans/credit-based-pricing-plans/)
- [Netlify Lambda-compatible Go Functions](https://docs.netlify.com/build/functions/lambda-compatibility/)
- [Netlify Function environment variables](https://docs.netlify.com/build/functions/environment-variables/)
- [GitHub Actions billing](https://docs.github.com/en/billing/concepts/product-billing/github-actions)
- [GitHub Packages billing](https://docs.github.com/en/billing/concepts/product-billing/github-packages)
- [Supabase database size](https://supabase.com/docs/guides/platform/database-size)
- [Supabase JWT signing keys](https://supabase.com/docs/guides/auth/signing-keys)

The tradeoff is availability rather than money: the Go Function can have cold-start latency; reminders can arrive late when GitHub delays a scheduled run; and Netlify or Supabase can pause/restrict the project when free quotas are exhausted. This profile is suitable for demonstration/hobby use, not a production SLA.

## Rejected/unselected Render alternatives

`render.free.yaml` remains schema-validated reference work, but it is no longer selected because Render required a card before provisioning. `render.yaml` remains a valid paid alternative for an always-on worker. Do not create or sync either file while the strict zero-charge requirement remains active.

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

The Go API and worker share application/store/domain code. The public API entry point is a Netlify Go Function; migrations and the bounded scheduled worker continue to use the pinned public container image through GitHub Actions.

`web`, `api`, the River worker, the shared application/River migrator, and PostgreSQL exist locally. `apps/web/Dockerfile` and `services/api/Dockerfile` still produce non-root production runtimes for container-capable alternatives. `netlify.toml` serves the Next static export and rewrites API/health routes to `services/api/netlify/functions/api/main.go`, which boots the same authenticated Go HTTP handler with a bounded serverless PostgreSQL pool.

`.github/workflows/publish-images.yml` publishes the API/worker/migrator image. `.github/workflows/free-migrate.yml` applies migrations manually, and `.github/workflows/free-worker.yml` runs the same pinned image for a bounded 45-second window every 15 minutes. The GHCR image was anonymously pullable before this profile was selected.

The checked-in paid Blueprint selects Render plans for web/API/worker and PostgreSQL. Those are paid resources. Blueprint creation or sync is prohibited unless the owner later replaces the strict zero-charge policy with explicit cost approval.

On 2 September 2026, Netlify CLI 27.4.2 completed an offline official build: Next 16.3.3 produced `apps/web/out`, and Netlify packaged the Go Function successfully. On 4 September the same topology was deployed publicly and its hosted browser/API journey passed. `render.free.yaml` and `render.yaml` remain schema-valid historical alternatives but are prohibited by the selected cost policy.

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
APP_ENV=production
DATABASE_URL=
DATABASE_MAX_CONNS=2
DATABASE_MIN_CONNS=0
WEB_ORIGIN=
SUPABASE_ISSUER=
SUPABASE_JWKS_URL=
SUPABASE_AUDIENCE=authenticated

# Container API alternatives only; Netlify Functions do not use these
HTTP_ADDR=
PORT=

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

`APP_ENV=production` requires HTTPS for the configured web origin, issuer, and JWKS URL, ignores the development secret, and does not expose the development-session route. On Netlify, `WEB_ORIGIN` may be omitted because the API derives the exact same-origin value from Netlify's platform `URL`; an explicit value must still be HTTPS. Keep the Function pool at `DATABASE_MAX_CONNS=2` and `DATABASE_MIN_CONNS=0` to avoid multiplying idle connections across cold instances.

Netlify configuration-file variables are build-time settings and are not a substitute for Function runtime secrets. Store `DATABASE_URL` only in the Netlify environment UI with Function scope. The Supabase publishable key is browser-safe; the database URL is not.

## Container strategy — implemented locally

- API/worker/migrator: multi-stage `golang:1.27.0-alpine3.24` build; static CGO-disabled binaries; `alpine:3.24.1` runtime; CA certificates and IANA timezone data; non-root `lifehub` user.
- Web: multi-stage `node:24.19.0-alpine3.24`; exact pnpm 11.25.0 frozen install; Next standalone output by default; opt-in static export for the hard-free profile; non-root `lifehub` user in the container profile.
- Build contexts use dedicated `.dockerignore` files so secrets, caches, reports, and unrelated development artifacts are not copied.
- The API honors an explicit `HTTP_ADDR`; when it is absent in production, the platform `PORT` becomes `0.0.0.0:$PORT`. Non-production listeners remain restricted to loopback.

## Database migrations

The API image includes `/usr/local/bin/migrate`, which applies the embedded seven-version Goose schema and River schema target 7. The hard-free profile runs this binary through the manual `free-migrate.yml` workflow before creating the Netlify site. API startup itself never races to migrate. The same pinned image includes `/usr/local/bin/worker` for the scheduled workflow.

The migrator is safe to rerun when both schemas are current and refuses an unsupported newer River schema. GitHub Actions run `33546915708` applied both schemas successfully to the Supabase Free database on 2 September 2026. A provider backup/restore procedure remains a production-readiness item.

## Rollout sequence

Safe hard-free order:

1. [done] create the Supabase Free project, use its asymmetric signing key, and verify the public ES256 JWKS;
2. [done] add only `LIFEHUB_DATABASE_URL` as a GitHub Actions secret and pass the manual migration workflow;
3. [done] create/link a Netlify Free account without adding a payment method and grant access only to the public LifeHub repository;
4. [done] configure the public Supabase build values and the scoped server-only Function values, including the bounded `2/0` pool;
5. [done] deploy `netlify.toml`, then configure Supabase Site URL/redirect URLs for the final `netlify.app` HTTPS origin;
6. [done] smoke-test hosted session recovery, Today/Agenda, and an authenticated task write/read cycle;
7. set `LIFEHUB_FREE_DEPLOYMENT_ENABLED=true`, run the worker manually once, and verify a scheduled reminder;
8. [done] verify ingress headers plus account dashboards show Free plans, no payment method, no auto-recharge, and no Google Cloud or Render LifeHub service.

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

CI repeats both production image builds after the web and Go gates, using non-secret public placeholders for the Supabase/web build boundary. The separate manual publish workflow emits immutable commit tags plus `latest`; its result alone is not deployment proof. GitHub Actions run `33551926366` passed the web, Go integration/race, container-build, and eight-case E2E jobs for the initial public-deployment commit. The manual hosted migration run `33546915708` migrated Supabase successfully. The hard-free static export and Netlify Go Function then deployed publicly.

On 4 September 2026, `https://rainbow-cocada-24a2a9.netlify.app` returned HTTP 200, `/healthz` and `/readyz` returned healthy/ready, the real ingress retained the expected production headers, private endpoints rejected unauthenticated requests with 401/no-store, and the development-session endpoint was absent. A confirmed Supabase session recovered in the browser, loaded Today and Agenda, and persisted an authenticated task create/complete/uncomplete/reload cycle. The fresh local suite now has nine passing browser cases, including a 1920×1080 Today-layout regression. Live scheduled reminder delivery, explicit hosted sign-out/re-login, backup/restore proof, and monitoring remain open.

## Production checklist

- [x] HTTPS
- [x] allowed same-origin CORS behavior
- [x] Supabase JWT/JWKS provisioned and reachable
- [x] no server secret configured at the browser build boundary
- [x] DB TLS through the hosted Supabase connection
- [ ] backups enabled
- [ ] restore procedure documented/tested
- [x] migrations tested against hosted Supabase
- [x] API health/readiness at public ingress
- [ ] worker restart policy
- [x] graceful shutdown in container/runtime smoke
- [x] log redaction behavior covered by tests and production error boundary
- [x] dependency audit
- [x] `govulncheck ./...`
- [x] Playwright critical journey locally plus hosted authenticated task smoke
- [x] LifeHub application-data deletion procedure (typed confirmation, owned cascade, queued reminder cancellation)
- [ ] Supabase authentication-identity deletion procedure, if required by the production policy
- [ ] monitoring/alerting proportional to real operational needs

## Deployment record

```text
Web provider: Netlify Free
Web URL: https://rainbow-cocada-24a2a9.netlify.app
API provider: Netlify Go Function (same origin)
API URL: https://rainbow-cocada-24a2a9.netlify.app/api/v1
Worker provider: GitHub Actions public-repository runner (configured, disabled pending live smoke)
PostgreSQL provider: Supabase Free
Region: Singapore (database); Netlify managed edge/function
Deployment date: 4 September 2026
Initial verified web/API commit: 6406d88
Migration version: application 7; River 7
Go version: 1.27.0
Node version: 24.19.0 target
Known limitations: free-quota suspension/cold starts; worker not enabled; reminder delivery, explicit hosted sign-out/re-login, backup/restore, and monitoring remain unverified
```
