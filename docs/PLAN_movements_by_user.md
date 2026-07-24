# Plan — Scoping financial data per user

> Status: **Draft for analysis. No code yet.**
> Goal: be able to fetch the movements (and the cut / monthly-income config)
> that belong to a particular user — eventually the user logged into the app.
> **Login/auth is explicitly out of scope** for now; this document only covers
> the data model and how data gets filtered by owner.

---

## 1. Where we are today

Current tables (from `migrations/001_init.sql`):

| Table | Owner column today? | Seed rows |
|---|---|---|
| `users` | — (id, username *UNIQUE*, name, email *UNIQUE*, hashed_password) | none |
| `movements` | **none** | **none** (empty) |
| `income_config_history` | **none** | **1 baseline row** |
| `categories` | none (shared) | 11 rows |

Nothing links a movement or an income/cut config to a user. Per
`CLAUDE.md`, `movements` is the single source of truth and **`balance`,
`reports` and `analytics` all read from it**.

### Consequence that must not be missed
Adding ownership is **not a `movements`-only change**. Every module that reads
movements or the income config has to become user-scoped:

- `movement` — list / summary / expenses / CRUD
- `balance` — available balance + billing periods (reads movements **and** income config)
- `reports` — export
- `analytics` — all aggregations
- `incomes` — config GET/PUT

If any one of them forgets the filter, it leaks another user's data. This
cross-cutting impact is the main reason to pick the model carefully.

### Migration reality
`001_init.sql` is the single source of truth and only runs on a **fresh**
volume (`docker-entrypoint-initdb.d`). So:

- `movements` is empty → adding a `NOT NULL` owner column is painless.
- `income_config_history` has 1 baseline row → it needs an owner, so we must
  **seed a default user** and point the baseline row at it (all three
  proposals share this).
- Existing/long-lived databases would need an `ALTER TABLE` migration; in dev
  we just reset the volume.

---

## 2. Cross-cutting concern (applies to all proposals): *where does the current user come from?*

Until login exists, the API still needs to know **who** the request is for.
Recommended approach, independent of the ownership model chosen:

- A small **HTTP middleware/interceptor** reads the caller's identity from a
  temporary source — e.g. a header `X-User` (username) or `X-User-Id`, or a
  `?username=` query param — and injects it into the request `context`.
- Every handler reads the current user from the context, never from raw query
  params scattered around.
- When real auth arrives, we swap the middleware's source (header → JWT claim)
  and **nothing else changes**.

This reuses the interceptor idea we discussed earlier and keeps the migration
to real login a one-file change.

---

## 3. The three proposals

The proposals differ on **one axis: how a row is linked to its owner.**

### Proposal A — Surrogate FK `user_id → users(id)`  *(recommended)*

Link every financial row to the user's **numeric id**.

**Schema changes**
- `movements`: add `user_id BIGINT UNSIGNED NOT NULL`,
  `FK → users(id)` `ON UPDATE CASCADE ON DELETE RESTRICT`.
  Replace the single-column indexes with composite ones that lead with the
  owner: `(user_id, date)`, `(user_id, category_id)`, `(user_id, type)`.
- `income_config_history`: add `user_id BIGINT UNSIGNED NOT NULL`, FK, and
  change `UNIQUE(year_month)` → `UNIQUE(user_id, year_month)` so each user has
  their own config timeline.
- Seed a **default user**; the baseline income row references its id.

**App/layer impact**
- Repositories: every `SELECT`/`INSERT`/`UPDATE`/`DELETE` filtered by
  `user_id`; the value comes from context (see §2).
- Handlers only know a username → resolve it to an id once (lookup or a join).

**Pros**
- Fully normalized; **rename-safe** — a user can change their username freely
  and no financial row is touched.
- Compact, fast indexes (8-byte bigint keys).
- Cleanest path to auth: a JWT naturally carries the user id.

**Cons**
- Needs a `username → id` resolution step when the client only has a username.
- Requires the seeded default user for the baseline config row.

---

### Proposal B — Natural key `username → users(username)`  *(the originally proposed idea)*

Link every financial row directly to the **username** string.

