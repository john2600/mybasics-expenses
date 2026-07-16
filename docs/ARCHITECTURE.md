# Arquitectura

MyBasics-Expenses sigue un patrón por capas estricto: **Repository → Service → Handler**.
Las dependencias fluyen hacia adentro y cada capa define su propia interfaz.

```
HTTP  ─────────────►  Handler  ─────►  Service  ─────►  Repository  ─────►  MySQL
(chi router)          (handler.go)     (service.go)     (repository.go)
        ▲                  │                │                  │
     JSON Envelope    decodifica       valida reglas      SQL directo
   (pkg/response)     request/response  de negocio        (database/sql)
```

## Capas

| Capa | Archivo | Responsabilidad |
|---|---|---|
| **Handler** | `handler.go` | Capa HTTP con chi. Decodifica la petición, llama al servicio y responde con los helpers de `pkg/response`. |
| **Service** | `service.go` | Lógica de negocio y validación. Depende de una **interfaz** de repositorio (permite mocks en tests). |
| **Repository** | `repository.go` | Acceso a datos con SQL directo (`database/sql`). Recibe `*sql.DB`, devuelve modelos de dominio. |

El wiring (crear repos → services → handlers y registrar rutas) ocurre en
`cmd/api/main.go`.

## Modelo de datos

Tres tablas (ver `migrations/001_init.sql`):

- **`categories`** — catálogo de categorías (nombre único, color para UI).
- **`movements`** — cada ingreso (`type='I'`) o gasto (`type='E'`). FK a `categories`.
  Es la **única fuente de verdad** financiera.
- **`income_config_history`** — configuración de ingreso fijo mensual, **versionada**:
  cada fila vale desde su `year_month` en adelante hasta que exista una más reciente.

```
categories 1 ────< movements     (fk_movements_category)
income_config_history  (independiente, consultada por balance)
```

`balance`, `reports` y `analytics` no tienen tablas propias: leen de `movements`
(y `balance` combina con `income_config_history` a través del módulo `incomes`).

## Convenciones clave

- **Respuestas**: siempre `Envelope{Data, Error, Message}` vía `pkg/response`. Nunca JSON crudo.
- **Errores**: cada módulo define `ErrNotFound` en `errors.go`; el servicio lo retorna y el handler lo traduce a `404`.
- **Tests**: solo unit tests de la capa de servicio, con mocks inline que implementan la interfaz del repositorio. Viven junto al código (`service_test.go`).
- **Pool de conexiones**: configurado en `internal/platform/database/mysql.go` (`MaxOpenConns=25`, `MaxIdleConns=10`).

## Diferencias con el proyecto base (MyExpenses)

Este proyecto es una réplica simplificada. Se **eliminó** toda la capa de ingesta
automática de correos:

- Módulos removidos: `ingestion` (orquestador), `mail` (cliente IMAP).
- Módulo `expense` (legacy, sin tabla) removido; `movements` es el modelo único.
- La tabla `movements` ya no tiene los campos `mail_uid` / `mail_message_id`.
- El esquema se consolidó en un único `001_init.sql` definitivo (sin datos de ejemplo).
