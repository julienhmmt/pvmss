# Remove Deprecated Functions

## Findings

### 1. Explicitly Deprecated: `NewHandlerContext`

**File:** `backend/handlers/helpers.go:149`

```go
// NewHandlerContext creates a new handler context with common setup
// Deprecated: Use HandlerContextWith instead
func NewHandlerContext(w http.ResponseWriter, r *http.Request, handlerName string) *HandlerContext {
    return HandlerContextWith(w, r, handlerName)
}
```

`HandlerContextWith` is defined in `backend/handlers/handler_context.go:30` and is the current implementation.
`NewHandlerContext` is a thin pass-through wrapper with no logic.

**Call sites to update (25 occurrences in 10 files):**

| File | Occurrences |
|------|-------------|
| `backend/handlers/auth.go` | 3 |
| `backend/handlers/common.go` | 1 |
| `backend/handlers/profile.go` | 2 |
| `backend/handlers/vm_actions_resources.go` | 2 |
| `backend/handlers/vm_delete.go` | 4 |
| `backend/handlers/vm_actions_lifecycle.go` | 8 |
| `backend/handlers/disks.go` | 1 |
| `backend/handlers/admin_cloudinit.go` | 1 |
| `backend/handlers/vm_actions_misc.go` | 2 |
| `backend/handlers/search.go` | 1 |
| `backend/handlers/vm_details_info.go` | 1 |
| `backend/handlers/security_test.go` | 1 |

**Fix:** Replace every `NewHandlerContext(` with `HandlerContextWith(` across all call sites, then delete the deprecated function from `helpers.go`.

---

### 2. Telmate Migration TODOs (not deprecated yet, but flagged for removal)

These are not marked `Deprecated` but are explicitly marked with `TODO Telmate migration` comments indicating they must be replaced with Resty-based equivalents before being deleted. They are a separate, larger effort tracked in the Telmate migration.

| File | Function / Note |
|------|----------------|
| `backend/proxmox/telmate_client.go:146` | JSON helper — replace with Resty helpers |
| `backend/proxmox/telmate_client.go:252,269` | Cache layer tied to Telmate client |
| `backend/proxmox/vms.go` | `GetVMConfig`, `UpdateVMConfig`, `GetVMCurrent`, `VMAction`, `DeleteVM` — all have Resty equivalents |
| `backend/proxmox/access.go` | Ticket, admin user, password, pool, ACL, roles helpers |
| `backend/proxmox/vnc.go` | VNC proxy ticket |
| `backend/proxmox/nodes.go` | Node detail and name helpers |
| `backend/state/manager.go` | Telmate client field on `AppState` |
| `backend/state/interface.go` | `GetProxmoxClient` / `SetProxmoxClient` interface methods |
| `backend/state/manager_proxmox.go` | Getters, setters, health check |
| `backend/main.go` | Bootstrap, health check using Telmate |
| `backend/handlers/search.go` | Search using `GetVMConfigWithContext` |
| `backend/handlers/user_pool.go` | Pool/user management using Telmate |
| `backend/handlers/profile.go` | Profile pool membership and password flow |
| `backend/handlers/vm_actions_misc.go` | VM config update using Telmate |
| `backend/handlers/vm_details_info.go` | Guest agent data using Telmate |
| `backend/handlers/storage.go` | Node listing fallback using Telmate |
| `backend/handlers/vm_delete.go` | Cache invalidation using Telmate |
| `backend/handlers/vm_console_helpers.go` | VNC proxy using Telmate cookie client |
| `backend/handlers/settings_limits.go` | Limit validation using Telmate nodes |

These are **blocked on the Resty migration** — they cannot be deleted until Resty replacements exist. They are **out of scope** for this plan and tracked separately by the `TODO Telmate migration` comments in-code.

---

## Implementation Plan

### Step 1: Replace all `NewHandlerContext` call sites

Run from repo root:

```bash
find backend -name '*.go' | xargs sed -i '' 's/\bNewHandlerContext\b/HandlerContextWith/g'
```

### Step 2: Delete the deprecated function

Remove lines 149–153 from `backend/handlers/helpers.go`:

```go
// NewHandlerContext creates a new handler context with common setup
// Deprecated: Use HandlerContextWith instead
func NewHandlerContext(w http.ResponseWriter, r *http.Request, handlerName string) *HandlerContext {
    return HandlerContextWith(w, r, handlerName)
}
```

### Step 3: Verify

```bash
make go-fmt
make test-offline
make go-lint
```

Also confirm no remaining references:

```bash
grep -r 'NewHandlerContext' backend/
# Should return no results
```

---

## Summary

| Category | Count | Action |
|----------|-------|--------|
| Explicitly deprecated functions | 1 (`NewHandlerContext`) | Delete now |
| Call sites to update | 25 across 10 files | Replace with `HandlerContextWith` |
| Telmate migration TODOs | ~20 functions | Blocked — remove after Resty migration |
