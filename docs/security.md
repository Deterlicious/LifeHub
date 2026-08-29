# LifeHub Security & Privacy

LifeHub may store private personal planning data. Security is a product requirement.

Status note: task/Today, event, bill, document expiry/management, Agenda & Corrections, durable reminders/notifications, recurrence, and draft-only Smart Capture are implemented and locally verified as recorded. Hosted auth, backup/restore operations, and production deployment remain outside the verified scope.

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

The implemented profile, Today, create, and completion paths are subject-scoped. Real-PostgreSQL tests prove that another user cannot see or complete an owned task and receives 404 for cross-owner completion.

For the implemented event create-to-Today slice:

- `user_id` must never appear in the accepted create body;
- event inserts derive ownership from the verified token subject;
- Today event queries bind that same subject;
- real-PostgreSQL tests prove event persistence and Today isolation between owners;
- cross-owner event reads or mutations added in later slices must be indistinguishable from missing data.

For the implemented bill create/payment-to-Today slice:

- bill inserts and Today reads derive ownership only from the verified token subject;
- `mark-paid` binds both bill UUID and subject, returns 404 for cross-owner data, and preserves the first payment timestamp on retries;
- real-PostgreSQL tests prove bill persistence, ownership isolation, local-day payment boundaries, database constraints, and no truncation.

For the implemented document expiry/management slice:

- create/list/get/update/delete derive ownership only from the verified token subject;
- every item predicate binds both document UUID and subject; invalid, missing, and cross-owner items are indistinguishable 404 responses;
- the API accepts metadata only and never accepts a scan, uploaded file, or browser-provided `user_id`;
- Today/Upcoming and the full manager remain ownership-scoped and uncapped;
- real-PostgreSQL tests prove CRUD persistence, update/delete isolation, date boundaries, and database constraints.

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

The implemented client persists the development token in `localStorage`; Supabase JS manages the configured production browser session with persistence enabled. API calls send the access token only in `Authorization: Bearer ...`, use `credentials: "omit"`, and do not use ambient authentication cookies. Classic CSRF is therefore not the primary risk for these API mutations. XSS/token theft is the more important browser threat: avoid arbitrary HTML, keep dependencies patched, and minimize third-party scripts. The web now emits a restrictive CSP plus opener/resource, permissions, referrer, MIME, frame, and production HSTS headers; the API emits an API-appropriate deny-all CSP and equivalent baseline headers. If transport later changes to cookies, re-evaluate secure/httpOnly/sameSite settings and CSRF defenses.

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
- event notes or locations;
- sensitive document details;
- private raw AI payloads.

Log IDs/event types instead.

## Documents

MVP tracks metadata/expiry only. The UI explicitly warns against entering document numbers or uploading scans. The API accepts only name, a bounded category, optional notes, and a date-only expiry. Private document responses use `Cache-Control: no-store`, and the CORS allowlist explicitly handles authenticated `DELETE` preflight without widening allowed origins.

File storage later requires separate design for encryption, access control, malware/type limits, retention, deletion, backups, and recovery.

## Smart capture

The implemented deterministic and mock providers receive bounded input plus the authenticated user's timezone and return an untrusted draft. Go validates the output, applies a two-second timeout and a 20-request/minute per-user rate limit, and never grants the provider repository/database access. Parsing has no domain write path; the user must confirm through the ordinary ownership-scoped API. Raw input/provider output is not logged.

The current rate limiter is process-local and memory-bounded, which is sufficient for the single API instance in the selected initial topology. A horizontally scaled API would need an ingress- or shared-store-enforced limit before claiming one global quota.

If a remote AI provider is added:
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

The latest scans on 29 August 2026 reported no Go vulnerabilities and no known high-severity pnpm vulnerabilities after recurrence, Smart Capture, production hardening, Go 1.27.0, and the final stable web dependency update. All 502 resolved npm packages have verified registry signatures; only the documented TypeScript/ESLint/Node-type compatibility pins remain behind their registry latest versions.

## Current limits

- Hosted Supabase login has not been manually exercised because no project credentials were provided; asymmetric verifier behavior is covered by automated JWKS tests.
- Event list/get/edit/delete, Agenda, bill list/get/edit/delete/mark-unpaid, document metadata CRUD, reminder CRUD, notification read actions, recurrence series/occurrences, and ordinary writes created from reviewed Smart Capture drafts are implemented with server-side ownership enforcement. Document file storage remains explicitly excluded.
- Authenticated users can permanently delete all of their LifeHub application data through an exact typed confirmation. The transaction cancels their scheduled River reminder jobs before the profile cascade and cannot accept a browser-supplied owner ID. The Supabase authentication identity remains separate and is clearly disclosed in the UI.
- Development login is for local use only and is not a production identity provider.
- Non-production API startup requires a loopback listener, and Docker publishes the known-password development database on `127.0.0.1` only.
- Auth-provider identity deletion, backup retention/restore proof, hosted CSP/HSTS confirmation at the real ingress, and deployment monitoring remain production-readiness work.

## Security headers

Implemented and covered by API tests or production-container smoke checks:
- CSP with `frame-ancestors 'none'`;
- Referrer-Policy;
- X-Content-Type-Options;
- X-Frame-Options;
- Permissions-Policy;
- Cross-Origin-Opener-Policy and Cross-Origin-Resource-Policy on the web;
- HSTS in production mode.

The final hosted smoke test must confirm the platform does not remove or weaken these headers and that HTTPS is active before release.

Do not cargo-cult a policy that breaks the app.

## Data deletion

Implemented for LifeHub application data:
- exact typed confirmation in the settings dialog;
- verified-token ownership only;
- transactional scheduled-reminder cancellation;
- profile deletion with database cascades across owned rows and notifications;
- sign-out plus clear disclosure that the Supabase login identity is separate.

Before real production use, define the Supabase identity-deletion policy and disclose/test managed-backup retention limitations. Push-subscription deletion remains irrelevant until Web Push exists.

## Acceptance tests

- user A cannot read/update/delete user B data;
- user A's event cannot appear in user B's Today feed;
- user A cannot pay or see user B's bill in Today;
- invalid/missing/expired token rejected;
- forged subject rejected;
- client user ID ignored;
- secrets absent from client bundle;
- deleted entity cannot later notify;
- duplicate worker execution cannot duplicate visible notification.
