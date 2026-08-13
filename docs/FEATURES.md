# Features

What MyBasics-Expenses does today. This is the functionality catalog; for how it
is built see [`ARCHITECTURE.md`](ARCHITECTURE.md), and for a runnable walkthrough
see [`user_flow_smoke_test.md`](user_flow_smoke_test.md).

All routes are under the base path `/api/v1`. Responses are wrapped in a standard
`Envelope { data, error, message }`.

---

## Authentication & users

Server-side sessions (cookie `session`, stored in MySQL via `alexedwards/scs`).
Passwords are hashed with bcrypt (cost 12) — never stored in plaintext.

| Method | Route | Auth | Description |
|---|---|---|---|
| POST | `/user` | Public | Register a user (`username`, `name`, `email`, `password`) |
| POST | `/user/login` | Public | Log in (`email`, `password`); sets the `session` cookie |
| POST | `/user/logout` | Session | Destroy the current session (only that device's) |
| POST | `/change_password` | Session | Change password; re-verifies the current password |

- **Protected endpoints:** everything below requires a valid session. Without a
  cookie → `401 { "error": "not authenticated" }`.
- **Per-user scoping:** movements, income config, balance, reports and analytics
  only ever return the authenticated user's data. Categories are **shared**.

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

- **Session auth at the edge:** a middleware validates the session and injects the
  user id into the request context; handlers read it from there, never from
  client input. (Designed to swap to JWT later without touching the domain code.)
- **Per-user data isolation:** every financial query is scoped by `user_id`.
- **Consistent responses:** all handlers use the `Envelope` helpers; errors map to
  proper HTTP status codes (`400`, `401`, `404`, `500`).
- **Manual entry only:** no automatic email ingestion yet (planned; schema ready).
