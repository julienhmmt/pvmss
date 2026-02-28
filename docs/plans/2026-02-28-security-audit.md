# Security Audit — PVMSS Go Backend

**Date:** 2026-02-28
**Scope:** Go backend only, light audit based on code review.
**Severity:** Critical / High / Medium / Low

---

## Findings

### 1. [CRITICAL] No rate limiting on `/api/v1/auth/login`

**File:** `backend/handlers/handlers.go:31-43`
**Issue:** Rate limit rules are only added for legacy routes (`/login`, `/admin/login`, `/admin/proxmox-login`). The new JWT endpoint `/api/v1/auth/login` (and `/api/v1/auth/refresh`) has no rule — the rate limiter middleware is present but `Allow()` returns `true` when no rule matches.
**Risk:** Unlimited brute-force attempts on admin credentials (bcrypt still slows attacks, but no lockout).
**Fix:** Add `AddRule` calls for `POST /api/v1/auth/login` and `POST /api/v1/auth/refresh` with the same `LoginRateLimitCapacity`/`LoginRateLimitRefill` constants.

---

### 2. [HIGH] `X-Forwarded-For` trusted unconditionally — rate limit bypass

**File:** `backend/middleware/util.go:13-17`
**Issue:** `clientIP()` always trusts the first value of `X-Forwarded-For`. Any client can send `X-Forwarded-For: 1.2.3.4` to appear as a different IP, bypassing per-IP rate limits.
**Risk:** Rate limiting is fully bypassable with a single header.
**Fix:** Add a `TRUSTED_PROXY_CIDRS` setting (e.g. `"127.0.0.1/32,10.0.0.0/8"`). Only trust `X-Forwarded-For` when `r.RemoteAddr` is in that list; otherwise use `r.RemoteAddr` directly.

---

### 3. [HIGH] JWT signing error silently ignored

**File:** `backend/api/v1/auth.go:189`
**Issue:** `signed, _ := tok.SignedString([]byte(secret))` discards the error. If signing fails (empty secret race condition, crypto error), an empty string is set as cookie. The client gets a blank token; subsequent requests fail in a confusing, untracked way.
**Fix:**
```go
signed, err := tok.SignedString([]byte(secret))
if err != nil {
    logger.Get().Error().Err(err).Msg("JWT signing failed")
    // return error up the call chain
}
```

---

### 4. [MEDIUM] Security headers not applied to `/api/v1/` responses

**File:** `backend/handlers/middleware_utils.go:218-234`
**Issue:** `buildAPIMiddleware` does not call `securityMiddleware.Headers()`. API JSON responses have no `X-Frame-Options`, `X-Content-Type-Options`, `Content-Security-Policy`, etc.
**Risk:** Lower than for HTML pages, but API responses can be embedded/framed in some attack scenarios.
**Fix:** Add `handler = securityMiddleware.Headers(handler)` in `buildAPIMiddleware`, between `recoverMiddleware` and `maxBodySizeMiddleware`.

---

### 5. [MEDIUM] CSP uses `unsafe-inline` and `unsafe-eval`

**File:** `backend/security/middleware/headers.go:37-38`
**Issue:** Both directives are allowed globally for `script-src`. This negates most XSS protection provided by CSP.
**Context:** Comment says "needed for Go templates" — but the Go templates already use `html/template` escaping. The Vue SPA uses ES modules (no `eval`). The HTMX/Alpine inline handlers do require `unsafe-inline`.
**Fix (phased):**
1. Short-term: Remove `unsafe-eval` (no code uses it currently).
2. Medium-term (post-Alpine removal per migration plan): Replace `unsafe-inline` with a per-request nonce injected via the `Headers` middleware and embedded in layout.templ.

---

### 6. [MEDIUM] Refresh token is never rotated

**File:** `backend/api/v1/auth.go:124-151`
**Issue:** `POST /api/v1/auth/refresh` issues a new `access_token` but reuses the same `refresh_token` (7-day lifetime). A stolen refresh token stays valid indefinitely until expiry.
**Fix:** On every successful refresh, also issue a new `refresh_token` cookie (same TTL from current time). This is token rotation — the old refresh token becomes useless.

---

### 7. [MEDIUM] `PVMSS_ENV` read directly in `auth.go` — inconsistency

**File:** `backend/api/v1/auth.go:163-164` and `auth.go:191-192`
**Issue:** `Logout` and `setTokenCookie` call `os.Getenv("PVMSS_ENV")` directly, duplicating the logic that already exists in `utils.IsProduction()`. If the env var name ever changes, there are now two places to update.
**Fix:** Replace with `utils.IsProduction()`.

---

### 8. [LOW] No JWT revocation on logout

**File:** `backend/api/v1/auth.go:162-168`
**Issue:** Logout only clears client-side cookies. The JWT remains valid until expiry (15 min for access, 7 days for refresh). If a token was copied before logout, it still works.
**Context:** Full revocation requires a server-side blocklist (adds statefulness). For a 15-min access token, the risk window is acceptable.
**Fix (pragmatic):** Keep the short `accessTokenTTL = 15 * time.Minute`. Add a server-side blocklist only for the `refresh_token` on logout — store invalidated `jti` (JWT ID) claims in the in-memory store with TTL=7d. Add `jti` claim (UUID) when minting tokens.

---

### 9. [LOW] Session store is in-memory only

**File:** `backend/security/init.go:43`
**Issue:** `scsm.Store = memstore.New()` — all sessions lost on restart. Users are logged out on every deploy.
**Fix:** Use `scs/v2/pgstore`, `scs/v2/boltstore`, or a file-based store. Or document this as a known limitation and ensure zero-downtime restarts.

---

### 10. [LOW] `PROXMOX_VERIFY_SSL=false` disables TLS verification — confusing flag name

**File:** `backend/main.go:203`, `backend/proxmox/helpers.go:15`
**Issue:** `PROXMOX_VERIFY_SSL=false` means "skip verification" — the semantics are inverted from what most users expect. Default is `true` (verify), which is correct, but the flag name is a footgun.
**Fix:** Rename to `PROXMOX_SKIP_TLS_VERIFY=true` (positive = explicit opt-in to insecure behavior). Update all 6 call sites and documentation.

---

## Recommended Fix Order

| Priority | Task | Effort |
|---|---|---|
| 1 | Rate limit `/api/v1/auth/login` and `/api/v1/auth/refresh` | 15 min |
| 2 | Fix JWT signing error ignored (`signed, _`) | 5 min |
| 3 | Restrict `X-Forwarded-For` trust (add `TRUSTED_PROXY_CIDRS`) | 1-2h |
| 4 | Add security headers to API middleware | 5 min |
| 5 | Remove `unsafe-eval` from CSP | 5 min |
| 6 | Rotate refresh token on refresh | 30 min |
| 7 | Replace `os.Getenv("PVMSS_ENV")` with `utils.IsProduction()` | 10 min |
| 8 | Add `jti` + refresh token blocklist on logout | 2-3h |
| 9 | Persistent session store | 1-2h |
| 10 | Rename `PROXMOX_VERIFY_SSL` flag | 30 min |

Items 1, 2, 4, 5, 7 are quick wins with no architectural impact — implement together in a single PR.
Items 3, 6, 8, 9, 10 require more design and can be batched in a follow-up.
