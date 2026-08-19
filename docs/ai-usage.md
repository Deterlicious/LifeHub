# LifeHub Smart Capture / AI Usage

AI is optional. LifeHub must remain useful without an AI key.

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
1. RuleProvider
2. MockProvider
3. optional remote provider

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
