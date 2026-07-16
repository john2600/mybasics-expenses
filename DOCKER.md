# Docker Compose — MyBasics-Expenses

El `docker-compose.yml` levanta dos servicios:

| Servicio | Imagen           | Puerto interno | Puerto expuesto |
|----------|------------------|----------------|-----------------|
| `db`     | `mysql:8.0`      | 3306           | **3308**        |
| `api`    | Dockerfile local | 8081           | **8081**        |

## Levantar todo

```bash
docker compose up --build
```

La API queda disponible en `http://localhost:8081/api/v1`.
En el primer arranque, MySQL ejecuta `migrations/001_init.sql` automáticamente
(crea el esquema y siembra las categorías base, sin movimientos de ejemplo).

## Solo la base de datos

```bash
docker compose up db
```

Útil para desarrollar el API en local con `go run ./cmd/api/...`.

## Variables

Se configuran en `docker-compose.yml` bajo `environment`. Valores por defecto:

| Variable      | Valor               |
|---------------|---------------------|
| `DB_NAME`     | `mybasics_expenses` |
| `DB_USER`     | `mybasics_user`     |
| `DB_PASSWORD` | `secret`            |
| `PORT`        | `8081`              |

> No hay configuración de correo/IMAP: este proyecto no ingiere correos.

## Conectarse a la base

```bash
mysql -h 127.0.0.1 -P 3308 -u mybasics_user -psecret mybasics_expenses
```

## Verificar

```bash
curl -i http://localhost:8081/health
curl http://localhost:8081/api/v1/categories
```

## Volúmenes

Los datos de MySQL persisten en el volumen `db_data`. Para empezar de cero:

```bash
docker compose down -v
```
