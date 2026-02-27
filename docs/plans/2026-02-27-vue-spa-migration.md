# Vue 3 SPA Migration — Phase 1 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a clean `/api/v1/` JSON backend layer with JWT auth and deliver `VmCard` + `VmActionButtons` as Vue 3 + TypeScript components, while leaving all existing templ handlers completely untouched.

**Architecture:** A new `backend/api/v1/` package registers JWT-authenticated JSON endpoints on the existing `httprouter`. Existing session-based templ handlers share the same `StateManager` but are independent. The Vue SPA is built with Vite in `frontend/src/`, output to `frontend/dist/`, served by Go at `/assets/*`. The templ layout gets a `<div id="vue-app">` mount point, and a small script exchanges the existing session for JWT cookies on page load.

**Tech Stack:** Go `golang-jwt/jwt/v5`, Vue 3, TypeScript, Vite, Pinia, Tailwind CSS v4, Axios. Existing: `proxmox.VMActionResty`, `proxmox.GetVMsResty`, `proxmox.NewRestyClientFromEnv`.

---

## Task 1: Add JWT dependency to Go module

**Files:**
- Modify: `backend/go.mod`

**Step 1: Add the dependency**

```bash
cd backend && go get github.com/golang-jwt/jwt/v5
```

**Step 2: Verify it appears in go.mod**

```bash
grep "golang-jwt" backend/go.mod
```
Expected: `github.com/golang-jwt/jwt/v5 v5.x.x`

**Step 3: Run tests to confirm nothing broke**

```bash
make test-offline
```
Expected: all PASS

**Step 4: Commit**

```bash
git add backend/go.mod backend/go.sum
git commit -m "chore: add golang-jwt/jwt/v5 dependency"
```

---

## Task 2: Add JWT_SECRET validation to security package

**Files:**
- Modify: `backend/security/validation.go` (around line 22 — the `required` map)

**Step 1: Write the failing test**

Add to `backend/tests/security_test.go` (or create `backend/security/validation_test.go`):

```go
func TestJWTSecretRequired(t *testing.T) {
    // JWT_SECRET must be present and >= 32 bytes in production
    // In test mode (GO_TEST_ENVIRONMENT=1) it's a warning, not fatal
    // This test just verifies the validation function runs without panic
    t.Setenv("GO_TEST_ENVIRONMENT", "1")
    t.Setenv("SESSION_SECRET", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
    t.Setenv("ADMIN_PASSWORD_HASH", "$2y$10$Ppg7Wl3sNYrmxZmWgcq4reOyznt7AeqMrQucaH4HY.dBrzavhPP1e")
    err := ValidateRequiredEnvVars()
    // In test mode, missing JWT_SECRET is a warning, not an error
    _ = err
}
```

**Step 2: Add JWT_SECRET to the validation logic**

In `backend/security/validation.go`, after line 26 (`"ADMIN_PASSWORD_HASH": "Admin password hash (bcrypt)"`), add JWT_SECRET to the same `required` map and add a length check:

```go
// In the required map (same block as SESSION_SECRET):
"JWT_SECRET": "JWT signing key (32+ bytes)",
```

Then after the SESSION_SECRET length check (around line 52), add:

```go
// Validate JWT_SECRET length (minimum 32 bytes)
jwtSecret := os.Getenv("JWT_SECRET")
if jwtSecret != "" && len(jwtSecret) < 32 {
    if isTest {
        log.Warn().Int("length", len(jwtSecret)).Msg("JWT_SECRET too short in test mode; continuing")
    } else {
        return fmt.Errorf("JWT_SECRET must be at least 32 characters long")
    }
}
```

**Step 3: Run tests**

```bash
make test-offline
```
Expected: all PASS (JWT_SECRET check is lenient in test mode)

**Step 4: Update example.env and CLAUDE.md to document JWT_SECRET**

In `example.env`, add:
```
JWT_SECRET=changeme_use_at_least_32_random_bytes_here
```

In `CLAUDE.md`, add `JWT_SECRET` row to the env vars table.

**Step 5: Commit**

```bash
git add backend/security/validation.go example.env CLAUDE.md
git commit -m "feat(security): require JWT_SECRET env var for API auth"
```

---

## Task 3: Create `backend/api/v1/types.go`

**Files:**
- Create: `backend/api/v1/types.go`

**Step 1: Create the file**

```go
package apiv1

// LoginRequest is the body for POST /api/v1/auth/login
type LoginRequest struct {
    Username string `json:"username"`
    Password string `json:"password"`
    Admin    bool   `json:"admin"` // true → check ADMIN_PASSWORD_HASH
}

// AuthResponse is returned after a successful login or exchange
type AuthResponse struct {
    Username string `json:"username"`
    IsAdmin  bool   `json:"is_admin"`
}

// MeResponse is returned by GET /api/v1/auth/me
type MeResponse struct {
    Username string `json:"username"`
    IsAdmin  bool   `json:"is_admin"`
}

// VMSummary is the per-VM data returned in list and detail endpoints.
// Uses int for VMID (Proxmox always returns numeric IDs).
type VMSummary struct {
    VMID     int     `json:"vmid"`
    Name     string  `json:"name"`
    Node     string  `json:"node"`
    Status   string  `json:"status"`
    CPU      float64 `json:"cpu"`      // fraction 0..1
    CPUs     int     `json:"cpus"`
    MemMB    int64   `json:"mem_mb"`   // bytes → displayed as MB by frontend
    MaxMemMB int64   `json:"max_mem_mb"`
    DiskMB   int64   `json:"disk_mb"`
    Uptime   int64   `json:"uptime"`   // seconds
    Tags     string  `json:"tags"`     // semicolon-separated
}

// VMListResponse wraps a slice of VMSummary
type VMListResponse struct {
    VMs   []VMSummary `json:"vms"`
    Total int         `json:"total"`
}

// VMActionRequest is the body for POST /api/v1/vms/:id/action
type VMActionRequest struct {
    Action string `json:"action"` // start|stop|shutdown|reboot|reset
    Node   string `json:"node"`
}

// VMActionResponse is returned after executing a VM action
type VMActionResponse struct {
    Success bool   `json:"success"`
    TaskID  string `json:"task_id,omitempty"` // Proxmox UPID when async
    Message string `json:"message,omitempty"`
}

// ErrorResponse is the standard JSON error body
type ErrorResponse struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

// contextKey is an unexported type for context keys in this package
type contextKey string

const (
    contextKeyUsername contextKey = "username"
    contextKeyIsAdmin  contextKey = "is_admin"
)
```

**Step 2: Verify it compiles**

```bash
cd backend && go build ./api/...
```
Expected: no errors

**Step 3: Commit**

```bash
git add backend/api/v1/types.go
git commit -m "feat(api/v1): add request/response types"
```

---

## Task 4: Create `backend/api/v1/errors.go`

**Files:**
- Create: `backend/api/v1/errors.go`

**Step 1: Create the file**

```go
package apiv1

import (
    "encoding/json"
    "net/http"
)

// writeError writes a JSON error response with the given HTTP status code.
func writeError(w http.ResponseWriter, status int, code, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(ErrorResponse{Code: code, Message: message})
}

// writeJSON writes any value as a JSON response with HTTP 200.
func writeJSON(w http.ResponseWriter, v interface{}) {
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(v)
}

// Common error responses
func errUnauthorized(w http.ResponseWriter)  { writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required") }
func errForbidden(w http.ResponseWriter)     { writeError(w, http.StatusForbidden, "forbidden", "Insufficient permissions") }
func errBadRequest(w http.ResponseWriter, msg string) { writeError(w, http.StatusBadRequest, "bad_request", msg) }
func errInternal(w http.ResponseWriter)      { writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error") }
func errOffline(w http.ResponseWriter)       { writeError(w, http.StatusServiceUnavailable, "proxmox_offline", "Proxmox is unavailable") }
```

**Step 2: Compile**

```bash
cd backend && go build ./api/...
```

**Step 3: Commit**

