# Email Sender (SMTP / Mailtrap)

Outbound email for MyBasics-Expenses. The app sends messages to users (e.g. a
future welcome/activation email) through an SMTP server. In development we point
at the **Mailtrap sandbox**, a hosted SMTP that *traps* every message in a web
inbox instead of delivering it — so nothing reaches real mailboxes while testing.

## Do we need a mail server in Docker?

**No.** Mailtrap sandbox is a hosted SMTP (`sandbox.smtp.mailtrap.io`). We do not
run a mail-server container. The only thing Docker needs is for the `api`
container to know where to connect — the `SMTP_*` environment variables. The
container already has outbound network access via the compose bridge network.

```
[api container] --SMTP 587 (STARTTLS)--> sandbox.smtp.mailtrap.io  (Mailtrap web inbox)
                       ↑ username / password from your Mailtrap inbox
```

## How it is wired

| Piece | Responsibility |
|---|---|
| `internal/mailer/mailer.go` | `Mailer` interface + a [go-mail](https://github.com/wneessen/go-mail) implementation. `Send(ctx, to, subject, body)` composes a plain-text message and delivers it over SMTP (STARTTLS). |
| `cmd/api/app.go` → `initMailer()` | Reads the `SMTP_*` env, builds the mailer, stores it on `app.Mailer.Sender`. `mailer.New` only *builds* the client — it does not dial — so the app boots even with no credentials; a missing/invalid credential fails later on `Send`, not at startup. |
| `docker-compose.yml` (`api` service) | Injects `SMTP_*`. Host/port default to Mailtrap; the **secret** username/password come from a local `.env` (gitignored) via `${...}` substitution. |
| `.env.example` | Template for the `SMTP_*` variables. |

Consumers depend on the `mailer.Mailer` interface, never on the concrete SMTP
client — same pattern as the rest of the codebase.

## Configuration

Environment variables (see `.env.example`):

| Variable | Default | Notes |
|---|---|---|
| `SMTP_HOST` | `sandbox.smtp.mailtrap.io` | SMTP server host. |
| `SMTP_PORT` | `587` | STARTTLS port. Mailtrap also accepts `2525`. |
| `SMTP_USERNAME` | *(empty)* | From Mailtrap inbox. **Empty disables sending.** |
| `SMTP_PASSWORD` | *(empty)* | From Mailtrap inbox. |
| `SMTP_FROM` | `MyBasics-Expenses <no-reply@mybasics.local>` | Envelope/header From. |

### Local setup (2 steps)

1. In Mailtrap → **Inbox → SMTP Settings**, copy the sandbox **username** and
   **password**.
2. Create a local `.env` (gitignored) at the repo root:
   ```env
   SMTP_USERNAME=your_mailtrap_user
   SMTP_PASSWORD=your_mailtrap_pass
   ```
   Then `docker-compose up -d`. docker-compose substitutes these into the `api`
   container. Running locally with `go run` works too — `godotenv` loads `.env`.

## Go toolchain note

`go-mail` requires **Go ≥ 1.25**, so adopting it bumped the module and every
build surface from Go 1.23 → **1.25**: `go.mod`, the `Dockerfile`
(`golang:1.25-alpine`), the CI workflow (`go-version: "1.25"`), and the README.
Build, `go vet` and all tests pass unchanged on 1.25; `go-mail` pulls in no new
transitive dependencies beyond `x/crypto` / `x/text`, which were already used.

## Status / next steps

The mailer is **configured but not yet triggered** — no handler calls it yet.
Next step is to wire it into a real use case:

- **Welcome / activation email** on user registration (pairs with the WIP
  `activated` / `version` fields on the `users` model — see the `TODO(ADVANCE)`
  in `internal/users/model.go`).
- Or a **temporary test endpoint** (e.g. `POST /api/v1/test-email`) to validate
  the Mailtrap connection end-to-end, then remove it.

To actually send during testing, the local `.env` must have valid
`SMTP_USERNAME` / `SMTP_PASSWORD`; otherwise `Send` returns an auth error.
