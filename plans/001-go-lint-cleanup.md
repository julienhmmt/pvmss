# Plan 001: Clean up Go lint failures

> **Executor instructions**: Follow the steps in order. Run every verification
> command and confirm the expected result before moving to the next step. If any
> STOP condition occurs, stop and report — do not improvise.

## Status

- **Priority**: P1
- **Effort**: L
- **Risk**: MED
- **Depends on**: none
- **Category**: tech-debt
- **Planned at**: commit `a84917fa`, 2026-08-24
- **Issue**: (none)

## Why this matters

`make server-lint` currently reports 412 issues. The repo cannot pass the
`qualif` target until lint is green. The biggest buckets live in coverage test
files: missing `wsl_v5` whitespace, duplicated JSON-decode patterns, and
repeated string literals. This plan brings `server/` lint to zero while
preserving the existing test coverage and `//nolint` conventions.

## Current state

- Linter config: `server/.golangci.yml` enables 50 linters and does not exclude
  the stale advisor worktrees under `.worktrees/`.
- The latest lint output (`golint.txt`) shows 412 issues, many at paths like
  `../.worktrees/plan-006/server/...`. Those files belong to stale worktrees and
  must not be linted as part of the main `server/` module.
- Top categories once worktrees are excluded:
  - `wsl_v5`: 250
  - `goconst`: 73
  - `dupl`: 24
  - `revive`: 14
  - `gosec`: 16
  - `paralleltest`: 13
  - `nolintlint`: 12
  - `errcheck`: 4
  - `errorlint`: 2
  - `funlen`: 1
  - `intrange`: 1
  - `noctx`: 2
- Example `wsl_v5`: `server/internal/httpapi/auth_coverage_test.go:14` cuddles
  `var body struct { ... }` directly after the `func` signature; a blank line is
  required above it.
- Example `dupl`: `server/internal/httpapi/admin_audit_test.go:220-237`
  duplicates `admin_ops_coverage_test.go:41-58`. Both do:
  `newAdminOpsHandler` → cookie → request → `json.Unmarshal` into
  `map[string]string` → check `body["code"]`.
- Example `goconst`: `auth_coverage_test.go:52` uses `"invalid_request"`
  literally even though `apiCodeInvalidRequest` already exists in the package.

## Commands you will need

| Purpose | Command | Expected on success |
|---|---|---|
| Baseline | `make server-lint` | shows current issue count |
| Worktree-free baseline | `make server-lint` | no `../.worktrees/` paths |
| Single linter | `cd server && golangci-lint run --timeout=5m -E wsl_v5 ./...` | exit 0 |
| Tests | `make server-test` | exit 0 |

## Suggested executor toolkit

- `golang-code-style` and `golang-lint` skills for Go conventions and `//nolint`
  usage.
- Reuse existing package constants (`apiCodeInvalidRequest`, `apiCodeNotFound`,
  `apiCodeForbidden`, `auditTestCluster`, `crossSecondaryCluster`, etc.) before
  creating new ones.

## Scope

**In scope**:

- `server/.golangci.yml`
- All `server/**/*.go` files flagged by `golangci-lint`, with priority on
  `*_coverage_test.go` and `*_test.go` files.
- `//nolint` directive cleanup.

**Out of scope**:

- Disabling linters.
- Refactoring production code beyond the minimal changes needed to satisfy lint.
- `web/`, root-level files, or any other module.
- The content of `.worktrees/plan-*`; only exclude those paths from lint.

## Git workflow

- Branch: `advisor/001-go-lint-cleanup`
- Commit per linter pass or per package group; message style: lowercase
  imperative, e.g. `fix wsl_v5 in httpapi coverage tests`.
- Do not push.

## Steps

### Step 1: Exclude stale worktrees and re-baseline

1. Edit `server/.golangci.yml` and add `\.worktrees$` (or the smallest regex
   that matches the observed `../.worktrees/plan-*/server/...` paths) to
   `linters.exclusions.paths`, alongside the existing `vendor$`, `third_party$`,
   `testutils$`, `examples$` entries.
2. Run `make server-lint`.
3. Record the remaining issue count and category breakdown.

**Verify**: `make server-lint` no longer reports any path under `.worktrees/`.

**STOP**: If the issue count does not decrease or worktree paths remain, check
that the exclusion regex matches the actual path format and report.

### Step 2: Fix `wsl_v5` whitespace

1. Run `make server-lint` and capture all `wsl_v5` issues.
2. For each reported line, insert a blank line immediately before the flagged
   statement, unless it is the first statement in a block (the config sets
   `wsl_v5.allow-first-in-block: true`).
3. Work in small batches (one package or one `_coverage_test.go` file at a time).
   After each batch, run
   `cd server && golangci-lint run --timeout=5m -E wsl_v5 ./...`.
4. Preserve logic and order; only add or remove blank lines.

