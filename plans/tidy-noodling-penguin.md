# PVMSS Workflow Review — Action Plan

## Context

A deep review of the VM creation, search, and details workflows revealed one critical bug, several security gaps, performance issues, and code quality improvements. This plan prioritizes fixes by impact.

---

## Phase 1: Critical Bug Fix

### 1.1 NetworkInterface JSON serialization mismatch

- **File**: `backend/proxmox/vms.go:30-43`
- **Problem**: `NetworkInterface` struct has no `json:` tags → Go serializes PascalCase, frontend expects lowercase
- **Impact**: Network tab shows blank data; hardware modal can't pre-fill existing NICs
- **Fix**: Add proper `json:` tags matching what the frontend expects (`name`, `mac`, `model`, `bridge`, `tag`, `firewall`, `rate`, `ips`, `index`, `mtu`, `link_down`)

---

## Phase 2: Security Hardening

### 2.1 Validate creation inputs against settings allowlists

- **File**: `backend/api/v1/vm_create.go` (in `CreateVM`)
- **Fix**: Before forwarding to Proxmox, validate:
  - `req.Node` ∈ `settings.EnabledNodes`
  - `req.Storage` ∈ `settings.EnabledStorages`
  - `req.ISO` ∈ `settings.ISOs` (if set)
  - `req.Networks[*].Bridge` ∈ `settings.VMBRs`
- Return 400 with generic "invalid selection" message on mismatch

### 2.2 Stop leaking Proxmox error details

- **Files**: `backend/api/v1/vm_create.go:898`, `backend/api/v1/vms.go:231`
- **Fix**: Log the real error server-side, return generic "Failed to create/delete VM" to client

### 2.3 Use Proxmox `GET /cluster/nextid` for atomic VMID allocation

- **Files**: `backend/api/v1/vm_create.go:741-759`, `backend/proxmox/vms.go`
- **Fix**: Add `GetClusterNextIDResty()` function, call it instead of snapshot-based max+1
- Eliminates race condition when concurrent users create VMs

### 2.4 Validate rate limit field

- **File**: `backend/api/v1/vm_create.go:881` and `vm_hardware.go`
- **Fix**: Parse `net.RateLimit` as a positive number, reject non-numeric or negative values before building config string

---

## Phase 3: Performance

### 3.1 Parallelize node VM queries

- **File**: `backend/proxmox/vms.go:243-267` (`GetVMsResty`)
- **Fix**: Use goroutines with semaphore (same pattern as `nodes_aggregator.go:FetchAllNodeDetailsResty`) to query nodes in parallel
- Expected improvement: N sequential requests → ~1 request latency

### 3.2 Deduplicate GetVMsResty calls in GetVMConfig

- **File**: `backend/api/v1/vm_details.go:241-378`
- **Fix**: Call `GetVMsResty` once, pass the result to both `resolveNode` and the vmSummary lookup

### 3.3 Wire up LRUCache for VM listings

- **Files**: `backend/proxmox/cache.go`, `backend/proxmox/vms.go`
- **Fix**: Cache `GetVMsForNodeResty` results with a short TTL (5-10s) to avoid hammering Proxmox on rapid successive requests (polling, multiple users)

---

## Phase 4: Validation & Correctness

### 4.1 Validate additional disk sizes against limits

- **File**: `backend/api/v1/vm_create.go`
- **Fix**: Apply `limits.Disk.Min/Max` check to all disks, not just `disks[0]`

### 4.2 Enforce disk-per-bus-type limits

- **File**: `backend/api/v1/vm_create.go`
- **Fix**: Count disks by bus type, reject if exceeding `MaxDisksIDE/SATA/VirtIO/SCSI`

### 4.3 Fix NIC limit check in hardware update

- **File**: `backend/api/v1/vm_hardware.go:142-148`
- **Fix**: Count existing NICs + new NICs - deleted NICs, compare against `maxNICs`

### 4.4 Expand ExtractNetworkInterfaces range

- **File**: `backend/proxmox/vms.go:66`
- **Fix**: Change loop from `i < 10` to `i < 32` to match `MaxNetworkCards` default

### 4.5 Unify tag check functions

- **Files**: `backend/api/v1/vms.go` (`hasTag`), `backend/api/v1/admin_vms.go` (`hasPVMSSTag`)
- **Fix**: Replace both with a single case-insensitive `hasTag(tags, target)` function

---

## Phase 5: UX Improvements

### 5.1 Auto-stop VM before delete

- **File**: `frontend-svelte/src/routes/(app)/vm/[id]/+page.svelte`
- **Fix**: If VM is running and user confirms delete, call `shutdown` action first, wait for stopped state (poll), then delete. Show progress in the confirmation dialog.

### 5.2 Remove debug comment

- **File**: `frontend-svelte/src/routes/docs/[type]/+page.svelte:203`
- **Fix**: Remove `<!-- DEBUG: html length = {html.length} -->`

---

## Out of Scope (Future Work)

These are real gaps but require separate feature design:

- Server-side pagination for VM listings
- Disk resize/add/remove from detail page
- Cloud-init editing from detail page
- Snapshot vmstate checkbox in UI
- Async task tracking for VM provisioning
- Metrics polling backoff / visibility-based pause
- Shared Resty client with connection pooling

---

## Verification

1. `make test-offline` — all existing tests pass
2. `make go-lint` — no new lint warnings
3. Manual testing checklist:
   - Create VM → verify node/storage/bridge validated against settings
   - VM detail → network tab shows correct data (JSON fix)
   - Hardware modal → existing NICs pre-filled correctly
   - Delete running VM → auto-stops then deletes
   - Concurrent VM creation → unique VMIDs (if live Proxmox available)
