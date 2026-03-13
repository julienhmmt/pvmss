# Remove Telmate Dependency: Full Migration to Resty

## Objective

Fully eliminate `github.com/Telmate/proxmox-api-go` from the codebase.
All Proxmox API calls will use the `RestyClient` (`backend/proxmox/resty_client.go`).

## Current State

The codebase is midway through the migration. Most VM, node, storage, snapshot,
cloud-init, and VMBR operations already use Resty. What remains is:

| Category | Telmate Functions Still in Use | Call Sites |
|----------|-------------------------------|------------|
| VM config | `GetVMConfigWithContext`, `UpdateVMConfigWithContext` | search.go ×5, vm_actions_misc.go ×2 |
| Nodes | `GetNodeNamesWithContext`, `GetNodeNames` | main.go, vm_details_info.go, storage.go ×2, vmbr.go, manager_proxmox.go |
| Cluster | `GetClusterStatus` | admin.go |
| VNC | `GetVNCProxy` | vm_console_helpers.go |
| Auth/Tickets | `CreateTicket`, `HasRole` | auth.go ×4, profile.go ×2 |
| Password | `UpdateUserPassword` | profile.go |
| User/Pool admin | `EnsureUser`, `EnsureRole`, `EnsurePool`, `EnsurePoolACL` | user_pool.go ×8 |
| StateManager | `GetProxmoxClient()`, `SetProxmoxClient()` | 20+ handlers |

---

## Groups of Work

### Group A — Simple Swaps (Resty equivalents already exist)

These are pure call-site replacements. Resty versions have identical semantics.

#### A1. `GetVMConfigWithContext` → `GetVMConfigResty`

**File:** `backend/handlers/search.go`

| Line | Current | Replacement |
|------|---------|-------------|
| 286 | `proxmox.GetVMConfigWithContext(ctx, client, vm.Node, vm.VMID)` | `proxmox.GetVMConfigResty(ctx, restyClient, vm.Node, vm.VMID)` |
| 320 | same | same |
| 361 | same | same |
| 422 | same | same |
| 763 | same | same |
| 792 | same | same |

Also remove `client := h.stateManager.GetProxmoxClient()` at lines 179 and 648.

#### A2. `UpdateVMConfigWithContext` → `UpdateVMConfigResty`

**File:** `backend/handlers/vm_actions_misc.go`

| Line | Current | Replacement |
|------|---------|-------------|
| 53 | `proxmox.UpdateVMConfigWithContext(r.Context(), client, node, vmidInt, map[string]string{...})` | `proxmox.UpdateVMConfigResty(r.Context(), restyClient, node, vmidInt, map[string]string{...})` |
| 102 | same | same |

Remove `client := ctx.StateManager.GetProxmoxClient()` at lines 47 and 96.

#### A3. `GetNodeNamesWithContext` / `GetNodeNames` → `GetNodeNamesResty`

| File | Line | Current | Replacement |
|------|------|---------|-------------|
| `main.go` | 228 | `proxmox.GetNodeNamesWithContext(ctx, client)` | `proxmox.GetNodeNamesResty(ctx, restyClient)` |
| `vm_details_info.go` | 54 | `proxmox.GetNodeNamesWithContext(r.Context(), client)` | `proxmox.GetNodeNamesResty(r.Context(), restyClient)` |
| `storage.go` | 295 | `proxmox.GetNodeNames(client)` | `proxmox.GetNodeNamesResty(ctx, restyClient)` |
| `storage.go` | 480 | `proxmox.GetNodeNames(client)` | `proxmox.GetNodeNamesResty(ctx, restyClient)` |
| `vmbr.go` | 221 | `proxmox.GetNodeNames(client)` | `proxmox.GetNodeNamesResty(ctx, restyClient)` |
| `state/manager_proxmox.go` | 132 | `proxmox.GetNodeNamesWithContext(ctx, client)` | `proxmox.GetNodeNamesResty(ctx, restyClient)` |

#### A4. `GetClusterStatus` → `GetClusterStatusResty`

**File:** `backend/handlers/admin.go:344`

```go
// Before
proxmox.GetClusterStatus(r.Context(), client)

// After
proxmox.GetClusterStatusResty(r.Context(), restyClient)
```

Remove `client := h.stateManager.GetProxmoxClient()` at line 342.

---

### Group B — New Resty Functions Required

These Telmate functions have no Resty counterparts yet. Write them in `backend/proxmox/`.

#### B1. `GetVNCProxyResty` (new file: `proxmox/vnc_resty.go`)

**Endpoint:** `POST /nodes/{node}/qemu/{vmid}/vncproxy`

