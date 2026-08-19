# LifeHub Implementation Plan

Status: **First vertical slice implemented and locally verified**
Last updated: **19 August 2026**

## Inspected baseline

The supplied workspace was a flattened documentation-only cold-start bundle:

- 13 Markdown files and no application code, package manifest, Go module, migration, tests, or environment file;
- no `.git` metadata in the opened folder;
- the intended `AGENTS.md`, `README.md`, and `SKILL.md` arrived with filename collision suffixes and have been normalized without changing their content;
- the intended engineering documents arrived at repository root and have been moved under `docs/`;
- tools at inspection: Node.js 22.21.0, pnpm 11.20.0, Go 1.26.1, Docker 29.7.2, and Docker Compose 5.4.0;
- Docker Desktop is available; `psql`, `sqlc`, Goose, Supabase CLI, and `govulncheck` are not installed globally;
- no Supabase project configuration or other application credentials exist in the workspace.

The workspace now activates pnpm 11.22.0 and Go's module toolchain downloads Go 1.26.6. Node 24.19.0 remains the CI/production target; the inspected Node 22 host satisfies the dependency engine ranges used for local verification. Exact selections and compatibility pins are recorded in `docs/stack-versions.md`.

## Phase 0 — Inspect — complete

- [x] inventory the repository and environment;
- [x] read every supplied project document and the LifeHub skill;
- [x] confirm that no manifests, modules, migrations, scripts, or UI exist yet;
- [x] normalize the supplied documentation layout;
- [x] verify stable dependency versions and compatibility constraints;
- [x] update this plan and `docs/stack-versions.md`.

Exit met: this plan reflects the documentation-only baseline.

## Phase 1 — Minimal production skeleton — complete for the first slice

Web:
- Next.js App Router;
- strict TypeScript;
- Tailwind;
- design tokens;
- app shell;
- API client boundary.

Backend:
- Go module;
- config;
- `net/http` + chi;
- structured logs;
- request IDs;
- recovery;
- health/readiness;
- graceful shutdown;
- PostgreSQL pool;
- migrations;
- explicit parameterized pgx queries behind a store boundary;
- safe API error envelope.

Local/CI:
- `compose.yaml`;
- `.env.example`;
- web and Go quality gates.

Exit:
- web builds;
- API runs;
- DB connection/migrations work;
- health endpoints work;
- no secrets committed.

Exit met locally. PostgreSQL 18.4 runs in Docker on port `55432`, both Goose migrations are applied, the web production build passes, and API health/readiness were exercised. sqlc remains a verified, pinned future option but was deliberately not added to the compiled/tool graph for this compact query surface.

Shadcn/Base UI is intentionally deferred until repeated component patterns justify importing generated component code.

## Phase 2 — First useful vertical slice — complete locally

Target journey:

```text
verified identity
    → persisted profile/timezone
    → create task due today
    → Go Today aggregation shows it
    → complete it
    → reload and observe persisted state
```

Backend scope:

- production Supabase-compatible asymmetric JWKS verification with issuer, audience, expiry, and subject validation;
- an explicitly local-only development issuer so the flow can run without external credentials, with production startup rejecting that mode;
- user profile with validated IANA timezone;
- task schema, validation, ownership-scoped writes, and completion action;
- deterministic, user-timezone Today window and ordering;
- PostgreSQL-backed persistence through explicit, parameterized pgx SQL;
- request IDs, safe errors, body limits, CORS allowlist, health/readiness, and graceful shutdown.

Frontend scope:

- stable Supabase browser-session boundary for configured environments;
- clearly labelled local development login fallback;
- responsive Today screen, timezone setting, quick task creation, completion, loading/empty/error states, and keyboard-visible focus;
- no fake navigation, analytics, or unimplemented feature surfaces.

Required proof:

- Go domain and HTTP tests;
- real-PostgreSQL migration/repository/ownership integration tests;
- Vitest tests for frontend logic;
- Playwright journey at a mobile viewport, including reload persistence;
- typecheck, lint, production build, `go test`, `go vet`, vulnerability scan, and a Linux race run when the Windows host lacks a race-capable C toolchain.

Exit: the complete journey above passes against PostgreSQL. Missing external Supabase credentials may leave the production provider manually unverified, but its JWT verifier must be covered with local asymmetric-key tests.

Exit met for the local development provider on 19 August 2026:

