# MyBasics-Expenses — Roadmap

## Qué es

API en Go para finanzas personales donde **el usuario registra manualmente** sus
movimientos (ingresos y gastos). Los movimientos se agrupan por categoría y
alimentan balance, reportes y analíticas. No hay ingesta automática de correos.

## Flujo actual

```
Usuario
    │  POST /api/v1/movements   (type = I | E)
    ▼
[Movement Service]  ── valida (monto, tipo, fecha, categoría)
    ▼
[Movement Repository]  ── INSERT INTO movements
    ▼
balance / reports / analytics  ── leen de movements
```

## Módulos

| Módulo | Rol | Rutas principales |
|---|---|---|
| **category** | CRUD de categorías | `GET/POST /categories`, `GET/PUT/DELETE /categories/{id}` |
| **movement** | CRUD de movimientos | `GET/POST/PUT/DELETE /api/v1/movements` |
| **incomes** | Config de ingreso fijo (versionado) | `GET/PUT /incomes/config` |
| **balance** | Balance disponible y periodos | `GET /balance`, `GET /balance/periods` |
| **reports** | Exportación | `GET /reports/export` |
| **analytics** | Agregaciones y tendencias | `GET /analytics/*` |

## Ideas a futuro

1. **Paginación en `GET /movements`** — hoy devuelve todo; con volumen real necesita `limit`/`offset`.
2. **Categorización masiva** — `PUT /movements/bulk-categorize` para reasignar categoría a varios movimientos en un solo `UPDATE ... WHERE id IN (...)`.
3. **Autenticación multiusuario** — hoy es de un solo usuario; agregar `user_id` y auth.
4. **Presupuestos por categoría** — límites mensuales y alertas al superarlos.
5. **Tests de integración** — hoy solo hay tests unitarios de servicio.
6. **Corregir tests de `balance`** — dos casos (`carry_over_out`, ingresos registrados) fallan; heredados del proyecto base, pendientes de revisar la lógica esperada.
