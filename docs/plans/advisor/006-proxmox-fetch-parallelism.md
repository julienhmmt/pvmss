# Plan 006: Parallelize Proxmox fetches (pool detail N+1, VM detail 3-call waterfall)

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report. When done, update the status row in `docs/plans/advisor/README.md`.
>
> **Drift check (run first)**: `git diff --stat d427838..HEAD -- backend/api/v1/admin_pools.go backend/api/v1/vm_details.go backend/proxmox/nodes_aggregator.go`
> If any in-scope file changed since this plan was written, compare "Current
> state" excerpts against live code before proceeding; on mismatch, treat as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: MED (concurrent fetches change ordering + error-handling surface; must preserve response shape and partial-failure tolerance)
- **Depends on**: plan 005 (characterization tests must exist to catch regressions in vm_details)
- **Category**: perf
- **Planned at**: commit `d427838`, 2026-07-01

## Why this matters

Two admin/user-facing paths do sequential HTTP calls to Proxmox where the calls are independent. (1) `admin_pools.go` fetches pool details one-at-a-time in a loop — N pools = N sequential round trips (~5-10s for 50 pools). (2) `vm_details.go` GetVMConfig makes 3 sequential calls; after node resolution, `GetVMCurrentResty` and `GetVMConfigResty` are independent and could run concurrently, cutting ~200-500ms off every VM detail page. The codebase already has a battle-tested concurrent pattern (`nodes_aggregator.go`) to mimic.

## Current state

**N+1 pool details (`backend/api/v1/admin_pools.go:63-78`):**
```go
result := make([]AdminPoolResponse, 0, len(listResp.Data))
for _, p := range listResp.Data {
    if !strings.HasPrefix(p.PoolID, constants.PoolPrefix) {
        continue
    }
    var detail detailResp
    if err := restyClient.Get(r.Context(), "/pools/"+url.PathEscape(p.PoolID), &detail); err != nil {
        logger.Get().Warn().Err(err).Str("pool", p.PoolID).Msg("failed to fetch pool detail")
        result = append(result, AdminPoolResponse{PoolID: p.PoolID, Comment: p.Comment, Members: []string{}, VMCount: 0})
        continue
    }
    // ... build response from detail ...
}
```
Sequential per-pool `restyClient.Get`. Partial failure already tolerated (appends a stub on error, continues).

**VM detail waterfall (`backend/api/v1/vm_details.go:312-345`):**
- Line 312: `allVMs, err := proxmox.GetVMsResty(ctx, client)` (needed for node resolution — keep first).
- Line 318: `node, err := resolveNodeFromList(allVMs, vmid)` (depends on allVMs — keep second).
- Line 334: `current, err := proxmox.GetVMCurrentResty(ctx, client, node, vmid)` — depends on `node` only.
- Line 341: `cfg, err := proxmox.GetVMConfigResty(ctx, client, node, vmid)` — depends on `node` only, independent of `current`.

**Concurrent pattern to mimic (`backend/proxmox/nodes_aggregator.go:26-57`):**
```go
const maxConcurrent = 8
semaphore := make(chan struct{}, maxConcurrent)
var wg sync.WaitGroup
detailsChan := make(chan *NodeDetails, len(nodes))
for _, nodeName := range nodes {
    wg.Add(1)
    name := nodeName
    go func() {
        defer wg.Done()
        semaphore <- struct{}{}
        defer func() { <-semaphore }()
        nodeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
        defer cancel()
        // ... fetch ...
        detailsChan <- details
    }()
}
go func() { wg.Wait(); close(detailsChan) }()
for detail := range detailsChan {
    collected = append(collected, detail)
}
sort.Slice(collected, ...)  // restore deterministic order
```

Repo conventions to match:
- `golang.org/x/sync` is already a dependency (`go.mod`) — `errgroup` is available and preferred over raw WaitGroup when you need to surface the first error. Check it's imported; if `errgroup` is used elsewhere, match that; otherwise the `nodes_aggregator` channel+WaitGroup pattern is the established one.
- Errors: pool fetch uses `logger.Warn` + stub (preserve); VM detail uses `writeAppError` (preserve — fail the request on Proxmox error).
- Deterministic ordering: `nodes_aggregator.go:64-66` sorts after collection — pool list must stay sorted by PoolID to keep API output stable.

## Commands you will need

| Purpose   | Command                                                                  | Expected on success |
|-----------|--------------------------------------------------------------------------|---------------------|
| Test      | `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -race -timeout=5m ./...` | exit 0, no races |
| Lint      | `cd backend && golangci-lint run --timeout=3m`                           | exit 0              |

## Scope

**In scope**:
- `backend/api/v1/admin_pools.go` (ListPools — concurrent pool-detail fetch)
- `backend/api/v1/vm_details.go` (GetVMConfig — concurrent current+config fetch)

**Out of scope**:
- `backend/proxmox/nodes_aggregator.go` (the exemplar — do not change)
- `GetVMsResty` (the first call in vm_details — stays sequential, needed for node resolution)
- Response JSON shapes (must not change)
- Frontend (no payload contract change)

## Git workflow

- Branch: `advisor/006-proxmox-parallelism`
- Commits: one per file (`perf(admin): fetch pool details concurrently`, `perf(vm): parallelize current+config fetch in GetVMConfig`)
- Do NOT push unless instructed.

## Steps

### Step 1: Concurrent pool-detail fetch in ListPools

