# Plan 001: Delete verified dead code (cloud-init leftovers, stale constructors)

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 8a86e280..HEAD -- server/internal/cluster/ server/internal/recovery/ server/internal/httpapi/vm_create.go server/internal/store/store.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: tech-debt
- **Planned at**: commit `8a86e280`, 2026-11-01

## Why this matters

`deadcode` (reachability from every `main` in the module, test callers
excluded) plus a manual grep of *all* `.go` files found exported and
internal functions with zero callers anywhere — production or test. Two are
leftovers of the cloud-init removal (2026-10-31, see `TECH_DEBT.md`); the
rest are stale constructors and an unused fake test knob. Dead code still
has to be read, linted, and migrated during refactors.

## Current state

Each item below was verified with: `grep -rEn '<Name>\(' --include='*.go' .`
returning only the definition (0 production refs, 0 test refs).

| Symbol | File | Notes |
| --- | --- | --- |
| `cluster.IsSnippetCapableStorage` | `server/internal/cluster/client.go:425` | cloud-init snippet-storage leftover; sole reader of the `storageContentSnippets` const (delete the const too) |
| `cluster.IsCloudImageFile` | `server/internal/cluster/proxmox_catalog.go:136` | cloud-image leftover; sole reader of the `cloudImageFileExtensions` var at line 132 (delete the var too) |
| `recovery.MapCatalog` | `server/internal/recovery/catalog.go:14` | exported, zero callers; its helpers `mapNodes`/`mapStorages`/`mapBridges` stay alive via `MapProfiles` — delete only `MapCatalog` |
| `recovery.MapPolicy` | `server/internal/recovery/policy.go:22` | exported, zero callers; helpers `mapVMLimits`/`mapNodeLimits` may be shared — verify before deleting them |
| `httpapi.NewVMCreate` | `server/internal/httpapi/vm_create.go:112` | stale pre-registry constructor; all callers use `NewVMCreateWithRegistry` |
| `store.NewFromDBWithSecret` | `server/internal/store/store.go:39` | zero callers; `NewFromDB` (line 33) has one test caller and stays |
| `cluster.SetFakeSSHKeyError` | `server/internal/cluster/fake_guest.go:175` | fake test knob nobody calls; check whether `state.sshErr` still has readers afterward |

Also flagged by deadcode but **intentionally kept** (do NOT delete — test
fixtures or interface methods): `vm.List` (19 test refs), `cluster.NewRegistry`,
`inventory.NewRefresher`/`NewRegistryFromIndexes`/`NewProjectionFromIndex`/`BuildIndex`,
`cloudinit.ValidateSSHKeys` (7 test refs), all other `SetFake*`/`FakeCalls*` knobs,
`store.NewFromDB`, `testfixture.*`, `ConsoleTicketStore.expireForTest`/`oldestTokenForTest`,
`newProxmoxHTTPClient`/`newProxmoxVNCClient` (test refs), `config.Configuration.LogValue`
(`slog.LogValuer`, reflection-reachable), `inventory.WithClock`.

## Commands you will need

| Purpose | Command | Expected on success |
| --- | --- | --- |
| Build | `cd server && go build ./...` | exit 0 |
| Vet | `cd server && go vet ./...` | exit 0 |
| Tests | `cd server && go test ./...` | all pass |
| Lint | `make server-lint` (repo root) | `0 issues.` |
| Dead check | `cd server && go run golang.org/x/tools/cmd/deadcode@latest ./...` | listed symbols absent |

## Scope

**In scope** (the only files you should modify):
- `server/internal/cluster/client.go`
- `server/internal/cluster/proxmox_catalog.go`
- `server/internal/cluster/fake_guest.go`
- `server/internal/recovery/catalog.go`
- `server/internal/recovery/policy.go`
- `server/internal/httpapi/vm_create.go`
- `server/internal/store/store.go`
- A test file **only if** it references a deleted symbol (none expected — all greps returned zero test refs)

**Out of scope** (do NOT touch, even though they look related):
- Everything deadcode flags that is in the "intentionally kept" list above
- `server/internal/cloudinit/sshkeys.go` — the whole `cloudinit` package is kept on purpose per `AGENTS.md`
- Any other `SetFake*`/`FakeCalls*`/`ResetFake` function — active test fixtures
- Files currently carrying uncommitted WIP in the working tree: `server/internal/httpapi/auth.go`, `ratelimit.go`, `vm_bulk.go`, `server/internal/vm/bulk.go`, `server/go.mod`, and their `*_test.go` counterparts, plus untracked `main_test.go` files in `cluster`, `httpapi`, `inventory`. Never stage these.