**Schema changes**
- `users.username` is already `UNIQUE` → it can be a FK target.
- `movements` and `income_config_history`: add `username VARCHAR(255) NOT NULL`,
  `FK → users(username)` `ON UPDATE CASCADE ON DELETE RESTRICT`.
  Composite indexes `(username, date)`, etc. `UNIQUE(username, year_month)`.
- Baseline income row references the seeded default username.

**App/layer impact**
- Filter directly with `WHERE username = ?`. The logged-in username (from
  context) is used as-is — **no id lookup, no join.**

**Pros**
- Simplest end-to-end: the value you filter by is exactly what the session
  gives you.
- Matches the mental model ("bring me *this user's* movements").

**Cons**
- `username` is a **mutable natural key**. A rename must cascade across every
  financial table (`ON UPDATE CASCADE` handles it, but it can rewrite many
  rows and their index entries).
- Wider keys → larger indexes, more storage, slightly slower joins/scans than
  an 8-byte id, repeated across potentially thousands of movements.
- Couples core financial data to a value users may consider cosmetic.

---

### Proposal C — `account` / owner abstraction  *(household / multi-tenant ready)*

Don't tie financial data to an individual **user**; tie it to an **account**
(a budget / household). Users belong to an account.

**Schema changes**
- New table `accounts(id, name, created_at, …)`.
- `users`: add `account_id BIGINT UNSIGNED` FK → `accounts(id)`.
- `movements` and `income_config_history`: add `account_id` FK.
- Filtering is by the logged-in user's `account_id`.

**Pros**
- Future-proof for **shared finances** — a couple/family sharing one budget,
  or one person with multiple logins/devices.
- Clean separation of **identity** (user) vs **ownership** (account); auth
  later just maps user → account.

**Cons**
- Heaviest option now: extra table, extra join, more moving parts.
- Over-engineered (YAGNI) if the app stays strictly one-user-per-dataset.
- Only worth it if shared budgets are actually on the roadmap.

---

## 4. Side-by-side

| Criterion | A · `user_id` FK | B · `username` FK | C · `account_id` |
|---|---|---|---|
| Matches "filter by logged-in user" | ✅ (after id lookup) | ✅✅ (direct) | ✅ (via account) |
| Rename-safe | ✅ | ⚠️ cascades | ✅ |
| Index size / speed | ✅ best | ⚠️ wider | ✅ good |
| Query simplicity (no login) | ⚠️ resolve id | ✅ simplest | ⚠️ resolve account |
| Referential integrity | ✅ | ✅ | ✅ |
| Ready for shared budgets | ❌ | ❌ | ✅ |
| Implementation effort | Medium | Low | High |
| Path to real auth (JWT) | ✅ cleanest | ✅ | ✅ |

---

## 5. Recommendation

- **Default choice: Proposal A (`user_id` FK).** Best long-term data hygiene,
  rename-safe, cheapest indexes, and the most natural fit for JWT-based auth
  later. The only real cost is a one-time `username → id` resolution, which the
  §2 middleware can do once per request.
- **Proposal B** is a legitimate, lower-effort option and matches the original
  instinct. Given the app is currently single-user-simple, it is defensible —
  the main thing to accept is the mutable-key cascade on username changes.
- **Proposal C** only if **shared/household budgets** are a real goal (worth a
  look at `ROADMAP.md`). Otherwise it is premature complexity.

A pragmatic middle path: **adopt A now**, and keep the door open to C by never
letting handlers assume "current user == owner" directly — always go through
the §2 context accessor, so swapping "user" for "account" later is contained.

---

## 6. Authentication & session strategy (login)

This section refines §2 ("where does the current user come from?") into a
concrete auth design, and lays out how to grow into a JWT / AWS Cognito setup
**without rewriting the app**.

### 6.1 The one principle: authenticate at the edge, carry identity in `context`

```
request → [auth middleware] → ctx.Identity{UserID} → handler → service → repo
                │                    ▲
         verifies session/token      └── immutable, server-set, never from the client
```

1. A single **auth middleware** runs before every protected route. It verifies
   the caller (session cookie or bearer token). No valid credential → **`401`**,
   request stops.
2. On success it resolves the caller to an internal `users.id` and stores an
   `Identity{ UserID }` in the request **`context`**.
