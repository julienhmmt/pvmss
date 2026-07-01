# Plan 004: Add table-driven tests for the auth handler (login, exchange, refresh, proxmox-admin-login)

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report. When done, update the status row in `docs/plans/advisor/README.md`.
>
> **Drift check (run first)**: `git diff --stat d427838..HEAD -- backend/api/v1/auth.go backend/api/v1/admin_db_test.go`
> If `auth.go` changed since this plan was written, compare the "Current state"
> excerpts against live code before proceeding; on mismatch, treat as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: L
- **Risk**: HIGH (security-critical path; tests must not weaken auth checks — they characterize and lock down current behavior)
- **Depends on**: plan 003 (so CI runs the new tests) — can start without it, but CI verification needs 003
- **Category**: tests
- **Planned at**: commit `d427838`, 2026-07-01

## Why this matters

`backend/api/v1/auth.go` is the entire authentication surface — admin bcrypt login, Proxmox user credential verification, JWT exchange/refresh, PVE-realm admin login with role checks. It has **zero tests**. This is the highest-risk untested code in the repo: a regression here means broken authn or privilege escalation. Plan 001 (security Phase A) changes auth-adjacent validation; these characterization tests must exist first/alongside so the security fixes don't silently shift auth behavior.

## Current state

**Handler structure (`backend/api/v1/auth.go`):**
- `AuthHandler` struct (line 30) holds `state state.StateManager` and `jwtSecret string`. Constructor `MakeAuthHandler(s, jwtSecret)` (line 37).
- `Login` (line 45): `req.Admin` true → bcrypt compare vs `h.state.GetEnvConfig().AdminPasswordHash` (line 64-69); false → `verifyProxmoxCredentials(...)` (line 76) which fails in offline mode (`errOffline`). On success calls `issueTokens` + `writeJSON(AuthResponse)`.
- `Exchange` (line 97): reads `access_token` cookie, `jwt.ParseWithClaims` with alg-confusion guard (line 112: `*jwt.SigningMethodHMAC` check), returns `AuthResponse` or 401.
- `Refresh` (line 127): reads `refresh_token` cookie, validates, issues new access token via `setTokenCookie`.
- `ProxmoxAdminLogin` (line 176): `proxmox.MakeRestyClientCookieAuthFromEnvConfig` + `CreateTicketResty` with realm "pve"; checks `HasRole(cap, "PVEAdmin") || HasRole(cap, "PVMSS_Admin")` (line 212); issues admin tokens.
- `Me` (line 157): reads from ctx (set by JWTMiddleware).
- `Logout` (line 165): clears both cookies.
- `setTokenCookie` (line 235): mints HS256 JWT, sets HttpOnly/SameSite=Strict cookie. Note line 245 `signed, _ := tok.SignedString(...)` ignores error (security plan S10, Phase B — not in scope here, but a test should document current behavior).
- Constants (lines 22-27): `accessTokenTTL = 15*time.Minute`, `refreshTokenTTL = 7*24*time.Hour`.

**Existing test pattern to mimic (`backend/api/v1/admin_db_test.go`):**
- `package apiv1_test` (black-box).
- Helper `newAdminDBTestHandlerAndDB(t)` (line 24): `database.OpenMemory()`, `db.CompleteBootstrap("test")`, `state.MakeAppStateWithDB(db)`, `sm.LoadSettingsFromDB()`, returns handler + db. `t.Cleanup(func(){ _ = db.Close() })`.
- Tests use `httptest.NewRequest(method, path, body)` + `httptest.NewRecorder()`, call handler methods directly (bypass JWT middleware), assert with `testify/assert` + `require`.
- `usernameFromCtx` returns `""` when the context key is absent (line 22-23 comment) — so handlers can be called without injecting auth context.