## Git workflow

- Branch: already on `refactor/audit-2026-11-cleanup`.
- One commit for the whole deletion. Message style follows the repo
  (`refactor(server): ...`, `chore(server): ...` — see `git log --oneline`).
  Suggested: `refactor(server): delete dead code left by cloud-init removal and stale constructors`.
- Stage only the in-scope files by name (`git add <file> ...`); `git add -A`
  or `git add -u` would sweep in the unrelated WIP listed above.
- Do NOT push or open a PR.

## Steps

### Step 1: re-verify zero callers

For each symbol in the table, run
`grep -rEn '<SymbolName>\(' --include='*.go' server/ | grep -v 'func '`.

**Verify**: every symbol still returns zero references (definitions and
doc comments excluded). If any symbol gained a caller, drop it from the
deletion list and note it in the commit message.

### Step 2: delete the functions and their sole-use data

Delete each function body plus its doc comment. Additionally:
- `client.go`: delete the `storageContentSnippets` const declaration (find
  it with `grep -n 'storageContentSnippets' server/internal/cluster/*.go`).
- `proxmox_catalog.go`: delete the `cloudImageFileExtensions` var and its
  comment block.
- After deleting `SetFakeSSHKeyError`, grep `sshErr` in
  `internal/cluster/`. If it still has readers/writers from other
  `SetFake*` knobs, leave them; if `sshErr` is now write-only or fully
  dead, remove the field and its uses too.
- After deleting `MapCatalog`/`MapPolicy`, check whether `mapNodes`,
  `mapStorages`, `mapBridges`, `mapVMLimits`, `mapNodeLimits` still have
  callers (`grep -rn 'mapNodes\|mapStorages\|mapBridges\|mapVMLimits\|mapNodeLimits' server/`).
  deadcode's output shows they stay reachable via `MapProfiles`; if a
  helper becomes dead, delete it as well. If an entire source file becomes
  empty except the package clause, delete the file.
- After each file edit, check for imports that are now unused and remove
  them (`go build` will flag them).

**Verify**: `cd server && go build ./... && go vet ./...` → exit 0.

### Step 3: test, lint, and confirm nothing new is dead

**Verify**: `cd server && go test ./...` → all pass.
**Verify**: `make server-lint` → `0 issues.`
**Verify**: re-run deadcode → none of the deleted symbols appear, and no
*new* unreachable symbol appears that this plan created (any new entry must
be deleted too or the plan stops).

### Step 4: commit

Stage only the in-scope files and commit with the message above.

**Verify**: `git status --short` → the committed files are gone from the
list; the WIP files (`auth.go`, `ratelimit.go`, `vm_bulk.go`, `bulk.go`,
`go.mod`, their tests, the three `main_test.go`) remain modified/untracked.

## Test plan

No new tests — this plan only deletes unreferenced code. The existing
suite is the net: `go test ./...` must stay green.

## Done criteria

- [ ] `grep -rEn 'IsSnippetCapableStorage\(|IsCloudImageFile\(|MapCatalog\(|MapPolicy\(|httpapi\.NewVMCreate\(|NewFromDBWithSecret\(|SetFakeSSHKeyError\(' --include='*.go' server/` returns nothing but this plan's own docs
- [ ] `storageContentSnippets` and `cloudImageFileExtensions` declarations removed
- [ ] `cd server && go build ./... && go vet ./...` exits 0
- [ ] `cd server && go test ./...` all pass
- [ ] `make server-lint` reports `0 issues.`
- [ ] `git status` shows no WIP file staged or committed
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back (do not improvise) if:

- Any listed symbol has gained a production caller since the plan was
  written (the grep in Step 1 finds a non-definition hit).
- Deleting a helper (`mapNodes`, `mapStorages`, etc.) turns out to break a
  caller you did not expect — that means deadcode's reachability result was
  wrong; report instead of patching.
- `go test` fails after the deletions in a way that traces to a deleted
  symbol (i.e. a reflection- or template-driven reference the greps missed).

## Maintenance notes

- Reviewer should confirm each deletion against `git log -S<symbol>` — every
  symbol's last real caller should be a deleted feature (cloud-init) or a
  superseded constructor.
- Future deadcode runs will keep flagging the intentional test fixtures;
  that list in "Current state" is the standing false-positive set.