```bash
git add backend/api/v1/errors.go
git commit -m "feat(api/v1): add JSON error helpers"
```

---

## Task 5: Create `backend/api/v1/middleware.go` — JWT middleware

**Files:**
- Create: `backend/api/v1/middleware.go`
- Create: `backend/api/v1/middleware_test.go`

**Step 1: Write the failing test**

```go
// backend/api/v1/middleware_test.go
package apiv1

import (
    "context"
    "net/http"
    "net/http/httptest"
    "os"
    "testing"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

func TestJWTMiddleware_MissingCookie(t *testing.T) {
    t.Setenv("JWT_SECRET", "testsecretthatis32byteslongexact!!")
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })
    handler := JWTMiddleware(next)
    req := httptest.NewRequest(http.MethodGet, "/api/v1/vms", nil)
    rr := httptest.NewRecorder()
    handler.ServeHTTP(rr, req)
    if rr.Code != http.StatusUnauthorized {
        t.Errorf("expected 401, got %d", rr.Code)
    }
}

func TestJWTMiddleware_ValidToken(t *testing.T) {
    secret := "testsecretthatis32byteslongexact!!"
    t.Setenv("JWT_SECRET", secret)

    // Issue a valid token
    claims := JWTClaims{
        Username: "testuser",
        IsAdmin:  false,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    signed, err := token.SignedString([]byte(secret))
    if err != nil {
        t.Fatal(err)
    }

    var capturedUsername string
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        capturedUsername = r.Context().Value(contextKeyUsername).(string)
        w.WriteHeader(http.StatusOK)
    })
    handler := JWTMiddleware(next)

    req := httptest.NewRequest(http.MethodGet, "/api/v1/vms", nil)
    req.AddCookie(&http.Cookie{Name: "access_token", Value: signed})
    rr := httptest.NewRecorder()
    handler.ServeHTTP(rr, req)

    if rr.Code != http.StatusOK {
        t.Errorf("expected 200, got %d", rr.Code)
    }
    if capturedUsername != "testuser" {
        t.Errorf("expected username 'testuser', got '%s'", capturedUsername)
    }
}

func TestJWTMiddleware_ExpiredToken(t *testing.T) {
    secret := "testsecretthatis32byteslongexact!!"
    t.Setenv("JWT_SECRET", secret)

    claims := JWTClaims{
        Username: "testuser",
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)), // expired
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    signed, _ := token.SignedString([]byte(secret))

    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })
    handler := JWTMiddleware(next)

    req := httptest.NewRequest(http.MethodGet, "/api/v1/vms", nil)
    req.AddCookie(&http.Cookie{Name: "access_token", Value: signed})
    rr := httptest.NewRecorder()
    handler.ServeHTTP(rr, req)
    if rr.Code != http.StatusUnauthorized {
        t.Errorf("expected 401, got %d", rr.Code)
    }
}
```

**Step 2: Run to verify it fails**

```bash
cd backend && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -v -run TestJWTMiddleware ./api/v1/...
```
Expected: compile error (JWTMiddleware, JWTClaims not defined)

**Step 3: Implement `middleware.go`**

```go
// backend/api/v1/middleware.go
package apiv1

import (
    "context"
    "net/http"
    "os"

    "github.com/golang-jwt/jwt/v5"
    "github.com/julienschmidt/httprouter"
)

// JWTClaims are the custom claims embedded in every access token.
type JWTClaims struct {
    Username string `json:"username"`
    IsAdmin  bool   `json:"is_admin"`
    jwt.RegisteredClaims
}

// jwtSecret reads JWT_SECRET from the environment.
// Called at request time (not startup) so tests can set it via t.Setenv.
func jwtSecret() []byte {
    return []byte(os.Getenv("JWT_SECRET"))
}

// JWTMiddleware validates the access_token cookie and injects username + is_admin into context.
func JWTMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        cookie, err := r.Cookie("access_token")
        if err != nil {
            errUnauthorized(w)
            return
        }

        claims := &JWTClaims{}
        token, err := jwt.ParseWithClaims(cookie.Value, claims, func(t *jwt.Token) (interface{}, error) {
            if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, jwt.ErrSignatureInvalid
            }
            return jwtSecret(), nil
        })
        if err != nil || !token.Valid {
            errUnauthorized(w)
            return
        }

        ctx := context.WithValue(r.Context(), contextKeyUsername, claims.Username)
        ctx = context.WithValue(ctx, contextKeyIsAdmin, claims.IsAdmin)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// JWTAdminMiddleware wraps JWTMiddleware and additionally requires is_admin=true.
func JWTAdminMiddleware(next http.Handler) http.Handler {
    return JWTMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !r.Context().Value(contextKeyIsAdmin).(bool) {
            errForbidden(w)
            return
        }
        next.ServeHTTP(w, r)
    }))
}

// httprouterWrap adapts an http.Handler to httprouter.Handle
func httprouterWrap(h http.Handler) httprouter.Handle {
    return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
        // Inject httprouter params into context for handlers that need them
        ctx := context.WithValue(r.Context(), httprouter.ParamsKey, ps)
        h.ServeHTTP(w, r.WithContext(ctx))
    }
}

// usernameFromCtx extracts username from the request context (set by JWTMiddleware).
func usernameFromCtx(r *http.Request) string {
    v, _ := r.Context().Value(contextKeyUsername).(string)
    return v
}

// isAdminFromCtx extracts the is_admin flag from the request context.
func isAdminFromCtx(r *http.Request) bool {
    v, _ := r.Context().Value(contextKeyIsAdmin).(bool)
    return v
}
```

**Step 4: Run tests**

```bash
cd backend && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -v -run TestJWTMiddleware ./api/v1/...
```
Expected: all PASS

**Step 5: Commit**

```bash
git add backend/api/v1/middleware.go backend/api/v1/middleware_test.go
git commit -m "feat(api/v1): add JWT middleware with context injection"
```

---

## Task 6: Create `backend/api/v1/auth.go` — login, exchange, refresh, me, logout

**Files:**
- Create: `backend/api/v1/auth.go`
- Create: `backend/api/v1/auth_test.go`

**Step 1: Write tests**

```go
// backend/api/v1/auth_test.go
package apiv1

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "os"
    "testing"

    "pvmss/state"
)

func TestLoginAdmin_WrongPassword(t *testing.T) {
    t.Setenv("JWT_SECRET", "testsecretthatis32byteslongexact!!")
    t.Setenv("ADMIN_PASSWORD_HASH", "$2y$10$Ppg7Wl3sNYrmxZmWgcq4reOyznt7AeqMrQucaH4HY.dBrzavhPP1e")

    h := NewAuthHandler(nil) // no state needed for admin auth
    body, _ := json.Marshal(LoginRequest{Username: "admin", Password: "wrongpassword", Admin: true})
    req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    rr := httptest.NewRecorder()
    h.Login(rr, req)
    if rr.Code != http.StatusUnauthorized {
        t.Errorf("expected 401, got %d", rr.Code)
    }
}

func TestLogout_ClearsCookies(t *testing.T) {
    t.Setenv("JWT_SECRET", "testsecretthatis32byteslongexact!!")
    h := NewAuthHandler(nil)
    req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
    rr := httptest.NewRecorder()
    h.Logout(rr, req)
    if rr.Code != http.StatusOK {
        t.Errorf("expected 200, got %d", rr.Code)
    }
    cookies := rr.Result().Cookies()
    var foundAccess, foundRefresh bool
    for _, c := range cookies {
        if c.Name == "access_token" && c.MaxAge < 0 {
            foundAccess = true
        }
        if c.Name == "refresh_token" && c.MaxAge < 0 {
            foundRefresh = true
        }
    }
    if !foundAccess || !foundRefresh {
        t.Error("expected both token cookies to be cleared")
    }
}
```

**Step 2: Run to verify they fail**

```bash
cd backend && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -v -run "TestLogin|TestLogout" ./api/v1/...
```
Expected: compile error

**Step 3: Implement `auth.go`**

