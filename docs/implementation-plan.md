# LifeHub Implementation Plan

Status: **MVP domain slices, durable reminders, recurrence, Smart Quick Add, and local release gates are verified; hosted deployment is pending external configuration and approval**
Last updated: **30 August 2026**

## Inspected baseline

The supplied workspace was a flattened documentation-only cold-start bundle:

- 13 Markdown files and no application code, package manifest, Go module, migration, tests, or environment file;
- no `.git` metadata in the opened folder;
- the intended `AGENTS.md`, `README.md`, and `SKILL.md` arrived with filename collision suffixes and have been normalized without changing their content;
- the intended engineering documents arrived at repository root and have been moved under `docs/`;
- tools at inspection: Node.js 22.21.0, pnpm 11.20.0, Go 1.26.1, Docker 29.7.2, and Docker Compose 5.4.0;
- Docker Desktop is available; `psql`, `sqlc`, Goose, Supabase CLI, and `govulncheck` are not installed globally;
- no Supabase project configuration or other application credentials exist in the workspace.

The workspace now activates pnpm 11.24.0 and Go's module toolchain downloads Go 1.27.0. Node 24.19.0 remains the CI/production target; the inspected Node 22 host satisfies the dependency engine ranges used for local verification. Exact selections and compatibility pins are recorded in `docs/stack-versions.md`.

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

Exit met locally. PostgreSQL 18.4 runs in Docker on port `55432`, all five Goose migrations are applied, the web production build passes, and API health/readiness were exercised. sqlc remains a verified, pinned future option but was deliberately not added to the compiled/tool graph for this compact query surface.

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

## Phase 4 — Events — focused create-to-Today slice complete locally

Current production-worthy slice:

```text
verified identity + persisted timezone
    → create a timed or all-day event
    → Go validates local time/date semantics and ownership
    → PostgreSQL persists the event
    → Go merges it into the ordered Today timeline
    → reload and observe persisted event state
```

Included in this slice:

- an owned event schema with mutually exclusive `timestamptz` timed fields and `date` all-day fields;
- optional notes/location with database and Go validation;
- timezone-safe local start/end parsing that rejects DST gaps/folds and end-before-start;
- single-day and multi-day all-day events without converting date-only values to UTC midnight;
- event creation through the authenticated Go API;
- mixed task/event Today aggregation and deterministic ordering;
- responsive, keyboard-accessible task/event creation controls in the existing Today-first UI;
- Go domain/HTTP/store integration coverage, frontend normalization/date tests, and a 390×844 persistence journey.

Explicitly deferred from this slice:

- event edit/delete and a separate Calendar/history view;
- recurrence and reminders;
- hosted Supabase validation, which still requires external project credentials.

Required edge proof: timed events at local-day boundaries, ongoing overlap, all-day date ranges, DST-invalid wall times, end-before-start, ownership isolation, mixed Today ordering, and reload persistence.

Exit met locally on 20 August 2026:

- migration 3 creates the strict timed/all-day event representation and its ownership/index constraints;
- HTTP and unit tests cover required shape selection, profile readiness, local-time conversion, invalid DST gaps/folds, end ordering, all-day ranges, and mixed deterministic Today ordering;
- real-PostgreSQL integration tests cover overlap boundaries, point and cross-midnight events, ownership isolation, persistence, database constraints, and an uncapped result set;
- TypeScript, ESLint, 22 Vitest tests, and the Next.js production build pass;
- Playwright passes the fresh-stack 390×844 journey that creates a task and all-day event, verifies both in Today, completes the task, reloads, and verifies persisted event/location and task state;
- manual 390×844 browser QA confirms the task/event chooser, all-day form, event row, toast, location, and reload persistence render correctly;
- Go unit/HTTP tests, real-database integration tests, `go vet`, `govulncheck`, and Linux Go 1.26.6 integration race tests pass;
- pnpm reports no known high-severity vulnerabilities and verifies registry signatures for all 502 installed packages.

## Phase 5 — Bills — focused create-to-Today/payment slice complete locally

Current vertical slice:

```text
verified identity + persisted timezone
    → create an owned bill with integer money and a local due moment
    → PostgreSQL persists it
    → Go includes due/overdue bills in Today
    → mark it paid idempotently
    → reload and observe persisted paid state
```

Included now:

- positive integer `amount` with three-letter uppercase currency defaulting to `IDR`; no floats;
- `due_local` interpreted by Go in the authenticated profile's IANA timezone and stored as `timestamptz`;
- ownership-scoped create, Today query, and explicit idempotent `mark-paid` action;
- bill-aware Today DTO, ordering, summary, Indonesian row/form copy, and 390×844 reload-persistence coverage;
- database constraints plus unit, HTTP, real-PostgreSQL, frontend, browser, build, and race proof.

Deferred from this focused slice:

- list/get/edit/delete and `mark-unpaid`;
- recurrence and reminders, which are handled in their dedicated phases;
- a separate Bills page and longer upcoming horizon.

No accounting ledger.

Exit met locally on 20 August 2026:

- migration 4 adds ownership-scoped bills with database-enforced title, notes, safe-integer amount, uppercase currency, and due-time constraints;
- the Go API implements bill creation and idempotent `mark-paid`, derives ownership from the verified subject, preserves the first `paid_at`/`updated_at`, and merges unpaid/paid-today bills into the uncapped deterministic Today feed;
- HTTP/unit and real-PostgreSQL tests cover default/invalid currency, integer bounds, profile readiness, IANA/DST conversion, ownership, payment idempotency, local-day boundaries, constraints, no truncation, and mixed task/event/bill ordering;
- TypeScript, ESLint, 29 Vitest tests, and the Next.js production build pass;
- Playwright passes a fresh-stack 390×844 journey through task, event, and bill creation, task completion, bill payment, and reload persistence against migration 4;
- `go test`, real-database integration tests, `go vet`, `govulncheck`, and Linux Go 1.26.6 integration race tests pass.

## Phase 6 — Documents — focused expiry-to-Today and management slice complete locally

Included now:

- owned metadata only: name, category, optional notes, and date-only expiry;
- no upload, scan, sensitive identifier, or document-file storage;
- profile-local calendar-date status: expired before today, expiring from today through day 30 inclusive, and valid after day 30;
- expired and expiring-today documents in the primary Today feed without inflating resolved counts;
- tomorrow through day 30 in a separate, uncapped Upcoming feed;
- a complete owned-document manager so records outside the Upcoming horizon remain reachable;
- create, list, edit, clear-notes, and explicitly confirmed delete flows;
- ownership-scoped SQL, `Cache-Control: no-store`, and CORS support for `DELETE`.

Exit met locally on 20 August 2026:

- migration 5 adds database-enforced metadata/category/date constraints and an ownership/expiry index;
- Go unit, HTTP, and real-PostgreSQL tests cover strict create/PATCH shapes, `notes: null`, day 30/31 boundaries, Today/Upcoming separation, full CRUD persistence, ownership isolation, uncapped results, and deterministic mixed ordering;
- `go test`, real-database integration tests, `go vet`, `govulncheck`, and Linux Go 1.26.6 integration race tests pass;
- TypeScript and ESLint pass, all 36 Vitest tests pass, and the Next.js production build passes;
- Playwright passes a fresh-stack 390×844 journey against migration 5 that exercises task/event/bill behavior plus document Today, separate day-10 Upcoming, day-60 manager reachability, edit/category change, note clearing, delete, and reload persistence.

## Phase 7 — Agenda & Corrections — complete locally

Purpose: keep Today primary while making future work reachable and existing task/event/bill data safely correctable without creating four generic admin dashboards.

Backend scope:

- `GET /api/v1/agenda` with an authenticated profile-local inclusive date range, a 31-calendar-date maximum, no silent cap, deterministic mixed-domain ordering, and no N+1 queries;
- extend Today Upcoming to unresolved tasks, events, bills, and documents for tomorrow through day 30 inclusive;
- ownership-scoped task get/edit/delete/uncomplete;
- ownership-scoped event bounded list/get/edit/delete, including strict full-schedule replacement for timed/all-day conversion;
- ownership-scoped bill list/get/edit/delete/mark-unpaid, with stable cursor pagination for paid history;
- strict omitted/null/value PATCH semantics, idempotent inverse state actions, private `no-store` responses, and indistinguishable missing/cross-owner 404 responses.

