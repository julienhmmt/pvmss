# Spike Findings 010: VM clone functionality

> **Type**: design/spike deliverable (no production code changed). Feeds a
> future build plan. Investigated at commit `9f1bb25` (main after plan 009),
> branch `advisor/010-spike-vm-clone`.
>
> **STOP-condition checks (both clear):**
> - No clone endpoint/wrapper exists (`grep -rni "clone" backend/proxmox/ backend/api/v1/` → none). Spike stays a spike.
> - No drift in `router.go` / `vm_create.go` / `vm_details_snapshot.go` / `resty_client.go` since `d427838`.
> - Proxmox clone params confirmable from existing wrapper conventions (no live-API probe needed).

---

## 1. Proxmox clone endpoint

```
POST /nodes/{node}/qemu/{vmid}/clone     (form-encoded)
```

Relevant parameters:

| Param | Meaning | PVMSS use |
|-------|---------|-----------|
| `newid` | target VMID (required) | allocate via existing next-VMID logic |
| `name` | hostname of the clone | from request body |
| `target` | destination node (optional; same node if omitted) | from body, allowlist-checked |
| `full` | `1` = full copy, `0`/absent = linked clone | default recommended `1` (Q1) |
| `storage` | target storage for a full clone | from body, allowlist-checked |
| `pool` | add clone to a resource pool | **set to the user's pool** so quota/ACL stay consistent |
| `description` | notes | optional |
| `snapname` | clone from a specific snapshot | optional, defer to build plan |

Response is a **Proxmox task UPID** (clone is asynchronous — long-running for full clones).

---

## 2. PVMSS wrapper pattern (from `resty_snapshots.go`)

The house style for a mutating qemu call (`CreateVMSnapshotResty`, `resty_snapshots.go:28`):

```go
func CloneVMResty(ctx context.Context, client *RestyClient, node, vmid string, cfg VMCloneConfig) (string, error) {
    path := fmt.Sprintf("/nodes/%s/qemu/%s/clone", node, vmid)
    values := url.Values{}
    values.Set("newid", strconv.Itoa(cfg.NewID))
    if cfg.Name != "" { values.Set("name", cfg.Name) }
    if cfg.Target != "" { values.Set("target", cfg.Target) }
    if cfg.Full { values.Set("full", "1") }
    if cfg.Storage != "" { values.Set("storage", cfg.Storage) }
    if cfg.Pool != "" { values.Set("pool", cfg.Pool) }
    if cfg.Description != "" { values.Set("description", cfg.Description) }

    var resp struct{ Data string `json:"data"` }   // UPID
    if err := client.Post(ctx, path, values, &resp); err != nil {
        return "", fmt.Errorf("failed to clone VM: %w", err)
    }
    return resp.Data, nil
}
```

- `client.Post(ctx, path, url.Values, &target)` is the confirmed method signature (`resty_client.go:211`) — form-encoded, exactly as snapshot create uses.
- Unlike snapshot create (returns `error` only, discards UPID), **clone should return the UPID** — it's long-running and the frontend may want to poll (Q5).

---

## 3. Handler pattern (from snapshot CRUD exemplar)

Every `vm_details_snapshot.go` handler follows the same skeleton — clone reuses it verbatim:

1. `requireVMID(w, r)` → source vmid, 400 on bad id.
2. `decodeBody` → clone request.
3. offline gate (`h.isOffline()` → `errOffline`).
4. Build resty client (`MakeRestyClientFromEnvConfig`) + bounded ctx.
5. **Ownership**: `ownsVM(ctx, client, username, isAdmin, vmid)` → 404 if not owner/admin. (Clone checks ownership of the **source**.)
6. **Node resolve**: `resolveNode(ctx, client, vmid)` → source node, 404 if unresolved.
7. Proxmox call → response.

Clone inserts **quota + allowlist + newid allocation** between steps 6 and 7 (see §4).

---

## 4. Limit / ownership / allocation enforcement (the crux)

Clone **creates a new VM**, so it must reuse `vm_create`'s guards — not just the snapshot skeleton. Order matters: check everything **before** the clone POST.

### 4a. New VMID allocation (mirror `vm_create.go:498-518`)

```go
vmid := 0
if snap := h.state.GetProxmoxSnapshot(); snap != nil && len(snap.VMs) > 0 {
    highest := 0
    for _, svm := range snap.VMs { if svm.VMID > highest { highest = svm.VMID } }
    if highest > 0 { vmid = highest + 1 }
}
if vmid == 0 {
    vmid, err = proxmox.GetClusterNextIDResty(ctx, client)  // fallback
}
```
This is the source-of-truth allocator; the build plan **must reuse it** (shared with plan 009 per maintenance note — extract a helper if both land).