3. Handlers/services read the current user **only** from that context accessor
   (e.g. `auth.UserID(ctx)`), never from query params or request headers.

> ⚠️ **Do not re-inject the authenticated id as a request header** (e.g.
> `X-User-Id`) that downstream code then trusts. Any header can be spoofed by
> the client; if authorization reads an inbound header, a caller can set it to
> someone else's id. The Go `context` is the correct **immutable, in-process**
> carrier — it is set by the server and never travels back to the client. (Only
> if identity must cross a *process/service* boundary should it be carried as a
> **signed** token, never a plaintext header.)

This is exactly the "value that cannot be altered once obtained" the design
asks for: context-scoped, request-lifetime, server-owned.

### 6.2 Two enforcement layers (defense in depth)

The extra validation layer the design mentions is worth having — as **two**
distinct checks:

1. **Query scoping (primary):** every repository query is filtered by
   `UserID` from context. A user simply cannot *see* rows that aren't theirs.
2. **Per-resource ownership assertion (secondary):** on routes that take an id
   (`GET/PUT/DELETE /movements/{id}`), confirm the row's owner equals the
   context user. On mismatch return **`404`, not `403`** — a `403` leaks the
   fact that the id exists and enables enumeration.

Layer 1 is the real guard; layer 2 catches mistakes (a query that forgot the
filter) and is cheap insurance.

### 6.3 Session vs. token — the two schemes

| | Server-side session (cookie) | Stateless JWT (bearer) |
|---|---|---|
| What the client holds | opaque session id in an `HttpOnly` cookie | signed token with claims (`sub`, `exp`, …) |
| Server state | a session store (DB/Redis/in-mem) | none |
| Revocation | trivial (delete the session) | hard (needs short TTL + refresh, or a denylist) |
| Horizontal scaling | needs shared/sticky store | trivial (nothing to share) |
| XSS exposure | low (`HttpOnly` cookie unreadable by JS) | higher if the token is stored in JS-readable storage |
| CSRF | needs protection (`SameSite`, tokens) | not applicable for `Authorization: Bearer` |
| CORS today (`Allow-Origin: *`) | ❌ `*` + credentials is invalid — must list exact origins | ✅ works with `*` (token in header, not cookie) |
| Fit for AWS Cognito later | swap needed (Cognito issues JWTs) | ✅ native — Cognito *is* JWTs |

### 6.4 Recommendation — design once, swap the verifier

Put the credential check behind an **`Authenticator` boundary** (the middleware
in §6.1). Everything downstream depends on `Identity` in context, never on *how*
it was proven. Then the only thing that changes between phases is the verifier:

- **Phase 1 — our own username/password (now).**
  Keep the bcrypt scheme already built. Because Cognito is the stated target,
  **prefer a self-issued JWT (`Authorization: Bearer`) over cookie sessions**:
  it matches Cognito's model, keeps CORS `*` working, and means the client
  integration barely changes at migration time. Issue a short-lived access
  token from a `POST /login`, signed with HS256 using a server secret;
  middleware validates the signature and loads `sub → users.id`.
  *(If near-term revocation/logout matters more than Cognito-readiness, a
  server-side session is the simpler, safer default — accept the later swap.)*

- **Phase 2 — AWS Cognito (later).**
  Cognito User Pool issues RS256-signed JWTs. The middleware changes from
  "verify our HS256 secret" to "verify the token against Cognito's **JWKS**
  endpoint and check `iss`/`aud`/`exp`". **Nothing else in the app changes** —
  handlers/services still read `Identity` from context.

### 6.5 Bridging the two worlds — local `users.id` ↔ external `sub`

The hardest part to picture: Cognito's token identifies the caller by **`sub`**
(a Cognito-generated UUID), while your financial rows reference **`users.id`**
(your bigint PK). **These do not have to be the same value — they are linked,
not merged.**

Split of responsibility:

| Concern | Owner | Key |
|---|---|---|
| Authentication (prove who you are) | Cognito | `sub` |
| Application data (movements, config, authorization) | your DB | `users.id` |

Cognito does **not** replace the `users` table; it only proves identity. Your
`users.id` stays the anchor that `movements.user_id` (etc.) reference. The
bridge is **one stored correspondence**: `users.cognito_sub = <token sub>`. So
`sub` becomes just another lookup key on the user row — exactly like `email` or
`username` are today.

