# ADR-002 — Reports as a separate service (conceptual design)

> **Status:** Proposed — **open to change** · **Date:** 2026-09 · **Scope:** reports, scheduling, service-to-service auth
>
> This records a design agreed in discussion, **before any code exists**. It is
> deliberately conceptual: the sections marked *Open* are expected to change, and
> the "Extension points" section names where new layers can be added without
> reshaping the rest. Revisit it at the start of the implementation session
> rather than treating it as settled.

## Context

Reports today live inside the monolith as `GET /reports/export`: synchronous,
request-response, returning JSON/CSV/PDF to the caller. The goal is to move
report generation into **its own service and repository**, generating on a
schedule and delivering **by email** instead of over HTTP.

The driver is not size but **runtime shape**. An emailed periodic report is
scheduled, asynchronous and long-running; the rest of the API is
request-response. Those two want different failure handling, different retry
semantics and different scaling.

### Relationship to ADR-001

ADR-001 says extract **users/auth first** and keep movement/balance/reports/
analytics together as one cohesive `core`, because premature splitting is where
boundaries get set wrong. Taking reports out first is a **deviation from that
ordering**, and it is a conscious one.

It is not a deviation from the principle: ADR-001's own trigger list names
*"independent scaling need (e.g. analytics/reports heavy vs a light API)"* as a
legitimate reason to extract. The ordering is what changes, not the reasoning.

