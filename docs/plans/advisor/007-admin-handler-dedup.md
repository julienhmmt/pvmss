# Plan 007: Extract shared settings-update helper to de-duplicate admin tag/limit/pool handlers

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report. When done, update the status row in `docs/plans/advisor/README.md`.
>
> **Drift check (run first)**: `git diff --stat d427838..HEAD -- backend/api/v1/admin_tags.go backend/api/v1/admin_limits.go backend/api/v1/admin_pools.go backend/api/v1/request.go`
> If any in-scope file changed since this plan was written, compare "Current
> state" excerpts against live code before proceeding; on mismatch, treat as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: M
- **Risk**: MED (refactor across ~10 admin handlers; must preserve DB-vs-in-memory branching and audit logging)
- **Depends on**: plan 005 (characterization tests for at least the tag/limit handlers — needed to refactor safely; if 005 doesn't cover tags/limits, add those tests first as Step 0)
- **Category**: tech-debt
- **Planned at**: commit `d427838`, 2026-07-01

## Why this matters

The admin mutation handlers (`admin_tags.go`, `admin_limits.go`, `admin_pools.go`, `admin_profiles.go`) repeat an identical "mutate settings → branch on `h.state.HasDB()` → either `SetTags/SetLimits(...)` via DB with `usernameFromCtx(r)` audit, OR copy settings, mutate, `SetSettings` in-memory" block. In `admin_tags.go` alone it appears twice (CreateTag and DeleteTag) and is near-identical. Divergent copies drift: one path may forget `copyNodeLimits`, another may skip audit. Extracting one helper removes the risk and shrinks the handlers.

## Current state

**The duplicated block — `admin_tags.go` CreateTag (lines 87-100):**
```go
if h.state.HasDB() {
    if err := h.state.SetTags(newTags, usernameFromCtx(r)); err != nil {
        writeAppError(w, err)
        return
    }
} else {
    newSettings := *settings
    newSettings.Tags = newTags
    newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
    if err := h.state.SetSettings(&newSettings); err != nil {
        writeAppError(w, err)
        return
    }
}
```

**DeleteTag (lines 189-202) — structurally identical** but `newTags` is the filtered list. Both branches do the same DB-vs-memory split with the same error handling. `admin_limits.go` and `admin_profiles.go` follow the same shape with `SetLimits`/`SetProfiles` instead of `SetTags`.

Repo conventions to match:
- Helpers for request handling live in `backend/api/v1/request.go` (e.g. `decodeBody`, `writeJSON`, `writeAppError`, `errBadRequest`). Add the new helper there.
- The `copyNodeLimits` helper exists (used at `admin_tags.go:95,197`) — preserve its use in the in-memory path.
- Handlers are methods on `*AdminMutationsHandler` (see `admin_tags.go:21,66,108,163`).
- Error handling: `writeAppError(w, err)` + `return` (see `admin_tags.go:88-89,96-98`).

## Commands you will need

| Purpose   | Command                                                                  | Expected on success |
|-----------|--------------------------------------------------------------------------|---------------------|
| Test      | `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -timeout=5m ./...` | all pass |
| Lint      | `cd backend && golangci-lint run --timeout=3m`                           | exit 0              |

## Scope

**In scope**:
- `backend/api/v1/request.go` (add the shared helper)
- `backend/api/v1/admin_tags.go` (refactor CreateTag, DeleteTag to use it)
- `backend/api/v1/admin_limits.go` (refactor UpdateLimits to use it, if the shape matches)
- `backend/api/v1/admin_profiles.go` (refactor create/update/delete to use it, if the shape matches)
- `backend/api/v1/admin_pools.go` (only if it has the same settings-mutation block; pool handlers also do Proxmox calls — do NOT fold the Proxmox calls into the helper)

**Out of scope**:
- `admin_pools.go` Proxmox orchestration (EnsureRole/EnsureUser/EnsurePool) — complex, not duplicated boilerplate
- `vm_create.go`, `vm_details.go`
- The `ListTags`/`ListPools` read handlers (no settings mutation)
- Any change to `state.StateManager` interface

## Git workflow

- Branch: `advisor/007-admin-handler-dedup`
- Commits: introduce helper + refactor one handler per commit so a regression bisects cleanly.
- Do NOT push unless instructed.

## Steps

### Step 0 (if needed): Ensure characterization tests exist for tag/limit handlers

If plan 005 did not add `admin_tags_test.go`/`admin_limits_test.go`, add minimal characterization tests first (model on `admin_db_test.go`): `TestCreateTag_<Scenario>`, `TestDeleteTag_<Scenario>` covering DB and in-memory paths. This is a prerequisite so the refactor in Step 2 is verifiable.

**Verify**: `go test -run 'TestCreateTag|TestDeleteTag' -v ./api/v1/` → passes before refactoring.

### Step 1: Add the shared helper in request.go

Add a helper that encapsulates the DB-vs-in-memory settings mutation. Design it around the common shape. Because the setters differ (`SetTags`, `SetLimits`, `SetProfiles`), pass the DB-write as a function and the in-memory mutation as a function:
```go
// persistSettingsChange runs the DB-backed mutation when a DB is present,
// otherwise applies the in-memory mutation and calls SetSettings. Both paths
// preserve the copyNodeLimits defensive copy used by admin handlers.
func (h *AdminMutationsHandler) persistSettingsChange(
    w http.ResponseWriter,
    r *http.Request,
    dbWrite func(username string) error,
    memMutate func(settings *Settings) *Settings,
) bool {
    if h.state.HasDB() {
        if err := dbWrite(usernameFromCtx(r)); err != nil {
            writeAppError(w, err)
            return false
        }
        return true
    }
    settings := h.state.GetSettings()
    newSettings := memMutate(settings)
    newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
    if err := h.state.SetSettings(newSettings); err != nil {
        writeAppError(w, err)
        return false
    }
    return true
}
```
(Adjust the `Settings` type name to the actual exported type used in `admin_tags.go` — confirm via `h.state.GetSettings()` return type. The `bool` return = success; caller returns on false.)

**Verify**: `cd backend && go build ./...` + `go vet ./...` → exit 0.

### Step 2: Refactor admin_tags.go CreateTag + DeleteTag

Replace the inline `if h.state.HasDB() { ... } else { ... }` blocks (lines 87-100 and 189-202) with calls to `h.persistSettingsChange(...)`. For CreateTag:
```go
if !h.persistSettingsChange(w, r,
    func(user string) error { return h.state.SetTags(newTags, user) },
    func(s *Settings) *Settings { ns := *s; ns.Tags = newTags; return &ns },
) {
    return
}
```
Mirror for DeleteTag with the filtered `newTags`. Preserve the `w.WriteHeader`/`writeJSON` responses that follow.

**Verify**: `go test -run 'TestCreateTag|TestDeleteTag' -v ./api/v1/` → passes (behavior unchanged). `grep -n "h.state.HasDB()" backend/api/v1/admin_tags.go` → no matches (both inline branches gone).

### Step 3: Refactor admin_limits.go and admin_profiles.go (where the shape matches)

For each handler whose settings-mutation block matches the CreateTag shape, apply the same replacement. If a handler's mutation is more complex (e.g. upsert-then-delete node limits), leave it inline and note it in the maintenance section — do not force-fit.

**Verify**: `go test -timeout=5m ./...` → all pass. `grep -rn "h.state.HasDB()" backend/api/v1/admin_*.go` → reduced count (only complex handlers remain).

## Test plan

- Step 0 characterization tests (CreateTag/DeleteTag DB + in-memory paths) must pass before and after refactor.
- Full offline suite: `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -timeout=5m ./...` → all pass.
- Manual diff review: the refactor must not change which setter is called or drop `copyNodeLimits` or the `usernameFromCtx(r)` audit argument.

## Done criteria

- [ ] `persistSettingsChange` (or equivalent) exists in `request.go`
- [ ] `admin_tags.go` CreateTag and DeleteTag use the helper; no inline `HasDB` branches remain in that file
- [ ] `admin_limits.go`/`admin_profiles.go` use the helper where the shape matched (complex mutations documented as intentionally left inline)
- [ ] `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -timeout=5m ./...` exits 0
- [ ] `cd backend && golangci-lint run --timeout=3m` exits 0
- [ ] No handler's public response shape or status code changed (diff review)
- [ ] `docs/plans/advisor/README.md` status row updated

## STOP conditions

Stop and report if:
- The `Settings` type / `GetSettings` return type isn't a struct pointer or doesn't allow the `*ns = *s` shallow copy (the helper assumes value-copy semantics — adjust or report).
- `SetTags`/`SetLimits`/`SetProfiles` have different signatures (e.g. extra args) — the `dbWrite func(username string) error` closure may not fit; report the actual signatures.
- A handler's in-memory path does something other than "copy settings, set one field, SetSettings" (e.g. merges node limits) — do not force it through the helper; leave inline and document.
- Step 0 tests reveal existing behavior differs between DB and in-memory paths (a latent bug) — report; do not "fix" it silently during refactor.

## Maintenance notes

- The helper deliberately keeps `copyNodeLimits` in the in-memory path — any future settings field that needs defensive copying must be added to the helper, not per-handler.
- If `state.StateManager` later drops the in-memory (non-DB) mode, this helper can be simplified to just the `dbWrite` path.
- A reviewer should confirm the `usernameFromCtx(r)` audit argument still flows through `dbWrite` for every refactored handler.
