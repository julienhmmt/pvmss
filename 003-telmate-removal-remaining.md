# 003-telmate-removal: Remaining Work

Branch: `003-telmate-removal`

## Current Build Status

Two errors must be fixed before `go build ./handlers/` passes:

```bash
handlers/vm_create.go:398:28: not enough arguments in call to h.handleVMCreation
handlers/vm_delete.go:50:2: declared and not used: stateManager
```

The `vm_delete.go` error has already been fixed (stateManager declaration removed).
The `vm_create.go` call site has already been updated to pass no `client` arg.
But `handleVMCreation` still has internal `client` usages — see below.

---

## Files Requiring Changes

### 1. `backend/handlers/vm_create_handler.go`

`handleVMCreation` still uses `client proxmox.ClientInterface` internally (the parameter was
removed from the signature but the body was not fully updated).

#### a) `countVMsInPool` call (line ~185)

```go
// CURRENT (broken — client is no longer a param)
currentVMCount, err := countVMsInPool(ctx, client, pool)
```

`countVMsInPool` is in `vm_create_helpers.go:48` with signature:

```go
func countVMsInPool(ctx context.Context, client proxmox.ClientInterface, poolName string) (int, error)
```

**Action:** Migrate `countVMsInPool` to use a resty client internally (see section 3 below),
then change the call to:

```go
currentVMCount, err := countVMsInPool(ctx, pool)
```

#### b) `ValidateVMResourcesAgainstNodeLimits` call (line ~459)

```go
// CURRENT (broken)
if err := ValidateVMResourcesAgainstNodeLimits(ctx, client, h.stateManager, node, ...); err != nil {
```

**Action:** Migrate this function and its callee `CalculateNodeResourceUsage` to resty
(see section 4 below), then change the call to:

```go
if err := ValidateVMResourcesAgainstNodeLimits(ctx, h.stateManager, node, ...); err != nil {
```

#### c) VM creation POST (line ~469)

```go
// CURRENT (broken)
if _, err := client.PostFormWithContext(ctx, path, params); err != nil {
```

**Action:** Replace with a resty POST. The resty client has a `Post` method. Example:

```go
restyClient, err := getDefaultRestyClient()
if err != nil {
    data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.ProxmoxClientUnavailable")
    renderVMCreateTempl(w, r, data)
    return
}
var createResp interface{}
if err := restyClient.Post(ctx, path, params, &createResp); err != nil {
    logger.VMFailure(...).Err(err)....Msg("VM creation failed")
    data["ValidationError"] = fmt.Sprintf("Failed to create VM: %v", err)
    renderVMCreateTempl(w, r, data)
    return
}
```

Note: `params` is a `url.Values` — check `proxmox.RestyClient.Post` signature to confirm
how form params are passed (it may need `params.Encode()` as body with content-type header,
or the resty client already handles `url.Values`). Check `resty_client.go` for the `Post` method.

#### d) `client.InvalidateCache` calls (lines ~476-479)

```go
client.InvalidateCache("/nodes/" + url.PathEscape(node) + "/qemu")
if pool != "" {
    client.InvalidateCache("/pools/" + url.PathEscape(pool))
}
```

**Action:** Delete these lines entirely. Resty client has no cache.

#### e) `startVM` call (line ~490)

```go
if err := h.startVM(ctx, client, node, vmid); err != nil {
```

**Action:** Change to `h.startVM(ctx, node, vmid)` after updating the function signature (see f).

#### f) `startVM` function (line ~528)

```go
func (h *VMCreateOptimizedHandler) startVM(ctx context.Context, client proxmox.ClientInterface, node string, vmid int) error {
    path := fmt.Sprintf("/nodes/%s/qemu/%d/status/start", url.PathEscape(node), vmid)
    _, err := client.PostFormWithContext(ctx, path, nil)
    return err
}
```

**Action:** Replace entirely with:

```go
func (h *VMCreateOptimizedHandler) startVM(ctx context.Context, node string, vmid int) error {
    restyClient, err := getDefaultRestyClient()
    if err != nil {
        return fmt.Errorf("failed to create resty client: %w", err)
    }
    path := fmt.Sprintf("/nodes/%s/qemu/%d/status/start", url.PathEscape(node), vmid)
    _, err = proxmox.VMActionResty(ctx, restyClient, node, strconv.Itoa(vmid), "start")
    return err
}
```

(Or use `restyClient.Post(ctx, path, nil, &struct{}{})` directly if `VMActionResty` is not appropriate.)

---

### 2. `backend/handlers/network_helpers.go` (line ~73)

`collectVMBRs` calls `sm.GetProxmoxClient()` and falls back to cache if nil.
The function already creates a resty client internally (line ~85) and only uses `client`
as a nil-guard.

**Action:** Remove the `client := sm.GetProxmoxClient()` nil check block (lines 73-82).
The resty-client failure path at line 85 already provides the same fallback behaviour.

```go
// DELETE these lines:
client := sm.GetProxmoxClient()
if client == nil {
    log.Warn()....Msg("Proxmox client not initialized; using cache fallback")
    return collectVMBRsFromCache(), nil
}
```

---

### 3. `backend/handlers/vm_create_helpers.go` — `countVMsInPool`

Current signature:

```go
func countVMsInPool(ctx context.Context, client proxmox.ClientInterface, poolName string) (int, error)
```

