# Plan 005: Add characterization tests for VM create + VM details handlers (high-churn, untested)

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report. When done, update the status row in `docs/plans/advisor/README.md`.
>
> **Drift check (run first)**: `git diff --stat d427838..HEAD -- backend/api/v1/vm_create.go backend/api/v1/vm_details.go backend/api/v1/vm_create_resolve.go backend/api/v1/admin_db_test.go`
> If any in-scope source file changed since this plan was written, compare
> "Current state" excerpts against live code before proceeding; on mismatch, treat as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: HIGH (high-churn files; characterization tests lock current behavior — must not change source)
- **Depends on**: plan 003 (CI runs the tests); ideally plan 004 (shared test scaffold)
- **Category**: tests
- **Planned at**: commit `d427838`, 2026-07-01

## Why this matters

`vm_create.go` and `vm_details.go` are top-churn files (10 and 10 commits in the last 100) and core user-facing features, yet have **no tests**. VM creation is the feature the portal exists for; VM detail drives the main page. The detail handler makes 3 sequential Proxmox calls (perf plan 006 parallelizes them) — without characterization tests, that refactor is unsafe. These tests record current behavior so the perf refactor and future feature work regress loudly instead of silently.

## Current state

**`backend/api/v1/vm_details.go` — `GetVMConfig` (line ~295+):**
- Line 303: `ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)`.
- Line 306: `ownsVM(ctx, client, username, isAdmin, vmid)` — ownership/IDOR check (confirmed clean in security audit).
- Line 312: `proxmox.GetVMsResty(ctx, client)` — fetches all VMs once, reused for node resolution + summary.
- Line 318: `resolveNodeFromList(allVMs, vmid)`.
- Line 334: `proxmox.GetVMCurrentResty(ctx, client, node, vmid)` — sequential.
- Line 341: `proxmox.GetVMConfigResty(ctx, client, node, vmid)` — sequential.
- Then parses networks (`ExtractNetworkInterfaces`), disks, cloud-init, EFI/TPM.

**`backend/api/v1/vm_create.go` — `GetSettings` (line ~178+):**
- Assembles the VM-creation form data: nodes, storages, bridges, ISOs (filtered by allowlists per S4/S5 DONE), profiles, limits. Reads from `h.state.GetSettings()` and the Proxmox snapshot.

**`backend/api/v1/vm_create_resolve.go`:**
- Pool/VM-ID resolution logic (line 112: `u.ramGB += int((vm.MemoryMB + 512) / 1024)` — the gosec-suppressed line, not in scope).

**Existing test pattern (`backend/api/v1/admin_db_test.go`):**
- `package apiv1_test`, `database.OpenMemory()`, `state.MakeAppStateWithDB(db)`, `httptest.NewRequest`+`httptest.NewRecorder()`, testify. Handlers called directly, bypassing JWT middleware (ctx keys absent → `usernameFromCtx` returns `""`).

**Offline mode:** `PVMSS_OFFLINE=true` makes Proxmox calls return mocked/empty data or `errOffline`. The VM handlers read `h.state.IsOfflineMode()` and the snapshot. Tests run offline — characterize offline behavior primarily.

## Commands you will need

| Purpose   | Command                                                                  | Expected on success |
|-----------|--------------------------------------------------------------------------|---------------------|
| Test (filtered) | `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -run 'TestVM(Create|Details|Config)' ./api/v1/ -v -race` | all pass, no races |
| Test (full) | `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -timeout=5m ./...` | all pass |
| Lint      | `cd backend && golangci-lint run --timeout=3m`                           | exit 0              |

## Scope

**In scope**:
- `backend/api/v1/vm_create_test.go` (create)
- `backend/api/v1/vm_details_test.go` (details)

**Out of scope**:
- `vm_create.go`, `vm_details.go`, `vm_create_resolve.go` source (characterization only — no source change)
- Live Proxmox paths (online tests)
- `vm_actions.go`, `admin_pools.go` (separate coverage work)

## Git workflow

- Branch: `advisor/005-vm-handler-tests`
- Commit: `test(vm): characterization tests for vm_create and vm_details handlers`
- Do NOT push unless instructed.

## Steps

### Step 1: Shared handler scaffold