Web scope:

- real `Today`/`Agenda` navigation with browser history support while Today remains the default;
- a date-grouped Agenda timeline with a seven-day strip, date picker, and domain filters;
- nearest-five Upcoming preview plus an explicit route to the full Agenda result;
- one accessible shared editor sheet, responsive as a mobile bottom sheet and desktop side sheet;
- explicit delete confirmation and undo actions for task completion and bill payment.

Verified proof:

- calendar and DST boundaries, event overlap, +30/+31 horizon behavior, ordering, ownership, strict PATCH unions, inverse-action idempotency, cursor stability, no truncation, and real-PostgreSQL persistence;
- unit, HTTP, integration, race, vet, vulnerability, typecheck, lint, build, mobile/desktop E2E, keyboard, focus, and browser back/forward checks.

Exit met locally:

- future items are reachable, corrections persist, and Today/Agenda transitions refresh coherently;
- backend unit, HTTP, real-PostgreSQL integration, vet, vulnerability, and Linux race gates pass;
- boundary regressions cover ordinary DST changes, Cairo/Santiago midnight gaps, Apia's skipped civil date, event overlap, and neighboring-date leakage;
- frontend typecheck, lint, all 52 Vitest tests, and production build pass;
- the combined fresh-stack Playwright suite passes three mobile/desktop journeys, including browser history, editor focus, mutation locking, 401 recovery, local-day rollover, and reload persistence;
- no migration or new runtime dependency was required.

No recurrence or reminder behavior is implied by this phase.

## Phase 8 — Reminder engine — complete locally

Schema:
- owned reminder definitions for tasks, events, bills, and documents;
- immutable schedule generations and one durable occurrence identity per fire time;
- idempotent in-app notifications;
- pinned River PostgreSQL job infrastructure, with no Redis or in-memory timer as the source of truth.

Worker:
- job definitions;
- bounded concurrency;
- retry;
- graceful shutdown;
- generation checks and transaction-level idempotency.

Scheduling:
- task/event/bill/document reminders;
- source edits, completion/payment, and deletion invalidate stale schedules;
- one-off reminder controls remain independent of recurrence, which is Phase 9.

Notification center:
- unread count;
- stable cursor list;
- mark one or all read;
- polling only while the page is visible; web push remains deferred.

Integration:
- process a reminder;
- retry same job;
- restart the worker and retry the same job;
- verify exactly one visible notification.

Exit met locally:

- migration 6 adds owned reminder definitions, immutable schedule generations, and idempotent notifications; the migrator pins River schema target 7 and rejects missing or newer incompatible versions;
- task, timed/all-day event, bill, and document adapters validate strict moment/date schedule unions and atomically enqueue, cancel, or replace River jobs with source state changes;
- the worker has bounded concurrency, retries, job timeouts, bounded graceful/hard shutdown, terminal reconciliation, and private job arguments containing only schedule identity and generation;
- a real-PostgreSQL integration test forces an actual River failure, stops the first client, completes attempt two after a worker restart, and proves exactly one visible notification;
- the web provides four-domain reminder controls plus a cursor-paginated notification center with persisted unread/read state and visible-page-only polling;
- typecheck, lint, 58 Vitest tests, production build, five fresh-stack Playwright mobile/desktop journeys, backend unit/HTTP/integration, vet, `govulncheck`, and Linux integration race all pass;
- pnpm reports no known high-severity vulnerabilities and all 502 registry package signatures verify.

Web Push remains deferred. Exit: restart-safe and duplicate-safe.

## Phase 9 — Recurrence hardening — complete locally

Support:
- daily;
- weekly;
- monthly;
- yearly;
- interval;
- optional end.

LifeHub now materializes a bounded 90-day window in PostgreSQL, extends it with a durable twice-daily River sweep, and retains the original anchor so old or future series do not drift. Individual occurrence edits become exceptions; individual deletion becomes a soft exclusion; series edits regenerate only eligible future occurrences; stopping a series preserves completed/paid history and cancels future work/reminders.

