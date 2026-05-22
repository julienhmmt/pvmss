# Backend Security Hardening Audit & Plan

**Date:** 2026-05-22
**Scope:** `backend/` Go code + deploy manifests (Dockerfile, helm/, pvmss-deployment.yaml)
**Standards:** OWASP Top 10, CIS, baseline pentest checklist

---

## Context

PVMSS backend handles VM provisioning against live Proxmox infra with admin-level API tokens. Compromise = full Proxmox takeover. Goal: full security audit + remediation plan to reach production-grade compliance posture. Three parallel audits ran (auth/session/CSRF, input/injection/OWASP, secrets/headers/deploy). This plan consolidates verified findings only — false positives stripped after manual verification.

**Overall assessment:** Strong baseline. Distroless non-root, HS256 with alg-confusion check, scs sessions with proper flags, CSRF via synchronizer + constant-time compare, parameterized SQL, bcrypt for admin password, comprehensive security headers (CSP, HSTS, X-Frame-Options), env-var fail-fast validation. No SQL injection. No shell exec on user input. No hardcoded production secrets in git.

**Real exposure:** input validation gaps on Proxmox-bound names, SFTP path-traversal defense missing at boundary, `PROXMOX_VERIFY_SSL=false` in shipped Helm/K8s defaults, allowlist enforcement missing on ISO/bridge selection.

---

## Verified findings

### False positives removed after verification
- ❌ "CRITICAL: `.env` git-tracked" — `.env` is in `.gitignore`, never appeared in `git log --all --full-history`. Local-only file. **Not a finding.**
- ❌ "CRITICAL: type assertion panic at middleware.go:55" — call site is internal to `JWTAdminMiddleware`, always reached via `JWTMiddleware` which sets the key on line 47. `recoverMiddleware` catches panics anyway. Downgrade to **LOW (style)** — use safe assertion pattern like `isAdminFromCtx()` for consistency.
- ❌ "CRITICAL: path traversal in SFTP" — filename source is `state.CloudInitTemplatePrefix + generateCloudInitID(name) + ".yml"`, where `generateCloudInitID` already sanitizes via regex (admin_mutations.go:749). Real risk = defense-in-depth, not active vector. Downgrade to **MEDIUM**.

### Real findings (prioritized)

| # | Sev | Area | File:line | Issue |
|---|-----|------|-----------|-------|
| S1 | HIGH | Transport | `pvmss-deployment.yaml:113`, `helm/values.yaml:52`, `example.env:6` | `PROXMOX_VERIFY_SSL=false` shipped as default → MITM on Proxmox API |
| S2 | HIGH | Validation | `api/v1/admin_mutations.go:161` | Pool name not validated before `"pvmss_" + req.Pool` concat → bad input bubbled to Proxmox API |
| S3 | HIGH | Validation | `api/v1/admin_mutations.go:458` (`DeleteTag`, `SetTagColor`) | `tagNameRegex` exists at line 24 but not applied on delete/color mutators |
| S4 | HIGH | AuthZ | `api/v1/vm_create.go:104` | `req.ISO` passed to Proxmox without allowlist check vs `settings.ISOs` → user can mount arbitrary ISO |
| S5 | HIGH | AuthZ | `api/v1/vm_create.go` (network section) | `VMCreateNetwork.Bridge` not validated vs `settings.EnabledVMBRs` |
| S6 | MED | Defense-in-depth | `proxmox/cloudinit.go:493, 547` | SFTP upload/delete trusts filename. Add boundary regex even though current callers sanitize |
| S7 | MED | Rate limit | `api/v1/router.go` | Admin mutations (pool create/delete, db import, cloudinit upload) lack explicit rate limits |
| S8 | MED | YAML | `cloudinit/validator.go:34-83` | Only syntax/size validation. If user-supplied YAML reaches a VM, `runcmd`/`bootcmd` → arbitrary code at boot. Add semantic blacklist or document trust model |
| S9 | MED | K8s | `pvmss-deployment.yaml:150-157` | `readOnlyRootFilesystem` not set. Distroless minimizes blast radius but still set it |
| S10 | MED | JWT | `api/v1/auth.go:246` | `signed, _ := tok.SignedString(...)` — error swallowed → empty token cookie set, user gets 401 loop |
| S11 | MED | Token lifetime | `api/v1/auth.go:25` | Refresh token TTL = 7 days, no server-side revocation. Reduce to 24-48h OR add revocation list |
| S12 | LOW | Error leak | `api/v1/admin_db.go:171` | SQLite validation error string returned to client ("database disk image is malformed") |
| S13 | LOW | WebSocket | `api/v1/vnc.go:330-350` | Origin check parses port inconsistently with `X-Forwarded-Host` — bypass possible behind misconfigured proxy |
| S14 | LOW | Helm safety | `helm/values.yaml:15-19` | `changemebase64hash` placeholder — needs pre-flight check to refuse install |
| S15 | LOW | Network | `pvmss-deployment.yaml` | No `NetworkPolicy` restricting egress to Proxmox endpoint |
| S16 | LOW | Auth | `handlers/handlers.go:31-38` | Rate-limit (5 attempts/60s) only; no account lockout |
| S17 | LOW | Style | `api/v1/middleware.go:55` | Unsafe `.(bool)` assertion in `JWTAdminMiddleware` — switch to `isAdminFromCtx(r)` for consistency |
| S18 | LOW | CSP | `security/middleware/headers.go:50-51` | `script-src 'unsafe-inline' 'unsafe-eval'` — tech debt for SvelteKit migration. Drop both once Go templates removed |
| S19 | INFO | DB perms | `database/database.go:105` | Dir perms `0o750` good. Document that PVMSS_DB_PATH parent must have restrictive umask for WAL/SHM siblings |

