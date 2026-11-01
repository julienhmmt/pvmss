# Plan 002: Remove legacy single-projection fallbacks and make the policy service a required dependency

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 8a86e280..HEAD -- server/internal/policy/ server/internal/httpapi/vm_detail.go server/internal/httpapi/vm_snapshots.go server/internal/httpapi/vm_cloudinit.go server/internal/httpapi/admin_pools.go server/internal/httpapi/vms.go server/internal/httpapi/vm_create.go server/internal/vm/ server/cmd/pvmss/`

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: MED
- **Depends on**: plans/001-delete-dead-code.md (it deletes `httpapi.NewVMCreate`, a constructor this plan would otherwise have to change too)
- **Category**: tech-debt
- **Planned at**: commit `8a86e280`, 2026-11-01

## Why this matters

This is the standing `TECH_DEBT.md` item "Legacy single projection in
`policy`": `policy.New` still accepts one `*inventory.Projection`, and four
httpapi call sites plus `vm.selectPolicyService` build a per-request
fallback bound to the *default* cluster's projection when no service is
injected. In production `main.go` always injects the shared service (which
has `SetProjectionSource` wired), so the fallback is latent — but if it
ever fires for a non-default cluster, quota/usage/name checks evaluate the
wrong cluster's VMs: exactly the cross-cluster bug class the registry
refactor closed. A latent wrong-cluster path is worse than a nil panic
because it fails silently.

The documented target: callers take the shared service; the `projection`
parameter (and field) goes away; policy reads projections only through the
wired `ProjectionSource`.

## Current state

`server/internal/policy/policy.go:128-159`:

```go
type Policy struct {
	store *store.Store
	// projection is the legacy single-cluster projection. It is only read when
	// no ProjectionSource is wired (unit tests, degraded deployments).
	projection  *inventory.Projection
	projections ProjectionSource // per-cluster projections; wins over projection when set
	client      Snapshotter
	clusters    cluster.ClientProvider
	auditor     adminActionRecorder
	...
}