```go
// backend/api/v1/auth.go
package apiv1

import (
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "os"
    "strings"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "golang.org/x/crypto/bcrypt"

    "pvmss/logger"
    "pvmss/proxmox"
    "pvmss/state"
)

const (
    accessTokenCookie  = "access_token"
    refreshTokenCookie = "refresh_token"
    accessTokenTTL     = 15 * time.Minute
    refreshTokenTTL    = 7 * 24 * time.Hour
)

// AuthHandler handles JWT authentication endpoints.
type AuthHandler struct {
    state state.StateManager
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(s state.StateManager) *AuthHandler {
    return &AuthHandler{state: s}
}

// Login handles POST /api/v1/auth/login.
// Body: {"username":"...", "password":"...", "admin": true/false}
// If admin=true → bcrypt check against ADMIN_PASSWORD_HASH env var.
// If admin=false → verify via Proxmox /access/ticket.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
    var req LoginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        errBadRequest(w, "invalid JSON body")
        return
    }
    if req.Username == "" || req.Password == "" {
        errBadRequest(w, "username and password are required")
        return
    }

    var isAdmin bool

    if req.Admin {
        // Admin login: check bcrypt
        hash := os.Getenv("ADMIN_PASSWORD_HASH")
        if hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
            logger.SecurityEvent("api_auth_admin_fail").Str("username", req.Username).Msg("Invalid admin credentials")
            errUnauthorized(w)
            return
        }
        isAdmin = true
    } else {
        // User login: verify via Proxmox ticket API
        if h.state != nil && h.state.IsOfflineMode() {
            errOffline(w)
            return
        }
        if err := verifyProxmoxCredentials(req.Username, req.Password); err != nil {
            logger.SecurityEvent("api_auth_user_fail").Str("username", req.Username).Err(err).Msg("Invalid Proxmox credentials")
            errUnauthorized(w)
            return
        }
        isAdmin = false
    }

    if err := issueTokens(w, req.Username, isAdmin); err != nil {
        logger.Get().Error().Err(err).Msg("Failed to issue JWT tokens")
        errInternal(w)
        return
    }

    logger.AuthEvent("api_login").Str("username", req.Username).Bool("is_admin", isAdmin).Msg("API login successful")
    writeJSON(w, AuthResponse{Username: req.Username, IsAdmin: isAdmin})
}

// Exchange handles POST /api/v1/auth/exchange.
// Reads the SCS session cookie, validates it, and issues JWT tokens.
// Used by the Vue app on load to get JWT tokens when the user already has a session.
func (h *AuthHandler) Exchange(w http.ResponseWriter, r *http.Request) {
    if h.state == nil {
        errInternal(w)
        return
    }
    sm := h.state.GetSessionManager()
    if sm == nil {
        errInternal(w)
        return
    }

    username, ok := sm.Get(r.Context(), "username").(string)
    if !ok || username == "" {
        errUnauthorized(w)
        return
    }
    isAdmin, _ := sm.Get(r.Context(), "is_admin").(bool)

    if err := issueTokens(w, username, isAdmin); err != nil {
        errInternal(w)
        return
    }
    writeJSON(w, AuthResponse{Username: username, IsAdmin: isAdmin})
}

// Refresh handles POST /api/v1/auth/refresh.
// Reads the refresh_token cookie and issues a new access_token.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
    cookie, err := r.Cookie(refreshTokenCookie)
    if err != nil {
        errUnauthorized(w)
        return
    }

    claims := &JWTClaims{}
    token, err := jwt.ParseWithClaims(cookie.Value, claims, func(t *jwt.Token) (interface{}, error) {
        if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, jwt.ErrSignatureInvalid
        }
        return jwtSecret(), nil
    })
    if err != nil || !token.Valid {
        errUnauthorized(w)
        return
    }

    // Issue fresh access token only
    setTokenCookie(w, accessTokenCookie, claims.Username, claims.IsAdmin, accessTokenTTL)
    writeJSON(w, AuthResponse{Username: claims.Username, IsAdmin: claims.IsAdmin})
}

// Me handles GET /api/v1/auth/me. Requires JWTMiddleware upstream.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, MeResponse{
        Username: usernameFromCtx(r),
        IsAdmin:  isAdminFromCtx(r),
    })
}

// Logout handles POST /api/v1/auth/logout. Clears both token cookies.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
    http.SetCookie(w, &http.Cookie{Name: accessTokenCookie, Value: "", MaxAge: -1, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
    http.SetCookie(w, &http.Cookie{Name: refreshTokenCookie, Value: "", MaxAge: -1, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
    writeJSON(w, map[string]bool{"ok": true})
}

// issueTokens creates and sets both access_token and refresh_token cookies.
func issueTokens(w http.ResponseWriter, username string, isAdmin bool) error {
    setTokenCookie(w, accessTokenCookie, username, isAdmin, accessTokenTTL)
    setTokenCookie(w, refreshTokenCookie, username, isAdmin, refreshTokenTTL)
    return nil
}

// setTokenCookie mints a signed JWT and writes it as an HttpOnly cookie.
func setTokenCookie(w http.ResponseWriter, name, username string, isAdmin bool, ttl time.Duration) {
    claims := JWTClaims{
        Username: username,
        IsAdmin:  isAdmin,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    signed, _ := token.SignedString(jwtSecret())

    http.SetCookie(w, &http.Cookie{
        Name:     name,
        Value:    signed,
        Path:     "/",
        MaxAge:   int(ttl.Seconds()),
        HttpOnly: true,
        Secure:   os.Getenv("PVMSS_ENV") == "production" || os.Getenv("PVMSS_ENV") == "prod",
        SameSite: http.SameSiteStrictMode,
    })
}

// verifyProxmoxCredentials POSTs to /access/ticket to confirm user credentials.
// Returns nil if credentials are valid, error otherwise.
func verifyProxmoxCredentials(username, password string) error {
    resty, err := proxmox.NewRestyClientFromEnv(10 * time.Second)
    if err != nil {
        return fmt.Errorf("no proxmox client: %w", err)
    }

    // Ensure username has realm suffix
    if !strings.Contains(username, "@") {
        username = username + "@pve"
    }

    values := url.Values{}
    values.Set("username", username)
    values.Set("password", password)

    var response struct {
        Data struct {
            Ticket string `json:"ticket"`
        } `json:"data"`
    }
    if err := resty.Post(r.Context(), "/access/ticket", values, &response); err != nil || response.Data.Ticket == "" {
        return fmt.Errorf("proxmox auth failed")
    }
    return nil
}
```

> **Note:** `verifyProxmoxCredentials` above has a bug — it uses `r.Context()` which doesn't exist in that scope. Fix it to use `context.Background()` or accept a context parameter. The corrected version:

```go
// verifyProxmoxCredentials POSTs to /access/ticket to confirm user credentials.
func verifyProxmoxCredentials(username, password string) error {
    restyClient, err := proxmox.NewRestyClientFromEnv(10 * time.Second)
    if err != nil {
        return fmt.Errorf("no proxmox client: %w", err)
    }
    if !strings.Contains(username, "@") {
        username = username + "@pve"
    }
    values := url.Values{}
    values.Set("username", username)
    values.Set("password", password)
    var resp struct {
        Data struct{ Ticket string `json:"ticket"` } `json:"data"`
    }
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    if err := restyClient.Post(ctx, "/access/ticket", values, &resp); err != nil || resp.Data.Ticket == "" {
        return fmt.Errorf("proxmox authentication failed")
    }
    return nil
}
```

Add `"context"` to the import block.

**Step 4: Run tests**

```bash
cd backend && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -v -run "TestLogin|TestLogout" ./api/v1/...
```
Expected: PASS

**Step 5: Commit**

```bash
git add backend/api/v1/auth.go backend/api/v1/auth_test.go
git commit -m "feat(api/v1): add JWT auth endpoints (login, exchange, refresh, me, logout)"
```

---

## Task 7: Create `backend/api/v1/vms.go`

**Files:**
- Create: `backend/api/v1/vms.go`
- Create: `backend/api/v1/vms_test.go`

