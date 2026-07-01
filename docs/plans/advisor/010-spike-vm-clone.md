# Plan 010: Spike — VM clone functionality (design + API + open questions)

> **Executor instructions**: This is a DESIGN/SPIKE plan, not a build plan.
> Investigate, prototype minimally, define the API, and list open questions —
> NOT ship a complete feature. Produce a written findings doc at
> `docs/plans/advisor/010-spike-vm-clone-findings.md`. When done, update the
> status row in `docs/plans/advisor/README.md`.
>
> **Drift check (run first)**: `git diff --stat d427838..HEAD -- backend/api/v1/router.go backend/api/v1/vm_create.go backend/api/v1/vm_details_snapshot.go backend/proxmox/resty_client.go`
> If any in-scope file changed since this plan was written, compare excerpts
> against live code before proceeding; on mismatch, treat as a STOP condition.

## Status

- **Priority**: P3 (direction)
- **Effort**: M (spike — investigation + minimal prototype)
- **Risk**: LOW (read-mostly; prototype is throwaway)
- **Depends on**: none
- **Category**: direction
- **Planned at**: commit `d427838`, 2026-07-01

## Why this matters

VM lifecycle has create, destroy, start/stop/shutdown/reboot/reset, and full snapshot CRUD — but no clone. Proxmox's `/nodes/{node}/qemu/{vmid}/clone` is a standard operation surfaced in the Proxmox UI but missing from the PVMSS self-service portal. Users wanting a similar VM must reconfigure from scratch, risking config drift. Clone is the missing "C" in the VM lifecycle and is one route + one Proxmox call away from the existing `vm_create` infrastructure.

## Current state

**VM routes (`backend/api/v1/router.go`):** create (`POST /api/v1/vms`), delete, config patch, snapshot CRUD — no clone.

**Snapshot CRUD (`backend/api/v1/vm_details_snapshot.go`):** `GetVMSnapshots`, `CreateSnapshot`, `DeleteSnapshot`, `RollbackSnapshot` — a complete CRUD exemplar showing how PVMSS wraps Proxmox qemu endpoints with ownership checks and validation. Clone should follow this pattern.

**VM create (`backend/api/v1/vm_create.go`):** shows how PVMSS validates inputs against allowlists (ISO/bridge/node/storage — S4/S5 DONE), enforces limits (`vm_create_resolve.go` pool/VM-ID resolution), and calls Proxmox. Clone must respect the same limits (max VMs, per-user quotas) since it creates a new VM.

**Proxmox clone endpoint:** `POST /nodes/{node}/qemu/{vmid}/clone` with params (`newid`, `name`, `target` node, `full` vs linked, `storage`). Investigate the exact param set via the Proxmox API docs or existing client wrappers in `backend/proxmox/`.

## Commands you will need

| Purpose   | Command                                  | Expected on success |
|-----------|------------------------------------------|---------------------|
| Read routes/handlers | (grep/read tools)                | map current surface |
| Check Proxmox client wrappers | `grep -n "clone\|Clone" backend/proxmox/` | find any existing wrapper |

## Scope

**In scope (investigation only — no production code changes):**
- `backend/api/v1/router.go`, `vm_create.go`, `vm_details_snapshot.go`, `vm_actions.go` (read)
- `backend/proxmox/*.go` (read — find the resty wrapper pattern; check if a clone wrapper already exists)
- `docs/plans/advisor/010-spike-vm-clone-findings.md` (create — the deliverable)

**Out of scope:**
- Implementing clone (follow-up build plan)
- Linked clones vs full clones decision (an open question for the findings doc)
- Frontend clone UI (design only)

## Steps

### Step 1: Confirm no clone wrapper exists

`grep -rn "clone\|Clone" backend/proxmox/ backend/api/v1/` — confirm no existing clone endpoint/wrapper. If one exists, the spike becomes "complete the half-built feature" — adjust scope accordingly.

### Step 2: Map the Proxmox clone API + PVMSS wrapper pattern

Read `vm_details_snapshot.go` (the cleanest CRUD exemplar) and one `proxmox/resty_client.go` wrapper. Document in the findings file:
- The Proxmox clone endpoint params (`newid`, `name`, `target`, `full`, `storage`, `description`).
- How PVMSS wrappers shape a request (`restyClient.Post(ctx, path, body, &resp)` — confirm the exact method).
- How `vm_create_resolve.go` allocates a new VMID (`fetchPoolVMIDs` + next-free logic) — clone needs a `newid` from the same allocator.

### Step 3: Design the clone API

Propose in the findings doc:
```
POST /api/v1/vms/:id/clone
Body: { "name": "new-vm-name", "full": true, "target_node": "", "storage": "" }
Response: { "vmid": 200, "task": "UPID:..." }
```
Consider: ownership check (only clone VMs you own — reuse `ownsVM`), limit enforcement (clone counts against `max_vms`/`max_vm_per_user` — must check BEFORE cloning), allowlist (target storage/node must be enabled), task polling (clone is async in Proxmox — does PVMSS poll tasks elsewhere? check snapshot create for the pattern), linked vs full default.

### Step 4: List open questions

Document:
- Full vs linked clone default? (Full is safer/portable; linked saves space but couples to source.)
- Should clone copy tags including the mandatory `pvmss` tag? (Must — `pvmss` tag is mandatory per CLAUDE.md.)
- Does clone need to respect the source VM's cloud-init config, or start fresh?
- Admin-only or available to all users (within their quotas)?
- Async task handling: block until clone done, or return task ID and poll? (Match snapshot create's choice.)

## Test plan

- No production tests (spike). Delete any throwaway prototype before finishing.

## Done criteria

- [ ] `docs/plans/advisor/010-spike-vm-clone-findings.md` exists with: Proxmox clone API map, PVMSS wrapper pattern, proposed API, limit/ownership enforcement plan, open questions
- [ ] No production source files modified (only the findings doc created)
- [ ] `docs/plans/advisor/README.md` status row updated

## STOP conditions

Stop and report if:
- A clone endpoint/wrapper already exists in `backend/proxmox/` or `backend/api/v1/` (feature partially built — pivot to a completion plan).
- The Proxmox clone params can't be confirmed from existing client wrappers or code (the findings doc would then need a live Proxmox API probe — out of scope for offline spike; flag for the maintainer).

## Maintenance notes

- The findings doc feeds a future build plan. Keep the proposed API + limit-enforcement notes stable.
- Clone and bulk-ops (plan 009) share the fan-out/limits infrastructure — if both are built, coordinate the VMID allocator and limit-check helpers so they aren't duplicated.