// New creates a policy service backed by the store and inventory projection.
func New(st *store.Store, projection *inventory.Projection, client Snapshotter) *Policy {
	return &Policy{store: st, projection: projection, client: client}
}
```

`projectionFor` (policy.go:171) returns `service.projection` when
`service.projections == nil`; `indexFor` maps a nil projection to
"cluster's inventory has not completed a refresh yet".

The fallback construction sites (all `policy.New(x, y, nil)`):

- `server/internal/httpapi/vm_detail.go:126-134` — `NewVMDetailWithRegistry(deps, services ...*policy.Policy)` then `policyService = policy.New(deps.Store, deps.Projection, nil)`
- `server/internal/httpapi/vm_snapshots.go:50-58` — `deps.Services []*policy.Policy` field, same fallback
- `server/internal/httpapi/vm_cloudinit.go:55-62` — `NewVMCloudInit(deps, services ...*policy.Policy)`, same fallback
- `server/internal/httpapi/admin_pools.go:190` — `policy.New(h.store, projection, nil).Quota(...)` built inside the handler body; the handler has no injected policy
- `server/internal/vm/policy_service.go:10-16` — `selectPolicyService(st, services)` returns `policy.New(st, nil, nil)`; used by `internal/vm/clone_own.go:46` and `internal/vm/create.go:255` via `deps.Services []*policy.Policy`

Variadic optional-dependency signatures to collapse to required params:

- `server/internal/vm/query.go:122` `List`, `:127` `ListWithContext`, `:131` `list` — `services ...*policy.Policy`; `list` reads only `services[0]` guarded by `len(services) > 0 && services[0] != nil` (query.go:151)
- `server/internal/httpapi/vms.go:35` `NewVMsWithRegistry(..., clusterStore *store.Store, services ...*policy.Policy)`
- `server/internal/httpapi/vm_create.go:118,142` — both constructors take `services ...*policy.Policy` (note: `NewVMCreate` at :112 is deleted by plan 001 — if it still exists, STOP)
- `NewVMDetailWithRegistry`, `NewVMCloudInit` (above)

Production wiring: `server/cmd/pvmss/main.go:487-489` builds
`policyService := policy.New(st, projection, nil)` then
`SetClusterProvider`/`SetProjectionSource`; `:172` builds a second
`policy.New(st, defaultProjection, nil)` for the snapshot sweep; `:528`
passes `Services: []*policy.Policy{policyService}`.

Test blast radius: ~53 test call sites touch the changed constructors;
~27 call the variadic form with zero services (relying on the fallback or
on `nil` tolerance). `internal/vm` helpers `resolveGabarit` already
nil-check the service (policy_service.go:24-35), so `nil` is a legal value
in the vm package — but `selectPolicyService` currently returns a
*store-backed* non-nil service, and `resolveGabarit` reads `service.Gabarit`
in that case instead of the passed `fallback`; tests that relied on that
must inject a real service.

Conventions: tests live in `*_test.go` next to the code; httpapi test
helpers exist in `server/internal/httpapi/test_registry_helpers_test.go`
(`inventory.NewRegistryFromProjections` builds a `ProjectionSource` for
tests). One behavior rule from `AGENTS.md`: handlers that fail a request
call `httpapi.SetError`/`SetErrorMsg` and do not log; this plan adds no new
error paths.

## Commands you will need

| Purpose | Command | Expected on success |
| --- | --- | --- |
| Build | `cd server && go build ./...` | exit 0 |
| Vet | `cd server && go vet ./...` | exit 0 |
| Tests (affected) | `cd server && go test ./internal/policy/ ./internal/vm/ ./internal/httpapi/ ./cmd/pvmss/` | all pass |
| Tests (full) | `cd server && go test -race -timeout=5m ./...` | all pass |
| Lint | `make server-lint` (repo root) | `0 issues.` |
| Blast radius | `grep -rn 'policy.New' server/ --include='*.go'` | only the construction site(s) in `cmd/pvmss/` + tests |

## Scope

**In scope:**
- `server/internal/policy/policy.go`
- `server/internal/vm/policy_service.go`, `query.go`, `create.go`, `clone_own.go` (+ whichever deps structs declare `Services []*policy.Policy`)
- `server/internal/httpapi/vm_detail.go`, `vm_snapshots.go`, `vm_cloudinit.go`, `vms.go`, `vm_create.go`, `admin_pools.go`
- `server/cmd/pvmss/main.go` (and `wiring.go`/`startup.go` if plan 003 already split it — do them in order 001 → 002 → 003 to avoid that)
- Test files that call the changed constructors or `policy.New`

**Out of scope:**
- Any behavior change to `Policy` methods (`Quota`, `Gabarit`, `Settings`, `ReserveCreate`, capacity checks) — signature and field changes only
- `server/internal/httpapi/auth.go`, `ratelimit.go`, `vm_bulk.go`, `server/internal/vm/bulk.go`, `server/go.mod` and their tests — uncommitted WIP owned by another task; never stage them
- The `snapshot_sweep.go` Gabarit semantics — wiring the shared service to the sweep is in scope only as a constructor call-site change

## Git workflow

- Branch: `refactor/audit-2026-11-cleanup`.
- Two atomic commits, in order (both purely structural):
  1. `refactor(server): inject the shared policy service and drop per-request fallbacks`
  2. `refactor(policy): remove the legacy single-projection parameter from New`
- Stage only in-scope files by name. Do NOT push or open a PR.

## Steps

### Step 1: make the policy service a required parameter everywhere (commit 1)

- `NewVMDetailWithRegistry(deps VMDetailDeps, services ...*policy.Policy)` → `(deps VMDetailDeps, service *policy.Policy)`; delete the `len(services)` pick and the `policy.New(deps.Store, deps.Projection, nil)` fallback block (vm_detail.go:127-134).
- `NewVMCloudInit` — same change (vm_cloudinit.go:55-62).
- `NewVMSnapshotsWithRegistry`: change `VMSnapshotsRegistryDeps.Services []*policy.Policy` to `Policy *policy.Policy`; delete the pick-and-fallback block (vm_snapshots.go:50-58).
- `NewVMsWithRegistry(..., clusterStore, services ...*policy.Policy)` → `..., clusterStore *store.Store, service *policy.Policy)` (drop the variadic).
- `NewVMCreateWithRegistry` — same variadic → required change.
- `vm.List`, `vm.ListWithContext`, `vm.list`: `services ...*policy.Policy` → `service *policy.Policy`; the guard at query.go:151 becomes `if service != nil` reading `service.Quota`.
- `vm` deps structs (`clone_own.go`, `create.go`): `Services []*policy.Policy` → `Policy *policy.Policy`; delete `selectPolicyService` in `policy_service.go` and pass `deps.Policy` straight to `resolveGabarit` (nil remains legal there).
- `admin_pools.go`: add `Policy *policy.Policy` to `AdminPoolsRegistryDeps`; in the detail handler replace `policy.New(h.store, projection, nil).Quota(...)` with `h.policy.Quota(r.Context(), clusterName, auth.Identity{Pool: name})`. Keep the `projectionFor` call — `pools.Detail` still needs `projection`.
- `cmd/pvmss/main.go`: construct `policyService` in `run()` (move it out of `buildRouter` into a `routerDeps` field), pass the same instance to `vm.SweepDependencies{Policy: ...}` (replacing the second `policy.New` at :172) and to `VMSnapshotsRegistryDeps{Policy: policyService}`.
- Update every test call site: variadic callers pass the service positionally; zero-service callers pass either a real service or `nil` where the code nil-checks (vm package) — for httpapi constructors, tests that previously relied on the `policy.New(store, projection)` fallback must now pass a real `*policy.Policy` built the same way (`policy.New(st, proj, nil)` is still a valid signature at this step).

**Verify**: `cd server && go build ./... && go vet ./...` → exit 0; `go test ./internal/vm/ ./internal/httpapi/ ./cmd/pvmss/` → all pass. Commit 1.

### Step 2: drop the legacy `projection` parameter (commit 2)

- `policy.New(st, projection, client)` → `policy.New(st, client)`; delete the `projection` field; `projectionFor` becomes: if `service.projections == nil` return `nil, nil` (keeps the existing "not yet refreshed" downstream semantics of `indexFor`), else resolve from the source. Update the doc comments that mention the legacy field (policy.go:130-131, 156-157, 167-170).
- `cmd/pvmss/main.go`: `policy.New(st, nil)` (projection arg gone; `SetProjectionSource(inventoryRegistry)` already follows).
- Every remaining `policy.New` caller — tests included — moves to `policy.New(st, nil)` plus, where a projection was actually needed, `SetProjectionSource(inventory.NewRegistryFromProjections(map[string]*inventory.Projection{<cluster>: <proj>}))` (pattern: `test_registry_helpers_test.go`). If more than ~3 test files need that two-line dance, add a small `func singleClusterPolicy(st *store.Store, clusterName string, proj *inventory.Projection) *policy.Policy` helper to `test_registry_helpers_test.go` instead of repeating it.

**Verify**: `cd server && go build ./... && go vet ./...` → exit 0; `go test -race ./internal/policy/ ./internal/vm/ ./internal/httpapi/ ./cmd/pvmss/` → all pass. Commit 2.

### Step 3: full-suite gate and residual check

**Verify**: `cd server && go test -race -timeout=5m ./...` → all pass.
**Verify**: `make server-lint` → `0 issues.`
**Verify**: `grep -rn 'policy.New' server/ --include='*.go' | grep -v _test.go` → only `cmd/pvmss` construction site(s) remain; `grep -rn 'services ...\*policy.Policy\|Services.*\[\]\*policy.Policy' server/` → no matches.
**Verify**: `grep -rn 'Policy:\s*policy.New\|projection.*\*inventory.Projection' server/internal/policy/policy.go` → `projection` field gone.

## Test plan

No new tests required — the change is signature/wiring only and the suite is the net. While updating call sites, do NOT weaken tests: a test that asserted a quota/gabarit outcome via the old fallback must receive an equivalently wired service, not `nil`.

## Done criteria

- [ ] No `policy.New(` call inside `server/internal/httpapi/` or `server/internal/vm/` (excluding `_test.go`)
- [ ] No `services ...*policy.Policy` or `[]*policy.Policy` deps field anywhere
- [ ] `policy.New` signature is `New(st *store.Store, client Snapshotter) *Policy` and `Policy` has no `projection` field
- [ ] One `policyService` instance in `cmd/pvmss` serves the router and the snapshot sweep
- [ ] `cd server && go test -race -timeout=5m ./...` all pass; `make server-lint` → `0 issues.`
- [ ] `git status` shows no WIP file staged or committed
- [ ] `plans/README.md` status row updated

## STOP conditions

- A test fails because it *needed* the store-backed fallback semantics (not just a compile fix) and injecting a properly wired service changes its expected outcome — report; that may reveal the fallback was load-bearing somewhere real.
- `admin_pools.go`'s `h.policy` ends up `nil` on a path the tests exercise — the deps wiring must guarantee the service.
- The drift check shows `server/internal/httpapi/vm_create.go` still contains `NewVMCreate` (plan 001 not applied) — apply 001 first or change both constructors here and note the overlap.

## Maintenance notes

- `TECH_DEBT.md`'s "Legacy single projection in `policy`" row can be moved to resolved once this lands — do that edit in this plan's second commit.
- `main.go:172`'s sweep previously used the default projection; after the change it uses the shared service — `Gabarit` is store-backed per-cluster so this is identical behavior, but a reviewer should confirm no sweep test asserted the fallback.
- If `ProjectionSource` errors surface at runtime for a cluster whose registry entry is missing, `projectionFor` already returns the error unswallowed (policy.go:169-170) — keep that contract.
