# LifeHub Stack Versions

Verified: **30 August 2026** against official release pages, npm registry metadata, container registries, and the Go module proxy. Runtime pins and post-update gates reflect the recurrence, Smart Capture, data deletion, and production-hardening slices.

This file distinguishes the host environment, the compatible versions selected for the first slice, and dependencies intentionally deferred. The root `pnpm-lock.yaml`, `apps/web/package.json`, and `services/api/go.mod` are the installed source of truth.

## Policy

Use latest stable, security-patched, mutually compatible versions.

Never use beta/canary/RC/deprecated/abandoned dependencies unless explicitly approved and documented.

**Compatibility beats the largest version number.**

## Runtime and database

| Technology | Current stable target | Host at inspection | Decision |
|---|---:|---:|---|
| Go | 1.27.0 | 1.26.1 system; 1.27.0 module toolchain | `go 1.27.0` makes the Go tool download/use the current stable release through `GOTOOLCHAIN=auto`. |
| Node.js | 24.19.0 LTS | 22.21.0 | Target Node 24 LTS in CI/production. Host Node 22 still satisfies selected package engines and may bootstrap locally. |
| pnpm | 11.24.0 | 11.20.0 initially; 11.24.0 activated | Pinned by `packageManager`; the frozen workspace install uses 11.24.0. |
| PostgreSQL | 18.6 | 18.6-alpine in Docker; 18.6 Windows service and 16.15 WSL test cluster | Selected current supported 18.x minor; PostgreSQL 19 beta is excluded. Local host port is `55432` because the usual range was reserved on this Windows host. |
| Docker Engine | 29.7.2 | 29.7.2 | Local infrastructure only; not an application dependency. |
| Docker Compose | 5.4.0 | 5.4.0 | Available locally. |

## CI action selection

The local workflow added on 20 August 2026 pins each third-party action to the immutable commit behind its current stable release:

| Action | Stable release | Pinned commit |
|---|---:|---|
| `actions/checkout` | v7.0.1 | `3d3c42e5aac5ba805825da76410c181273ba90b1` |
| `pnpm/setup` | v2.0.2 | `84cb39b217b10273981911c288cd62326dc7c6d2` |
| `actions/setup-go` | v7.0.0 | `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` |
| `actions/upload-artifact` | v7.0.1 | `043fb46d1a93c77aae656e7c1c64a875d1fc6a0a` |

`pnpm/setup` is the stable pnpm-maintained action for pnpm 11+ self-contained binaries and installs the selected Node 24.19.0 runtime without a second Node setup action. The workflow has passed `actionlint` 1.7.12 locally; it is not claimed as a successful GitHub-hosted run until pushed.

## Frontend selection

| Package | Registry latest | Selected for first slice | Reason |
|---|---:|---:|---|
| next | 16.3.3 | 16.3.3 | Stable App Router patch release. |
| react / react-dom | 19.2.8 | 19.2.8 | Exact matching patches; satisfies Next peers. |
| typescript | 7.0.2 | **6.0.3** | Intentional compatibility pin: the current `typescript-eslint` peer range is `<6.1.0`; TypeScript 7 tooling APIs are not yet supported by this lint stack. |
| tailwindcss | 4.3.3 | 4.3.3 | Stable. |
| @tailwindcss/postcss | 4.3.3 | 4.3.3 | Exact Tailwind match. |
| postcss | 8.5.26 | 8.5.26 | Satisfies Tailwind's peer/dependency range. |
| eslint | 10.8.1 | **9.39.5** | Intentional compatibility pin: plugins resolved by the current Next config support ESLint 9. |
| eslint-config-next | 16.3.3 | 16.3.3 | Exact Next match. |
| @types/node | 26.2.0 | **24.13.3** | Match the Node 24 production target. |
| @types/react | 19.2.18 | 19.2.18 | Match React 19. |
| @types/react-dom | 19.2.5 | 19.2.5 | Match React DOM 19. |
| vitest | 4.1.11 | 4.1.11 | Stable unit-test runner; compatible with Node 22/24 and Vite 8. |
| @playwright/test | 1.62.1 | 1.62.1 | Stable browser test runner. |
| lucide-react | 1.37.0 | 1.37.0 | Stable accessible icon primitives. |
| @supabase/supabase-js | 2.112.4 | 2.112.4 | Stable browser auth/session boundary. |

