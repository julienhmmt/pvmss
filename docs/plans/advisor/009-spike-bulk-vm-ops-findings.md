# Spike Findings 009: Bulk VM operations for admin workflows

> **Type**: design/spike deliverable (no production code changed). Feeds a
> future build plan. Investigated at commit `4a753b3` (main after plan 008),
> branch `advisor/009-spike-bulk-vm-ops`.
>
> **STOP-condition checks (both clear):**
> - Admin VM routes in `router.go:104-107` match the audit exactly (no drift).
> - No bulk-action endpoint exists yet (`grep -rn "bulk" backend/api/v1/` → none) — this remains a spike, not a review.

---

## 1. Current admin VM action surface

### Routes (`backend/api/v1/router.go:104-107`)

All four are wrapped in `adminJWTWrap(jwtSecret, …)` — **admin-only, JWT-guarded**:

| Method | Path | Handler |
|--------|------|---------|
| GET | `/api/v1/admin/vms` | `AdminVMsAPIHandler.ListAllVMs` |
| GET | `/api/v1/admin/vms/paginated` | `AdminVMsAPIHandler.ListAllVMsPaginated` |
| POST | `/api/v1/admin/vms/:id/action` | `AdminVMsAPIHandler.VMAction` |
| DELETE | `/api/v1/admin/vms/:id` | `AdminVMsAPIHandler.DeleteVM` |

### Single-VM admin action (`admin_vms.go:108-161`)

Flow for `POST /api/v1/admin/vms/:id/action`:

1. `IsOfflineMode()` gate → `errOffline` (503) if offline.
2. `requireVMID(w, r)` → parses `:id`, 400 on non-positive/non-int.
3. `decodeBody` → `AdminVMActionRequest{ Action, Node }`.
4. Validate `Action` against `{start, stop, shutdown, reboot, reset}` → 400 otherwise.
5. Build resty client (`MakeRestyClientFromEnvConfig`, 10s timeout).
6. **Node resolution**: use `req.Node` if provided; else call `GetVMsResty` (full cluster VM list) and linear-scan for `vm.VMID == vmid` to find its node; 400 if not found.
7. `proxmox.VMActionResty(ctx, client, node, strconv.Itoa(vmid), action)` → returns `upid` (Proxmox task ID) or error.
8. Response: `VMActionResponse{ Success: true, TaskID: upid }`.

**Note — action is asynchronous.** `VMActionResty` returns a UPID; the action is queued as a Proxmox task and runs in the background. Success here means *accepted*, not *completed*. Bulk semantics inherit this: a bulk response reports per-VM *acceptance*, not terminal state.

### Non-admin single-VM action (`vm_actions.go:32-81`) — for contrast

`POST /api/v1/vms/:id/action` (`VMActionHandler`): same action set, but **requires `node` in the body** (no cluster-list lookup fallback), 60s client / 30s ctx timeout, returns **502** (`proxmox_error`) on failure. This is the owner-facing path; not in scope for admin bulk but relevant to open question Q1 (should owners get bulk too?).

### Node/authorization facts that shape bulk

- **VMs span nodes.** Node is a per-VM property; a bulk request over arbitrary VMIDs must resolve node per VM. The single-VM handler does this by scanning the full `GetVMsResty` list. **Bulk should fetch that list once** and build a `vmid → node` map, not call `GetVMsResty` N times.
- **Tag scoping.** `ListAllVMs`/`DeleteVM` only operate on VMs carrying `constants.RequiredTag` (`pvmss`). `VMAction` (admin) currently does **not** re-check the tag on the resolved VM. Bulk should decide explicitly (recommend: enforce `pvmss` tag on every target — reject/skip untagged VMIDs, matching the rest of the admin surface).
- **Authorization** is uniform: `adminJWTWrap`. Bulk endpoint reuses it verbatim.

---

## 2. Proposed bulk-action API

### Endpoint

```
POST /api/v1/admin/vms/bulk-action        (adminJWTWrap)
```

### Request

```jsonc
{
  "action": "stop",              // one of: start, stop, shutdown, reboot, reset
  "vmids": [101, 102, 103]       // 1..MaxBatch unique positive ints
}
```

- No per-VM `node` field: bulk resolves nodes server-side from one `GetVMsResty` call (clients rarely know node placement, and requiring it per VM is error-prone).
- **Best-effort, not atomic** (see Q3). Proxmox cannot transactionalize across VMs.

### Response — always `200`, per-VM results array

```jsonc
{
  "action": "stop",
  "requested": 3,
  "accepted": 2,
  "failed": 1,
  "results": [
    { "vmid": 101, "ok": true,  "task_id": "UPID:pve1:..." },
    { "vmid": 102, "ok": true,  "task_id": "UPID:pve2:..." },
    { "vmid": 103, "ok": false, "error": "VM not found or not pvmss-tagged" }
  ]
}
```