**Step 1: Write the failing test**

```go
// backend/api/v1/vms_test.go
package apiv1

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

// makeAuthedRequest creates a request with a valid JWT access_token cookie
func makeAuthedRequest(method, path string) *http.Request {
    secret := "testsecretthatis32byteslongexact!!"
    claims := JWTClaims{
        Username: "testuser",
        IsAdmin:  false,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    signed, _ := token.SignedString([]byte(secret))
    req := httptest.NewRequest(method, path, nil)
    req.AddCookie(&http.Cookie{Name: "access_token", Value: signed})
    return req
}

func TestListVMs_OfflineMode(t *testing.T) {
    t.Setenv("JWT_SECRET", "testsecretthatis32byteslongexact!!")
    t.Setenv("PVMSS_OFFLINE", "true")

    h := NewVMHandler(nil) // nil state → offline
    req := makeAuthedRequest(http.MethodGet, "/api/v1/vms")
    rr := httptest.NewRecorder()

    // Run through JWT middleware first
    JWTMiddleware(http.HandlerFunc(h.ListVMs)).ServeHTTP(rr, req)

    // In offline mode, returns empty list (not an error)
    if rr.Code != http.StatusOK {
        t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
    }
}
```

**Step 2: Run to verify it fails**

```bash
cd backend && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -v -run TestListVMs ./api/v1/...
```
Expected: compile error (VMHandler not defined)

**Step 3: Implement `vms.go`**

```go
// backend/api/v1/vms.go
package apiv1

import (
    "context"
    "net/http"
    "os"
    "strconv"
    "strings"
    "time"

    "github.com/julienschmidt/httprouter"

    "pvmss/logger"
    "pvmss/proxmox"
    "pvmss/state"
)

// VMHandler handles VM listing and detail endpoints.
type VMHandler struct {
    state state.StateManager
}

// NewVMHandler creates a new VMHandler.
func NewVMHandler(s state.StateManager) *VMHandler {
    return &VMHandler{state: s}
}

// isOffline returns true when PVMSS_OFFLINE=true or state is nil.
func (h *VMHandler) isOffline() bool {
    return strings.EqualFold(os.Getenv("PVMSS_OFFLINE"), "true") ||
        h.state == nil || h.state.IsOfflineMode()
}

// restyClient creates a fresh Resty client from env vars with a 30s timeout.
func restyClient() (*proxmox.RestyClient, error) {
    return proxmox.NewRestyClientFromEnv(30 * time.Second)
}

// ListVMs handles GET /api/v1/vms
// Returns all VMs the authenticated user has access to.
func (h *VMHandler) ListVMs(w http.ResponseWriter, r *http.Request) {
    if h.isOffline() {
        writeJSON(w, VMListResponse{VMs: []VMSummary{}, Total: 0})
        return
    }

    client, err := restyClient()
    if err != nil {
        logger.Get().Error().Err(err).Msg("api/v1: failed to create resty client for ListVMs")
        errInternal(w)
        return
    }

    ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
    defer cancel()

    vms, err := proxmox.GetVMsResty(ctx, client)
    if err != nil {
        logger.Get().Error().Err(err).Msg("api/v1: GetVMsResty failed")
        errInternal(w)
        return
    }

    summaries := make([]VMSummary, 0, len(vms))
    for _, vm := range vms {
        summaries = append(summaries, vmToSummary(vm))
    }

    writeJSON(w, VMListResponse{VMs: summaries, Total: len(summaries)})
}

// GetVM handles GET /api/v1/vms/:id
// Returns details for a single VM including live metrics.
func (h *VMHandler) GetVM(w http.ResponseWriter, r *http.Request) {
    ps := httprouter.ParamsFromContext(r.Context())
    vmidStr := ps.ByName("id")
    vmid, err := strconv.Atoi(vmidStr)
    if err != nil || vmid <= 0 {
        errBadRequest(w, "invalid vm id")
        return
    }

    if h.isOffline() {
        errOffline(w)
        return
    }

    client, err := restyClient()
    if err != nil {
        errInternal(w)
        return
    }

    // We need the node — find it by listing VMs
    ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
    defer cancel()

    vms, err := proxmox.GetVMsResty(ctx, client)
    if err != nil {
        errInternal(w)
        return
    }

    for _, vm := range vms {
        if vm.VMID == vmid {
            writeJSON(w, vmToSummary(vm))
            return
        }
    }

    writeError(w, http.StatusNotFound, "not_found", "VM not found")
}

// vmToSummary converts a proxmox.VM to the API VMSummary type.
func vmToSummary(vm proxmox.VM) VMSummary {
    return VMSummary{
        VMID:     vm.VMID,
        Name:     vm.Name,
        Node:     vm.Node,
        Status:   vm.Status,
        CPU:      vm.CPU,
        CPUs:     vm.CPUs,
        MemMB:    vm.Mem,
        MaxMemMB: vm.MaxMem,
        DiskMB:   vm.MaxDisk,
        Uptime:   vm.Uptime,
        Tags:     vm.Tags,
    }
}
```

**Step 4: Run tests**

```bash
cd backend && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -v -run TestListVMs ./api/v1/...
```
Expected: PASS

**Step 5: Commit**

```bash
git add backend/api/v1/vms.go backend/api/v1/vms_test.go
git commit -m "feat(api/v1): add GET /api/v1/vms and GET /api/v1/vms/:id endpoints"
```

---

## Task 8: Create `backend/api/v1/vm_actions.go`

**Files:**
- Create: `backend/api/v1/vm_actions.go`
- Create: `backend/api/v1/vm_actions_test.go`

**Step 1: Write the failing test**

```go
// backend/api/v1/vm_actions_test.go
package apiv1

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestVMAction_InvalidAction(t *testing.T) {
    t.Setenv("JWT_SECRET", "testsecretthatis32byteslongexact!!")
    t.Setenv("PVMSS_OFFLINE", "true")

    h := NewVMActionHandler(nil)
    body, _ := json.Marshal(VMActionRequest{Action: "fly", Node: "node1"})
    req := makeAuthedRequest(http.MethodPost, "/api/v1/vms/100/action")
    req.Body = io.NopCloser(bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    rr := httptest.NewRecorder()
    JWTMiddleware(http.HandlerFunc(h.VMAction)).ServeHTTP(rr, req)
    if rr.Code != http.StatusBadRequest {
        t.Errorf("expected 400 for invalid action, got %d", rr.Code)
    }
}

func TestVMAction_OfflineMode(t *testing.T) {
    t.Setenv("JWT_SECRET", "testsecretthatis32byteslongexact!!")
    t.Setenv("PVMSS_OFFLINE", "true")

    h := NewVMActionHandler(nil)
    body, _ := json.Marshal(VMActionRequest{Action: "start", Node: "node1"})
    req := makeAuthedRequest(http.MethodPost, "/api/v1/vms/100/action")
    req.Body = io.NopCloser(bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    rr := httptest.NewRecorder()
    JWTMiddleware(http.HandlerFunc(h.VMAction)).ServeHTTP(rr, req)
    if rr.Code != http.StatusServiceUnavailable {
        t.Errorf("expected 503 in offline mode, got %d", rr.Code)
    }
}
```

Add `"io"` to imports.

**Step 2: Run to verify it fails**

```bash
cd backend && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -v -run TestVMAction ./api/v1/...
```
Expected: compile error

**Step 3: Implement `vm_actions.go`**