**The seam is a single function** that translates a validated token to your
local id:

```
resolveIdentity(claims) → users.id
  Phase 1 (our JWT):   sub == users.id                              → return directly
  Phase 2 (Cognito):   SELECT id FROM users WHERE cognito_sub = claims.sub
```

Everything downstream (handlers, services, `movements.user_id`) depends on
`resolveIdentity`, **not** on the IdP. Changing IdP changes only this function's
body — this is the one place the two worlds meet.

**First contact (provisioning):** the first time a valid `sub` has no matching
row, **just-in-time create** a `users` row from the token claims (email /
username) and set `cognito_sub`. After that the lookup always hits. (Or
provision at signup, or inside the migrate-on-login Lambda.)

**Why not just use `sub` as the PK everywhere?**
- **Vendor lock-in:** the schema would hard-depend on Cognito's id; adding
  Google/Apple or swapping IdP becomes a schema migration.
- **Wider keys:** `sub` is a 36-char UUID vs an 8-byte bigint, repeated on every
  movement and index (same argument as Proposal A vs B).
- **Separation:** the domain should not care *how* someone authenticated.

Rule of thumb: **the IdP authenticates; your DB identifies.** Keep `users.id` as
the anchor and treat any external `sub` as a foreign login key mapped to it.

### 6.6 Cognito migration specifics

- **Identity mapping:** add a nullable `cognito_sub VARCHAR(255)` (UNIQUE) to
  `users`. The middleware maps the token's `sub` claim → local `users.id`, so
  all the per-user financial data (Proposal A/B/C) keeps working untouched.
- **Moving existing users:** two viable paths —
  1. **Migrate-on-login Lambda** (`UserMigration` trigger): on first Cognito
     login, the Lambda verifies the *old* bcrypt password against our DB and
     transparently creates the Cognito user. Zero forced resets, seamless UX.
  2. **Bulk import** via Cognito's CSV import (users re-set passwords) — simpler
     but forces a reset.
  The migrate-on-login path preserves the current username/password experience.
- **Keep issuing our tokens until cutover:** run both verifiers behind the same
  middleware during transition (accept our HS256 *or* Cognito RS256), then drop
  ours.

### 6.7 Cross-cutting notes

- If Phase 1 uses **cookies**, tighten CORS: replace `Allow-Origin: *` with an
  explicit origin allow-list and set `Allow-Credentials: true`; add `SameSite`
  and CSRF protection. If Phase 1 uses **bearer tokens**, the current `*` CORS
  is fine and no CSRF handling is needed.
- Public routes (e.g. `POST /user` registration, `POST /login`, `/health`) must
  bypass the auth middleware; everything under `/api/v1/**` sits behind it.
- Token TTLs: short access token (e.g. 15 min) + refresh flow, so a leaked
  token has a small blast radius — this also softens JWT's weak revocation.

---

## 7. Open questions to settle before coding

1. **Ownership model:** A, B, or C?
2. **Delete semantics:** if a user is deleted, `RESTRICT` (block while they have
   movements) or `CASCADE` (wipe their financial history)?
3. **Default/seed user:** what username/name for the baseline row owner?
4. **Categories:** stay global/shared, or also become per-user?
5. **Temporary identity source** before login: header `X-User` vs query param?
6. **Existing DBs:** dev-only reset of the volume, or write an `ALTER` migration
   as well?
7. **Scope confirmation:** do we update `balance`, `reports`, `analytics`,
   `incomes` in the same change, or land `movements` first behind the new
   filter and follow up? (Leaving any unfiltered is a data-leak risk.)
8. **Auth scheme for Phase 1:** self-issued JWT bearer (Cognito-aligned, keeps
   CORS `*`) or server-side session cookie (simpler revocation/logout)?
9. **Cognito timeline:** near-term (→ lean JWT now) or "someday" (→ sessions are
   fine now)? Decides #8.
10. **User migration to Cognito:** migrate-on-login Lambda (no forced reset) vs
    bulk CSV import (users reset passwords)?
11. **Ownership mismatch response:** confirm **`404`** (not `403`) for accessing
    another user's resource by id, to avoid existence leaks.
