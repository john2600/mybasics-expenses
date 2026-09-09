# Docker Compose — MyBasics-Expenses

`docker-compose.yml` brings up two services:

| Service  | Image            | Internal port  | Exposed port    |
|----------|------------------|----------------|-----------------|
| `db`     | `mysql:8.0`      | 3306           | **3308**        |
| `api`    | Local Dockerfile | 8081           | **8081**        |

## Bring everything up

```bash
docker compose up --build
```

The API becomes available at `http://localhost:8081/api/v1`.

The schema is managed by **golang-migrate** — it is not auto-loaded by the MySQL
container. Once the database is up, apply the pending migrations (see
[README.md](README.md) for the `migrate/migrate` command). That creates the
schema and seeds the base categories, with no sample movements.

## Database only

```bash
docker compose up db
```

Useful for developing the API locally with `go run ./cmd/api/...`.

## Variables

They are configured in `docker-compose.yml` under `environment`. Default values:

| Variable      | Value               |
|---------------|---------------------|
| `DB_NAME`     | `mybasics_expenses` |
| `DB_USER`     | `mybasics_user`     |
| `DB_PASSWORD` | `secret`            |
| `PORT`        | `8081`              |

> There is no mail/IMAP configuration: this project does not ingest emails.

## Connect to the database

```bash
mysql -h 127.0.0.1 -P 3308 -u mybasics_user -psecret mybasics_expenses
```

## Verify

```bash
curl -i http://localhost:8081/health
curl http://localhost:8081/api/v1/categories
```

## Volumes

MySQL data persists in the `db_data` volume. `docker compose down` (without
flags) stops the containers and **keeps** the data.

> **Warning:** `docker compose down -v` deletes the volume and wipes every local
> record (users, movements, income config, sessions). There is no automatic
> backup — use it only when you deliberately want a clean database, and consider
> a `mysqldump` first.