It likely calls `client.GetJSON(ctx, "/pools/"+poolName, &resp)`.

**Action:** Change to:

```go
func countVMsInPool(ctx context.Context, poolName string) (int, error) {
    restyClient, err := getDefaultRestyClient()
    if err != nil {
        return 0, fmt.Errorf("failed to create resty client: %w", err)
    }
    var resp struct {
        Data struct {
            Members []struct {
                Type string `json:"type"`
                VMID int    `json:"vmid"`
            } `json:"members"`
        } `json:"data"`
    }
    if err := restyClient.Get(ctx, "/pools/"+url.PathEscape(poolName), &resp); err != nil {
        return 0, err
    }
    count := 0
    for _, m := range resp.Data.Members {
        if m.VMID > 0 && (strings.EqualFold(m.Type, "qemu") || strings.EqualFold(m.Type, "lxc")) {
            count++
        }
    }
    return count, nil
}
```

Remove the `"pvmss/proxmox"` import if it's no longer needed after this change.

---

### 4. `backend/handlers/limits_helpers.go`

Two functions need migration:

#### a) `CalculateNodeResourceUsage` (line 40)

```go
func CalculateNodeResourceUsage(ctx context.Context, client proxmox.ClientInterface, sm LimitsGetter) (...)
```

Replace `client` usage (`client.GetJSON` or similar) with `getDefaultRestyClient()` + `restyClient.Get`.
Remove `client proxmox.ClientInterface` from the signature.

#### b) `ValidateVMResourcesAgainstNodeLimits` (line 480)

```go
func ValidateVMResourcesAgainstNodeLimits(ctx context.Context, client proxmox.ClientInterface, sm LimitsGetter, ...) error
```

After (a) is done, remove `client` from this signature and update the `CalculateNodeResourceUsage` call.

#### c) `ValidateNodeLimitsAgainstCapacity` (line 434)

```go
func ValidateNodeLimitsAgainstCapacity(ctx context.Context, client proxmox.ClientInterface, nodeName string, ...) error
```

Replace internal `client` usage with resty. Remove `client proxmox.ClientInterface` from signature.

---

### 5. `backend/handlers/settings_limits.go`

Two call sites remain after fixing limits_helpers.go:

**Line ~86-103** (GET handler):

```go
client := h.stateManager.GetProxmoxClient()
offlineMode := h.stateManager.IsOfflineMode()
proxmoxConnected := client != nil && !offlineMode

if snapshot != nil && len(snapshot.NodeNames) > 0 {
    nodeNames = append(nodeNames, snapshot.NodeNames...)
} else if proxmoxConnected {
    pc, ok := client.(*proxmox.Client)
    if ok {
        nodes, err := proxmox.GetNodeNamesWithContext(ctx, pc)
        ...
    }
}
```

**Action:** Replace with:

```go
offlineMode := h.stateManager.IsOfflineMode()
proxmoxConnected := !offlineMode

if snapshot != nil && len(snapshot.NodeNames) > 0 {
    nodeNames = append(nodeNames, snapshot.NodeNames...)
} else if proxmoxConnected {
    restyClient, restyErr := getDefaultRestyClient()
    if restyErr == nil {
        ctx2, cancel2 := context.WithTimeout(r.Context(), 5*time.Second)
        defer cancel2()
        nodes, err := proxmox.GetNodeNamesResty(ctx2, restyClient)  // check if this exists; if not use GetNodesResty and extract names
        if err != nil {
            log.Warn().Err(err).Msg("Unable to retrieve Proxmox nodes for limits page")
        } else {
            nodeNames = nodes
        }
    }
}
```

Check `proxmox/nodes.go` for a Resty-based equivalent of `GetNodeNamesWithContext`. If none exists,
add one to `proxmox/nodes.go` (it's an existing file so careful — add a new function, don't modify existing ones beyond what's already been done).

**Line ~299-310** (POST handler for node limits):

```go
client := h.stateManager.GetProxmoxClient()
if client != nil && coresMax > 0 && ramMax > 0 {
    if err := ValidateNodeLimitsAgainstCapacity(ctx, client, nodeName, coresMax, ramMax, localizer); err != nil {
```

**Action:** After migrating `ValidateNodeLimitsAgainstCapacity` (section 4c above):

```go
if coresMax > 0 && ramMax > 0 {
    if err := ValidateNodeLimitsAgainstCapacity(ctx, nodeName, coresMax, ramMax, localizer); err != nil {
```

---

### 6. Remove unused imports

After all the above, check each modified file for unused imports:

- `"pvmss/proxmox"` — remove from files where only `ClientInterface` was used
- `"pvmss/security"` in `vm_create_handler.go` — verify still needed (used for session in username extraction)
- Check `vm_delete.go` for `"pvmss/proxmox"` — still needed for `proxmox.VM` and `proxmox.GetVMsResty`

Run `go build ./handlers/` after each file change to catch stale imports early.

---

## Verification Steps

After all changes:

```bash
cd backend && go build ./handlers/
cd backend && go build ./...
make go-fmt
make go-lint
```

Tests are NOT run at this stage (fixed in Task 3).

---

## Files NOT to touch

- `backend/state/` — no changes
- `backend/proxmox/telmate_client.go` — no changes (removed in Task 3)
- Any `_test.go` file — no changes
- Existing `proxmox/*.go` files — only ADD new functions, never modify existing signatures
