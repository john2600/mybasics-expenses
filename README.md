# MyBasics-Expenses — API de Finanzas Personales

API en Go para llevar el control de tus finanzas personales. **Tú registras cada
movimiento manualmente** (ingreso o gasto); la API los agrupa por categoría y los
convierte en balances, reportes y analíticas.

> Este proyecto es una versión simplificada de MyExpenses: **no incluye ingesta
> automática de correos**. Todos los movimientos los crea el usuario vía API.

---

## Stack

- **Go 1.23** · router [chi](https://github.com/go-chi/chi) · `database/sql` + `go-sql-driver/mysql`
- **MySQL 8.0** (Docker)
- Patrón por capas **Repository → Service → Handler**
- Módulo Go: `github.com/jscodelab/mybasics-expenses`

---

## Estructura

```
mybasics-expenses/
├── cmd/api/main.go            # Punto de entrada y wiring
├── internal/
│   ├── category/              # Categorías        (model, repository, service, handler)
│   ├── movement/              # Movimientos I/E   (model, repository, service, handler)
│   ├── incomes/              # Config de ingreso fijo (versionado)
│   ├── balance/              # Balance disponible y periodos de corte
│   ├── reports/              # Exportación de datos
│   ├── analytics/            # Agregaciones y tendencias
│   └── platform/database/    # Factory de conexión MySQL
├── pkg/response/             # Helpers de respuesta (Envelope)
├── migrations/001_init.sql   # Esquema definitivo + seed base
├── docker-compose.yml
└── Dockerfile
```

---

## Puesta en marcha

### Opción A — Local

```bash
cp .env.example .env          # ajusta credenciales de tu MySQL
docker compose up db          # levanta solo la base de datos
go run ./cmd/api/...          # levanta el API
```

El API queda en **http://localhost:8080**.

### Opción B — Docker Compose (MySQL + API)

```bash
docker compose up --build
```

Aquí el API queda en **http://localhost:8081** y la base en el puerto `3308`.
La migración `migrations/001_init.sql` se ejecuta automáticamente en el primer
arranque: crea las tablas `categories`, `movements`, `income_config_history`,
siembra categorías base y **ningún movimiento de ejemplo** (empiezas limpio).

---

## Variables de entorno

| Variable      | Default   | Descripción            |
|---------------|-----------|------------------------|
| `PORT`        | `8080`              | Puerto del API         |
| `DB_HOST`     | `localhost`         | Host de MySQL          |
| `DB_PORT`     | `3306`              | Puerto de MySQL        |
| `DB_USER`     | `root`              | Usuario de MySQL       |
| `DB_PASSWORD` | *(vacío)*           | Contraseña de MySQL    |
| `DB_NAME`     | `mybasics_expenses` | Nombre de la base      |

---

## Endpoints

Base URL: `http://localhost:8080/api/v1`

### Categorías
| Método | Ruta                  | Descripción                     |
|--------|-----------------------|---------------------------------|
| GET    | `/categories`         | Lista categorías                |
| POST   | `/categories`         | Crea una categoría              |
| GET    | `/categories/{id}`    | Obtiene una categoría           |
| PUT    | `/categories/{id}`    | Edita una categoría             |
| DELETE | `/categories/{id}`    | Elimina una categoría           |

### Movimientos
| Método | Ruta                     | Descripción                                      |
|--------|--------------------------|--------------------------------------------------|
| POST   | `/movements`             | Crea un movimiento (`type` `I`=ingreso, `E`=gasto) |
| GET    | `/movements`             | Lista movimientos agrupados por categoría         |
| GET    | `/movements/expenses`    | Lista plana de gastos (`?limit=&date_from=&date_to=`) |
| GET    | `/movements/summary`     | Totales de gasto por mes                          |
| GET    | `/movements/{id}`        | Obtiene un movimiento                             |
| PUT    | `/movements/{id}`        | Edita un movimiento                              |
| DELETE | `/movements/{id}`        | Elimina un movimiento                            |

Filtros de `GET /movements`: `category_id`, `type`, `date_from`, `date_to`, `limit`.

### Ingreso fijo, balance, reportes y analítica
| Método | Ruta                             | Descripción                            |
|--------|----------------------------------|----------------------------------------|
| GET    | `/incomes/config`                | Config de ingreso fijo vigente         |
| PUT    | `/incomes/config`                | Crea/actualiza el ingreso fijo         |
| GET    | `/balance`                       | Balance disponible                     |
| GET    | `/balance/periods`               | Balance por periodo de corte           |
| GET    | `/reports/export`                | Exporta los datos                      |
| GET    | `/analytics/summary`             | Resumen general                        |
| GET    | `/analytics/by-category`         | Gasto por categoría                    |
| GET    | `/analytics/trend`               | Tendencia temporal                     |
| GET    | `/analytics/top-expenses`        | Mayores gastos                         |
| GET    | `/analytics/income-vs-expense`   | Ingreso vs gasto                       |

---

## Ejemplos con curl

Crear un gasto:

```bash
curl -s -X POST http://localhost:8080/api/v1/movements \
  -H "Content-Type: application/json" \
  -d '{
    "category_id": 1,
    "type": "E",
    "amount": 42500,
    "description": "Mercado de la semana",
    "date": "2026-07-15",
    "hour": "10:30"
  }' | jq .
```

Registrar un ingreso:

```bash
curl -s -X POST http://localhost:8080/api/v1/movements \
  -H "Content-Type: application/json" \
  -d '{"category_id": 11, "type": "I", "amount": 3000000, "description": "Salario", "date": "2026-07-01"}' | jq .
```

Listar movimientos agrupados por categoría:

```bash
curl -s http://localhost:8080/api/v1/movements | jq .
```

Fijar el ingreso mensual:

```bash
curl -s -X PUT http://localhost:8080/api/v1/incomes/config \
  -H "Content-Type: application/json" \
  -d '{"amount": 3000000, "cut_day": 24}' | jq .
```

---

## Formato de respuesta

Todas las respuestas se envuelven en un `Envelope`:

```json
{
  "data": { "...": "..." },
  "error": null,
  "message": "optional"
}
```

Los handlers **siempre** usan los helpers de `pkg/response` (`Success`, `Created`,
`NotFound`, …) — nunca escriben JSON crudo.

---

## Health check

```bash
curl -i http://localhost:8080/health
```

Responde `200 {"status":"ok"}` si la base responde, o `503 {"status":"degraded"}`.

---

## Cómo agregar un módulo nuevo

1. Crear `internal/<nombre>/` con `model.go`, `repository.go`, `service.go`, `handler.go`
   siguiendo el patrón de `movement/`.
2. Definir interfaces en cada capa (permite mocks en tests).
3. Registrar rutas con `RegisterRoutes(r)` y hacer el wiring en `cmd/api/main.go`.
4. Agregar tests de servicio con un mock inline del repositorio.