### Confirmed clean (no action)
- SQL injection (all queries parameterized — `database/lists.go`, `database/auth.go`, etc.)
- Command injection (no `os/exec` on user input)
- Hardcoded production secrets (none in git)
- Deserialization (JSON only, no gob/xml; cloud-init YAML parsed in isolation)
- JWT alg-confusion (`SigningMethodHMAC` check at `middleware.go:36`)
- Session cookie flags (`security/init.go:28-32`)
- CSRF token entropy + constant-time compare (`security/csrfgen.go:46-57`)
- Bcrypt for admin password (`auth.go:65`)
- IDOR on VM endpoints (pool membership enforced — `vms.go:174,242`)
- Distroless non-root (`Dockerfile:41`, runAsUser 65532)
- TLS 1.2 minimum (`proxmox/shared_transport.go:50-53`)
- CORS exact-match origin (`security/middleware/headers.go:75-80`)
- Log redaction of Authorization/Cookie/CSRF (`handlers/middleware_utils.go:103-117`)
- HSTS 1-year preload in prod (`security/middleware/headers.go:84-86`)

---

## Phased remediation

### Phase A — Production blockers (must ship before next deploy)
1. **S1**: Flip `PROXMOX_VERIFY_SSL` default to `true` in `helm/values.yaml:52`, `pvmss-deployment.yaml:113`, `example.env:6`. Add Proxmox CA-cert mount path doc.
2. **S4 + S5**: Add allowlist validation in `vm_create.go` before Proxmox call:
   - ISO must be in `settings.ISOs`
   - Bridge must be in `settings.EnabledVMBRs`
   - Node must be in `settings.EnabledNodes` (verify already done)
   - Storage must be in `settings.EnabledStorages`
3. **S2**: Pool name regex `^[a-z0-9_-]{1,50}$` before `"pvmss_" + req.Pool`.
4. **S3**: Apply `tagNameRegex` to every tag mutator (DeleteTag, SetTagColor, any other).

### Phase B — Defense-in-depth (next sprint)
5. **S6**: Add filename regex check inside `UploadSnippetFileSFTP` / `DeleteSnippetFileSFTP` regardless of caller:
   ```go
   var snippetFilenameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
   if !snippetFilenameRe.MatchString(filename) || strings.Contains(filename, "..") {
       return fmt.Errorf("invalid snippet filename")
   }
   ```
6. **S8**: Either (a) refuse `runcmd`/`bootcmd`/`write_files` in user-supplied cloud-init YAML, or (b) document explicitly that cloud-init editing is an admin-only trusted operation. Pick one and enforce.
7. **S10**: Handle `SignedString` error — fail login with 500, log error.
8. **S7**: Add rate-limit rules in `handlers/handlers.go` for:
   - `POST /api/v1/admin/userpool` (1 req/10s)
   - `DELETE /api/v1/admin/userpool/:name` (1 req/10s)
   - `POST /api/v1/admin/db/import` (1 req/min)
   - `POST /api/v1/admin/cloudinit` (5 req/min)
9. **S11**: Reduce refresh TTL to 48h. (Revocation list = bigger effort, defer.)
10. **S9**: Add `readOnlyRootFilesystem: true` to deployment spec; verify `/data` rw volume mount sufficient.

### Phase C — Polish (backlog)
11. **S12**: Generic message for DB import errors; log detail server-side only.
12. **S13**: Parse Origin + X-Forwarded-Host as `url.URL`, compare `.Hostname()` only.
13. **S14**: Helm `NOTES.txt` pre-flight: fail if placeholder secrets detected.
14. **S15**: Add `NetworkPolicy` restricting egress to Proxmox endpoint + DNS only.
15. **S16**: Optional — temporary lockout after N failures (in-memory map with TTL).
16. **S17**: Replace direct `.(bool)` with `isAdminFromCtx(r)` for consistency.
17. **S18**: Drop `unsafe-inline`/`unsafe-eval` from CSP after Go templates removed (SvelteKit-only).
18. **S19**: Doc note in README-deploy on DB volume umask.

