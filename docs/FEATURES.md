# Features

What MyBasics-Expenses does today. This is the functionality catalog; for how it
is built see [`ARCHITECTURE.md`](ARCHITECTURE.md), and for a runnable walkthrough
see [`user_flow_smoke_test.md`](user_flow_smoke_test.md).

All routes are under the base path `/api/v1`. Responses are wrapped in a standard
`Envelope { data, error, message }`.

---

## Authentication & users

Token-based authentication (`Authorization: Bearer <token>`). Passwords are hashed
with bcrypt (cost 12) — never stored in plaintext; only the SHA-256 hash of each
token is stored, never the plaintext. The legacy cookie-session login
(`alexedwards/scs`) still exists but is **being deprecated** — protected endpoints
now validate the token, not the cookie.

| Method | Route | Auth | Description |
|---|---|---|---|
| POST | `/user` | Public | Register a user; issues an activation token + welcome email |
| GET | `/user/activate` | Public (token in query) | Activate the account via the emailed link |
| POST | `/tokens/authentication` | Public | **Login**: verify `email`+`password`, return an `authentication_token` (24 h) |
| POST | `/change_password` | Bearer | Change password; re-verifies the current password |
| POST | `/user/login` | Public | *(Legacy)* cookie-session login — being deprecated |
| POST | `/user/logout` | Session | *(Legacy)* destroy the cookie session |

- **Flow:** register → activate (emailed token) → `POST /tokens/authentication` to
  get a token → send `Authorization: Bearer <token>` on protected requests.
- **Middlewares:** `authenticate` resolves identity (anonymous or the token's user)
  on every request; `ProtectEndpoint` rejects anonymous users on protected routes.
  Both `ProtectEndpoint` (token) and the legacy `RestrictEndpoint` (session) feed
  the same user-id context, so handlers are agnostic to the auth method.
- **Errors:** bad credentials → `401 invalid email or password` (identical for
  unknown email and wrong password, to avoid user enumeration); missing/anonymous →
  `401 not authenticated`; malformed/expired token → `401 invalid or missing
  authentication token`.
- **Per-user scoping:** movements, income config, balance, reports and analytics
  only ever return the authenticated user's data. Categories are **shared**.
- **Note:** account activation does **not** yet gate login (a registered,
  non-activated user can still obtain a token).

## Categories

Shared catalog of spending/income categories used to classify movements.

| Method | Route | Description |
|---|---|---|
| GET | `/categories` | List categories |
| POST | `/categories` | Create a category |
| GET | `/categories/{id}` | Get a category |
| PUT | `/categories/{id}` | Update a category |
| DELETE | `/categories/{id}` | Delete a category |

## Movements (income `I` / expense `E`)

The single source of truth for financial data — the user records every movement
manually. Movements carry optional `mail_uid` / `mail_message_id` columns
reserved for a future email-ingestion feature (dedup by source message).

| Method | Route | Description |
|---|---|---|
| POST | `/movements` | Create a movement |
| GET | `/movements` | Movements **grouped by category** (each group has its subtotal) |
| GET | `/movements/expenses` | **Flat** list of expenses **+ the total** of the filter |
| GET | `/movements/summary` | Expense totals grouped by month |
| GET | `/movements/{id}` | Get a movement |
| PUT | `/movements/{id}` | Update a movement |
| DELETE | `/movements/{id}` | Delete a movement |

- **Filters** (list endpoints): `category_id`, `type` (`I`/`E`), `date_from`,
  `date_to`, `limit`.
- **`/movements/expenses`** returns `{ total, movements }`: the total is the sum
  of the expenses matching the filter (all expenses when unfiltered), computed in
  memory from the returned list.

## Fixed monthly income (config)

Versioned fixed income + billing cut day. Each entry is valid from its month
forward until a newer one exists, so past balances stay reproducible.

| Method | Route | Description |
|---|---|---|
| GET | `/incomes/config` | Current effective income config |
| PUT | `/incomes/config` | Create/update the income config (amount, cut day, month) |

## Balance

| Method | Route | Description |
|---|---|---|
| GET | `/balance` | Available balance for a date range (fixed income + incomes − expenses), with carry-over |
| GET | `/balance/periods` | Balance per billing period, with carry-over rolled forward and per-period deficit |

## Reports

| Method | Route | Description |
|---|---|---|
| GET | `/reports/export` | Export movements — `?format=json\|csv\|pdf`, `?months=` |

## Analytics

| Method | Route | Description |
|---|---|---|
| GET | `/analytics/summary` | Totals, monthly average, peak month, expense count |
| GET | `/analytics/by-category` | Spend per category with percentages |
| GET | `/analytics/trend` | Monthly spend trend |
| GET | `/analytics/top-expenses` | Largest expenses (`?limit=`) |
| GET | `/analytics/income-vs-expense` | Income vs expense per month + totals |

Common analytics filter: `?months=` (size of the trailing window).

## Operational

| Method | Route | Description |
|---|---|---|
| GET | `/health` | Liveness + DB ping (`{"status":"ok"}` / `degraded`) |

---

## Cross-cutting capabilities

- **Auth at the edge:** middlewares validate the bearer token (or, legacy, the
  session) and inject the user id into the request context; handlers read it from
  there, never from client input. The identity source is swappable (token/session/
  JWT) without touching the domain code.
- **Per-user data isolation:** every financial query is scoped by `user_id`.
- **Consistent responses:** all handlers use the `Envelope` helpers; errors map to
  proper HTTP status codes (`400`, `401`, `404`, `500`).
- **Manual entry only:** no automatic email ingestion yet (planned; schema ready).