```go
// backend/api/v1/vm_actions.go
package apiv1

import (
    "context"
    "encoding/json"
    "net/http"
    "strconv"
    "time"

    "github.com/julienschmidt/httprouter"

    "pvmss/logger"
    "pvmss/proxmox"
    "pvmss/state"
)

var allowedActions = map[string]bool{
    "start":    true,
    "stop":     true,
    "shutdown": true,
    "reboot":   true,
    "reset":    true,
}

// VMActionHandler handles VM lifecycle action requests.
type VMActionHandler struct {
    state state.StateManager
}

// NewVMActionHandler creates a new VMActionHandler.
func NewVMActionHandler(s state.StateManager) *VMActionHandler {
    return &VMActionHandler{state: s}
}

// VMAction handles POST /api/v1/vms/:id/action
// Body: {"action":"start|stop|shutdown|reboot|reset", "node":"nodename"}
func (h *VMActionHandler) VMAction(w http.ResponseWriter, r *http.Request) {
    ps := httprouter.ParamsFromContext(r.Context())
    vmidStr := ps.ByName("id")
    vmid, err := strconv.Atoi(vmidStr)
    if err != nil || vmid <= 0 {
        errBadRequest(w, "invalid vm id")
        return
    }

    var req VMActionRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        errBadRequest(w, "invalid JSON body")
        return
    }
    if !allowedActions[req.Action] {
        errBadRequest(w, "action must be one of: start, stop, shutdown, reboot, reset")
        return
    }
    if req.Node == "" {
        errBadRequest(w, "node is required")
        return
    }

    if h.state != nil && h.state.IsOfflineMode() {
        errOffline(w)
        return
    }
    // Also check env var directly (for nil state)
    if h.state == nil {
        errOffline(w)
        return
    }

    client, err := proxmox.NewRestyClientFromEnv(60 * time.Second)
    if err != nil {
        logger.Get().Error().Err(err).Msg("api/v1: failed to create resty client for VMAction")
        errInternal(w)
        return
    }

    ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
    defer cancel()

    username := usernameFromCtx(r)
    logger.VMEvent("api_vm_action").
        Str("username", username).
        Int("vmid", vmid).
        Str("node", req.Node).
        Str("action", req.Action).
        Msg("VM action requested via API")

    upid, err := proxmox.VMActionResty(ctx, client, req.Node, strconv.Itoa(vmid), req.Action)
    if err != nil {
        logger.Get().Error().Err(err).Int("vmid", vmid).Str("action", req.Action).Msg("api/v1: VMActionResty failed")
        writeError(w, http.StatusBadGateway, "proxmox_error", err.Error())
        return
    }

    writeJSON(w, VMActionResponse{Success: true, TaskID: upid})
}
```

**Step 4: Run tests**

```bash
cd backend && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -v -run TestVMAction ./api/v1/...
```
Expected: PASS

**Step 5: Commit**

```bash
git add backend/api/v1/vm_actions.go backend/api/v1/vm_actions_test.go
git commit -m "feat(api/v1): add POST /api/v1/vms/:id/action endpoint"
```

---

## Task 9: Create `backend/api/v1/router.go` and wire into `main.go`

**Files:**
- Create: `backend/api/v1/router.go`
- Modify: `backend/main.go`

**Step 1: Create `router.go`**

```go
// backend/api/v1/router.go
package apiv1

import (
    "net/http"

    "github.com/julienschmidt/httprouter"

    "pvmss/state"
)

// RegisterRoutes mounts all /api/v1/ routes onto the provided router.
// Call this from main.go after handlers.InitHandlers().
func RegisterRoutes(router *httprouter.Router, s state.StateManager) {
    auth := NewAuthHandler(s)
    vms := NewVMHandler(s)
    actions := NewVMActionHandler(s)

    // Auth endpoints (no JWT required)
    router.POST("/api/v1/auth/login",    wrap(auth.Login))
    router.POST("/api/v1/auth/exchange", wrapSession(s, auth.Exchange))
    router.POST("/api/v1/auth/refresh",  wrap(auth.Refresh))
    router.POST("/api/v1/auth/logout",   wrap(auth.Logout))

    // JWT-protected endpoints
    router.GET("/api/v1/auth/me",        jwtWrap(auth.Me))
    router.GET("/api/v1/vms",            jwtWrap(vms.ListVMs))
    router.GET("/api/v1/vms/:id",        jwtWrap(vms.GetVM))
    router.POST("/api/v1/vms/:id/action", jwtWrap(actions.VMAction))
}

// wrap converts http.HandlerFunc to httprouter.Handle.
func wrap(h http.HandlerFunc) httprouter.Handle {
    return httprouterWrap(h)
}

// jwtWrap wraps a handler with JWT middleware.
func jwtWrap(h http.HandlerFunc) httprouter.Handle {
    return httprouterWrap(JWTMiddleware(h))
}

// wrapSession wraps a handler with the SCS session manager (needed for Exchange).
func wrapSession(s state.StateManager, h http.HandlerFunc) httprouter.Handle {
    if s == nil {
        return wrap(h)
    }
    sm := s.GetSessionManager()
    if sm == nil {
        return wrap(h)
    }
    return httprouterWrap(sm.LoadAndSave(http.HandlerFunc(h)))
}
```

**Step 2: Modify `backend/main.go`**

In `main.go`, in the `main()` function after the line `srv := &http.Server{...}` is set up (around line 80), find where `handlers.InitHandlers(stateManager)` is called and chain it:

Current code (around line 80):
```go
srv := &http.Server{
    Addr:    ":" + port,
    Handler: handlers.InitHandlers(stateManager),
    ...
}
```

Change to:
```go
router := handlers.InitHandlers(stateManager)
apiv1.RegisterRoutes(router.(*http.ServeMux) /* wrong */)
```

Wait — `handlers.InitHandlers` returns `http.Handler`, not `*httprouter.Router`. We need a different approach. The router needs to be accessible to both.

The correct approach is to expose the httprouter from `handlers` package, or to pass the router to both. Looking at the existing code, `handlers.InitHandlers` creates and returns an `http.Handler` (a `*http.ServeMux`). The `httprouter.Router` is internal.

**Alternative approach:** Expose the router from `handlers.InitHandlers`. Modify the function signature to also return the `*httprouter.Router`, then call `apiv1.RegisterRoutes(router, stateManager)` before the ServeMux wraps it.

In `backend/handlers/handlers.go`, change `InitHandlers` to return `(http.Handler, *httprouter.Router)`:

```go
// Change InitHandlers signature to expose the internal router:
func InitHandlers(stateManager state.StateManager) (http.Handler, *httprouter.Router) {
    ...
    // (existing code unchanged until the return)
    return handler, router
}
```

Then in `main.go`:
```go
import apiv1 "pvmss/api/v1"

// In main():
handler, router := handlers.InitHandlers(stateManager)
apiv1.RegisterRoutes(router, stateManager)

srv := &http.Server{
    Addr:    ":" + port,
    Handler: handler,
    ...
}
```

**Step 3: Apply the changes**

In `backend/handlers/handlers.go`:
- Change `func InitHandlers(stateManager state.StateManager) http.Handler`
  to `func InitHandlers(stateManager state.StateManager) (http.Handler, *httprouter.Router)`
- Change final `return handler` to `return handler, router`

In `backend/main.go`:
- Add `import apiv1 "pvmss/api/v1"`
- Change `handlers.InitHandlers(stateManager)` call as shown above

**Step 4: Run tests to verify nothing broke**

```bash
make test-offline
```
Expected: all PASS

**Step 5: Commit**

```bash
git add backend/api/v1/router.go backend/handlers/handlers.go backend/main.go
git commit -m "feat(api/v1): wire /api/v1/ routes into main router"
```

---

## Task 10: Add `/assets/*` static route for Vue build output

**Files:**
- Modify: `backend/handlers/handlers.go` (the `setupStaticFiles` function)

**Step 1: Add the assets route**

In `backend/handlers/handlers.go`, in the `setupStaticFiles` function, add:

```go
// Serve Vite-built Vue SPA (frontend/dist/)
distPath := filepath.Join(filepath.Dir(basePath), "frontend", "dist")
// Use basePath directly if it already points to frontend/
if _, err := os.Stat(filepath.Join(basePath, "dist")); err == nil {
    distPath = filepath.Join(basePath, "dist")
}
registerStaticHandler(router, "/assets/*filepath",
    http.StripPrefix("/assets/", createCachedFileServer(distPath, "")))
```

Actually, `basePath` is already `frontend/` (the path to the frontend directory, set in `initTemplates()`). So:

```go
// In setupStaticFiles(), add after the existing registerStaticHandler calls:
registerStaticHandler(router, "/assets/*filepath",
    http.StripPrefix("/assets/", withStaticCaching(http.FileServer(http.Dir(filepath.Join(basePath, "dist"))))))
```

Also add an `import "os"` if needed (check existing imports).

**Step 2: Run tests**

```bash
make test-offline
```
Expected: all PASS

**Step 3: Commit**

```bash
git add backend/handlers/handlers.go
git commit -m "feat: serve Vue SPA build output at /assets/*"
```

---

## Task 11: Update `backend/components/layout.templ` — add Vue mount point

**Files:**
- Modify: `backend/components/layout.templ`

**Step 1: Add the Vue mount point and exchange script**

In `layout.templ`, just before `</body>`, add:

```html
<!-- Vue 3 SPA mount point -->
<div id="vue-app"
     data-page="vm-list"
     data-username={ props.Username }
     data-is-admin={ fmt.Sprintf("%t", props.IsAdmin) }
     x-ignore>
</div>
<!-- Exchange session for JWT, then load Vue app -->
<script>
(async () => {
  try {
    await fetch('/api/v1/auth/exchange', { method: 'POST', credentials: 'include' });
  } catch (_) {}
})();
</script>
<script type="module" src="/assets/main.js"></script>
```

Add `"fmt"` to the templ file imports if not already present.

**Step 2: Regenerate templ**

```bash
make go-template
```
Expected: `layout_templ.go` regenerated

**Step 3: Run tests**

```bash
make test-offline
```
Expected: all PASS

**Step 4: Commit**

```bash
git add backend/components/layout.templ backend/components/layout_templ.go
git commit -m "feat(layout): add Vue app mount point and JWT exchange script"
```

---

## Task 12: Scaffold Vite + Vue 3 + TypeScript project in `frontend/`

**Files:**
- Create: `frontend/package.json`
- Create: `frontend/vite.config.ts`
- Create: `frontend/tsconfig.json`
- Create: `frontend/tsconfig.app.json`
- Create: `frontend/index.html`
- Create: `frontend/src/main.ts`
- Create: `frontend/src/App.vue`

**Step 1: Scaffold with create-vue**

```bash
cd frontend && npm create vue@latest . -- --ts --pinia --router false --eslint false --prettier false
```

When prompted, select: TypeScript yes, JSX no, Vue Router no, Pinia yes, Vitest no, ESLint no, Prettier no.

This creates `package.json`, `vite.config.ts`, `tsconfig*.json`, `src/`, `index.html`.

**Step 2: Install Tailwind CSS v4 and Axios**

```bash
cd frontend && npm install -D @tailwindcss/vite tailwindcss && npm install axios
```

**Step 3: Configure `vite.config.ts`**

```typescript
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [
    vue(),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  build: {
    outDir: 'dist',
    rollupOptions: {
      input: 'src/main.ts',
      output: {
        entryFileNames: 'main.js',          // fixed name, no hash (Go serves it by name)
        chunkFileNames: 'chunks/[name].js',
        assetFileNames: '[name][extname]',
      }
    }
  },
  server: {
    proxy: {
      '/api': 'http://localhost:50000',
      '/css': 'http://localhost:50000',
      '/js':  'http://localhost:50000',
    }
  }
})
```

**Step 4: Add Tailwind import to `src/style.css`** (or create it)

```css
@import "tailwindcss";
```

**Step 5: Verify build works**

```bash
cd frontend && npm run build
```
Expected: `frontend/dist/main.js` created, no errors

**Step 6: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/vite.config.ts frontend/tsconfig*.json frontend/index.html frontend/src/
git commit -m "feat(frontend): scaffold Vite + Vue 3 + TS + Pinia + Tailwind"
```

---

## Task 13: Create `frontend/src/api/` — Axios client, auth, and VMs

**Files:**
- Create: `frontend/src/api/client.ts`
- Create: `frontend/src/api/auth.ts`
- Create: `frontend/src/api/vms.ts`

**Step 1: Create `client.ts`**

```typescript
// frontend/src/api/client.ts
import axios from 'axios'

const client = axios.create({
  baseURL: '/api/v1',
  withCredentials: true,  // send HttpOnly cookies automatically
  headers: { 'Content-Type': 'application/json' },
})

// On 401, attempt token refresh then retry once
client.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config
    if (error.response?.status === 401 && !originalRequest._retried) {
      originalRequest._retried = true
      try {
        await axios.post('/api/v1/auth/refresh', {}, { withCredentials: true })
        return client(originalRequest)
      } catch {
        // Refresh failed — let the caller handle the 401
      }
    }
    return Promise.reject(error)
  }
)

export default client
```

**Step 2: Create `auth.ts`**

```typescript
// frontend/src/api/auth.ts
import client from './client'

export interface AuthUser {
  username: string
  is_admin: boolean
}

export async function getMe(): Promise<AuthUser> {
  const { data } = await client.get<AuthUser>('/auth/me')
  return data
}

export async function login(username: string, password: string, admin: boolean): Promise<AuthUser> {
  const { data } = await client.post<AuthUser>('/auth/login', { username, password, admin })
  return data
}

export async function logout(): Promise<void> {
  await client.post('/auth/logout')
}
```

**Step 3: Create `vms.ts`**

```typescript
// frontend/src/api/vms.ts
import client from './client'

export interface VMSummary {
  vmid: number
  name: string
  node: string
  status: 'running' | 'stopped' | 'paused' | string
  cpu: number       // fraction 0..1
  cpus: number
  mem_mb: number
  max_mem_mb: number
  disk_mb: number
  uptime: number
  tags: string
}

export interface VMListResponse {
  vms: VMSummary[]
  total: number
}

export interface VMActionResponse {
  success: boolean
  task_id?: string
  message?: string
}

export async function getVMs(): Promise<VMSummary[]> {
  const { data } = await client.get<VMListResponse>('/vms')
  return data.vms ?? []
}

export async function getVM(vmid: number): Promise<VMSummary> {
  const { data } = await client.get<VMSummary>(`/vms/${vmid}`)
  return data
}

export async function postVMAction(
  vmid: number,
  action: string,
  node: string
): Promise<VMActionResponse> {
  const { data } = await client.post<VMActionResponse>(`/vms/${vmid}/action`, { action, node })
  return data
}
```

**Step 4: Verify TypeScript compiles**

```bash
cd frontend && npm run build
```
Expected: no TypeScript errors

**Step 5: Commit**

```bash
git add frontend/src/api/
git commit -m "feat(frontend): add Axios API client for auth and VMs"
```

---

## Task 14: Create Pinia stores

**Files:**
- Create: `frontend/src/stores/auth.ts`
- Create: `frontend/src/stores/vms.ts`

**Step 1: Create `stores/auth.ts`**

```typescript
// frontend/src/stores/auth.ts
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { getMe, logout as apiLogout } from '@/api/auth'

export const useAuthStore = defineStore('auth', () => {
  const username = ref<string>('')
  const isAdmin = ref<boolean>(false)
  const initialized = ref<boolean>(false)

  const isAuthenticated = computed(() => initialized.value && username.value !== '')

  async function init() {
    try {
      const user = await getMe()
      username.value = user.username
      isAdmin.value = user.is_admin
    } catch {
      // Not authenticated — leave empty
    } finally {
      initialized.value = true
    }
  }

  async function logout() {
    await apiLogout()
    username.value = ''
    isAdmin.value = false
  }

  return { username, isAdmin, initialized, isAuthenticated, init, logout }
})
```

**Step 2: Create `stores/vms.ts`**

```typescript
// frontend/src/stores/vms.ts
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getVMs, postVMAction, type VMSummary } from '@/api/vms'