### Verified but deferred

| Package | Current version | Decision |
|---|---:|---|
| zod | 4.4.3 | Defer until shared client validation is complex enough to justify it; Go remains authoritative. |
| motion | 13.1.0 | Defer; the first slice does not need animation infrastructure. |
| react-hook-form | 7.85.0 | Defer; the first task form is simpler with native semantics. |
| @supabase/ssr | 0.12.4 | Defer. The npm tag is non-prerelease, but official Supabase documentation still calls the API beta/unstable, which conflicts with this project's strict no-beta policy. |

Shadcn/ui is code-distribution tooling rather than a runtime package. Defer it until repeated component patterns justify adding generated component code.

### Events → Today dependency decision

The implemented Events → Today vertical slice required **no new frontend dependency**. The existing React/native form primitives, date helpers, Lucide icons, Vitest, and Playwright were sufficient. In particular, Zod and React Hook Form remain deferred; Go remains authoritative for the strict timed/all-day union and timezone validation.

### Bills → Today dependency decision

The implemented Bills → Today vertical slice also required **no new frontend dependency**. Native number/text/date controls, `Intl.NumberFormat`, the existing API boundary, Vitest, and Playwright cover integer-money capture, rupiah display, payment state, and persistence. Go and PostgreSQL remain authoritative for safe-integer bounds, ownership, and idempotent payment.

### Documents → Today/management dependency decision

The implemented document slice required **no new frontend dependency**. Native date/select/details/dialog semantics, the existing Lucide set, Vitest, and Playwright cover metadata creation, Today/Upcoming presentation, full-list management, note clearing, deletion, and persistence. Date-only status and boundaries remain Go/PostgreSQL responsibilities; no upload/storage SDK was added because document files are outside MVP scope.

## Go selection

| Module/tool | Verified version | First-slice decision |
|---|---:|---|
| github.com/go-chi/chi/v5 | v5.3.2 | Runtime HTTP router. |
| github.com/jackc/pgx/v5 | v5.10.0 | Runtime PostgreSQL driver/pool. |
| github.com/golang-jwt/jwt/v5 | v5.3.1 | JWT claims/signature validation. |
| github.com/MicahParks/keyfunc/v3 | v3.8.1 | Stable JWKS retrieval, caching, and key-rotation support instead of custom key parsing. |
| github.com/pressly/goose/v3 | v3.27.3 | Embedded migration runner and explicit migration command. |
| golang.org/x/vuln/cmd/govulncheck | v1.7.0 | Tracked Go tool invoked with `go tool govulncheck`; no global install required. |
| modernc.org/libc | v1.75.3 | Indirect compatibility override: Goose 3.27.3 declares retracted v1.74.3, whose retraction reports a DNS deadlock. The current stable patch removes that retracted version from the resolved graph. |
| github.com/riverqueue/river | v0.44.0 | Exact runtime pin for durable PostgreSQL jobs. Registry latest is v0.46.0, but River remains pre-1.0 and the schema/worker contracts are verified together at v0.44.0; an upgrade requires an explicit compatibility/migration review. |
| github.com/riverqueue/river/riverdriver/riverpgxv5 | v0.44.0 | Exact pgx v5 driver pin matching River and pgx 5.10.0. |
| github.com/riverqueue/river/rivertype | v0.44.0 | Direct worker/error-handler job-row contract, kept at the same exact version. |

Only runtime modules actually used belong in the compiled dependency graph. The application generates UUIDv4 values with the standard library and uses explicit parameterized pgx SQL. sqlc v1.31.1 was verified but deferred until the query surface is large enough to justify generation; it is not falsely presented as installed.

The implemented Events → Today slice required **no new Go module**. Standard-library time parsing plus the existing chi, pgx, and Goose dependencies cover its API, persistence, and migration work.

Bills → Today likewise required no new Go module; `int64`, pgx, chi, and Goose cover its safe integer-money, API, persistence, and migration behavior.

Documents → Today/management likewise required no new Go module; standard-library calendar handling plus pgx, chi, and Goose cover strict date semantics, ownership, CRUD, and migration behavior.

Agenda & Corrections likewise required no new runtime module or migration. Its final local gates include 52 passing Vitest tests, three passing fresh-stack mobile/desktop Playwright journeys, real-PostgreSQL backend integration, vulnerability scanning, and Linux race detection.

