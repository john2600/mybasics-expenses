# Architecture

MyBasics-Expenses follows a strict layered pattern: **Repository → Service → Handler**.
Dependencies flow inward and every layer defines its own interface.

```
HTTP  ─────────────►  Handler  ─────►  Service  ─────►  Repository  ─────►  MySQL
(chi router)          (handler.go)     (service.go)     (repository.go)
        ▲                  │                │                  │
     JSON Envelope     decodes          validates          direct SQL
   (pkg/response)     request/response  business rules     (database/sql)
```

## Layers

| Layer | File | Responsibility |
|---|---|---|
| **Handler** | `handler.go` | HTTP layer with chi. Decodes the request, calls the service and responds through the `pkg/response` helpers. |
| **Service** | `service.go` | Business logic and validation. Depends on a repository **interface** (enables mocks in tests). |
| **Repository** | `repository.go` | Data access with direct SQL (`database/sql`). Takes a `*sql.DB`, returns domain models. |

The wiring (create repos → services → handlers and register routes) happens in
`cmd/api/main.go`.

## Data model

Three tables (see `migrations/000001_init.up.sql`):

- **`categories`** — catalog of categories (unique name, color for the UI).
- **`movements`** — each income (`type='I'`) or expense (`type='E'`). FK to `categories`.
  This is the **single source of truth** for financial data.
- **`income_config_history`** — fixed monthly income config, **versioned**: each row
  is valid from its `year_month` onward until a newer one exists.

```
categories 1 ────< movements     (fk_movements_category)
income_config_history  (independent, queried by balance)
```

`balance`, `reports` and `analytics` have no tables of their own: they read from
`movements` (and `balance` combines it with `income_config_history` through the
`incomes` module).

## Key conventions

- **Responses**: always `Envelope{Data, Error, Message}` via `pkg/response`. Never raw JSON.
- **Errors**: each module defines `ErrNotFound` in `errors.go`; the service returns it and the handler translates it into a `404`.
- **Tests**: service-layer unit tests only, with inline mocks that implement the repository interface. They live next to the code (`service_test.go`).
- **Connection pool**: configured in `internal/platform/database/mysql.go` (`MaxOpenConns=25`, `MaxIdleConns=10`).

## Differences from the base project (MyExpenses)

This project is a simplified replica. The whole automatic email ingestion layer
was **removed**:

- Removed modules: `ingestion` (orchestrator), `mail` (IMAP client).
- The `expense` module (legacy, no table) was removed; `movements` is the single model.
- The `movements` table no longer has the `mail_uid` / `mail_message_id` fields.
- The schema was consolidated into a single definitive init migration (no sample data).
