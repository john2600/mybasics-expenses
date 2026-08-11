# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

IMPORTANT:

Before you make any change, create and checkout a feature branch named "feature_some_short_name"
Make and then commit your changes in this branch.

## About this project

MyBasics-Expenses is a personal finance API written in Go. The user records every
movement (income or expense) **manually** through the API — there is no automatic
email ingestion. Movements are grouped by category and aggregated into balances,
reports and analytics.

## Commands

```bash
# Run the API locally (requires .env file from .env.example)
go run ./cmd/api/...

# Build binary
go build -o bin/api ./cmd/api/...

# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/movement/...
go test ./internal/category/...

# Run a single test
go test ./internal/movement/... -run TestCreateMovement_Success

# Run with Docker Compose (starts MySQL + API)
docker compose up --build

# Start only the database
docker compose up db
```

## Data safety (running the app locally)

**Never destroy the database volume unless the user explicitly asks.** The Docker
volume `db_data` holds all local data (users, movements, income config, sessions).
Losing it is unrecoverable — there is no automatic backup.

- To apply code changes, use `docker compose up --build -d` — it rebuilds the
  image and **keeps** the data.
- `docker compose down` (without `-v`) is safe: it stops containers but keeps the
  volume.
- **Do NOT run `docker compose down -v`.** The `-v` deletes the volume and wipes
  all data. Use it only when the user has **explicitly** asked for a clean/fresh
  database.
- If a schema/migration change genuinely requires recreating the volume, **stop
  and ask first**, and offer to back up the data with `mysqldump` before wiping.
- Prefer a real migration over recreating the volume whenever possible.

## Architecture

The project follows a strict **Repository → Service → Handler** layered pattern:

- **Repository** (`repository.go`): Direct SQL queries via `database/sql`. Accepts `*sql.DB`, returns domain models.
- **Service** (`service.go`): Business logic and validation. Depends on a repository interface (enabling unit test mocks).
- **Handler** (`handler.go`): HTTP layer using chi. Decodes requests, calls service, writes responses via `pkg/response`.

Dependencies flow inward: Handler → Service → Repository. All three layers define interfaces. Wiring happens in `cmd/api/main.go`.

### Modules (`internal/`)

| Module | Responsibility | Routes (base `/api/v1`) |
|---|---|---|
| `category` | CRUD of categories | `GET/POST /categories`, `GET/PUT/DELETE /categories/{id}` |
| `movement` | CRUD of movements (income `I` / expense `E`) | `GET/POST /movements`, `GET /movements/expenses`, `GET /movements/summary`, `GET/PUT/DELETE /movements/{id}` |
| `incomes` | Fixed monthly income config (versioned) | `GET/PUT /incomes/config` |
| `balance` | Available balance and billing periods | `GET /balance`, `GET /balance/periods` |
| `reports` | Data export | `GET /reports/export` |
| `analytics` | Aggregations and trends | `GET /analytics/{summary,by-category,trend,top-expenses,income-vs-expense}` |

`movements` is the single source of truth for financial data. `balance`, `reports`
and `analytics` all read from it.

## Key Conventions

**Response format** — all handlers use `pkg/response` helpers which wrap responses in an `Envelope{Data, Error, Message}` struct. Always use `response.Success()`, `response.Created()`, `response.NotFound()`, etc. — never write raw JSON.

**Error handling** — each module's `errors.go` defines `ErrNotFound`. Services return this sentinel; handlers check for it to decide the HTTP status code.

**Testing** — only service-layer unit tests exist, using inline mock structs that implement the repository interface. No integration tests. Tests live alongside the source (`service_test.go`).

**Database** — connection pool is configured in `internal/platform/database/mysql.go` (MaxOpenConns=25, MaxIdleConns=10). The migration file `migrations/001_init.sql` is the single, definitive source of truth for schema and base seed data (categories + one baseline income row, no sample movements); it runs automatically via Docker Compose on first start.

**Environment** — copy `.env.example` to `.env` before running locally. The app reads `PORT`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`.