The Proxmox VNC proxy endpoint accepts API token auth.
The existing Telmate version used a cookie client, but API token auth works for vncproxy too.

```go
type VNCProxyResponse struct {
    Ticket     string `json:"ticket"`
    Port       string `json:"port"`
    User       string `json:"user"`
    Cert       string `json:"cert"`
}

func GetVNCProxyResty(ctx context.Context, client *RestyClient, node string, vmid int, opts map[string]string) (*VNCProxyResponse, error)
```

Form params: `websocket=1`, optional `generate-password`.

**Handler update:** `backend/handlers/vm_console_helpers.go:76`
- Replace `proxmox.GetVNCProxy(ctx, client, node, vmidInt, nil)` with `proxmox.GetVNCProxyResty(ctx, restyClient, node, vmidInt, nil)`
- Remove the cookie client setup in the function

#### B2. `CreateTicketResty` + `HasRoleFromCap` (new file: `proxmox/auth_resty.go`)

**`CreateTicket`** is used for Proxmox-account login (admin login page + profile password flow).
It calls `POST /access/ticket` with username/password — no auth token needed.

```go
type TicketRequest struct {
    Username string
    Password string
    Realm    string // "pve" default
}

type TicketResponse struct {
    Ticket              string                 `json:"ticket"`
    CSRFPreventionToken string                 `json:"CSRFPreventionToken"`
    Username            string                 `json:"username"`
    Cap                 map[string]interface{} `json:"cap"`
}

// CreateTicketResty calls POST /access/ticket without API token auth.
// Uses a plain (unauthenticated) Resty client with just the base URL.
func CreateTicketResty(ctx context.Context, baseURL string, username, password string) (*TicketResponse, error)
```

Note: `HasRole` is pure logic (parses `Cap` field) — it does not call the Proxmox API.
It stays as-is or is renamed; no Resty migration needed for it.

**Handler updates:**
- `backend/handlers/auth.go:462` — replace `proxmox.CreateTicket(ctx, pxClient, ...)` with `proxmox.CreateTicketResty(ctx, baseURL, ...)`
- `backend/handlers/auth.go:602` — same
- `backend/handlers/profile.go:472` — same
- `backend/handlers/profile.go:495` — same

#### B3. `UpdateUserPasswordResty` (add to `proxmox/auth_resty.go`)

**Endpoint:** `PUT /access/password` with `CSRFPreventionToken` header + `PVEAuthCookie` cookie.

This requires cookie-based auth (ticket + CSRF token). After calling `CreateTicketResty`,
use the returned `Ticket` as cookie `PVEAuthCookie` and `CSRFPreventionToken` as header.

```go
type CookieAuthOptions struct {
    PVEAuthCookie       string
    CSRFPreventionToken string
}

func UpdateUserPasswordResty(ctx context.Context, baseURL, username, newPassword, currentPassword, realm string, cookieAuth CookieAuthOptions) error
```

**Handler update:** `backend/handlers/profile.go:486`

#### B4. Admin access functions (new file: `proxmox/access_resty.go`)

Replace `EnsureUser`, `EnsureRole`, `EnsurePool`, `EnsurePoolACL` with Resty equivalents.
These are admin operations using API token auth — direct Resty migration.

```go
// EnsureUserResty — GET /access/users/{userid}, POST /access/users if not found
func EnsureUserResty(ctx context.Context, client *RestyClient, username, password, email, comment, realm string, enabled bool) error

// EnsureRoleResty — GET /access/roles/{roleid}, POST /access/roles if not found
func EnsureRoleResty(ctx context.Context, client *RestyClient, roleID string, privileges []string) error

// EnsurePoolResty — GET /pools/{poolid}, POST /pools if not found
func EnsurePoolResty(ctx context.Context, client *RestyClient, poolID, comment string) error

// EnsurePoolACLResty — PUT /access/acl
func EnsurePoolACLResty(ctx context.Context, client *RestyClient, userID, poolID, role string, propagate bool) error
```

**Handler updates:** `backend/handlers/user_pool.go` — 8 call sites replacing the Telmate versions.

---

### Group C — StateManager Cleanup

Once all handlers use Resty directly (via `stateManager.GetRestyClient()`), remove the Telmate client from the state manager.

#### C1. Remove from `StateManager` interface (`backend/state/interface.go`)

Remove:
```go
GetProxmoxClient() proxmox.ClientInterface   // line 29–30
SetProxmoxClient(pc proxmox.ClientInterface) error
```

The `GetRestyClient()` method already exists and is what all migrated code uses.

#### C2. Remove from `appState` struct (`backend/state/manager.go`)

