# Plan — Read the user id from the session (replace the hardcoded id)

> Status: **Draft for implementation. Not done yet.**
> Goal: replace the temporary `const currentUserID = 1` in the domain handlers
> with the **authenticated** user id, so a second user only ever sees their own
> data. Login already stores it via
> `sm.Put(r.Context(), "authenticatedUserID", id)`.

---

## The rule: extract at the edge, carry in `context`

Neither of the two tempting shortcuts is right:

- ❌ **Pass it as a header** (middleware → `X-User-Id` → handler). A header is
  client-spoofable and it's a pointless round-trip: taking a server-side value
  out of the session only to re-expose it as text the client could set. The Go
  `context` is the correct **in-process, immutable, server-set** carrier.
- ❌ **Call `sm.GetInt` in every handler.** That couples *every* domain handler
  to `scs`. The whole point is that `movement` / `balance` / `analytics` should
  not know **how** identity was established. Swapping sessions for JWT/Cognito
  later should touch only the middleware.

✅ **Correct:** the `RestrictEndpoint` middleware reads the id from the session
**once**, puts it in the request `context`, and handlers read it from there.

```
login handler ──Put("authenticatedUserID", id)──► [scs session]
                                                        │
request → LoadAndSave → RestrictEndpoint ──GetInt──► ctx.WithValue(userID) → handler
                          (has the SessionManager)                            (reads ctx, knows nothing about scs)
```

This works because `LoadAndSave` hangs the scs session off `r.Context()`, so
inside the middleware `sm.GetInt(r.Context(), "authenticatedUserID")` sees the
value stored at login.

---

## Implementation steps

### 1. `internal/security` — context accessor + injection

Add an unexported context key and an exported accessor:

```go
type contextKey string

const userIDKey contextKey = "userID"

// UserID returns the authenticated user id placed in the context by
// RestrictEndpoint, and whether it was present.
func UserID(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(userIDKey).(int)
	return id, ok
}
```

Inject it inside `RestrictEndpoint`, right after the auth check passes:

```go
userID := app.SessionManager.GetInt(r.Context(), "authenticatedUserID")
ctx := context.WithValue(r.Context(), userIDKey, userID)
// ...
next.ServeHTTP(w, r.WithContext(ctx))   // pass the enriched context
```

### 2. Domain handlers — drop the hardcoded const

Replace `currentUserID` with the value from the context in each protected
handler:

```go
// before:  f.UserID = currentUserID
userID, _ := security.UserID(r.Context())
f.UserID = userID
```

Touch points (where `currentUserID` is used today):
- `internal/analytics/handler.go` — `parseFilter` sets `Filter.UserID`.
- `internal/balance/handler.go` — `get` / `periods` pass the id to the service.
- `internal/movement/handler.go` — `create`, `list`, `expenses`, `get`,
  `update`, `delete`, `summary`.

Then delete the `const currentUserID = 1` from each handler.

### 3. Safety / edge cases

- These handlers are only mounted **behind** `RestrictEndpoint`, so the id is
  always present. Still, decide the fallback if `UserID` returns `ok == false`
  (programming error = a route escaped the protected group): respond `401`
  (defensive) rather than silently querying with id `0`.
- Keep `POST /user` and `POST /user/login` **public** (outside the protected
  group) — they must work without a session.

---

## After this change

- A second user logging in gets their own `authenticatedUserID`; every query is
  scoped to it, so cross-user data cannot leak.
- The `scs` dependency stays confined to the login handler and the middleware —
  the domain layers never import it.

## Related follow-ups (not part of this change)

- **CORS + cookies:** `Allow-Origin: *` does not work with credentials. Once a
  browser frontend is involved, pin explicit origins and set
  `Allow-Credentials: true`.
- **Pre-existing balance test failures** (`TestGetPeriods_SinglePeriodSurplus`,
  `TestGetPeriods_RegisteredIncomesIncluded`) — unrelated to this work, worth a
  separate look.
- See [[PLAN_movements_by_user]] for the broader ownership model and the auth
  strategy this builds on.