Rationale for **always-200 + results array** over HTTP 207/multi-status:
- 207 is awkward for JSON clients and the existing handlers never use it.
- Partial success is the *expected* case, not an error; the summary counts (`accepted`/`failed`) let the frontend render a toast without parsing each result.
- Reserve non-2xx for whole-request failures: `400` (bad action / empty or oversized `vmids` / duplicate IDs), `503` (offline), `502`/`500` (couldn't even fetch the VM list to resolve nodes).

### Validation (fail-fast, before any Proxmox call)

- `action` ∈ allowed set → else 400.
- `len(vmids)` in `[1, MaxBatch]` → else 400. **Recommend `MaxBatch = 50`** (Step 2 cap), constant first, config later only if needed.
- De-dup `vmids`; reject or collapse duplicates (recommend collapse silently, report each once).
- Each VMID positive int.

---

## 3. Fan-out validation

**No native Proxmox batch endpoint exists.** Bulk = **N concurrent single-VM `VMActionResty` calls**, reusing the established semaphore pattern.

### Shape (reuse `nodes_aggregator.go` / plan 006 pattern, cap 8)

```go
// 1. One list call → vmid→node map (also enforces pvmss tag membership).
vms, err := proxmox.GetVMsResty(ctx, client)   // whole-request 502 on error
nodeByVMID := map[int]string{}
for _, vm := range vms {
    if hasTag(vm.Tags, constants.RequiredTag) {
        nodeByVMID[vm.VMID] = vm.Node
    }
}

// 2. Fan out, cap 8, results indexed by position (order-preserving, no post-sort).
const maxConcurrent = 8
results := make([]BulkVMResult, len(vmids))
sem := make(chan struct{}, maxConcurrent)
var wg sync.WaitGroup
for i, vmid := range vmids {
    node, ok := nodeByVMID[vmid]
    if !ok {
        results[i] = BulkVMResult{VMID: vmid, OK: false, Error: "VM not found or not pvmss-tagged"}
        continue
    }
    wg.Add(1)
    go func() {
        defer wg.Done()
        sem <- struct{}{}; defer func() { <-sem }()
        upid, err := proxmox.VMActionResty(ctx, client, node, strconv.Itoa(vmid), action)
        if err != nil {
            results[i] = BulkVMResult{VMID: vmid, OK: false, Error: "proxmox action failed"}
            return
        }
        results[i] = BulkVMResult{VMID: vmid, OK: true, TaskID: upid}
    }()
}
wg.Wait()
```

Key points:
- **One `GetVMsResty`, not N** — the single-VM handler's per-call lookup does not scale.
- **Cap 8** matches `nodes_aggregator` and plan 006 `ListPools`; deliberate throttle against per-token Proxmox rate limits.
- **Index-addressed `results` slice** preserves request order without a post-sort; each goroutine writes a distinct index → race-free (same technique verified under `-race` in plan 006).
- **go-resty is concurrency-safe** per call (`client.R()` fresh request; `SetResult` per request) — confirmed in plan 006.
- **Context/timeout**: give the whole bulk op a bounded ctx (e.g. `WithTimeout(r.Context(), 60s)`); each action is a fast "accept task" call, not a wait-for-completion. Do **not** wait on UPID completion in the request — return task IDs and let the frontend poll status if needed.
- **Don't fail siblings on one error**: unlike `errgroup.WithContext` (plan 006 VM-detail path), bulk must NOT cancel peers when one VM fails — best-effort means every VM is attempted. Use plain `WaitGroup` + semaphore, not `errgroup`.

---

## 4. Open questions for the maintainer

1. **Scope of caller.** Admin-only (reuse `adminJWTWrap`), or also let pool owners bulk-act on VMs they own? Owner-facing bulk would need per-VM ownership checks and a different route (`/api/v1/vms/bulk-action`). **Recommend: admin-only for v1**; owner bulk is a separate spike.
2. **Max batch size.** `MaxBatch = 50` proposed. Fixed constant, or env/settings-configurable? **Recommend: constant for v1**, promote to config only if a real fleet needs more.
3. **Atomicity.** Best-effort per-VM (recommended — Proxmox can't transactionalize) vs all-or-nothing pre-check. **Recommend: best-effort with per-VM results.**
4. **Tag enforcement.** Should bulk skip/reject VMIDs lacking the `pvmss` tag? **Recommend: yes** — reject untagged targets per-VM (`ok:false`), matching `ListAllVMs`/`DeleteVM`. (Note: single-VM admin `VMAction` currently skips this check — a minor latent inconsistency; do NOT change it in this spike.)
5. **Audit logging.** One audit entry per VM, or one per bulk request with the VMID list? **Recommend: one entry per bulk request** (`action`, `vmids`, `accepted`/`failed` counts, `changedBy`) plus the existing per-action `logger.VMEvent` structured logs. Fewer audit rows, still traceable.
6. **Async completion.** Response returns UPIDs (task accepted, not done). Does the frontend need a companion "poll task status" endpoint, or is fire-and-forget + a later list-refresh enough for v1? **Recommend: fire-and-forget for v1**; the VM list already reflects status on next refresh.
7. **Rate limiting.** `router.go` shows no per-route rate limit on the admin VM routes today. Plan 001 S7 tracks admin rate limits — bulk (a fan-out amplifier) is a prime candidate. Confirm whether S7 lands before or alongside the bulk build.
8. **Frontend selection UX** (design-only): checkbox column on the paginated admin VM table + "select all on page" / "select all matching current filter (node/search)". The paginated endpoint already supports `node`/`search` filters, so "select all matching filter" maps cleanly to sending the filtered VMID set.

---

## 5. Recommendation

**Green-light a follow-up build plan.** The bulk endpoint is genuinely the adjacent possible: one new `adminJWTWrap` route + one handler reusing (a) the existing `VMActionResty` call, (b) the `GetVMsResty` node-resolution already in `admin_vms.go`, and (c) the proven cap-8 semaphore fan-out from `nodes_aggregator`/plan 006. New types needed: `BulkVMActionRequest{ Action string; VMIDs []int }` and `BulkVMResult{ VMID int; OK bool; TaskID,Error string }` + a response envelope. **Defer bulk delete** to its own spike (higher blast radius; reuse this fan-out + per-VM-results pattern).