Refactor the loop at `admin_pools.go:63-78` to fetch details concurrently using the `nodes_aggregator` pattern (semaphore cap 8, WaitGroup, results channel). Preserve:
- The `PoolPrefix` filter (skip non-pvmss pools — filter BEFORE launching goroutines, or filter inside and send nil to channel; filtering before is cleaner).
- Partial-failure tolerance: on per-pool error, emit the stub `AdminPoolResponse{PoolID, Comment, Members: []string{}, VMCount: 0}` exactly as today.
- Deterministic order: sort the collected results by `PoolID` before returning (the current sequential loop is implicitly list-ordered; the API consumers likely don't depend on order, but sort to be safe and match `nodes_aggregator`).

Skeleton:
```go
type poolFetch struct {
    idx    int
    resp   AdminPoolResponse
}
// pre-filter to pvmss pools, keep original order index
results := make([]AdminPoolResponse, len(filtered))
sem := make(chan struct{}, 8)
var wg sync.WaitGroup
for i, p := range filtered {
    wg.Add(1)
    i, p := i, p
    go func() {
        defer wg.Done()
        sem <- struct{}{}
        defer func() { <-sem }()
        var detail detailResp
        if err := restyClient.Get(r.Context(), "/pools/"+url.PathEscape(p.PoolID), &detail); err != nil {
            logger.Get().Warn().Err(err).Str("pool", p.PoolID).Msg("failed to fetch pool detail")
            results[i] = AdminPoolResponse{PoolID: p.PoolID, Comment: p.Comment, Members: []string{}, VMCount: 0}
            return
        }
        results[i] = buildPoolResponse(p, detail) // extract current inline body into a helper
    }()
}
wg.Wait()
// results is already in original filtered order (indexed by i) — no sort needed
```
Using a pre-sized slice indexed by position avoids the channel+sort and preserves original order exactly. This is simpler than `nodes_aggregator`'s channel approach and order-safe.

**Verify**: `cd backend && go build ./...` + `go vet ./...` → exit 0. `go test -race -run TestListPools ./api/v1/` if a test exists, else full suite.

### Step 2: Parallelize GetVMCurrentResty + GetVMConfigResty in GetVMConfig

At `vm_details.go`, after `node` is resolved (line 318) and `vmSummary` extracted (line 331), launch the two independent fetches concurrently. Use `errgroup` (import `golang.org/x/sync/errgroup`) for clean first-error cancellation, or the WaitGroup+channel pattern. `errgroup` is preferred because both calls must succeed (current code fails the whole request on either error).

Skeleton with errgroup:
```go
var (
    current *proxmox.VMCurrent
    cfg     *proxmox.VMConfig
)
g, gctx := errgroup.WithContext(ctx)
g.Go(func() (err error) { current, err = proxmox.GetVMCurrentResty(gctx, client, node, vmid); return err })
g.Go(func() (err error) { cfg, err = proxmox.GetVMConfigResty(gctx, client, node, vmid); return err })
if err := g.Wait(); err != nil {
    writeAppError(w, err)
    return
}
```
Preserve the exact downstream parsing (networks, disks, cloud-init, EFI/TPM) unchanged — only the two fetch lines move into goroutines. The `current`/`cfg` variables must be declared before the goroutines and assigned inside (closure capture is fine since each goroutine writes a distinct variable and `g.Wait()` provides the happens-before).

**Verify**: `cd backend && go build ./...` + `go vet ./...`. `go test -race -run TestGetVMConfig ./api/v1/` (plan 005 tests) → passes, no race.

### Step 3: Verify with race detector

Run the full offline suite with `-race` to confirm no data race was introduced (closure writes, shared `results` slice indexing, etc.).

**Verify**: `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -race -timeout=10m ./...` → exit 0, no races.

## Test plan

- Plan 005's `TestGetVMConfig_*` and (if added) `TestListPools_*` must still pass unchanged — they characterize the response shape, which must not change.
- Add a focused test `TestListPools_PartialFailure_StubsMissing` if plan 005 didn't: seed N pools, make one detail fetch fail, assert the response still contains all N with a stub for the failed one.
- Verify: `go test -race -timeout=10m ./...` → all pass, no races.

## Done criteria

- [ ] `admin_pools.go` ListPools fetches pool details concurrently (cap 8), preserves partial-failure stubs, preserves response order/shape
- [ ] `vm_details.go` GetVMConfig fetches current+config concurrently after node resolution; response shape unchanged
- [ ] `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -race -timeout=10m ./...` exits 0 (no races)
- [ ] `cd backend && golangci-lint run --timeout=3m` exits 0
- [ ] No response JSON field added/removed/renamed (diff the handler output struct usage)
- [ ] No files outside `admin_pools.go`/`vm_details.go` modified
- [ ] `docs/plans/advisor/README.md` status row updated

## STOP conditions

Stop and report if:
- `golang.org/x/sync/errgroup` is not in `go.mod` (it's listed, but if it was removed, `go get golang.org/x/sync/errgroup` first — do not proceed without the dep).
- `GetVMCurrentResty`/`GetVMConfigResty` signatures changed (drift) — the concurrent calls assume both take `(ctx, client, node, vmid)`.
- The race detector reports a real race in the new code — do not suppress with `//nolint`; fix the synchronization first.
- A handler test (plan 005) fails after the change — the response shape changed unexpectedly; revert and report.
- `restyClient.Get` is not safe for concurrent use on the same client (check `proxmox/resty_client.go`; go-resty is generally concurrency-safe, but if the wrapper adds shared mutable state, report it).

## Maintenance notes

- The pool fetch cap (8) matches `nodes_aggregator`'s `maxConcurrent`; if a central constant is later introduced, update both.
- If Proxmox rate-limits per token, concurrent fan-out may hit limits sooner — monitor; the cap of 8 is a deliberate throttle.
- A reviewer should confirm `errgroup.WithContext` cancels the sibling call on first error (it does) so a slow second call doesn't leak.
