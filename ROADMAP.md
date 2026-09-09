# MyBasics-Expenses — Roadmap

## What it is

A Go API for personal finances where **the user records movements manually**
(income and expenses). Movements are grouped by category and feed balance,
reports and analytics. There is no automatic email ingestion.

## Current flow

```
User
    │  POST /api/v1/movements   (type = I | E)
    ▼
[Movement Service]  ── validates (amount, type, date, category)
    ▼
[Movement Repository]  ── INSERT INTO movements
    ▼
balance / reports / analytics  ── read from movements
```

## Modules

| Module | Role | Main routes |
|---|---|---|
| **category** | CRUD of categories | `GET/POST /categories`, `GET/PUT/DELETE /categories/{id}` |
| **movement** | CRUD of movements | `GET/POST/PUT/DELETE /api/v1/movements` |
| **incomes** | Fixed income config (versioned) | `GET/PUT /incomes/config` |
| **balance** | Available balance and periods | `GET /balance`, `GET /balance/periods` |
| **reports** | Export | `GET /reports/export` |
| **analytics** | Aggregations and trends | `GET /analytics/*` |

## Future ideas

1. **Pagination on `GET /movements`** — it currently returns everything; with real volume it needs `limit`/`offset`.
2. **Bulk categorization** — `PUT /movements/bulk-categorize` to reassign the category of several movements in a single `UPDATE ... WHERE id IN (...)`.
3. **Multi-user authentication** — currently single-user; add `user_id` and auth.
4. **Per-category budgets** — monthly limits and alerts when they are exceeded.
5. **Integration tests** — today there are only service unit tests.
6. **Fix the `balance` tests** — two cases (`carry_over_out`, recorded income) fail; inherited from the base project, the expected logic still needs review.
