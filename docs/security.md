# LifeHub Security & Privacy

LifeHub may store private personal planning data. Security is a product requirement.

## Threats

Protect against:
- cross-user data access;
- leaked tokens;
- client-exposed secrets;
- SQL injection;
- forged JWT;
- insecure CORS;
- replayed/duplicate side effects;
- private data in logs;
- unnecessary sensitive-document collection;
- compromised dependencies;
- stale reminder jobs.

## Authentication

Preferred: Supabase Auth.

Go must:
- cryptographically verify JWT;
- validate issuer/time claims;
- handle key rotation;
- use JWKS;
- fail closed.

Never trust decoded-only JWT or request-body `user_id`.

Implemented production verification accepts only configured RS256/ES256 JWKS keys and validates signature, algorithm, HTTPS issuer/JWKS configuration, audience, expiry, issued-at, and UUID subject. Keys refresh periodically. The local development issuer requires an explicit secret of at least 32 bytes, is visibly labelled in the UI, and is not registered when `APP_ENV=production`.

## Authorization

Every private query must scope ownership.

Bad:
```sql
SELECT * FROM tasks WHERE id = $1;
```

Preferred:
```sql
SELECT * FROM tasks
WHERE id = $1 AND user_id = $2;
```

Same rule for update/delete.

The first-slice profile, Today, create, and completion paths are subject-scoped. Real-PostgreSQL tests prove that another user cannot see or complete an owned task and receives 404 for cross-owner completion.

## Secrets

Never expose/commit:
- Supabase service secret;
- database URL;
- private VAPID key;
- remote AI key;
- deploy credentials.

## CORS

Production:
- known-origin allowlist;
- no wildcard with credentials;
- explicit methods/headers.

## CSRF

Document based on actual auth transport.

The implemented client persists the development token in `localStorage`; Supabase JS manages the configured production browser session with persistence enabled. API calls send the access token only in `Authorization: Bearer ...`, use `credentials: "omit"`, and do not use ambient authentication cookies. Classic CSRF is therefore not the primary risk for these API mutations. XSS/token theft is the more important browser threat: avoid arbitrary HTML, keep dependencies patched, minimize third-party scripts, and add a tested CSP during deployment hardening. If transport later changes to cookies, re-evaluate secure/httpOnly/sameSite settings and CSRF defenses.

## XSS

- no arbitrary HTML rendering;
- avoid dangerous injection;
- use CSP where compatible;
- keep dependencies patched.

## SQL

Parameterized queries only. Prefer sqlc/pgx.

## Logging

Never log:
- tokens;
- task notes;
- sensitive document details;
- private raw AI payloads.

Log IDs/event types instead.

## Documents

MVP tracks metadata/expiry only.

File storage later requires separate design for encryption, access control, malware/type limits, retention, deletion, backups, and recovery.

## Smart capture

If remote AI is used:
- disclose third-party processing;
- minimize payload;
- server-side key;
- no raw logging;
- manual fallback;
- no autonomous writes.

## Database

Production:
- TLS;
- least-privileged app role;
- backups;
- restore procedure;
- avoid public DB exposure.

## Dependency security

Web:
```bash
pnpm audit
pnpm audit signatures
pnpm outdated
```

Go:
```bash
govulncheck ./...
go list -m -u all
```

Verified on 19 August 2026: the frozen pnpm install passed, `pnpm audit --audit-level high` found no known vulnerabilities, all 502 packages passed registry signature verification, and `go tool govulncheck ./...` reported no vulnerabilities.

## Current limits

- Hosted Supabase login has not been manually exercised because no project credentials were provided; asymmetric verifier behavior is covered by automated JWKS tests.
- Development login is for local use only and is not a production identity provider.
- Non-production API startup requires a loopback listener, and Docker publishes the known-password development database on `127.0.0.1` only.
- Account deletion, backup retention, CSP/HSTS at a real ingress, and deployment monitoring remain production-readiness work.

## Security headers

Review:
- CSP;
- Referrer-Policy;
- X-Content-Type-Options;
- Permissions-Policy;
- HSTS in HTTPS production;
- `frame-ancestors`.

Do not cargo-cult a policy that breaks the app.

## Data deletion

Before real production use define:
- account deletion;
- owned-row deletion;
- reminder/job cancellation;
- notification/push-subscription deletion;
- backup retention limitations.

## Acceptance tests

- user A cannot read/update/delete user B data;
- invalid/missing/expired token rejected;
- forged subject rejected;
- client user ID ignored;
- secrets absent from client bundle;
- deleted entity cannot later notify;
- duplicate worker execution cannot duplicate visible notification.