### 4b. Per-user quota (mirror `vm_create.go:448-462`)

```go
if settings.MaxVMPerUser > 0 && !isAdmin {
    poolVMIDs := fetchPoolVMIDs(poolCtx, client, constants.PoolPrefix+username)
    if len(poolVMIDs) >= settings.MaxVMPerUser {
        writeError(w, 409, "quota_exceeded", fmt.Sprintf("Maximum VM limit reached (%d/%d)", len(poolVMIDs), settings.MaxVMPerUser))
        return
    }
}
```
**Clone counts against the user's quota.** Set the clone's `pool` param to `PoolPrefix+username` so the new VM lands in the user's pool (keeps quota + ACL correct on the next request).

### 4c. Node aggregate limits (mirror `vm_create.go:437-446`)

`validateNodeAggregateLimits(state, targetNode, sockets, cores, memMB)` + reserve/release. Clone inherits the **source VM's** specs, so the handler must first read source config (`GetVMConfigResty`) to compute sockets/cores/mem for the target-node reservation. Release on failure via the same `defer`.

### 4d. Allowlist (S4/S5 done in vm_create)

`target_node` and `storage`, when provided, must be enabled in settings (`EnabledStorages`, node allowlist) — reject otherwise, same as create.

---

## 5. Proposed API

```
POST /api/v1/vms/:id/clone
```

Request:
```jsonc
{
  "name": "web-clone-01",     // required, hostname-validated
  "full": true,               // default true (Q1)
  "target_node": "",          // optional; source node if empty; allowlist-checked
  "storage": ""               // optional; required-ish for full clone; allowlist-checked
}
```

Response (`201`, matches snapshot create's created-semantics):
```jsonc
{ "vmid": 200, "task": "UPID:pve1:..." }
```

Error mapping (reuse existing codes): `400` bad name/params, `404` source not owned/found, `409` `quota_exceeded` / node-limit, `503` offline, `502`/`500` Proxmox failure.

**Tags**: Proxmox clone copies the source config including `tags`, so the mandatory `pvmss` tag is carried automatically (CLAUDE.md requirement satisfied without extra work — but the build plan should assert it post-clone as defense-in-depth, Q2).

---

## 6. Open questions for the maintainer

1. **Full vs linked default.** Recommend **full (`full=1`)** — portable, no coupling to source; linked breaks if source is deleted. Expose `full` in the body so power users can opt into linked. Linked also constrains `target`/`storage` (same storage as source) — extra validation if allowed.
2. **Tag copying.** Clone inherits `tags` incl. `pvmss` automatically. Recommend the build plan **re-assert** the `pvmss` tag post-clone (belt-and-suspenders) rather than trust inheritance.
3. **Cloud-init.** Does the clone keep the source's cloud-init (user/ssh keys/network) or reset? Recommend **keep source cloud-init by default** (clone = "another like this"); a "reset cloud-init" toggle is a follow-up.
4. **Admin-only vs all users.** Recommend **all users, within quota** (clone something you own) — ownership already enforced via `ownsVM`; quota via §4b. Admins bypass quota (matching `!isAdmin` in create).
5. **Async task handling.** Snapshot create is **fire-and-forget** (returns without polling the task). Clone is much longer (full disk copy). Recommend **return UPID immediately** (don't block the request); frontend polls task status or refreshes the VM list. Confirm whether a task-status endpoint is needed for v1 or a list-refresh suffices.
6. **Concurrent clone / storage pressure.** A full clone is disk-I/O heavy. Should PVMSS limit concurrent clones per user/node? Out of scope for v1; note for later if abused.

---

## 7. Recommendation

**Green-light a build plan.** Clone is one `adminJWTWrap`-free (owner-facing) route + one `CloneVMResty` wrapper + the snapshot-handler skeleton, with `vm_create`'s three guards (VMID alloc, per-user quota, node-aggregate limits) spliced in before the POST. No new Proxmox API research needed — params confirmed. **Coordinate the VMID allocator and quota/limit checks with plan 009** (maintenance note): both features re-use them, so extract shared helpers (`allocateNextVMID`, `checkUserQuota`) in the build plans rather than copy-pasting from `vm_create.go`.