Remove field:
```go
proxmoxClient proxmox.ClientInterface   // telmate client field
```

#### C3. Update `manager_proxmox.go`

- Remove `GetProxmoxClient()` and `SetProxmoxClient()` implementations
- Update `CheckProxmoxConnection()` (line 108) to use `GetNodeNamesResty` via `s.restyClient`
- Update `startProxmoxMonitor()` to not reference Telmate client

#### C4. Update `backend/main.go`

- Remove `NewClient(...)` / `NewClientCookieAuth(...)` calls
- Remove passing Telmate client to `StateManager`
- Update bootstrap health check to use Resty

#### C5. Update all handler files that call `GetProxmoxClient()`

The following files call `GetProxmoxClient()` but the actual Telmate client usage has already
been or will be replaced (Groups A/B above). After those replacements, the `GetProxmoxClient()`
call and the `client` variable become unused and are deleted.

Files to update after Groups A/B:
- `handlers/base_handlers.go:42`
- `handlers/vm_details_base.go:13`
- `handlers/vm_snapshots.go:135,232,284`
- `handlers/profile.go:89,182,449`
- `handlers/validation.go:41,60`
- `handlers/vm_actions_helpers.go:66`
- `handlers/vm_details_validation.go:65`
- `handlers/network_helpers.go:73`
- `handlers/vm_create.go:90`
- `handlers/vm_delete.go:51,155`
- `handlers/admin_vms.go:66`
- `handlers/vm_create_handler.go:518`
- `handlers/vm_console_helpers.go`
- `handlers/admin.go:342,413`
- `handlers/vm_details_metrics.go:37`
- `handlers/vm_actions_lifecycle.go:54`
- `handlers/storage.go:268`
- `handlers/user_pool.go:95,426,645,838`
- `handlers/vmbr.go:200`
- `handlers/settings_limits.go:86,299`

#### C6. Update test mocks

Remove `GetProxmoxClient()` from mock implementations:
- `backend/middleware/middleware_test.go:26`
- `backend/handlers/vm_actions_test.go:154`
- `backend/handlers/auth_guard_test.go:32`
- `backend/api/v1/middleware_test.go:35`

---

### Group D — Delete Dead Code

Once all call sites are migrated:

#### D1. Delete `backend/proxmox/telmate_client.go`

The entire file — `Client` struct, `NewClient`, `NewClientCookieAuth`, `Get`, `Post`, `Put`, `Delete`, `Login`, `InvalidateCache`, etc.

#### D2. Delete Telmate functions from existing files

**`backend/proxmox/vms.go`** — delete:
- `GetVMConfigWithContext`
- `UpdateVMConfigWithContext`
- `GetVMCurrentWithContext` (line 282 — has Resty equivalent)
- `VMActionWithContext` (line 326 — has Resty equivalent)
- `DeleteVMWithContext` (line 355 — has Resty equivalent)
- `GetGuestAgentNetworkInterfaces` (Telmate-based; replaced by `ExecuteQemuAgentCommandResty`)
- Comment block `LEGACY FUNCTIONS REMOVED` (line 308)

**`backend/proxmox/access.go`** — delete the entire file (all functions are Telmate-based; replaced by `access_resty.go`):
- `CreateTicket`
- `EnsureUser`
- `UpdateUserPassword`
- `EnsurePool`
- `EnsurePoolACL`
- `EnsureRole`
- `HasRole` — keep as a utility function (no API call); move to `auth_resty.go`

**`backend/proxmox/nodes.go`** — delete Telmate functions:
- `GetNodeDetailsWithContext`
- `GetNodeNamesWithContext`
- `GetNodeDetails` (convenience wrapper)
- `GetNodeNames` (convenience wrapper)
- `GetNodeStatus` (uses Telmate under the hood)

Keep the Resty functions in `nodes.go` or move them to `nodes_resty.go`.

**`backend/proxmox/vnc.go`** — delete `GetVNCProxy` (Telmate); keep file for `GetVNCProxyResty`.

**`backend/proxmox/cluster.go`** — delete `GetClusterStatus` (Telmate); keep `GetClusterStatusResty`.

#### D3. Delete `backend/proxmox/cache.go`

The LRU cache is only used by the Telmate client. Once Telmate is gone, it has no callers.

#### D4. Delete `ClientInterface` from `backend/proxmox/interfaces.go`

Remove the `ClientInterface` type definition entirely. If the file becomes empty, delete it.

#### D5. Remove from `go.mod` / `go.sum`

```bash
go get github.com/Telmate/proxmox-api-go@none
go mod tidy
```

---

## Implementation Order

Execute phases sequentially — each phase must compile and pass tests before the next.

