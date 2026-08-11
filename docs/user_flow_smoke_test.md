# User flow — manual smoke test (curl)

End-to-end walkthrough for a single user, exercising auth + per-user scoping.
Each step lists the request, the expected HTTP status, and a sample response.

Steps:

1. Create user
2. Try to set income config **without a session** → `401`
3. Log in (get the session cookie)
4. Set income config **with the session** → `200`
5. Register an income
6. Register an expense
7. List expenses
8. Change the password
9. Log out (kill the session)

---

## Prerequisites

- The API and MySQL running. Base URL depends on how you start it:
  - **Local** (`go run ./cmd/api/...`): port `8080`
  - **Docker Compose**: port `8081`
- `jq` is optional (only for pretty-printing).

Set a base URL and a cookie jar once, then paste the steps:

```bash
BASE=http://localhost:8080/api/v1   # use :8081 if running via docker compose
CJ=cookies.txt                      # session cookie is stored here
rm -f "$CJ"
```

---

## 1. Create user

Public endpoint. Password must be 8–72 chars; `username` and `email` are unique.

```bash
curl -s -X POST "$BASE/user" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "john",
    "name": "John Doe",
    "email": "john@example.com",
    "password": "supersecret"
  }' | jq .
```

**Expected — `201 Created`:**

```json
{ "data": "user created" }
```

---

## 2. Try to set income config WITHOUT a session

Protected endpoint, no cookie yet → must be rejected.

```bash
curl -s -o /dev/null -w 'HTTP %{http_code}\n' \
  -X PUT "$BASE/incomes/config" \
  -H "Content-Type: application/json" \
  -d '{"amount": 3000000, "cut_day": 24}'
```

**Expected — `401 Unauthorized`:**

```
HTTP 401
```

Body: `{ "error": "not authenticated" }`

---

## 3. Log in

Stores the authenticated user id in the session and returns the `session` cookie
(saved into `$CJ` via `-c`).

```bash
curl -s -c "$CJ" -X POST "$BASE/user/login" \
  -H "Content-Type: application/json" \
  -d '{"email": "john@example.com", "password": "supersecret"}' | jq .
```

**Expected — `200 OK`:**

```json
{ "data": "login successful" }
```

Wrong credentials → `401 { "error": "invalid email or password" }`.

---

## 4. Set income config WITH the session

Same request as step 2, now sending the cookie with `-b`.

```bash
curl -s -b "$CJ" -X PUT "$BASE/incomes/config" \
  -H "Content-Type: application/json" \
  -d '{"amount": 3000000, "cut_day": 24}' | jq .
```

**Expected — `200 OK`:**

```json
{
  "data": {
    "year_month": "2026-07-01T00:00:00Z",
    "amount": 3000000,
    "cut_day": 24,
    "description": "Ingreso fijo",
    "created_at": "2026-07-01T00:00:00Z"
  }
}
```

> Optional fields on this endpoint: `year_month` ("YYYY-MM", defaults to the
> current month), `description`. `cut_day` must be 1–28; `amount` >= 0.

---

## 5. Register an income

Movement with `type: "I"`. `category_id` 11 = "Otros" (see seeded categories).
`hour` is optional.

```bash
curl -s -b "$CJ" -X POST "$BASE/movements" \
  -H "Content-Type: application/json" \
  -d '{
    "category_id": 11,
    "type": "I",
    "amount": 500000,
    "description": "Freelance",
    "date": "2026-07-05"
  }' | jq .
```

**Expected — `201 Created`:**

```json
{
  "data": {
    "id": 1,
    "category_id": 11,
    "category": "Otros",
    "type": "I",
    "amount": 500000,
    "description": "Freelance",
    "date": "2026-07-05T00:00:00Z",
    "created_at": "…",
    "updated_at": "…"
  }
}
```

---

## 6. Register an expense

Movement with `type: "E"`. `category_id` 1 = "Alimentacion".

```bash
curl -s -b "$CJ" -X POST "$BASE/movements" \
  -H "Content-Type: application/json" \
  -d '{
    "category_id": 1,
    "type": "E",
    "amount": 42500,
    "description": "Mercado de la semana",
    "date": "2026-07-10",
    "hour": "10:30"
  }' | jq .
```

**Expected — `201 Created`:**

```json
{
  "data": {
    "id": 2,
    "category_id": 1,
    "category": "Alimentacion",
    "type": "E",
    "amount": 42500,
    "description": "Mercado de la semana",
    "date": "2026-07-10T00:00:00Z",
    "hour": "10:30:00",
    "created_at": "…",
    "updated_at": "…"
  }
}
```

---

## 7. List expenses (with total)

Flat list of expenses (`type = E`) for the logged-in user, newest first, plus the
`total` of the expenses matching the filter. With no filter it's the total of all
expenses; with `category_id` / date range it's the total of that subset.
Optional filters: `category_id`, `date_from`, `date_to`, `limit`.

```bash
curl -s -b "$CJ" "$BASE/movements/expenses" | jq .
```

**Expected — `200 OK`:** an object `{ total, movements }` — only this user's
expenses (the income from step 5 is excluded), and the total across them:

```json
{
  "data": {
    "total": 42500,
    "movements": [
      {
        "id": 2,
        "category_id": 1,
        "category": "Alimentacion",
        "type": "E",
        "amount": 42500,
        "description": "Mercado de la semana",
        "date": "2026-07-10T00:00:00Z",
        "hour": "10:30:00",
        "created_at": "…",
        "updated_at": "…"
      }
    ]
  }
}
```

Filtered total (e.g. one category) — same shape, `total` scoped to the filter:

```bash
curl -s -b "$CJ" "$BASE/movements/expenses?category_id=1" | jq '.data.total'
```

---

## 8. Change the password

Protected endpoint — requires the session cookie **and** re-verifies the current
password in the body (defence in depth). The `new_password` must be 8–72 chars
and different from the current one.

```bash
curl -s -b "$CJ" -X POST "$BASE/change_password" \
  -H "Content-Type: application/json" \
  -d '{
    "login_request": { "email": "john@example.com", "password": "supersecret" },
    "new_password": "evenbettersecret"
  }' | jq .
```

**Expected — `200 OK`:**

```json
{ "data": "password updated" }
```

Failure cases the UI should handle:
- Wrong current password → `400 { "error": "password not coincidences ..." }`
- `new_password` too short / equal to current → `400` with the validation message
- No session → `401 { "error": "not authenticated" }`

Confirm the change — logging in with the **new** password now works:

```bash
curl -s -c "$CJ" -X POST "$BASE/user/login" \
  -H "Content-Type: application/json" \
  -d '{"email": "john@example.com", "password": "evenbettersecret"}' | jq .
# → { "data": "login successful" }
```

---

## 9. Log out (kill the session)

Protected endpoint. Destroys the session tied to the cookie sent with this
request (scs identifies it by the cookie's token, not by user id) and expires
the cookie. After this, the same cookie no longer authenticates.

```bash
curl -s -b "$CJ" -c "$CJ" -X POST "$BASE/user/logout" | jq .
```

**Expected — `200 OK`:**

```json
{ "data": "logout successful" }
```

Verify the session is dead — the same cookie is now rejected:

```bash
curl -s -b "$CJ" -o /dev/null -w 'HTTP %{http_code}\n' "$BASE/incomes/config"
# → HTTP 401
```

> Only the session for **this** cookie is destroyed. If the same user is logged
> in on another device (a different token), that session stays active.

---

## Run the whole flow at once

Copy-paste this block to execute steps 1–7 back to back:

```bash
BASE=http://localhost:8080/api/v1   # :8081 for docker compose
CJ=cookies.txt; rm -f "$CJ"

echo "1) create user";            curl -s -X POST "$BASE/user" -H "Content-Type: application/json" \
  -d '{"username":"john","name":"John Doe","email":"john@example.com","password":"supersecret"}'; echo

echo "2) config WITHOUT session (expect 401)"; \
  curl -s -o /dev/null -w '   HTTP %{http_code}\n' -X PUT "$BASE/incomes/config" \
  -H "Content-Type: application/json" -d '{"amount":3000000,"cut_day":24}'

echo "3) login";                  curl -s -c "$CJ" -X POST "$BASE/user/login" -H "Content-Type: application/json" \
  -d '{"email":"john@example.com","password":"supersecret"}'; echo

echo "4) config WITH session";    curl -s -b "$CJ" -X PUT "$BASE/incomes/config" -H "Content-Type: application/json" \
  -d '{"amount":3000000,"cut_day":24}'; echo

echo "5) income";                 curl -s -b "$CJ" -X POST "$BASE/movements" -H "Content-Type: application/json" \
  -d '{"category_id":11,"type":"I","amount":500000,"description":"Freelance","date":"2026-07-05"}'; echo

echo "6) expense";                curl -s -b "$CJ" -X POST "$BASE/movements" -H "Content-Type: application/json" \
  -d '{"category_id":1,"type":"E","amount":42500,"description":"Mercado","date":"2026-07-10","hour":"10:30"}'; echo

echo "7) list expenses";          curl -s -b "$CJ" "$BASE/movements/expenses?limit=10"; echo

echo "8) change password";        curl -s -b "$CJ" -X POST "$BASE/change_password" -H "Content-Type: application/json" \
  -d '{"login_request":{"email":"john@example.com","password":"supersecret"},"new_password":"evenbettersecret"}'; echo

echo "9) logout";                 curl -s -b "$CJ" -c "$CJ" -X POST "$BASE/user/logout"; echo
echo "   verify (expect 401)";    curl -s -b "$CJ" -o /dev/null -w '   HTTP %{http_code}\n' "$BASE/incomes/config"
```

---

## Notes

- The `user_id` is **never** sent in the body or query — it is taken from the
  session on every protected request.
- Responses are wrapped in the standard `Envelope { data, error, message }`.
- Seeded category ids: 1 Alimentacion, 2 Restaurante, 3 Transporte, 4 Vivienda,
  5 Salud, 6 Entretenimiento, 7 Educacion, 8 Ropa, 9 Viajes,
  10 Ahorros e Inversiones, 11 Otros.
