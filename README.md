# MyBasics-Expenses — Personal Finance API

A Go API for keeping track of your personal finances. **You record every movement
manually** (income or expense); the API groups them by category and turns them
into balances, reports and analytics.

> This project is a simplified version of MyExpenses: it **does not include
> automatic email ingestion**. Every movement is created by the user via the API.

---

## Features

Overview of what the API does (full catalog in
[`docs/FEATURES.md`](docs/FEATURES.md)):

- **Users & authentication** — registration with email activation, **token-based
  login** (`Authorization: Bearer`), change password; passwords hashed with bcrypt.
  (Legacy cookie-session login is being deprecated.)
- **Movements** — CRUD of income/expenses, list grouped by category, flat expenses
  list **with total**, monthly summary; filters by category, type and dates.
- **Categories** — CRUD (shared across users).
- **Fixed income** — versioned monthly income config.
- **Balance** — available balance and per billing period (with carry-over).
- **Reports** — export to JSON / CSV / PDF.
- **Analytics** — summary, by-category, trend, top-expenses, income-vs-expense.
- **Per-user data** — every financial query is scoped to the logged-in user.

---

## Stack

- **Go 1.25** · router [chi](https://github.com/go-chi/chi) · `database/sql` + `go-sql-driver/mysql`
- **MySQL 8.0** (Docker)
- **Repository → Service → Handler** layered pattern
- Go module: `github.com/jscodelab/mybasics-expenses`

---

## Structure

```
mybasics-expenses/
├── cmd/api/main.go            # Entry point and wiring
├── internal/
│   ├── category/              # Categories       (model, repository, service, handler)
│   ├── movement/              # Movements I/E    (model, repository, service, handler)
│   ├── incomes/              # Fixed income config (versioned)
│   ├── balance/              # Available balance and billing periods
│   ├── reports/              # Data export
│   ├── analytics/            # Aggregations and trends
│   └── platform/database/    # MySQL connection factory
├── pkg/response/             # Response helpers (Envelope)
├── migrations/                # golang-migrate migrations (000001_init..., 000002_...)
├── docker-compose.yml
└── Dockerfile
```

---

## Architecture diagrams

### 1. Component overview

```mermaid
flowchart LR
    Client["Client<br/>curl · web · mobile"]

    subgraph Process["API process — cmd/api/main.go"]
        direction TB
        Router["chi Router<br/>Logger · Recoverer · RequestID · CORS"]
        Health["GET /health<br/>db.PingContext"]

        subgraph Modules["internal/ modules — base /api/v1"]
            direction LR
            Cat["category"]
            Mov["movement"]
            Inc["incomes"]
            Bal["balance"]
            Rep["reports"]
            Ana["analytics"]
        end

        Resp["pkg/response<br/>Envelope{Data, Error, Message}"]
        Pool["internal/platform/database<br/>MySQL pool — 25 open / 10 idle"]
    end

    DB[("MySQL 8.0<br/>categories · movements<br/>income_config_history")]

    Client -- "HTTP + JSON" --> Router
    Router --> Health
    Router -- "RegisterRoutes" --> Modules
    Modules --> Resp
    Resp -- "JSON envelope" --> Client
    Modules --> Pool
    Health --> Pool
    Pool -- "database/sql" --> DB
```

### 2. Layered pattern inside a module

Every module under `internal/` repeats the same three layers, and each layer is
defined as an interface so the layer above can be unit-tested with a mock.

```mermaid
flowchart LR
    Req["HTTP request"] --> H

    subgraph Module["internal/&lt;module&gt;"]
        direction TB
        H["handler.go<br/>decode request · map errors to status"]
        S["service.go<br/>validation · business rules"]
        R["repository.go<br/>SQL via database/sql"]
        M["model.go<br/>domain structs"]
        E["errors.go<br/>ErrNotFound"]

        H -- "Service interface" --> S
        S -- "Repository interface" --> R
        S -.-> E
        E -.-> H
        R -.-> M
        M -.-> H
    end

    R --> DB[("MySQL")]
    H --> Resp["pkg/response<br/>Success · Created · NotFound"]
```

### 3. Request lifecycle — `POST /api/v1/movements`

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant R as chi Router
    participant H as movement.Handler
    participant S as movement.Service
    participant Repo as movement.Repository
    participant DB as MySQL

    C->>R: POST /api/v1/movements (JSON)
    R->>H: middleware chain, then route
    H->>H: decode body into CreateRequest
    H->>S: CreateMovement(ctx, req)
    S->>S: validate amount > 0, description, category_id, type I/E
    S->>Repo: Create(ctx, movement)
    Repo->>DB: INSERT INTO movements ...
    DB-->>Repo: id
    Repo-->>S: *Movement
    S-->>H: *Movement
    H->>C: 201 Created · Envelope{Data: movement}

    Note over S,H: on ErrNotFound the handler answers 404,<br/>on validation errors 400
```

### 4. Module dependencies and data flow

`movements` is the single source of truth: `balance`, `reports` and `analytics`
never write, they only aggregate what the user recorded manually.

```mermaid
flowchart TD
    User["User — manual entry"]

    User --> CatM["category<br/>CRUD"]
    User --> MovM["movement<br/>CRUD income I / expense E"]
    User --> IncM["incomes<br/>fixed monthly income"]

    CatM --> TCat[("categories")]
    MovM --> TMov[("movements")]
    IncM --> TInc[("income_config_history")]

    TCat -- "FK category_id" --> TMov

    TMov --> BalM["balance<br/>available balance · billing periods"]
    TInc --> BalM
    TMov --> RepM["reports<br/>export JSON / CSV / PDF"]
    TMov --> AnaM["analytics<br/>summary · by-category · trend<br/>top-expenses · income-vs-expense"]

    BalM --> Out["Read-only responses"]
    RepM --> Out
    AnaM --> Out
```

> `balance` is the only module wired with two repositories: its own plus
> `incomes`, because the available balance needs the fixed income and its cut day
> (see the wiring in `cmd/api/main.go`).

### 5. Data model

```mermaid
erDiagram
    categories ||--o{ movements : "classifies"

    categories {
        bigint id PK
        varchar name UK
        varchar description
        varchar color "hex, for UI"
        datetime created_at
        datetime updated_at
    }

    movements {
        bigint id PK
        bigint category_id FK "ON DELETE RESTRICT"
        char type "I = income, E = expense"
        decimal amount
        text description
        date date
        time hour "nullable"
        datetime created_at
        datetime updated_at
    }

    income_config_history {
        int id PK
        date year_month UK "first day of month"
        decimal amount
        tinyint cut_day "1..28, default 24"
        varchar description
        datetime created_at
    }
```

`income_config_history` is versioned rather than updated in place: each row is
valid from its `year_month` forward until a newer row exists, so past balances
stay reproducible.

---

## Getting started

### Option A — Local

```bash
cp .env.example .env          # adjust your MySQL credentials
docker compose up db          # start the database only
go run ./cmd/api/...          # start the API
```

The API listens on **http://localhost:8080**.

### Option B — Docker Compose (MySQL + API)

```bash
docker compose up --build
```

Here the API listens on **http://localhost:8081** and the database on port `3308`.

The schema is managed by **golang-migrate** (an init script is no longer
auto-executed). Once the database is up, apply the migrations with the official
`migrate/migrate` image (data-safe — it never touches the volume):

```bash
NET=mybasics-expenses_mybasics_expenses_net
DSN="mysql://mybasics_user:secret@tcp(db:3306)/mybasics_expenses?multiStatements=true"
docker run --rm --network "$NET" -v "$(pwd)/migrations:/migrations" migrate/migrate \
  -path=/migrations -database "$DSN" up
```

This creates the tables and seeds the base categories. On a database that already
had the schema before golang-migrate existed, run `... force 1` once before `up`.

---

## Environment variables

| Variable      | Default             | Description            |
|---------------|---------------------|------------------------|
| `PORT`        | `8080`              | API port               |
| `DB_HOST`     | `localhost`         | MySQL host             |
| `DB_PORT`     | `3306`              | MySQL port             |
| `DB_USER`     | `root`              | MySQL user             |
| `DB_PASSWORD` | *(empty)*           | MySQL password         |
| `DB_NAME`     | `mybasics_expenses` | Database name          |

---

## GitHub MCP integration

`.mcp.json` declares the GitHub MCP server that Claude Code uses to operate on
this repository's issues and pull requests.

The token is **never committed**: the file references `${GITHUB_PERSONAL_ACCESS_TOKEN}`,
which Claude Code expands from the environment of the shell that launched it.
Export the real value in your profile (`~/.zshrc`, `~/.bashrc`) before starting
Claude Code:

```bash
export GITHUB_PERSONAL_ACCESS_TOKEN="github_pat_..."
```

A `.env` file will **not** work here: the Go app reads it, but the MCP process
does not. Verify the connection with `claude mcp list`.

---

## Endpoints

Base URL: `http://localhost:8080/api/v1`

### Authentication

The API uses **token-based authentication** (Bearer). The flow is: register a
user, **activate** the account with the link from the welcome email, **get a
token** with email + password, and send that token in the
`Authorization: Bearer <token>` header on every request to a protected endpoint.

> The **session cookie** login (`/user/login`, `alexedwards/scs`) still exists but
> is **being deprecated**: protected endpoints already validate the **token**, not
> the cookie.

| Method | Route | Auth | Description |
|--------|-------|------|-------------|
| POST | `/user` | Public | Registers a user. Issues an activation token and sends the welcome email |
| GET  | `/user/activate` | Public (token in query) | Activates the account with the token from the email link |
| POST | `/tokens/authentication` | Public | **New login**: verifies email+password and returns an `authentication_token` |
| POST | `/tokens/logout` | Protected | **New logout**: deletes the user's authentication tokens (signs out on every device) |
| POST | `/change_password` | Protected | Changes the password (re-verifies the current one) |
| POST | `/user/login` | Public | *(Legacy)* session cookie login — being deprecated |
| POST | `/user/logout` | Protected | *(Legacy)* ends the cookie session |

**Registration** — body `{ "username", "name", "email", "password" }`.
`password` between 8 and 72 characters; `username` and `email` unique.
Response `201` `{ "data": "user created" }`. Registering generates an **activation
token** (valid for 3 days) which is sent by email. A duplicate `username`/`email` →
`400 { "error": "username or email already in use" }`.

**Activation** — `GET /user/activate?token=<token>` (the link arrives in the
welcome email). Success → `200 { "data": "account activated" }`; an invalid or
expired token → `400 { "error": "invalid or expired activation link" }`.

**Login (token)** — `POST /tokens/authentication`, body `{ "email", "password" }`.
Success → `201`:
```json
{ "data": { "authentication_token": { "token": "…", "expiry": "2026-01-02T03:04:05Z" } } }
```
The token lasts **24 h**. Invalid credentials (a non-existent email **or** a wrong
password return the same error, so as not to reveal which accounts exist) →
`401 { "error": "invalid email or password" }`. Validation errors (empty fields, a
short password) → `400`.

**Using the token** — send it on every protected request:
`Authorization: Bearer <token>`. The `authenticate` middleware resolves the user;
`ProtectEndpoint` requires that the user is **not** anonymous.

**Logout (token)** — `POST /tokens/logout` with `Authorization: Bearer <token>`.
It deletes **all** of the user's authentication tokens (not just the current one),
so it signs the user out on every device. Success → `200 { "data": "logged out" }`.
There is no client-side state to invalidate beyond discarding the token.

**Password change** — protected, and it re-verifies the current password in the body:
```json
{
  "login_request": { "email": "john@example.com", "password": "currentPass123" },
  "new_password": "brandNewPass456"
}
```
Rules: `new_password` between 8 and 72 characters and **different** from the
current one. `200 { "data": "password updated" }` on success; `400` if it does not
match or validation fails; `401` if there is no valid token.

**Protected endpoints** — everything under `/api/v1` **except** `/user`,
`/user/activate` and `/tokens/authentication` requires a valid token **from an
activated account**. No token or an anonymous user → `401 {"error":"not
authenticated"}`; a malformed/expired token → `401 {"error":"invalid or missing
authentication token"}`; a valid token whose account was never activated →
`403`. The distinction is deliberate: `401` means the caller never proved who
they are, `403` means the identity is known but the account is not allowed
through yet. Every request is filtered by the authenticated user: movements,
balance, income config, reports and analytics only return that user's data.
**Categories are shared**.

### Categories
| Method | Route                 | Description                     |
|--------|-----------------------|---------------------------------|
| GET    | `/categories`         | Lists categories                |
| POST   | `/categories`         | Creates a category              |
| GET    | `/categories/{id}`    | Gets a category                 |
| PUT    | `/categories/{id}`    | Updates a category              |
| DELETE | `/categories/{id}`    | Deletes a category              |

### Movements
| Method | Route                    | Description                                      |
|--------|--------------------------|--------------------------------------------------|
| POST   | `/movements`             | Creates a movement (`type` `I`=income, `E`=expense) |
| GET    | `/movements`             | Lists movements grouped by category               |
| GET    | `/movements/expenses`    | Flat list of expenses **+ total** for the filter (`?category_id=&date_from=&date_to=&limit=`) |
| GET    | `/movements/summary`     | Expense totals per month                          |
| GET    | `/movements/{id}`        | Gets a movement                                   |
| PUT    | `/movements/{id}`        | Updates a movement                               |
| DELETE | `/movements/{id}`        | Deletes a movement                               |

Filters for `GET /movements`: `category_id`, `type`, `date_from`, `date_to`, `limit`.

**`GET /movements/expenses`** returns the flat list of expenses (`type=E`) plus the
`total` of the expenses matching the filter. With no filter it is the total of all
expenses; with `category_id` / a date range, the total of that subset. The response
is an object `{ total, movements }`:
```json
{
  "data": {
    "total": 148150,
    "movements": [
      { "id": 501, "category_id": 2, "category": "Alimentacion", "type": "E",
        "amount": 26900, "description": "RAPPI COLOMBIA*DL",
        "date": "2026-08-06T00:00:00Z", "created_at": "…", "updated_at": "…" }
    ]
  }
}
```
> Accepted filters: `category_id`, `date_from`, `date_to`, `limit` (`type` is
> always `E`). With `limit` (pagination) the `total` reflects the expenses
> returned on that page.

> Every Movements, Fixed income, Balance, Reports and Analytics endpoint is
> **protected**: they require the session cookie and are scoped to the
> authenticated user. The `user_id` is **not** sent in the body or the query — it
> is taken from the session.

### Fixed income, balance, reports and analytics
| Method | Route                            | Description                            |
|--------|----------------------------------|----------------------------------------|
| GET    | `/incomes/config`                | Current fixed income config            |
| PUT    | `/incomes/config`                | Creates/updates the fixed income       |
| GET    | `/balance`                       | Available balance                      |
| GET    | `/balance/periods`               | Balance per billing period             |
| GET    | `/reports/export`                | Exports the data                       |
| GET    | `/analytics/summary`             | General summary                        |
| GET    | `/analytics/by-category`         | Spending per category                  |
| GET    | `/analytics/trend`               | Trend over time                        |
| GET    | `/analytics/top-expenses`        | Largest expenses                       |
| GET    | `/analytics/income-vs-expense`   | Income vs expense                      |

---

## curl examples

Register a user:

```bash
curl -s -X POST http://localhost:8080/api/v1/user \
  -H "Content-Type: application/json" \
  -d '{"username": "john", "name": "John Doe", "email": "john@example.com", "password": "supersecret"}' | jq .
```

Get an authentication token and store it in the `TOKEN` variable:

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/tokens/authentication \
  -H "Content-Type: application/json" \
  -d '{"email": "john@example.com", "password": "supersecret"}' \
  | jq -r '.data.authentication_token.token')
```

> The user must be **activated** (link from the welcome email) and the token lasts
> 24 h. From here on, protected requests carry the
> `-H "Authorization: Bearer $TOKEN"` header.

Create an expense:

```bash
curl -s -H "Authorization: Bearer $TOKEN" -X POST http://localhost:8080/api/v1/movements \
  -H "Content-Type: application/json" \
  -d '{
    "category_id": 1,
    "type": "E",
    "amount": 42500,
    "description": "Weekly groceries",
    "date": "2026-07-15",
    "hour": "10:30"
  }' | jq .