In `vm_create_test.go` (or a shared `vm_test_helpers.go` if the package convention allows — check no existing helper file), add a helper reusing the `admin_db_test.go` pattern: `database.OpenMemory()` + `CompleteBootstrap` + `state.MakeAppStateWithDB` + `LoadSettingsFromDB`. Construct the VM handler via its constructor (check `api/v1/router.go` or `setup.go` for how `vm_create`/`vm_details` handlers are built — likely `MakeVMHandler(sm)` or similar; confirm the exact constructor name).

**Verify**: `cd backend && go vet ./api/v1/` → exit 0.

### Step 2: Test GetSettings (vm_create) offline

`TestGetVMCreateSettings_Offline_<Scenario>`:
- Offline mode → 200, response includes filtered nodes/storages/bridges/ISOs from settings (empty lists in offline if no snapshot).
- Verify allowlist filtering: a storage not in `settings.EnabledStorages` does NOT appear in the response.

Seed settings via `sm.SetSettings(...)` or DB rows before calling the handler. Model on how `admin_db_test.go` seeds state.

**Verify**: `go test -run TestGetVMCreateSettings -v ./api/v1/` → passes.

### Step 3: Test GetVMConfig (vm_details) offline + error paths

`TestGetVMConfig_<Scenario>`:
- VM not found (vmid absent from offline snapshot) → 404 (`not_found`).
- Missing/invalid vmid param → 400.
- Offline mode with a seeded snapshot containing the vmid → 200, response has the expected fields (node, status, networks, disks).

To seed a snapshot offline, check how `state` exposes snapshot injection (e.g. `sm.SetProxmoxSnapshot(...)` or a test-only setter). If none exists, this is a STOP condition (report — the test may need a small test-only seam added to `state`, which would touch out-of-scope code and needs maintainer approval).

**Verify**: `go test -run TestGetVMConfig -v ./api/v1/` → passes.

### Step 4: Test network/disk/cloud-init parsing

`TestGetVMConfig_ParsesNetworkAndDisks`:
- Seed a snapshot VM with a config containing known network/disk strings; assert `ExtractNetworkInterfaces`/disk parsing produces the expected structs. If the parsing funcs (`proxmox.ExtractNetworkInterfaces`) are exported, unit-test them directly in `backend/proxmox/` instead — that's lower-risk and out of this plan's file scope but acceptable as a companion. Keep the handler-level test for integration of the parse into the response.

**Verify**: test passes; documents the parse contract the perf refactor (plan 006) must preserve.

## Test plan

- Tests cover GetSettings (offline + allowlist filtering) and GetVMConfig (not-found, invalid-id, offline-success, parse integration).
- All pass with `-race`.
- Verify: `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -timeout=5m ./...` → all pass.

## Done criteria

- [ ] `backend/api/v1/vm_create_test.go` and `backend/api/v1/vm_details_test.go` exist (`package apiv1_test`)
- [ ] Tests cover GetSettings offline + allowlist, GetVMConfig not-found/invalid/offline-success/parse
- [ ] `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -race -run 'TestGetVMCreateSettings|TestGetVMConfig' ./api/v1/` exits 0
- [ ] Full offline suite passes (no regression)
- [ ] `vm_create.go`/`vm_details.go`/`vm_create_resolve.go` unmodified (`git diff` empty for those)
- [ ] `cd backend && golangci-lint run --timeout=3m` exits 0
- [ ] `docs/plans/advisor/README.md` status row updated

## STOP conditions

Stop and report if:
- The VM handler constructor name differs from what `router.go`/`setup.go` shows (drift — report the actual constructor).
- `state.StateManager` exposes no way to inject a Proxmox snapshot offline (Step 3 needs a test seam in `state/` — out of scope; report so the maintainer can approve a minimal test-only setter).
- A test reveals an actual bug (e.g. GetVMConfig returns 200 for a VM the user doesn't own — would contradict the `ownsVM` check) — report as a finding, do not encode buggy behavior as expected.
- `ownsVM` behavior in offline mode is ambiguous (no snapshot to check against) — characterize the actual code path and document it; report if it looks unsafe.

## Maintenance notes

- Plan 006 (perf) parallelizes the 3 sequential Proxmox calls in GetVMConfig. After 006 lands, these tests must still pass — if they assert ordering, relax them to assert presence not order.
- If snapshot injection required a test seam in `state`, document it so future tests reuse it instead of adding a second seam.
