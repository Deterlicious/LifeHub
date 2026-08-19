# LifeHub Stack Versions

Verified: **19 August 2026** against official release pages, npm registry metadata, and the Go module proxy.

This file distinguishes the host environment, the compatible versions selected for the first slice, and dependencies intentionally deferred. The root `pnpm-lock.yaml`, `apps/web/package.json`, and `services/api/go.mod` are the installed source of truth.

## Policy

Use latest stable, security-patched, mutually compatible versions.

Never use beta/canary/RC/deprecated/abandoned dependencies unless explicitly approved and documented.

**Compatibility beats the largest version number.**

## Runtime and database

| Technology | Current stable target | Host at inspection | Decision |
|---|---:|---:|---|
| Go | 1.26.6 | 1.26.1 system; 1.26.6 module toolchain | `go 1.26.6` makes the Go tool download/use the current patch through `GOTOOLCHAIN=auto`. |
| Node.js | 24.19.0 LTS | 22.21.0 | Target Node 24 LTS in CI/production. Host Node 22 still satisfies selected package engines and may bootstrap locally. |
| pnpm | 11.22.0 | 11.20.0 initially; 11.22.0 activated | Pinned by `packageManager`; the frozen workspace install uses 11.22.0. |
| PostgreSQL | 18.4 | 18.4-alpine in Docker | Selected current supported 18.x minor; PostgreSQL 19 beta is excluded. Local host port is `55432` because the usual range was reserved on this Windows host. |
| Docker Engine | 29.7.2 | 29.7.2 | Local infrastructure only; not an application dependency. |
| Docker Compose | 5.4.0 | 5.4.0 | Available locally. |

## Frontend selection

| Package | Registry latest | Selected for first slice | Reason |
|---|---:|---:|---|
| next | 16.3.1 | 16.3.1 | Stable App Router release. |
| react / react-dom | 19.2.8 | 19.2.8 | Exact matching patches; satisfies Next peers. |
| typescript | 7.0.2 | **6.0.3** | Intentional compatibility pin: the current `typescript-eslint` peer range is `<6.1.0`; TypeScript 7 tooling APIs are not yet supported by this lint stack. |
| tailwindcss | 4.3.3 | 4.3.3 | Stable. |
| @tailwindcss/postcss | 4.3.3 | 4.3.3 | Exact Tailwind match. |
| postcss | 8.5.26 | 8.5.26 | Satisfies Tailwind's peer/dependency range. |
| eslint | 10.8.1 | **9.39.5** | Intentional compatibility pin: plugins resolved by the current Next config support ESLint 9. |
| eslint-config-next | 16.3.1 | 16.3.1 | Exact Next match. |
| @types/node | 26.2.0 | **24.13.3** | Match the Node 24 production target. |
| @types/react | 19.2.18 | 19.2.18 | Match React 19. |
| @types/react-dom | 19.2.4 | 19.2.4 | Match React DOM 19. |
| vitest | 4.1.11 | 4.1.11 | Stable unit-test runner; compatible with Node 22/24 and Vite 8. |
| @playwright/test | 1.62.1 | 1.62.1 | Stable browser test runner. |
| lucide-react | 1.32.0 | 1.32.0 | Stable accessible icon primitives. |
| @supabase/supabase-js | 2.112.3 | 2.112.3 | Stable browser auth/session boundary. |

### Verified but deferred

| Package | Current version | Decision |
|---|---:|---|
| zod | 4.4.3 | Defer until shared client validation is complex enough to justify it; Go remains authoritative. |
| motion | 13.1.0 | Defer; the first slice does not need animation infrastructure. |
| react-hook-form | 7.85.0 | Defer; the first task form is simpler with native semantics. |
| @supabase/ssr | 0.12.4 | Defer. The npm tag is non-prerelease, but official Supabase documentation still calls the API beta/unstable, which conflicts with this project's strict no-beta policy. |

Shadcn/ui is code-distribution tooling rather than a runtime package. Defer it until repeated component patterns justify adding generated component code.

## Go selection

| Module/tool | Verified version | First-slice decision |
|---|---:|---|
| github.com/go-chi/chi/v5 | v5.3.1 | Runtime HTTP router. |
| github.com/jackc/pgx/v5 | v5.10.0 | Runtime PostgreSQL driver/pool. |
| github.com/golang-jwt/jwt/v5 | v5.3.1 | JWT claims/signature validation. |
| github.com/MicahParks/keyfunc/v3 | v3.8.1 | Stable JWKS retrieval, caching, and key-rotation support instead of custom key parsing. |
| github.com/pressly/goose/v3 | v3.27.3 | Embedded migration runner and explicit migration command. |
| golang.org/x/vuln/cmd/govulncheck | v1.7.0 | Tracked Go tool invoked with `go tool govulncheck`; no global install required. |
| modernc.org/libc | v1.75.3 | Indirect compatibility override: Goose 3.27.3 declares retracted v1.74.3, whose retraction reports a DNS deadlock. The current stable patch removes that retracted version from the resolved graph. |
| github.com/riverqueue/river | v0.44.0 | Defer until the reminder slice. This is the latest non-prerelease tag but remains a pre-1.0 API. |

Only runtime modules actually used in the slice belong in the compiled dependency graph. The slice generates UUIDv4 values with the standard library and uses explicit parameterized pgx SQL. sqlc v1.31.1 was verified but deferred until the query surface is large enough to justify generation; it is not falsely presented as installed. River is likewise absent until reminder work begins.

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

Web direct dependencies are exactly pinned in `apps/web/package.json` at the selected versions above; the single root `pnpm-lock.yaml` owns the workspace. No nested lockfile exists. A frozen install completed with pnpm 11.22.0, and the following gates passed on 19 August 2026:

```text
pnpm install --frozen-lockfile
pnpm typecheck
pnpm lint
pnpm test                 # 3 files, 11 tests
pnpm build                # Next 16.3.1 production build
pnpm audit --audit-level high   # no known vulnerabilities
pnpm audit signatures           # 502/502 packages verified
pnpm test:e2e             # fresh migrated API/web + 1 mobile Chromium journey at 390x844
```

`pnpm outdated --recursive` reports only the three intentional compatibility pins already explained above: TypeScript 6.0.3, ESLint 9.39.5, and Node 24 type definitions 24.13.3.

The Go module is pinned to Go 1.26.6 with the five direct runtime/migration dependencies shown above and `govulncheck` 1.7.0 as a tracked tool. These gates passed against PostgreSQL 18.4:

```text
TEST_DATABASE_URL=... go test -tags=integration -count=1 ./...
go vet ./...
go tool govulncheck ./...               # no vulnerabilities found
go test -tags=integration -race -count=1 ./...  # Linux Go 1.26.6 container
go list -m -retracted all               # no resolved retracted modules
```

The Windows host has `CGO_ENABLED=0` and no local C compiler, so the race gate intentionally runs in Linux Docker rather than being omitted.