**Auth handler construction for tests:** `MakeAuthHandler(sm, jwtSecret)` — needs a `state.StateManager`. Reuse `state.MakeAppStateWithDB(db)` from the admin_db pattern. For offline mode (most tests), `sm.IsOfflineMode()` is true when `PVMSS_OFFLINE=true` (set by the test env). For the admin-login path, set `sm`'s `AdminPasswordHash` via the env config.

## Commands you will need

| Purpose   | Command                                                                  | Expected on success |
|-----------|--------------------------------------------------------------------------|---------------------|
| Test (filtered) | `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -run 'TestAuth' ./api/v1/ -v -race` | all pass, no races |
| Test (full) | `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -timeout=5m ./...` | all pass |
| Lint      | `cd backend && golangci-lint run --timeout=3m`                           | exit 0              |

## Scope

**In scope**:
- `backend/api/v1/auth_test.go` (create, `package apiv1_test`)

**Out of scope**:
- `backend/api/v1/auth.go` source (do NOT fix S10 SignedString here — that's security plan 001/Phase B; a test may assert current behavior and add a `// TODO S10` comment, but no source change)
- `backend/security/` middleware (separate test effort)
- Frontend auth flows

## Git workflow

- Branch: `advisor/004-auth-tests`
- Commit: `test(auth): add table-driven tests for login/exchange/refresh/proxmox-admin-login`
- Do NOT push unless instructed.

## Steps

### Step 1: Create the test scaffold and helper

Create `backend/api/v1/auth_test.go` with `package apiv1_test`. Add a helper modeled on `newAdminDBTestHandlerAndDB`:
```go
func newAuthTestHandler(t *testing.T, adminHash string) (*apiv1.AuthHandler, state.StateManager) {
    t.Helper()
    db, err := database.OpenMemory()
    require.NoError(t, err)
    t.Cleanup(func() { _ = db.Close() })
    require.NoError(t, db.CompleteBootstrap("test"))
    sm := state.MakeAppStateWithDB(db)
    require.NoError(t, sm.LoadSettingsFromDB())
    // set AdminPasswordHash on the env config for admin-login tests:
    // (use the same mechanism the existing tests use to mutate env config;
    //  if MakeAppStateWithDB reads env at construction, set the env var
    //  BEFORE building sm, or use a setter if StateManager exposes one)
    return apiv1.MakeAuthHandler(sm, testJWTSecret), sm
}
const testJWTSecret = "test-jwt-secret-must-be-at-least-32-bytes!!"
```
If `MakeAppStateWithDB` reads `env.LoadAndValidate()` at construction (requiring real env vars), set the needed env vars in the test or in a `TestMain`. Check how `admin_db_test.go` constructs `sm` — it calls `state.MakeAppStateWithDB(db)` directly without env vars, suggesting it builds a minimal offline state. Confirm and match that exactly.

**Verify**: `cd backend && go vet ./api/v1/` → exit 0 (compiles).

### Step 2: Test Login — admin path (bcrypt)

Table-driven `TestLogin_Admin_<Scenario>`:
- valid admin password → 200, `AuthResponse.IsAdmin == true`, access_token cookie set.
- invalid password → 401.
- empty username or password → 400.
- `Admin=true` but `AdminPasswordHash` empty → 401 (or errNotConfigured — verify actual behavior).

Use a real bcrypt hash for the "valid" case: generate one in the test with `bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.MinCost)` to keep it fast.

**Verify**: `go test -run TestLogin_Admin -v ./api/v1/` → all subtests pass.

### Step 3: Test Login — Proxmox user path (offline)

`TestLogin_ProxmoxUser_OfflineReturnsError`:
- `Admin=false`, offline mode → handler returns `errOffline` (verify status code matches `errOffline`'s implementation).

(Skipping live Proxmox verification — that's online-test territory, out of scope.)

**Verify**: test passes; documents that offline mode refuses user login.

### Step 4: Test Exchange and Refresh

`TestExchange_<Scenario>`:
- valid access token cookie → 200, returns correct `AuthResponse`.
- missing cookie → 401.
- tampered/invalid token → 401.
- token signed with wrong secret → 401 (alg-confusion / wrong key).
- token with `*jwt.SigningMethodHMAC` mismatch (e.g. RS256) → 401.

`TestRefresh_<Scenario>`:
- valid refresh cookie → 200, new access_token cookie set (different from the refresh cookie).
- missing refresh cookie → 401.
- expired/invalid refresh token → 401.

To mint test tokens, use `jwt.NewWithClaims(jwt.SigningMethodHS256, apiv1.JWTClaims{...})` + `tok.SignedString([]byte(testJWTSecret))`. Note: `JWTClaims` must be accessible from `apiv1_test` — confirm it's exported (it is, used in `auth.go:236`). If `setTokenCookie` is unexported, mint tokens directly in the test.

**Verify**: `go test -run 'TestExchange|TestRefresh' -v ./api/v1/` → all pass.

### Step 5: Test ProxmoxAdminLogin (offline behavior)

`TestProxmoxAdminLogin_Offline_<Scenario>`:
- offline mode → either `errOffline` or a client-construction error (verify actual behavior; `MakeRestyClientCookieAuthFromEnvConfig` likely fails offline). Assert the documented behavior.
- empty username/password → 400.

(Live PVE realm auth is online-test territory — out of scope. Focus on input validation + offline refusal.)

**Verify**: `go test -run TestProxmoxAdminLogin -v ./api/v1/` → passes.

### Step 6: Test Logout and Me

- `TestLogout_ClearsCookies`: response sets both cookies with `MaxAge: -1`.
- `TestMe_RequiresContext`: calling `Me` without JWTMiddleware ctx returns `""`/`false` (documents the contract that middleware must run first).

**Verify**: `go test -run 'TestLogout|TestMe' -v ./api/v1/` → passes.

## Test plan

- All tests above (Steps 2-6) pass with `-race`.
- No mutation of `auth.go` — these are characterization tests.
- Verify: `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -timeout=5m ./...` → all pass.

## Done criteria

- [ ] `backend/api/v1/auth_test.go` exists with `package apiv1_test`
- [ ] Tests cover: Login admin (valid/invalid/empty), Login Proxmox offline, Exchange (valid/missing/tampered/wrong-secret/wrong-alg), Refresh (valid/missing/invalid), ProxmoxAdminLogin (offline/empty-input), Logout, Me
- [ ] `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -race -run 'TestAuth|TestLogin|TestExchange|TestRefresh|TestProxmoxAdminLogin|TestLogout|TestMe' ./api/v1/` exits 0
- [ ] `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -timeout=5m ./...` exits 0 (no regression)
- [ ] `cd backend && golangci-lint run --timeout=3m` exits 0
- [ ] `auth.go` is unmodified (`git diff backend/api/v1/auth.go` empty)
- [ ] `docs/plans/advisor/README.md` status row updated

## STOP conditions

Stop and report if:
- `JWTClaims` is not exported or `MakeAuthHandler` signature differs from `auth.go:37` (drift).
- `state.MakeAppStateWithDB` requires real env vars / fails to construct in-memory (the admin_db_test pattern must work; if auth tests need a different state setup, report the required constructor).
- A test reveals an actual auth BUG (e.g. Exchange accepts a wrong-secret token) — do NOT encode buggy behavior as expected; report it as a finding so it can be fixed first.
- `setTokenCookie`'s ignored error (S10) causes test flakiness — report; do not fix in this plan.

## Maintenance notes

- When security plan S10 lands (handle `SignedString` error), update the Login/Refresh tests to assert the error path returns 500 instead of silently setting an empty cookie.
- When S11 lands (refresh TTL 7d → 48h), update any test that asserts the refresh cookie MaxAge.
- These tests intentionally bypass JWTMiddleware; a future plan should add middleware-level tests in `backend/security/` for the full chain.