export const useVMStore = defineStore('vms', () => {
  const vms = ref<VMSummary[]>([])
  const loading = ref<boolean>(false)
  const error = ref<string | null>(null)
  const actionLoading = ref<Record<number, boolean>>({})  // vmid → loading

  async function fetchVMs() {
    loading.value = true
    error.value = null
    try {
      vms.value = await getVMs()
    } catch (e: any) {
      error.value = e.message ?? 'Failed to load VMs'
    } finally {
      loading.value = false
    }
  }

  async function executeAction(vmid: number, action: string, node: string) {
    actionLoading.value = { ...actionLoading.value, [vmid]: true }
    try {
      await postVMAction(vmid, action, node)
      // Refresh after action
      await fetchVMs()
    } finally {
      const updated = { ...actionLoading.value }
      delete updated[vmid]
      actionLoading.value = updated
    }
  }

  function isActionLoading(vmid: number): boolean {
    return actionLoading.value[vmid] ?? false
  }

  return { vms, loading, error, fetchVMs, executeAction, isActionLoading }
})
```

**Step 3: Build to verify**

```bash
cd frontend && npm run build
```

**Step 4: Commit**

```bash
git add frontend/src/stores/
git commit -m "feat(frontend): add Pinia auth and VM stores"
```

---

## Task 15: Create `AppButton.vue`

**Files:**
- Create: `frontend/src/components/AppButton.vue`

**Step 1: Create the component**

```vue
<!-- frontend/src/components/AppButton.vue -->
<script setup lang="ts">
defineProps<{
  variant?: 'primary' | 'danger' | 'success' | 'warning' | 'ghost'
  size?: 'sm' | 'md' | 'lg'
  loading?: boolean
  disabled?: boolean
  icon?: string    // Font Awesome class e.g. 'fa-play'
  label?: string   // aria-label for icon-only buttons
  type?: 'button' | 'submit'
}>()

const variantClasses: Record<string, string> = {
  primary: 'bg-blue-600 text-white hover:bg-blue-700 focus:ring-blue-500',
  danger:  'bg-red-600 text-white hover:bg-red-700 focus:ring-red-500',
  success: 'bg-green-600 text-white hover:bg-green-700 focus:ring-green-500',
  warning: 'bg-yellow-500 text-white hover:bg-yellow-600 focus:ring-yellow-400',
  ghost:   'bg-transparent text-gray-600 hover:bg-gray-100 focus:ring-gray-300 border border-gray-300',
}

const sizeClasses: Record<string, string> = {
  sm: 'px-2.5 py-1.5 text-xs',
  md: 'px-4 py-2 text-sm',
  lg: 'px-5 py-2.5 text-base',
}
</script>

<template>
  <button
    :type="type ?? 'button'"
    :disabled="disabled || loading"
    :aria-label="label"
    :class="[
      'inline-flex items-center gap-1.5 rounded-md font-medium',
      'focus:outline-none focus:ring-2 focus:ring-offset-1',
      'disabled:opacity-50 disabled:cursor-not-allowed',
      'transition-colors duration-150',
      variantClasses[variant ?? 'ghost'],
      sizeClasses[size ?? 'md'],
    ]"
  >
    <i v-if="loading" class="fas fa-spinner fa-spin text-xs" />
    <i v-else-if="icon" :class="['fas', icon, 'text-xs']" />
    <slot />
  </button>
</template>
```

**Step 2: Build**

```bash
cd frontend && npm run build
```

**Step 3: Commit**

```bash
git add frontend/src/components/AppButton.vue
git commit -m "feat(frontend): add AppButton.vue component"
```

---

## Task 16: Create `VmActionButtons.vue`

**Files:**
- Create: `frontend/src/components/VmActionButtons.vue`

**Step 1: Create the component**

```vue
<!-- frontend/src/components/VmActionButtons.vue -->
<script setup lang="ts">
import AppButton from './AppButton.vue'

const props = defineProps<{
  vmid: number
  node: string
  status: string
  loading?: boolean
}>()

const emit = defineEmits<{
  action: [vmid: number, action: string, node: string]
}>()

const isRunning = () => props.status === 'running'
const isStopped = () => props.status === 'stopped' || props.status === 'stopped (paused)'

function doAction(action: string) {
  emit('action', props.vmid, action, props.node)
}
</script>

<template>
  <div class="flex flex-wrap gap-2">
    <!-- Start: only when stopped -->
    <AppButton
      v-if="isStopped()"
      variant="success"
      size="sm"
      icon="fa-play"
      :loading="loading"
      @click="doAction('start')"
    >
      Start
    </AppButton>

    <!-- Console: only when running -->
    <AppButton
      v-if="isRunning()"
      variant="primary"
      size="sm"
      icon="fa-terminal"
      :disabled="loading"
    >
      Console
    </AppButton>

    <!-- Reboot / Shutdown / Stop: only when running -->
    <template v-if="isRunning()">
      <AppButton
        variant="ghost"
        size="sm"
        icon="fa-redo"
        :loading="loading"
        @click="doAction('reboot')"
      >
        Reboot
      </AppButton>

      <AppButton
        variant="warning"
        size="sm"
        icon="fa-power-off"
        :loading="loading"
        @click="doAction('shutdown')"
      >
        Shutdown
      </AppButton>

      <AppButton
        variant="danger"
        size="sm"
        icon="fa-stop"
        :loading="loading"
        @click="doAction('stop')"
      >
        Stop
      </AppButton>
    </template>
  </div>
</template>
```

**Step 2: Build**

```bash
cd frontend && npm run build
```

**Step 3: Commit**

```bash
git add frontend/src/components/VmActionButtons.vue
git commit -m "feat(frontend): add VmActionButtons.vue component"
```

---

## Task 17: Create `VmCard.vue`

**Files:**
- Create: `frontend/src/components/VmCard.vue`

**Step 1: Create the component**

```vue
<!-- frontend/src/components/VmCard.vue -->
<script setup lang="ts">
import VmActionButtons from './VmActionButtons.vue'
import type { VMSummary } from '@/api/vms'

const props = defineProps<{
  vm: VMSummary
  actionLoading?: boolean
}>()

const emit = defineEmits<{
  action: [vmid: number, action: string, node: string]
}>()

function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 B'
  const gb = bytes / (1024 ** 3)
  return gb >= 1 ? `${gb.toFixed(1)} GB` : `${(bytes / (1024 ** 2)).toFixed(0)} MB`
}

function formatCpu(cpu: number): string {
  return `${(cpu * 100).toFixed(1)}%`
}

const statusColor: Record<string, string> = {
  running: 'bg-green-100 text-green-700',
  stopped: 'bg-gray-100 text-gray-600',
  paused:  'bg-yellow-100 text-yellow-700',
}

function statusClass(status: string): string {
  return statusColor[status] ?? 'bg-gray-100 text-gray-500'
}

const tags = (s: string) => s ? s.split(';').filter(Boolean) : []
</script>

<template>
  <div class="bg-white rounded-xl border border-gray-200 shadow-sm hover:shadow-md transition-shadow p-4 flex flex-col gap-3">
    <!-- Header -->
    <div class="flex items-start justify-between gap-2">
      <div class="flex items-center gap-2 min-w-0">
        <i class="fas fa-server text-blue-500 shrink-0" />
        <span class="font-semibold text-gray-900 truncate" :title="vm.name">{{ vm.name }}</span>
      </div>
      <span :class="['shrink-0 px-2 py-0.5 rounded-full text-xs font-medium', statusClass(vm.status)]">
        {{ vm.status }}
      </span>
    </div>

    <!-- Meta -->
    <div class="flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500">
      <span><span class="font-medium text-gray-700">VMID</span> {{ vm.vmid }}</span>
      <span><span class="font-medium text-gray-700">Node</span> {{ vm.node }}</span>
    </div>

    <!-- Metrics (running only) -->
    <div v-if="vm.status === 'running'" class="grid grid-cols-3 gap-2 bg-gray-50 rounded-lg p-2 text-center">
      <div>
        <p class="text-xs text-gray-500">CPU</p>
        <p class="font-semibold text-sm text-gray-800">{{ formatCpu(vm.cpu) }}</p>
      </div>
      <div>
        <p class="text-xs text-gray-500">RAM</p>
        <p class="font-semibold text-sm text-gray-800">{{ formatBytes(vm.mem_mb) }}</p>
      </div>
      <div>
        <p class="text-xs text-gray-500">Disk</p>
        <p class="font-semibold text-sm text-gray-800">{{ formatBytes(vm.disk_mb) }}</p>
      </div>
    </div>

    <!-- Tags -->
    <div v-if="tags(vm.tags).length" class="flex flex-wrap gap-1">
      <span
        v-for="tag in tags(vm.tags)"
        :key="tag"
        class="px-1.5 py-0.5 bg-blue-50 text-blue-600 rounded text-xs"
      >{{ tag }}</span>
    </div>

    <!-- Actions -->
    <div class="pt-1 border-t border-gray-100">
      <VmActionButtons
        :vmid="vm.vmid"
        :node="vm.node"
        :status="vm.status"
        :loading="actionLoading"
        @action="(vmid, action, node) => emit('action', vmid, action, node)"
      />
    </div>
  </div>