### Phase D — Continuous (CI/CD)
- Add `gosec ./...` to `make qualif`.
- Add `govulncheck ./...` to CI.
- Add `trufflehog` or `gitleaks` pre-commit hook for secret scanning.
- Track findings in `SECURITY.md`.

---

## Critical files to modify

| File | Phase | Change |
|---|---|---|
| `helm/values.yaml` | A | `PROXMOX_VERIFY_SSL: "true"` |
| `pvmss-deployment.yaml` | A | `PROXMOX_VERIFY_SSL=true` + `readOnlyRootFilesystem: true` |
| `example.env` | A | `PROXMOX_VERIFY_SSL=true` |
| `backend/api/v1/vm_create.go` | A | Allowlist checks (ISO, bridge, node, storage) |
| `backend/api/v1/admin_mutations.go` | A | Pool name regex; tag regex on DeleteTag/SetTagColor |
| `backend/api/v1/validation.go` (NEW or extend) | A | Centralize validators — reuse for refactor plan |
| `backend/proxmox/cloudinit.go` | B | Filename regex boundary check |
| `backend/cloudinit/validator.go` | B | Semantic blacklist or trust-model doc |
| `backend/api/v1/auth.go` | B | Handle `SignedString` error; reduce refresh TTL |
| `backend/handlers/handlers.go` | B | Rate-limit rules for admin mutators |
| `backend/api/v1/admin_db.go` | C | Sanitize import error message |
| `backend/api/v1/vnc.go` | C | Origin parser fix |
| `backend/api/v1/middleware.go:55` | C | Use `isAdminFromCtx` |
| `backend/security/middleware/headers.go` | C | CSP tightening (post-SvelteKit) |
| `helm/templates/NOTES.txt` | C | Placeholder pre-flight |
| `pvmss-deployment.yaml` | C | NetworkPolicy |
| `Makefile` | D | `gosec`, `govulncheck` targets |

---

## Existing utilities to reuse

- `backend/security/csrfgen.go` — CSRF helpers (do NOT recreate)
- `backend/security/init.go` — session manager construction
- `backend/security/middleware/headers.go` — security headers stack
- `backend/env/loader.go` — env validation pattern (extend for new vars)
- `backend/errors/errors.go` — `*ValidationError`, sentinels for clean error mapping
- `backend/api/v1/middleware.go` — `isAdminFromCtx`, `usernameFromCtx` (safe assertion pattern)
- `backend/handlers/middleware_utils.go:103-117` — header redaction helper
- `backend/api/v1/admin_mutations.go:24` — `tagNameRegex` (extend pattern for pool/bridge/iso)
- `crypto/subtle.ConstantTimeCompare` — already used for CSRF, reuse for any new token compare

---

## Verification

After each phase:
```bash
make go-fmt
make go-lint
make test-offline-race
gosec ./backend/...                          # add to CI
govulncheck ./backend/...                    # add to CI
make dev                                     # smoke test
```

### Manual security regression
1. **TLS**: `curl -v https://pvmss.local/api/v1/health` — confirm HSTS, X-Frame-Options, CSP headers.
2. **AuthZ**: Login as non-admin, attempt `POST /api/v1/admin/userpool` → expect 403.
3. **Allowlist**: Submit VM create with `iso: "../../etc/passwd"` or unlisted ISO → expect 400.
4. **Pool name**: `POST /api/v1/admin/userpool {"pool":"bad name!"}` → expect 400.
5. **Tag mutation**: `DELETE /api/v1/admin/tags/bad%2Fname` → expect 400.
6. **CSRF**: POST without token → expect 403.
7. **Rate limit**: 10x failed login → expect 429 after 5.
8. **PROXMOX_VERIFY_SSL**: Start with invalid CA → expect startup failure (or explicit log warning if dev mode).
9. **K8s**: `kubectl exec` shell into pod → expect "exec: no such file" (distroless).
10. **Cookies**: Inspect Set-Cookie — `HttpOnly`, `Secure` (in prod), `SameSite=Lax` (session) or `Strict` (JWT).

### CI gate
Add to GitHub Actions:
```yaml
- run: gosec -severity medium -confidence medium ./...
- run: govulncheck ./...
- run: gitleaks detect --no-git -v
```

---

## Risk & sequencing

- **Phase A** = production blockers, ship as standalone PR. No behavioural risk if allowlist data is correct in settings DB.
- **Phase B** = single PR, low blast radius. Includes JWT refresh TTL reduction → users will re-auth more often (acceptable trade-off).
- **Phase C** = polish, individual small PRs.
- **Phase D** = CI infra PR, no runtime change.

**Total estimated effort:** Phase A: 4-6h. Phase B: 6-8h. Phase C: 3-4h. Phase D: 2h.
