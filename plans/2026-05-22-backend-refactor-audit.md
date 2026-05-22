# Backend Refactor & Factorization Audit

**Date:** 2026-05-22
**Scope:** `backend/` Go code (~32K LOC, 152 files, 14 packages)
**References:**
- [10 Go Error Handling Commandments](https://preslav.me/2026/05/19/10-golang-error-handling-commandments/)
- [Go Structs vs Pointers: Pointer-First](https://preslav.me/2026/01/08/golang-structs-vs-pointers-pointer-first/)

---

## Context

PVMSS backend grew organically. `api/v1/` now holds 10.3K LOC across 28 files, with three monolithic handler files (admin_mutations.go: 1482, vm_create.go: 1305, vm_details.go: 1002). Two parallel error-helper systems coexist (`api/v1/errors.go` 48 LOC vs `handlers/error_handling.go` 162 LOC). HTTP-layer test coverage is 0%. Goal: simplify, factorize, align with current Go idioms — without behavioural changes in this pass.

Existing strengths (keep): pointer-receiver discipline already consistent; custom error hierarchy in `errors/` already supports `errors.Is/As`; pointer-first struct passing is already the norm; no panics in production code; all `go.mod` deps used.

---

## Findings vs Preslav's Commandments

### Error handling — gaps

| # | Commandment | Current state | Action |
|---|---|---|---|
| 2 | Wrap at package boundaries with `%w` | Inconsistent — some sites use `%w`, many use raw `fmt.Errorf` or bare strings | Standardize wrapping at proxmox/database/handlers seams |
| 4 | Action-phrase messages ("placing order:") | Many "failed to X" / "cannot X" strings | Rewrite during touch |
| 6 | Branch via `errors.Is`/`errors.As`, not strings | Mostly good (`database/`), but a few string compares remain | Audit + replace |
| 7 | `%w` is API promise — wrap own, `%v` foreign | No discrimination — third-party errors leak via `%w` | Convert resty/sqlite errors to domain sentinels at boundary |
| 8 | Translate foreign → domain sentinels | Partial — `errors/errors.go` has sentinels but Proxmox/SQLite errors often bubble raw | Add translation layer in `proxmox/` and `database/` |
| 9 | Never log AND return | 30+ sites do both (handler logs then `errInternal(w)`) | Decide single owner (handler logs at terminal point only) |
| 10 | Goroutine errors via `errgroup`/buffered chan | Few goroutines, but `proxmox/` multi-node aggregation uses ad-hoc patterns | Migrate to `golang.org/x/sync/errgroup` |

### Structs vs pointers — already mostly compliant

Backend already pointer-first. Two minor exceptions to verify (not block):
- `cloudinit/` small value structs — leave as values (article exception)
- API request/response anonymous structs — fine as values for short-lived decode targets

**No work needed** on receiver/passing style. This article validates current approach.

---

## Findings — code smells

### P1 — Monolithic handler files
- `api/v1/admin_mutations.go` (1482 LOC, 13+ public methods)
- `api/v1/vm_create.go` (1305 LOC)
- `api/v1/vm_details.go` (1002 LOC)
- `api/v1/admin_settings_overview.go` (704 LOC)
- `api/v1/vm_disks.go` (666 LOC)

### P2 — Duplicate error-helper systems
- `api/v1/errors.go` (48 LOC) — `writeError`, `errInternal`, `errBadRequest`, `errNotFound`
- `handlers/error_handling.go` (162 LOC) — overlapping `codeToHTTPStatus`, similar helpers
- Confusion about which to use when

### P3 — Repeated handler boilerplate (>30 sites)
```go
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    errBadRequest(w, "invalid JSON body")
    return
}
```
Also: repeated CSRF/session/auth extraction, repeated route-param parsing.

### P4 — Inline anonymous request/response structs
Each large handler defines local `detailResp`, `listResp`, `createReq` types. Hurts grep, hurts reuse, hides API surface.

### P5 — Magic strings scattered
`"pvmss_"` pool prefix, `"@pve"` username suffix, `"pvmss"` tag — hardcoded across files. `constants/` package exists but underused.

### P6 — Scattered validation
Regex compiled per call (`tagNameRegex.MatchString`), inline `if req.Field == ""` checks, no schema layer.

### P7 — Test gaps
| Package | Coverage |
|---|---|
| `api/v1/` | 0 of 28 handler files tested (vnc_test, setup_test, admin_db_test, admin_settings_overview_test, vm_details_snapshot_test exist but cover slice) |
| `security/` | 0 of 5 |
| `handlers/` | 2 of 19 |
| `proxmox/` | 3 of 24 |

---

## Plan — phased

### Phase 1 — Consolidate error helpers (low risk, high clarity)
**Files:** `backend/api/v1/errors.go`, `backend/handlers/error_handling.go`
- Pick `api/v1/errors.go` as canonical for HTTP responses (closer to handlers).
- Move `codeToHTTPStatus` + status mapping from `handlers/` into `api/v1/errors.go`.
- Delete `handlers/error_handling.go` or shrink to non-HTTP helpers only.
- Add `writeAppError(w, err)` that unwraps `*errors.AppError` and maps to status automatically.
- All callers: replace `errInternal(w); log.Error()` with `writeAppError(w, fmt.Errorf("loading vm %s: %w", id, err))` — single log site inside helper.

**Verify:** `make test-offline` green; manual check that error JSON shape unchanged.

### Phase 2 — Extract handler boilerplate
**New files:**
- `backend/api/v1/request.go` — `decodeJSON[T](r) (T, error)` generic, `requireParam(r, name)`, `requireUser(r)` (session extraction)
- `backend/api/v1/response.go` — already covered by Phase 1's `writeJSON`

**Replace 30+ sites** of inline JSON decode + param parsing with helpers. Use Go 1.18+ generics (already on modern Go).

### Phase 3 — Split monolithic handler files
Target the three giants. Strategy: split by resource, keep package `v1`, one file per resource:

`admin_mutations.go` (1482) →
- `admin_pools.go` (Create/Delete/List Pools)
- `admin_tags.go` (Tag CRUD)
- `admin_limits.go` (VM resource limits)
- `admin_quotas.go` (per-user quotas)

`vm_create.go` (1305) →
- `vm_create.go` (orchestrator, ≤300 LOC)
- `vm_create_validation.go` (request validation)
- `vm_create_resolve.go` (node/storage/bridge resolution — extract `resolveNodes()`)
- `vm_create_cloudinit.go` (cloud-init wiring)

`vm_details.go` (1002) →
- `vm_details.go` (handler, ≤300 LOC)
- `vm_details_snapshot.go` (already partially separate via test file)
- `vm_details_usage.go` (extract `computeNodeUsageFromSnapshot`)

**Constraint:** pure file splitting. No signature changes. Tests pass unchanged.

### Phase 4 — Domain error translation (Preslav #7, #8)
**Files:** `backend/proxmox/*.go`, `backend/database/*.go`
- `proxmox/`: wrap all resty errors via `fmt.Errorf("calling proxmox %s: %v", op, err)` (use `%v` for foreign per #7) and convert HTTP 404 → `errors.ErrNotFound`, 409 → `errors.ErrConflict`, 401/403 → `errors.ErrUnauthorized`.
- `database/`: already does `errors.Is(err, sql.ErrNoRows)` → translate at query site to `errors.ErrNotFound`.
- Handlers can then `errors.Is(err, errors.ErrNotFound)` without caring about source.

### Phase 5 — Validation layer
**New file:** `backend/api/v1/validation.go`
- Hoist all `regexp.MustCompile` to package-level vars (some already are, audit).
- `validateTagName(s) error`, `validatePoolName(s) error`, `validateVMID(n) error` returning typed `*errors.ValidationError`.
- Replace inline `if req.X == ""` with `v.Required("field", req.X)` builder.

### Phase 6 — Extract magic strings
**File:** `backend/constants/proxmox.go` (new)
- `PoolPrefix = "pvmss_"`, `UserSuffix = "@pve"`, `RequiredTag = "pvmss"`.
- Replace literals across `api/v1/`, `proxmox/`, `database/`.

### Phase 7 — Test coverage (separate effort, scope-aware)
Out of scope for refactor PR. Track in follow-up:
- `api/v1/`: table-driven handler tests using `httptest` + offline mode (`PVMSS_OFFLINE=true`).
- `security/`: middleware unit tests.
- `proxmox/`: client tests with `httptest.Server` mock.

Target +30% coverage in dedicated PR.

### Phase 8 — Removals (verify before deleting)
- `handlers/error_handling.go` (after Phase 1)
- Any dead helpers surfaced during file splits (run `deadcode` tool)
- `backend/types.go` (348 bytes — check what's there, likely fold into `app/` or delete)

---

## Critical files to modify

| File | Action |
|---|---|
| `backend/api/v1/errors.go` | Expand: absorb codeToHTTPStatus, add writeAppError |
| `backend/handlers/error_handling.go` | Delete or shrink |
| `backend/api/v1/admin_mutations.go` | Split into 4 files by resource |
| `backend/api/v1/vm_create.go` | Split into 4 files by concern |
| `backend/api/v1/vm_details.go` | Split into 3 files |
| `backend/api/v1/request.go` | NEW — decode/param helpers |
| `backend/api/v1/validation.go` | NEW — input validators |
| `backend/api/v1/types.go` | Expand — hoist inline request/response structs |
| `backend/constants/proxmox.go` | NEW — magic strings |
| `backend/proxmox/*.go` | Add domain-error translation at boundary |
| `backend/database/*.go` | Translate sql errors to domain sentinels |

---

## Existing utilities to reuse (do NOT recreate)

- `backend/errors/errors.go` — `AppError`, sentinels (`ErrNotFound`, `ErrConflict`, `ErrUnauthorized`, `ErrValidation`), `*VMError`, `*ProxmoxError`, `*ValidationError`
- `backend/utils/generics.go` — generic helpers (audit before adding new)
- `backend/api/v1/errors.go` — existing `writeError`, `writeJSON`, `errInternal`, `errBadRequest`, `errNotFound`
- `backend/logger/` — structured zerolog wrappers
- `backend/state/StateManager` — shared session/settings/cache (DO NOT duplicate state)
- `golang.org/x/sync/errgroup` — for Phase 4 multi-node fanout (add to go.mod)

---

## Verification

After each phase:
```bash
make go-fmt
make go-lint
make test-offline-race    # race + offline tests
make coverage             # confirm no coverage regression
make dev                  # smoke test container boots, hits /api/v1/health
```

End-to-end smoke (manual, after all phases):
1. Login as admin, list pools, create pool, delete pool
2. Create VM end-to-end (validates vm_create.go split)
3. View VM details (validates vm_details.go split)
4. Tag CRUD, limits CRUD (validates admin_mutations.go split)
5. Trigger known error paths (404 vm, 409 duplicate pool) — confirm JSON error shape unchanged

---

## Risk & sequencing

- **Phase 1–2** safe, mechanical, mergeable independently.
- **Phase 3** large diff but no logic change — review file-by-file.
- **Phase 4** behavioural — error types change at handler edge; needs handler audit.
- **Phase 5–6** mechanical.
- **Phase 7** separate PR.
- **Phase 8** last, after all references gone.

Recommended PR split: P1+P2 together, P3 alone, P4+P5+P6 together, P7 separate, P8 final cleanup.