Month-end clamping without drift, leap years, DST gap/fold handling, skipped local dates, ownership, idempotency, edit/stop behavior, exclusions, reminder cancellation, and worker sweeps are covered by unit, HTTP, real-PostgreSQL integration, and mobile Playwright tests.

## Phase 10 — Smart Quick Add — complete locally

First:
- provider interface;
- deterministic parser;
- mock provider;
- structured draft;
- ambiguity review.

Optional remote AI only after current official docs are researched.

Never save without confirmation.

Implemented on 27 August 2026 with a deterministic Indonesian rule provider, a mock provider, bounded input/output validation, a two-second provider timeout, ambiguity reporting, and an authenticated draft-only API. The Today Quick Add UI fills the existing manual form and requires the normal explicit save action. Mobile Playwright proves the recurring-bill example does not write before review and does persist only after the missing time is supplied and Save is pressed. No remote AI dependency or credential is required.

## Phase 11 — PWA / Web Push

Only after in-app reminders are reliable:
- manifest/icons;
- service worker if current stack supports it;
- permission only after meaningful action;
- push subscriptions;
- cleanup/revocation;
- VAPID/server push if selected.

The installable manifest and application icon are implemented. A service worker and Web Push remain deliberately deferred: in-app notifications already provide the reliable MVP path, and push permission/subscription/cleanup would add a separate security and operations surface that has not yet been justified.

## Phase 12 — Polish — in progress

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

Completed so far: responsive Today/Agenda/forms at the primary 390×844 and desktop Playwright viewports, keyboard/focus behavior for navigation/dialogs/sheets, reduced-motion CSS, loading/error/empty states, calm privacy copy, production CSP/security headers, and non-root standalone containers. Final manual 360px, 200% zoom, browser accessibility, and production performance checks remain before release.

## Phase 13 — Production readiness — in progress

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

Completed locally:

- non-root API/worker/migrator and standalone web production images build and smoke successfully;
- a paid Render Singapore Blueprint defines API, worker, web, and private PostgreSQL 18 without embedding secrets;
- the Blueprint uses current plan identifiers, `checksPass` deployment triggers, and passes Render's current published JSON Schema; the official CLI's workspace-backed validation remains a provisioning check;
- immutable-action CI passes `actionlint` locally, and GitHub Actions run `33265715718` passed web/Go quality gates, both production image builds, and the persisted eight-case E2E suite;
- production startup validates HTTPS configuration, ignores development auth, supports platform `PORT`, and emits security headers;
- `/readyz` verifies PostgreSQL plus exact application/River migration versions;
- API/worker graceful shutdown and migrator no-op behavior were smoke-tested;
- Go 1.27.0 unit/HTTP tests, real-PostgreSQL integration tests, Linux/WSL integration race, vet, and `govulncheck` pass;
- Next 16.3.3 typecheck, lint, all 63 Vitest tests, and production build pass;
- the complete eight-case Playwright suite passes at 390×844 and 1280×900 against a fresh migrated PostgreSQL database; E2E is capped at two workers to keep the Next dev server and API responsive on small hosts;
- exact-confirmation application-data deletion is implemented through the verified identity, cancels scheduled reminder jobs transactionally, cascades owned PostgreSQL rows, and clearly leaves the separate Supabase identity intact. HTTP/client tests, PostgreSQL integration proof, and the mobile E2E journey all pass.
- the final lockfile has no known high-severity advisory, all 502 packages have verified registry signatures, the only outdated web packages are three documented compatibility pins, the Go graph resolves no retracted module, and `govulncheck` reports no vulnerability.

Release gates still open on 30 August 2026:

- repeat final API/worker/migrator and standalone web image builds on the local Docker host after the PostgreSQL 18.6 pin (Docker Desktop is unavailable on this host; GitHub Actions run `33265715718` completed the checked-in `containers` gate successfully);
- complete manual 360px/200%-zoom/accessibility verification;
- define and test production backup/restore plus any required Supabase identity-deletion procedure;
- obtain explicit approval for the paid Render plans and hosted Supabase/public-origin values;
- after provisioning, verify hosted auth/session recovery, TLS/CORS/headers, worker delivery, monitoring, and the critical production journey.

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
