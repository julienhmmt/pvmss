# Plan 009: Spike — bulk VM operations for admin workflows (design + API + open questions)

> **Executor instructions**: This is a DESIGN/SPIKE plan, not a build plan. The
> goal is to investigate, prototype minimally, define the API, and list open
> questions — NOT to ship a complete feature. Read the plan fully, do the
> investigation steps, and produce a written findings doc at
> `docs/plans/advisor/009-spike-bulk-vm-ops-findings.md`. When done, update
> the status row in `docs/plans/advisor/README.md`.
>
> **Drift check (run first)**: `git diff --stat d427838..HEAD -- backend/api/v1/router.go backend/api/v1/vm_actions.go backend/api/v1/admin_vms.go`
> If any in-scope file changed since this plan was written, compare excerpts
> against live code before proceeding; on mismatch, treat as a STOP condition.

## Status

- **Priority**: P3 (direction)
- **Effort**: M (spike — investigation + minimal prototype, not full build)
- **Risk**: LOW (read-mostly; any prototype is throwaway)
- **Depends on**: none
- **Category**: direction
- **Planned at**: commit `d427838`, 2026-07-01

## Why this matters

Admins managing fleets must act on VMs one-by-one: "stop all dev VMs for maintenance", "restart pool X". The admin VM surface already exists (`router.go:104-107`: list, paginated list, action, delete) with single-VM actions (`vm_actions.go:14-20`: start/stop/shutdown/reboot/reset). Bulk operations are the adjacent possible — one endpoint + a selection UI on top of existing per-VM machinery. This spike defines the API, validates the Proxmox fan-out, and surfaces design questions before committing to a build.

## Current state

**Admin VM routes (`backend/api/v1/router.go`):**
- `GET /api/v1/admin/vms`, `GET /api/v1/admin/vms/paginated`, `POST /api/v1/admin/vms/:id/action`, `DELETE /api/v1/admin/vms/:id` (per audit — confirm exact lines by reading `router.go`).

**Single-VM action (`backend/api/v1/vm_actions.go:14-20`):**
- `allowedActions` set: start, stop, shutdown, reboot, reset. Handler `VMAction` validates the action, resolves node, calls Proxmox.

**Concurrency precedent:** `backend/proxmox/nodes_aggregator.go` (semaphore cap 8, WaitGroup) — bulk fan-out should reuse this pattern (and plan 006's pool-concurrent approach).

**Open question to investigate:** does the Proxmox API have a batch endpoint, or must PVMSS fan out N single-VM calls? (Likely the latter — Proxmox has no native batch VM action.)

## Commands you will need

| Purpose   | Command                                  | Expected on success |
|-----------|------------------------------------------|---------------------|
| Read routes | (use grep/read tools, no shell)        | map current routes  |
| Prototype test | `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test ./api/v1/` | exit 0 |

## Scope

**In scope (investigation only — no production code changes):**
- `backend/api/v1/router.go` (read — map routes)
- `backend/api/v1/vm_actions.go` (read — extract action logic)
- `backend/api/v1/admin_vms.go` (read — admin VM handlers)
- `docs/plans/advisor/009-spike-bulk-vm-ops-findings.md` (create — the deliverable)

**Out of scope:**
- Implementing the bulk endpoint (that's a follow-up build plan after this spike)
- Frontend bulk-selection UI (design only in the findings doc)
- Bulk delete (higher risk than bulk action — defer to a separate spike)

## Steps

### Step 1: Map the current admin VM action surface

Read `router.go`, `vm_actions.go`, `admin_vms.go`. Document in the findings file: the exact route patterns, how `VMAction` resolves the node, how it authorizes (admin check), and how it calls Proxmox.

### Step 2: Design the bulk-action API

Propose (in the findings doc) an endpoint, e.g.:
```
POST /api/v1/admin/vms/bulk-action
Body: { "vmids": [101, 102, 103], "action": "stop" }
Response: { "results": [ { "vmid": 101, "ok": true }, { "vmid": 102, "ok": false, "error": "..." } ] }
```
Consider: max VMIDs per request (cap, e.g. 50), partial-success semantics (return per-VM results, 200 if all ok, 207-ish if mixed — or always 200 with a results array), rate limiting (plan 001 S7 covers admin rate limits), idempotency.

### Step 3: Validate the fan-out approach

Confirm whether bulk action = N concurrent single-VM Proxmox calls (reuse `nodes_aggregator` semaphore pattern, cap 8). Note Proxmox API rate limits / concurrent request considerations. Identify whether VMs may live on different nodes (likely yes — fan-out must resolve node per VM, like `vm_actions` does).

### Step 4: List open questions for the maintainer

Document questions such as:
- Should non-admin users get bulk actions on VMs they own, or admin-only?
- What's the max batch size, and should it be configurable?
- Should bulk-action be atomic (all-or-nothing) or best-effort with per-VM results? (Recommend best-effort — Proxmox can't transactionalize cross-VM.)
- Frontend selection UX: checkbox list? "select all in pool"? "select all matching filter"?
- Audit logging: one audit entry per VM, or one entry for the bulk request with the VMID list?

## Test plan

- No production tests (spike). If a throwaway prototype is built, delete it before finishing — the findings doc is the deliverable.

## Done criteria

- [ ] `docs/plans/advisor/009-spike-bulk-vm-ops-findings.md` exists with: current-surface map, proposed API, fan-out validation, open questions
- [ ] No production source files modified (only the findings doc created)
- [ ] `docs/plans/advisor/README.md` status row updated

## STOP conditions

Stop and report if:
- The admin VM routes in `router.go` don't match the audit's claim (drift).
- A bulk-action endpoint already exists (the feature was built since the audit) — report so the spike becomes a review instead.

## Maintenance notes

- The findings doc feeds a future build plan. Keep the proposed API stable so the build plan can reference it.
- If bulk delete is later spiked, reuse the fan-out + per-VM-results pattern defined here.