The durable reminder phase adds River/riverpgxv5/rivertype v0.44.0, all exact pins, and migration 6 while retaining PostgreSQL as the only queue dependency. Final local gates include 58 passing Vitest tests, five passing fresh-stack Playwright journeys, actual River failure + worker-restart delivery proof, real-PostgreSQL integration, `govulncheck`, Linux race detection, zero high-severity pnpm advisories, and 502 verified package signatures.

The recurrence phase adds migration 7 and domain/store/HTTP/web code without a new runtime package. The existing standard library, pgx, River, React, and native form controls cover typed recurrence, bounded materialization, durable sweeps, exceptions/exclusions, and series editing.

Smart Capture also adds no dependency: its deterministic Indonesian parser and provider boundary use the Go standard library, while the existing UI/form/API primitives provide editable review and ordinary explicit save. No prerelease AI SDK or remote provider is installed.

## Verification commands

Web:

```bash
pnpm view next version
pnpm view react version
pnpm view react-dom version
pnpm view typescript version
pnpm view tailwindcss version
pnpm view zod version
pnpm view motion version
pnpm view lucide-react version
pnpm view react-hook-form version
pnpm view vitest version
pnpm view @playwright/test version
pnpm view @supabase/supabase-js version
pnpm view @supabase/ssr version
pnpm outdated
```

Go:

```bash
go version
go list -m -u all
go mod tidy
go tool govulncheck ./...
```

## Research references

- https://go.dev/doc/devel/release
- https://nextjs.org/docs/app/getting-started/installation
- https://nextjs.org/blog/next-16-3
- https://react.dev/versions
- https://www.typescriptlang.org/
- https://tailwindcss.com/blog/tailwindcss-v4-3
- https://nodejs.org/en/about/previous-releases
- https://www.postgresql.org/support/versioning/
- https://pnpm.io/blog
- https://ui.shadcn.com/docs/changelog
- https://supabase.com/docs/guides/auth/signing-keys
- https://docs.sqlc.dev/en/stable/reference/changelog.html
- https://pkg.go.dev/

## Final installed and verified state

The results in this section include all implemented MVP slices through recurrence, Smart Capture, and production hardening.

Web direct dependencies are exactly pinned in `apps/web/package.json` at the selected versions above; the single root `pnpm-lock.yaml` owns the workspace. No nested lockfile exists. A frozen install completed with pnpm 11.24.0, and these gates passed after the final dependency update on 30 August 2026:

```text
pnpm install --frozen-lockfile
pnpm typecheck
pnpm lint
pnpm test                 # 8 files, 63 tests
pnpm build                # Next 16.3.3 production build
```

After the 1.37.0 Lucide update, `pnpm audit --audit-level high` reports no known vulnerabilities and `pnpm audit signatures` verifies all 502 packages. `pnpm outdated --recursive` now reports only the three intentional compatibility pins already explained above: TypeScript 6.0.3, ESLint 9.39.5, and Node 24 type definitions 24.13.3. The eight-case Playwright suite passed end-to-end on 30 August 2026 against a fresh migrated PostgreSQL database with the stable 2-worker cap documented in `apps/web/playwright.config.ts`.

The Go module is pinned to Go 1.27.0 with the direct runtime/migration dependencies shown above and `govulncheck` 1.7.0 as a tracked tool. These gates passed after migration 7 against PostgreSQL 18.6 on 30 August 2026:

```text
TEST_DATABASE_URL=... go test -tags=integration -count=1 ./...
go vet ./...
go tool govulncheck ./...               # no vulnerabilities found
```

`go list -mod=mod -m -retracted all` reports no resolved retracted module after the final graph. `go test -tags=integration -race -count=1 ./...` passed in WSL2 Ubuntu with Go 1.27.0, GCC 13.3, and a temporary PostgreSQL 16.15 cluster; CI repeats the same gate in Linux. The Windows host still lacks a race-capable C toolchain, so the race gate intentionally runs in Linux rather than being omitted.

Production images are pinned to stable `golang:1.27.0-alpine3.24`, `node:24.19.0-alpine3.24`, and `alpine:3.24.1` bases. Both images built and smoke-tested locally before the host Docker engine became unavailable; a final rebuild remains part of the release gate.