**Open:** whether JWT (ADR-001's "#1 enabler") lands before, alongside, or after
this extraction. See *Security* below — it is the single decision with the most
downstream consequences.

## Decision

### 1. Ownership and data flow

- **Service B owns the report configuration** — per user: frequency, format,
  recipient — in **its own database**. It is B's domain data.
- **Service A (the monolith) remains the source of truth for movements.** B
  pulls them when it needs them and never keeps a copy.
- **Config propagation A → B goes over REST, not a queue.** It is low volume,
  user-triggered, and the user should learn in the same request whether it was
  saved. A broker buys eventual consistency that is not wanted here.

A queue does have a place later, for a different thing: **lifecycle events**. If
a user is deleted or deactivated in A, B otherwise keeps generating reports for
someone who no longer exists. That is worth an event; the initial config write is
not.

### 2. Scheduling: a job table, not a broker

At current scale (tens of users, one report per period) the run table *is* the
queue:

```sql
report_runs(
  user_id, period,            -- UNIQUE (user_id, period)
  status,                     -- see state machine below
  attempts, next_attempt_at,
  last_error, sent_at
)
```

| Need | How the table serves it |
|---|---|
| Retries | `status IN ('pending','failed') AND attempts < N` |
| Backoff | `next_attempt_at <= NOW()` |
| Dead letter | `attempts >= N` → `abandoned`, inspected with a `SELECT` |
| No duplicates | `UNIQUE (user_id, period)` |
| Visibility | plain SQL |

The scheduling pass is `INSERT ... ON DUPLICATE KEY IGNORE`, so it is **idempotent
by construction**: it can run hourly, or twice by accident, without consequence.

One loop is enough — decide, claim, execute. What matters is not that these are
separate processes but that they happen **in that order**: the row is created
*before* the work starts, so a retry re-executes the work and never re-decides
the schedule.

### 3. The run is a state machine

```
pending ──► building ──► sending ──► sent
              │             │
              └──► failed ◄─┘
                     │  (attempts >= N)
                     ▼
                 abandoned
```

Writing the states down buys three things:

1. **A crash becomes readable.** The state the row is in says how far the run
   got, instead of leaving it to guesswork.
2. **It makes the irreversible boundary visible.** Everything before `sending` is
   free to retry — fetching movements and rendering a PDF are not observable
   outside the service. `sending` is the moment the outside world saw something,
   which is exactly why it deserves its own state.
3. **Transitions are enforced in SQL, not in code:**

```sql
UPDATE report_runs
SET    status = 'building', attempts = attempts + 1
WHERE  user_id = ? AND period = ? AND status IN ('pending','failed');
```

One row affected means the run is claimed; zero means someone else took it or it
had already moved on. That is an atomic claim with no locks, and it makes
multiple concurrent workers safe later at no extra cost.

### 4. Idempotency and the chosen failure mode

Retries are *at-least-once* by nature — a queue, or a crashed loop, will re-run
work. The unique `(user_id, period)` key is what makes that safe.

Sending the email and writing to the database cannot be made atomic, so a window
remains. It is narrowed by marking `sending` **before** the SMTP call and `sent`
after: a row left in `sending` says "this may have gone out", rather than looking
like it never happened.

**Decision:** prefer at-least-once. For a periodic financial report a duplicate
email is more acceptable than a missing one. This is a choice, not an accident —
revisit it if the report ever carries something that must not be duplicated.

### 5. Security: service-to-service

**Not a root or superuser account.** A credential that can read every user's
movements makes per-user isolation unenforceable at the data layer, gives an
unbounded blast radius to a long-lived secret, and destroys auditability. The
schema already resists it: `tokens.user_id` is `NOT NULL` with an FK to `users`,
so a "root token" would require inventing a fake user row.

Instead:

- **A service principal with explicit capabilities.** `movements:read`, and the
  call **must name the user** it is acting for, so row-level isolation stays
  enforced by the database rather than by B's own filtering.
- **OAuth2 `client_credentials` for machines, coexisting with the current human
  login.** Adopt the *flow*, not the whole protocol. Do **not** migrate the human
  login: OAuth2 is a delegation protocol for third parties, and with a
  first-party frontend there is no third party — authorization-code + PKCE would
  buy redirects, an authorization server and consent screens without solving a
  problem we have. Revisit only if third-party clients, "log in with Google", or
  SSO across several frontends appear.
- **One scope model, shared.** The resource permissions we want and OAuth2 scopes
  are the same concept; building both would be a mistake.
- **The `aud` (audience) claim is mandatory.** Without it a bearer token minted
  for one service can be replayed against another that trusts the same issuer —
  a skeleton key. Each service validates signature (`iss`), audience (`aud`),
  permission (`scope`) and expiry (`exp`).
- **Never forward a received token onward.** If B needs to call C it obtains its
  own token for C. Forwarding is how one token ends up opening three services.

The existing `tokens.scope` column is already a primitive form of this: it stops
an activation token being used as an authentication token. Audience extends the
same reasoning from *what a token is for* to *who it is for*. With opaque
database tokens that is another column to look up per request; with JWT it is a
standard claim validated locally — which is the concrete reason multi-service and
JWT belong together.

**Token lifetime is not an obstacle.** Tokens are minted **at run time** and live
for the duration of the run; nothing is stored between runs, so a daily or
monthly cadence is irrelevant. The real question is who may mint a token for a
user without their password, and there are two shapes:

- **A — the scheduler lives on the trusted side (A).** A mints a short-lived
  scoped token per user and hands the work to B. B holds no long-lived secret.
  Simplest; works with today's `TokenService`.
- **B — B holds a machine credential and exchanges it** for a short-lived,
  per-user token at run time. Standard client-credentials + token exchange.

**Recommended: start with A, move to B when B needs real autonomy** — likely the
same moment JWT arrives. Something long-lived has to exist somewhere; the design
goal is not "no durable secret" but that **the durable secret is not the one that
reads the data**.

**Open / deferred:** the A↔B link may start unauthenticated for local
development. Hard constraint: **it must never run unauthenticated outside
localhost.** An endpoint returning any user's movements without a credential is a
real problem the moment B lives on another host, staging included.

### 6. Encryption at rest — *Open*

Encryption at rest protects a **different threat** than credential compromise: a
stolen disk, volume or backup dump. (Our own `mysqldump` backups are plaintext
today.) It is worth doing on its own merits, but it is not a mitigation for a
leaked service credential — if the service can decrypt in order to build the
report, so can whoever holds its credential.

It also has a concrete cost. The app aggregates **in SQL**, not in Go:

```sql
-- internal/analytics/repository.go
SELECT m.category_id, c.name, SUM(m.amount), COUNT(*)
WHERE  m.user_id = ? AND m.type = 'E' AND m.date BETWEEN ? AND ?
```

`SUM(amount)`, `GROUP BY YEAR(date)` and date ranges appear across analytics,
balance and movements. Encrypting `amount` or `date` at column level means MySQL
can no longer sum or filter them, forcing every row into memory and rewriting
analytics and balance entirely.

Cheaper options that keep the aggregations working:

- **Encrypt `description` only** — the field with genuinely sensitive content;
  `amount` and `date` stay aggregable.
- **Disk/volume-level encryption** — covers the same stolen-media threat without
  touching a single query.

## Extension points — where a new layer can be added later

The design is meant to absorb changes at these seams without reshaping the rest:

- **Storage → broker.** If a real queue is wanted, the scheduling pass changes
  from `INSERT` to publish and the worker from `SELECT` to consume. The work
  itself does not change. Move when several workers must compete, when polling
  gets expensive, or when other services need the same events.
- **New pipeline stages.** Extra steps (rendering variants, an approval step, an
  archive-to-storage step) are added as **new states** in the machine, between
  the existing ones. The claim/retry mechanics are unaffected.
- **New delivery channels.** Email is one terminal transition. A second channel
  is another state plus another `*_at` column, not a new pipeline.
- **New report types.** The run row identifies `(user_id, period)`; a
  `report_type` column extends the unique key without touching the state machine.
- **Auth.** Shape A → shape B is a change of who mints the token, not of what the
  worker does with it. The `aud`/`scope` checks stay in the same place.

## Consequences

- B is deployable and restartable at any time: state lives in its table, never in
  memory.
- A gains a machine-facing read path for movements that must be scoped per user
  from day one — this is the part that touches existing code.
- The principal on the request context stops being "a user". Today
  `cmd/api/middleware.go` hardcodes `ScopeAuthentication` and always resolves to
  a `*data.User`; `UserFromContext`, `userIDKey`, `RequireUserID` and both guards
  assume a human. Generalising this to a **principal** (user *or* service client,
  each with scopes) is the real prerequisite work, and it is cheaper to do before
  two services exist than after.
- Reports leaving the monolith means `/reports/export` either proxies to B or is
  retired. **Open:** which.

## Open questions

1. **Period definition.** The app already has billing periods driven by
   `cut_day` (`/balance/periods`). A calendar-month report would contradict the
   balance the user sees in the app. Reports should probably align with the
   existing cut periods — decide before implementing.
2. **Email from B.** A already has `internal/mailer` and SMTP config. Duplicate
   it in B (fine initially) or introduce a notification service — decide it
   deliberately rather than by inertia.
3. **JWT ordering** relative to this extraction (see Context).
4. **Fate of `GET /reports/export`** in the monolith.
5. **Whether B gets its own database or a schema in the same instance** — ADR-001
   suggests logically separate schemas make database-per-service later "nearly a
   `mv`".

## Related

- `docs/ADR-001-microservices-strategy.md` — extraction strategy, JWT as enabler,
  strangler-fig, trigger list.
- `docs/FEATURES.md` — current reports and analytics surface.
- `internal/security/handler.go` — the guard family this work extends
  (`RequireAuthentication`, `RequireActivatedUserForThisEndpoint`).
- `cmd/api/middleware.go` — `authenticate`, where the principal is resolved.
