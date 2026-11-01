# Plan 003: main.go hygiene — generic capability assertion + split by phase

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 8a86e280..HEAD -- server/cmd/pvmss/`

## Status

- **Priority**: P2
- **Effort**: S–M
- **Risk**: LOW
- **Depends on**: plans/002-policy-shared-service.md (002 edits `policy.New` call sites inside `main.go`; split first and 002's edits land in moved code)
- **Category**: tech-debt
- **Planned at**: commit `8a86e280`, 2026-11-01

## Why this matters

`server/cmd/pvmss/main.go` is 802 lines and the highest-churn file in the
repo (48 touches in 3 months) — every feature wires another handler here.
Two cheap structural wins: `resolveClusterClientInterfaces` is 12
hand-written type assertions (~60 lines) that a generic helper collapses
to ~15, and the file splits cleanly along the phases it already names in
comments. Pure structure, no behavior change.

## Current state

`server/cmd/pvmss/` contains: `main.go` (802 lines), `lifecycle.go`
(`listenAddr`, `vcsSetting`, banner/log helpers), `lease_sweep.go`
(`runLeaseSweep`), `lifecycle_test.go`, `main_test.go`, `metrics_e2e_test.go`.

`main.go` is already phase-organized:

- `main`, `run` (composition root, :78), `serve` (:387)
- startup helpers: `loadConfig` (:203), `openStore` (:230), `reopenAfterQuarantine` (:252), `initCluster` (:268), `initInventory` (:350), `resolveWebBuildDir` (:675), `validateWebBuildDir` (:694)
- router wiring: `routerDeps` (:458), `buildRouter` (:476), `clusterClientInterfaces` (:609), `resolveClusterClientInterfaces` (:628), `inventoryFreshness` (:716), `registryClusterRefresh` (:598)
- background work: `discoverClusterDisplayNames` (:310), `runSnapshotSweep` (:749), `runAuditPrune` (:767)
- consts: timeouts (:36-41), `appVersion`, `errNoClustersConfigured`, `displayNameDiscoveryTimeout`, `auditPruneInterval`, `snapshotSweepInterval`

`resolveClusterClientInterfaces` (main.go:628-670) is 12 copies of:

```go
if c.writer, ok = clusterClient.(cluster.Writer); !ok {
	return c, errors.New("cluster client does not implement Writer")
}
```

## Commands you will need

| Purpose | Command | Expected on success |
| --- | --- | --- |
| Build | `cd server && go build ./...` | exit 0 |
| Vet | `cd server && go vet ./...` | exit 0 |
| Tests | `cd server && go test ./cmd/pvmss/` | all pass |
| Lint | `make server-lint` (repo root) | `0 issues.` |

## Scope

**In scope:**
- `server/cmd/pvmss/main.go`
- `server/cmd/pvmss/wiring.go` (create)
- `server/cmd/pvmss/startup.go` (create)
- `server/cmd/pvmss/sweeps.go` (create)

**Out of scope:**
- `server/cmd/pvmss/lifecycle.go`, `lease_sweep.go` — already have a single purpose; do not merge or rename them
- Any signature, timeout value, log message, or wiring order change — this plan moves and compresses code only
- `internal/` packages — nothing outside `cmd/pvmss`
- WIP files elsewhere in the tree (see plan 001's list)

## Git workflow

- Branch: `refactor/audit-2026-11-cleanup`.
- Two commits, in order (move before compress — keep the diff readable):
  1. `refactor(server): split cmd/pvmss main.go by startup phase`
  2. `refactor(server): collapse cluster capability assertions into a generic helper`
- Stage only `server/cmd/pvmss/*`. Do NOT push or open a PR.

## Steps

### Step 1: split main.go by phase (commit 1)

Move (verbatim — no edits beyond the file-level organization):

- `startup.go` ← `loadConfig`, `openStore`, `reopenAfterQuarantine`, `initCluster`, `initInventory`, `resolveWebBuildDir`, `validateWebBuildDir`, `discoverClusterDisplayNames` + `displayNameDiscoveryTimeout`
- `wiring.go` ← `routerDeps`, `buildRouter`, `clusterClientInterfaces`, `resolveClusterClientInterfaces`, `inventoryFreshness`, `registryClusterRefresh`
- `sweeps.go` ← `runSnapshotSweep`, `runAuditPrune` + `auditPruneInterval`, `snapshotSweepInterval` (sits next to `lease_sweep.go` which keeps the same pattern)
- `main.go` keeps `main`, `run`, `serve`, the timeout/version consts and the `//nolint:wsl_v5` package comment.

Distribute imports to the files that need them (`goimports`-style; `gofmt -s` will not remove unused imports — `go build` flags them, so fix until clean).

**Verify**: `cd server && go build ./... && go vet ./... && go test ./cmd/pvmss/` → exit 0 / all pass. `wc -l server/cmd/pvmss/main.go` ≈ 200. Commit 1.

### Step 2: generic capability assertion (commit 2)

In `wiring.go`, add:

```go
// must asserts the cluster client implements capability interface T.
// Every concrete client (Fake, Proxmox) satisfies all of them; a missing
// one is a build-time wiring error, not a runtime condition.
func must[T any](client cluster.Client, name string) (T, error) {
	capability, ok := client.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("cluster client does not implement %s", name)
	}
	return capability, nil
}
```

and rewrite `resolveClusterClientInterfaces` as sequential `must[...]` calls
(one per field, same error strings — they become the `name` argument).
Go 1.27 toolchain (`server/go.mod`); generics are fine. If `go vet` or a
linter prefers it, a two-value form returning `(T, bool)` is equally
acceptable — pick whichever reads better, but keep identical error
messages so `main_test.go` expectations (if any) hold.

Check `server/cmd/pvmss/main_test.go` for tests of
`resolveClusterClientInterfaces` error strings before finalizing — the
strings must not change.

**Verify**: `cd server && go build ./... && go vet ./... && go test ./cmd/pvmss/` → exit 0 / all pass; `make server-lint` → `0 issues.`. Commit 2.

## Test plan

No new tests — file moves and a mechanical compression. `main_test.go` /
`lifecycle_test.go` must pass unchanged; if a test references a moved
symbol it needs only an import/file update, never an assertion change.

## Done criteria

- [ ] `server/cmd/pvmss/` has `main.go` ≈200 lines plus `wiring.go`, `startup.go`, `sweeps.go`
- [ ] `resolveClusterClientInterfaces` contains no hand-written type assertions
- [ ] `cd server && go build ./... && go vet ./... && go test ./cmd/pvmss/` clean; `make server-lint` → `0 issues.`
- [ ] Error strings identical to before (`git show` the old function and diff the messages)
- [ ] `git status` shows no WIP file staged or committed
- [ ] `plans/README.md` status row updated

## STOP conditions

- A test in `cmd/pvmss` fails after the move without an import fix — the move changed initialization order or dropped a symbol; revert the commit and re-attempt smaller.
- `must[T]` cannot express one of the 12 assertions without changing behavior (e.g. a field typed as a concrete type, not an interface) — report which one and skip the generic helper for it.
- Lint demands a different pattern (e.g. `funlen`/`wsl_v5` on the new layout) — fix the lint properly; do not add `//nolint` to silence it.

## Maintenance notes

- `main_test.go` and `lifecycle_test.go` stay in `package main`; file moves don't affect them.
- After the split, `wiring.go` is where every new handler constructor lands — the review habit becomes "check wiring.go diff" for route additions.
- If plan 002 was skipped, `buildRouter` still constructs `policyService` internally — fine, this plan doesn't care where that line lives.