- a new authenticated user sees an explicit timezone onboarding state;
- `Asia/Jakarta` is persisted before task creation is permitted;
- a due-today task is created, aggregated and ordered by Go, completed idempotently, and still completed after browser reload;
- the Playwright journey passes at 390×844 against the live Go API and PostgreSQL;
- Go unit/HTTP/auth/time tests and real-PostgreSQL ownership/persistence tests pass;
- Linux `go test -race`, `go vet`, and `govulncheck` pass;
- TypeScript, ESLint, 11 Vitest tests, and the Next production build pass;
- React development-mode client cancellation is recorded as status 499 rather than an internal 500, with a regression test and a successful live E2E replay.
- non-production auth and the Docker database bind to loopback; `Local` is rejected as a profile timezone;
- Today returns all matching work rather than silently truncating trusted dashboard buckets;
- the E2E command starts a fresh migrated API/web stack instead of reusing an accidentally stale process;
- primary navigation exposes only working Today and Add actions; mobile places Today before Quick Add, and the timezone dialog traps/restores focus and closes with Escape.

Hosted Supabase sign-in and deployment are not claimed: no hosted credentials or production environment were supplied. The production verifier is instead covered by an asymmetric JWKS test server with issuer, audience, algorithm, expiry, and subject checks.

## Phase 3 — Hosted auth and production validation

- manually verify Supabase sign-in, refresh, sign-out, and browser session recovery against a real project;
- deploy the already implemented production JWKS path with real HTTPS issuer/JWKS configuration;
- add a second-user browser isolation/sign-out journey against the hosted provider;
- decide on a stable server-session package only when Supabase no longer documents `@supabase/ssr` as beta, or approve that exception explicitly.

Already automated at the API/store layer:
- missing, malformed, expired, forged, wrong-algorithm, wrong-issuer, and wrong-audience tokens are rejected;
- user A cannot read or complete user B's task, and cross-owner completion is indistinguishable from missing data.

Exit: the configured Supabase sign-in/refresh/sign-out flow is manually verified against a real project, and authenticated users reach Today with persisted timezones. Full task edit/delete, recurrence, reminders, and uncomplete remain follow-on task work.

## Phase 4 — Events

- event schema/domain;
- timezone-safe start/end;
- all-day;
- Today integration;
- simple recurrence foundation;
- create/edit UI;
- calendar/upcoming view.

Test midnight, all-day, timezone, invalid end-before-start.

## Phase 5 — Bills

- integer amount;
- due;
- paid state;
- recurrence;
- Today/upcoming;
- UI;
- mark paid.

No accounting ledger.

## Phase 6 — Documents

- metadata only;
- category;
- date-only expiry;
- expiry status;
- Today/upcoming;
- UI.

No file upload in MVP.

## Phase 7 — Reminder engine

Schema:
- reminders;
- notifications;
- River job infrastructure.

Worker:
- job definitions;
- bounded concurrency;
- retry;
- graceful shutdown;
- idempotency.

Scheduling:
- task/event/bill/document reminders;
- edit/delete invalidates stale schedules.

Notification center:
- unread count;
- list;
- mark read.

Integration:
- process a reminder;
- retry same job;
- verify one visible notification.

Exit: restart-safe and duplicate-safe.

## Phase 8 — Recurrence hardening

Support:
- daily;
- weekly;
- monthly;
- yearly;
- interval;
- optional end.

Clarify occurrence materialization before coding.

Test month-end, leap year, timezone, series edit/delete, bill payment behavior.

## Phase 9 — Smart Quick Add

First:
- provider interface;
- deterministic parser;
- mock provider;
- structured draft;
- ambiguity review.

Optional remote AI only after current official docs are researched.

Never save without confirmation.

## Phase 10 — PWA / Web Push

Only after in-app reminders are reliable:
- manifest/icons;
- service worker if current stack supports it;
- permission only after meaningful action;
- push subscriptions;
- cleanup/revocation;
- VAPID/server push if selected.

## Phase 11 — Polish

- 390×844;
- 360px;
- desktop;
- keyboard;
- 200% zoom;
- reduced motion;
- loading/error/empty;
- accessibility;
- performance;
- privacy copy.

## Phase 12 — Production readiness

- clean-DB migration smoke;
- backup/restore docs;
- deployment;
- TLS;
- CORS;
- environment secrets;
- worker deployment;
- health/readiness;
- data deletion;
- dependency audit;
- govulncheck;
- critical Playwright journey.

## Definition of done

A feature is done only when:
- domain behavior exists;
- authorization exists;
- validation exists;
- persistence exists;
- user-facing states exist;
- tests pass;
- docs are updated if needed;
- accessibility is considered;
- error recovery exists;
- relevant commands were actually run.