**Verify**: `cd server && golangci-lint run --timeout=5m -E wsl_v5 ./...` exits 0.

### Step 3: Fix `dupl` duplication in tests

1. Extract the repeated JSON-decode-and-check pattern into a helper such as:

   ```go
   func assertJSONErrorCode(t *testing.T, body []byte, want string)
   ```

   Place it in an existing test helper file in the package (create a small
   `*_test.go` helper file if necessary), or reuse one if it already exists.
2. Replace the duplicated blocks in the flagged pairs (e.g.,
   `admin_audit_test.go:220-237` / `admin_ops_coverage_test.go:41-58` and the
   method-not-allowed / invalid-JSON pairs in `admin_ops_coverage_test.go` and
   `vm_metrics_coverage_test.go`).

**Verify**: `cd server && golangci-lint run --timeout=5m -E dupl ./...` exits 0.

### Step 4: Fix `goconst`

1. Run `make server-lint` and capture all `goconst` issues.
2. When the linter says a constant already exists, use that constant.
3. When no constant exists, add a package-level `const` block in the test file or
   the package's shared test helper. Avoid creating a `testutils/` directory
   because it is excluded from lint.
4. Common literals to extract include: `"invalid_request"`, `"not_found"`,
   `"method_not_allowed"`, `"default"`, `"secondary"`, `"pve-node-01"`,
   `"secret"`, `"pvmss"`, `"alice@pve"`, `"list"`, `"create"`, `"delete"`,
   `"start"`, etc.

**Verify**: `cd server && golangci-lint run --timeout=5m -E goconst ./...` exits 0.

### Step 5: Fix `paralleltest`

1. For each top-level test flagged by `paralleltest`, add `t.Parallel()` as the
   first statement if the test is safe to run in parallel. If it uses shared
   mutable fixtures, serial database state, or `t.Setenv`, keep it serial and
   use the existing `//nolint:paralleltest // <reason>` style.
2. For subtests using `t.Run`, add `t.Parallel()` inside the subtest body when
   safe.

**Verify**: `cd server && golangci-lint run --timeout=5m -E paralleltest ./...` exits 0.

### Step 6: Fix `nolintlint`

1. The config requires `nolintlint.require-explanation: true` and
   `require-specific: true`. For each flagged `//nolint`:
   - Remove it if it is unused.
   - Replace broad `//nolint` with the specific linter names.
   - Add a one-line explanation after `//`.
2. If a file-level `//nolint` is no longer needed, remove it. If it is still
   needed, make it specific and add an explanation.

**Verify**: `cd server && golangci-lint run --timeout=5m -E nolintlint ./...` exits 0.

### Step 7: Fix the remaining linter issues

1. `revive`:
   - Rename variables that shadow `cap`.
   - Make `context.Context` the first parameter in helper signatures.
   - Rename unused parameters to `_`.
2. `gosec`: Add `//nolint:gosec // <reason>` for test fixtures that are
   intentionally using fake secrets or test commands (e.g., `G101`, `G204`,
   `G115`, `G124`, `G302`, `G304`).
3. `errcheck`: Check ignored error returns such as
   `remote.SetReadDeadline(...)`.
4. `errorlint`: Use `errors.Is` instead of direct `!=` when comparing
   `auth.ErrUnauthenticated`.
5. `funlen`: Split `TestLoad` in `config_test.go` into smaller subtests.
6. `intrange`: Replace `for i := 0; i < 25; i++` with `for i := range 25`.
7. `noctx`: Use `httptest.NewRequestWithContext` in
   `router_admin_internal_test.go`.

Work one linter at a time. After each:

**Verify**: `cd server && golangci-lint run --timeout=5m -E <linter> ./...` exits 0.

Finally:

**Verify**: `make server-lint` exits 0.

## Test plan

- Run `make server-test` after every major step. It must exit 0.
- Run `make server-vet` at the end. It must exit 0.
- Every `//nolint` directive must have a real, specific explanation.

## Done criteria

- [ ] `make server-lint` exits 0 with 0 issues.
- [ ] `make server-test` exits 0.
- [ ] `make server-vet` exits 0.
- [ ] No worktree paths appear in lint output.
- [ ] Only `server/` files (plus `server/.golangci.yml` and this plan) are
      modified.
- [ ] `plans/README.md` status row is updated.

## STOP conditions

- A fix requires touching files outside `server/`, a worktree directory, or a
  generated file whose source you cannot identify.
- `make server-test` fails after a lint-only change and the failure is not an
  obvious ordering side effect from `t.Parallel()`.
- The `.worktrees` exclusion does not stop worktree issues and you cannot
  determine why.
- The list of `wsl_v5` issues is not shrinking as you fix them.

## Maintenance notes

- Future coverage-test generators must respect `wsl_v5` and `goconst` rules, or
  emit specific `//nolint` directives with explanations.
- Run `make server-lint` before merging any new `_coverage_test.go` file.