### Phase 1 — Simple swaps (Group A)

All Resty equivalents already exist. Pure mechanical find-replace within each file.

1. `search.go` — replace `GetVMConfigWithContext` × 6 with `GetVMConfigResty`
2. `vm_actions_misc.go` — replace `UpdateVMConfigWithContext` × 2 with `UpdateVMConfigResty`
3. `vm_details_info.go` — replace `GetNodeNamesWithContext` with `GetNodeNamesResty`
4. `storage.go` — replace `GetNodeNames` × 2 with `GetNodeNamesResty`
5. `vmbr.go` — replace `GetNodeNames` with `GetNodeNamesResty`
6. `main.go` — replace `GetNodeNamesWithContext` with `GetNodeNamesResty`
7. `state/manager_proxmox.go` — replace `GetNodeNamesWithContext` with `GetNodeNamesResty` in health check
8. `admin.go` — replace `GetClusterStatus` with `GetClusterStatusResty`

After each file: `make test-offline` ✓

### Phase 2 — New Resty functions (Group B)

Write the new functions, then update their call sites.

1. Write `proxmox/vnc_resty.go` with `GetVNCProxyResty`
2. Update `handlers/vm_console_helpers.go` to use it
3. Write `proxmox/auth_resty.go` with `CreateTicketResty`, `UpdateUserPasswordResty`, move `HasRole`
4. Update `handlers/auth.go` × 2
5. Update `handlers/profile.go` × 3 (CreateTicket × 2, UpdateUserPassword × 1)
6. Write `proxmox/access_resty.go` with `EnsureUserResty`, `EnsureRoleResty`, `EnsurePoolResty`, `EnsurePoolACLResty`
7. Update `handlers/user_pool.go` × 8

After each file: `make test-offline` ✓

### Phase 3 — StateManager cleanup (Group C)

1. Remove `GetProxmoxClient()`/`SetProxmoxClient()` from `state/interface.go`
2. Remove Telmate client field from `state/manager.go`
3. Update `state/manager_proxmox.go`
4. Update `main.go` — remove Telmate client bootstrap
5. Clean up all handler files that had leftover `GetProxmoxClient()` calls
6. Update test mocks to remove `GetProxmoxClient()`

`make test-offline` ✓

### Phase 4 — Delete dead code (Group D)

1. Delete functions from `proxmox/vms.go`
2. Delete `proxmox/access.go`
3. Delete Telmate functions from `proxmox/nodes.go`
4. Delete `proxmox/vnc.go` Telmate function
5. Delete `proxmox/cluster.go` Telmate function
6. Delete `proxmox/cache.go`
7. Delete `proxmox/telmate_client.go`
8. Clean up `proxmox/interfaces.go` — delete `ClientInterface`
9. Remove Telmate from `go.mod`: `go get github.com/Telmate/proxmox-api-go@none && go mod tidy`

`make build` ✓ `make test-offline` ✓ `make go-lint` ✓

---

## New Files Summary

| File | Contents |
|------|----------|
| `backend/proxmox/vnc_resty.go` | `GetVNCProxyResty`, `VNCProxyResponse` |
| `backend/proxmox/auth_resty.go` | `CreateTicketResty`, `UpdateUserPasswordResty`, `HasRole` (moved), `TicketResponse`, `CookieAuthOptions` |
| `backend/proxmox/access_resty.go` | `EnsureUserResty`, `EnsureRoleResty`, `EnsurePoolResty`, `EnsurePoolACLResty` |

## Files Deleted

| File | Reason |
|------|--------|
| `backend/proxmox/telmate_client.go` | Entire Telmate wrapper removed |
| `backend/proxmox/access.go` | Replaced by `access_resty.go` |
| `backend/proxmox/cache.go` | Only used by Telmate client |

## Key Note: Cookie Auth for `UpdateUserPassword`

`UpdateUserPassword` requires a `PVEAuthCookie` + `CSRFPreventionToken` (obtained from `CreateTicket`).
This is a special case: the Resty client uses API token auth in its header, but this call needs
cookie headers instead. Implement `UpdateUserPasswordResty` by creating a plain `resty.New()`
instance (not the shared `RestyClient`) for that single call, setting the cookie and CSRF header
manually. This avoids adding cookie-auth complexity to the shared `RestyClient`.

## Outcome

- `github.com/Telmate/proxmox-api-go` removed from `go.mod`
- `proxmox.ClientInterface` deleted
- `StateManager.GetProxmoxClient()` deleted
- ~3 files deleted, ~3 files added, ~20 handler files simplified
- All Proxmox calls go through a single client type: `*proxmox.RestyClient`