```

Record an income:

```bash
curl -s -H "Authorization: Bearer $TOKEN" -X POST http://localhost:8080/api/v1/movements \
  -H "Content-Type: application/json" \
  -d '{"category_id": 11, "type": "I", "amount": 3000000, "description": "Salary", "date": "2026-07-01"}' | jq .
```

List movements grouped by category (for the logged-in user):

```bash
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/movements | jq .
```

List expenses with their total (flat list). With no filter it returns every
expense and the overall total; with filters, the total of that subset:

```bash
# every expense + overall total
curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/movements/expenses" | jq .

# a single category (e.g. Alimentacion = 2) → total for that category
curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/movements/expenses?category_id=2" | jq .

# by date range → total for the range
curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/movements/expenses?date_from=2026-07-01&date_to=2026-07-31" | jq .

# only the total (without the list)
curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/movements/expenses?category_id=2" | jq '.data.total'
```

Set the monthly income:

```bash
curl -s -H "Authorization: Bearer $TOKEN" -X PUT http://localhost:8080/api/v1/incomes/config \
  -H "Content-Type: application/json" \
  -d '{"amount": 3000000, "cut_day": 24}' | jq .
```

---

## Response format

Every response is wrapped in an `Envelope`:

```json
{
  "data": { "...": "..." },
  "error": null,
  "message": "optional"
}
```

Handlers **always** use the `pkg/response` helpers (`Success`, `Created`,
`NotFound`, …) — they never write raw JSON.

---

## Health check

```bash
curl -i http://localhost:8080/health
```

Answers `200 {"status":"ok"}` if the database responds, or `503 {"status":"degraded"}`.

---

## How to add a new module

1. Create `internal/<name>/` with `model.go`, `repository.go`, `service.go`, `handler.go`
   following the `movement/` pattern.
2. Define interfaces in every layer (enables mocks in tests).
3. Register the routes with `RegisterRoutes(r)` and do the wiring in `cmd/api/main.go`.
4. Add service tests with an inline repository mock.