</template>
```

**Step 2: Build**

```bash
cd frontend && npm run build
```

**Step 3: Commit**

```bash
git add frontend/src/components/VmCard.vue
git commit -m "feat(frontend): add VmCard.vue component with Tailwind styling"
```

---

## Task 18: Wire up `App.vue` and `main.ts`

**Files:**
- Modify: `frontend/src/App.vue`
- Modify: `frontend/src/main.ts`

**Step 1: Rewrite `App.vue`**

```vue
<!-- frontend/src/App.vue -->
<script setup lang="ts">
import { onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useVMStore } from '@/stores/vms'
import VmCard from '@/components/VmCard.vue'

const authStore = useAuthStore()
const vmStore = useVMStore()

// Read the page type from the templ layout's data attribute
const mountEl = document.getElementById('vue-app')
const page = mountEl?.dataset.page ?? ''

onMounted(async () => {
  await authStore.init()
  if (authStore.isAuthenticated && page === 'vm-list') {
    await vmStore.fetchVMs()
  }
})
</script>

<template>
  <template v-if="page === 'vm-list' && authStore.isAuthenticated">
    <!-- Loading state -->
    <div v-if="vmStore.loading" class="flex justify-center items-center py-12">
      <i class="fas fa-spinner fa-spin text-2xl text-blue-500" />
    </div>

    <!-- Error state -->
    <div v-else-if="vmStore.error" class="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700 text-sm">
      {{ vmStore.error }}
    </div>

    <!-- Empty state -->
    <div v-else-if="vmStore.vms.length === 0" class="text-center py-12 text-gray-500">
      <i class="fas fa-server text-4xl mb-3 block text-gray-300" />
      No virtual machines found.
    </div>

    <!-- VM grid -->
    <div v-else class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      <VmCard
        v-for="vm in vmStore.vms"
        :key="vm.vmid"
        :vm="vm"
        :action-loading="vmStore.isActionLoading(vm.vmid)"
        @action="(vmid, action, node) => vmStore.executeAction(vmid, action, node)"
      />
    </div>
  </template>
</template>
```

**Step 2: Rewrite `main.ts`**

```typescript
// frontend/src/main.ts
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './style.css'  // Tailwind

const mountEl = document.getElementById('vue-app')
if (mountEl) {
  const app = createApp(App)
  app.use(createPinia())
  app.mount(mountEl)
}
```

**Step 3: Build**

```bash
cd frontend && npm run build
```
Expected: `frontend/dist/main.js` built successfully

**Step 4: Commit**

```bash
git add frontend/src/App.vue frontend/src/main.ts
git commit -m "feat(frontend): wire App.vue and main.ts, mount Vue on #vue-app"
```

---

## Task 19: Update Makefile and Dockerfile

**Files:**
- Modify: `Makefile`
- Modify: `Dockerfile`

**Step 1: Add frontend targets to Makefile**

Add after the existing `go-template` target:

```makefile
# =============================================================================
# Frontend (Vue 3 + TypeScript + Vite)

frontend-install: ## Install frontend npm dependencies
	@echo "$(BLUE)Installing frontend dependencies...$(NC)"
	cd frontend && npm install

frontend-dev: ## Start Vite dev server with proxy to Go backend on :50000
	@echo "$(BLUE)Starting Vite dev server...$(NC)"
	cd frontend && npm run dev

frontend-build: ## Build Vue SPA to frontend/dist/
	@echo "$(BLUE)Building frontend...$(NC)"
	cd frontend && npm run build
	@echo "$(GREEN)✓ Frontend built to frontend/dist/$(NC)"
```

Also add `frontend-build` to the `.PHONY` line at the top.

**Step 2: Update Dockerfile — add Node build stage**

In `Dockerfile`, add a new Node build stage between the Go builder and the frontend stage:

```dockerfile
# Build stage - Frontend (Vue 3 + TypeScript + Vite)
FROM node:22-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci --prefer-offline
COPY frontend/ ./
RUN npm run build
```

Then in the final stage, copy from `frontend-builder` instead of the raw `frontend` stage for the Vue dist:

```dockerfile
# In the final COPY block, add:
COPY --from=frontend-builder --chown=nonroot:nonroot /app/frontend/dist /app/frontend/dist
```

Keep the existing `COPY --from=frontend` line for CSS, JS, webfonts (legacy static assets). The full final stage becomes:

```dockerfile
FROM gcr.io/distroless/static-debian13:nonroot
WORKDIR /app
COPY --from=builder         --chown=nonroot:nonroot /app/pvmss-backend      /app/pvmss-backend
COPY --from=frontend        --chown=nonroot:nonroot /app/frontend/           /app/frontend/
COPY --from=frontend-builder --chown=nonroot:nonroot /app/frontend/dist/    /app/frontend/dist/
COPY --from=builder         --chown=nonroot:nonroot /app/backend/i18n/      /app/backend/i18n/
COPY --from=builder         --chown=nonroot:nonroot /app/backend/docs/      /app/backend/docs/
EXPOSE 50000
ENTRYPOINT ["/app/pvmss-backend","-templates","/app/frontend"]
```

**Step 3: Add `frontend/dist/` and `frontend/node_modules/` to `.gitignore`**

```bash
echo "frontend/dist/" >> .gitignore
echo "frontend/node_modules/" >> .gitignore
```

**Step 4: Run full test suite**

```bash
make test-offline && make frontend-build
```
Expected: both pass

**Step 5: Update CLAUDE.md** — add frontend commands section (already in task 2 partially, ensure it's complete)

**Step 6: Commit**

```bash
git add Makefile Dockerfile .gitignore CLAUDE.md
git commit -m "chore: add frontend build targets to Makefile and Dockerfile Node stage"
```

---

## Task 20: Final integration check

**Step 1: Run the full offline test suite**

```bash
make test-offline-verbose
```
Expected: all PASS, no regressions

**Step 2: Build frontend**

```bash
make frontend-build
```
Expected: `frontend/dist/main.js` present

**Step 3: Run golangci-lint**

```bash
make go-lint
```
Fix any new lint issues in `backend/api/v1/`.

**Step 4: Update CLAUDE.md**

Add to the `## Commands` section:

```markdown
### Frontend (Vue 3 + TypeScript)

```bash
make frontend-install  # Install npm deps (first time)
make frontend-dev      # Vite dev server with proxy to :50000
make frontend-build    # Build to frontend/dist/
```

Add to the Architecture section:

```markdown
### API Layer (`backend/api/v1/`)

JWT-authenticated JSON endpoints, separate from templ handlers. Uses `proxmox.RestyClient` directly (no Telmate). Tokens stored in HttpOnly SameSite=Strict cookies (`access_token` 15min, `refresh_token` 7 days). See `backend/api/v1/router.go` for all routes. The `POST /api/v1/auth/exchange` endpoint trades an existing SCS session for JWT tokens — called by the templ layout on page load so the Vue SPA can authenticate.
```

**Step 5: Final commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with Vue frontend and API v1 commands"
```
