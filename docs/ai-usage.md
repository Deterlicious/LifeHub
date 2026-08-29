# LifeHub Smart Capture / AI Usage

AI is optional. LifeHub must remain useful without an AI key.

Status: **draft-only Smart Capture is implemented locally with deterministic rule and mock providers. No remote AI provider, paid credential, or autonomous write path is installed.**

## Purpose

Smart Capture shortens entry.

Example:

```text
Bayar internet 350 ribu tanggal 15 tiap bulan
```

Possible draft:

```json
{
  "kind": "bill",
  "title": "Internet",
  "amount": 350000,
  "currency": "IDR",
  "recurrence": {
    "frequency": "monthly",
    "interval": 1,
    "day_of_month": 15
  }
}
```

This is a draft only.

## Hard boundary

AI/provider never:
- inserts;
- updates;
- deletes;
- marks paid/completed;
- creates reminder jobs;
- decides authorization;
- silently resolves important ambiguous dates.

Flow:

```text
User input
  ↓
Provider
  ↓
Untrusted draft
  ↓
Validation
  ↓
Review UI
  ↓
Explicit confirmation
  ↓
Normal Go create/update API
```

## Provider interface

```go
type Provider interface {
    Parse(ctx context.Context, input string, now time.Time, timezone string) (Draft, error)
}
```

Implementations:
1. `RuleProvider` — active deterministic default;
2. `MockProvider` — active only when explicitly configured outside production, for deterministic tests;
3. optional remote provider — not implemented.

## Rule provider

Support common Indonesian patterns where practical:
- besok;
- lusa;
- jam;
- tanggal;
- tiap bulan;
- mingguan;
- prioritas tinggi;
- simple IDR shorthand.

When uncertain, leave fields unresolved and ask for review.

Do not build a huge NLP engine.

The current rule provider covers task/event/bill/document drafts, `besok`, `lusa`, explicit dates and wall-clock times, common IDR shorthand, priority, and simple daily/weekly/monthly/yearly recurrence. It reports unresolved or conflicting details as ambiguities instead of manufacturing certainty.

## Implemented API boundary

`POST /api/v1/smart-capture/parse`:

- requires a cryptographically verified user and completed timezone profile;
- accepts one non-empty `input` string of at most 1,000 characters;
- applies a two-second provider timeout;
- limits each authenticated user to 20 requests per rolling minute and returns standard rate-limit headers;
- validates and normalizes provider output before returning it;
- logs neither raw input nor raw output;
- cannot invoke a create/update/delete action.

The current bounded rate limiter is process-local. The selected initial deployment uses one API instance; horizontal scaling requires a platform/shared quota before claiming one global per-user rate.

The web places the validated values into the ordinary editable Quick Add form. Ambiguities remain visible and missing required fields block the ordinary Save action. The same manual structured form works when parsing is unavailable.

## Remote provider

If implemented:
- research current official API docs;
- server-only credentials;
- structured output when supported;
- timeout;
- rate limit;
- output validation;
- recoverable failure;
- no raw payload logging;
- manual fallback.

## Draft fields

Potential:
- kind;
- title;
- notes;
- start/end;
- due;
- expiry;
- amount/currency;
- priority;
- recurrence;
- reminder offsets;
- confidence;
- ambiguities.

## Ambiguity

Example:
`Meeting Senin jam 2`

Potential ambiguity:
- which Monday?
- 02:00 or 14:00?

Do not silently guess if context cannot safely resolve it.

## Testing

Mock provider must allow deterministic E2E.

Test:
- task;
- event;
- bill;
- expiry;
- Indonesian money;
- ambiguous date;
- malformed provider output;
- timeout;
- provider unavailable;
- manual fallback.

E2E must not require paid AI credentials.

Implemented automated proof covers task, event, bill, document, Indonesian money, recurrence, ambiguous input, invalid provider output, timeout, unavailable provider, authorization/profile readiness, input limits, rate limits, and no-write-before-confirmation. The dedicated mobile Playwright case uses the deterministic rule provider and proves that parsing alone creates no bill; persistence occurs only after the missing time is supplied and the ordinary Save action is pressed.
