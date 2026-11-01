# Plan 004: Cover the remaining error paths in internal/auth

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 8a86e280..HEAD -- server/internal/auth/`

## Status

- **Priority**: P3
- **Effort**: M
- **Risk**: LOW (tests only — no production code changes)
- **Depends on**: none
- **Category**: tests
- **Planned at**: commit `8a86e280`, 2026-11-01

## Why this matters

`internal/auth` holds the session manager (cookie issuance, CSRF tokens,
session resolve/revoke) — the package every authenticated request depends
on. Coverage is currently **84.1%** (fresh `go test -coverprofile`; the
60.9% figure in earlier notes was a stale test-cache reading predating
this branch's coverage commits). The remaining gaps are concentrated in
error and boundary paths — exactly the paths a future refactor here needs
characterized before it can claim behavior preservation.

## Current state

Uncovered/weak functions per `go tool cover -func` (run against
`./internal/auth/`):

| Function | Coverage | Likely missing paths |
| --- | --- | --- |
| `NewSessionManager` (session.go:111) | 66.7% | error returns: invalid/short `SESSION_SECRET`, cookie-secure plumbing |
| `CSRFToken` (session.go:236) | 75.0% | missing session, expired session, token regeneration |
| `randomHex` (session.go:267) | 75.0% | entropy-failure path (if injectable) — may be un-coverable |
| `SetCookie` (session.go:121) | 78.6% | cookie attribute edge cases |
| `Resolve` (session.go:150) | 81.8% | expired/missing/idle-timeout boundaries |
| `Logout` (session.go:173) | 83.3% | unknown-session case |
| `RevokeOthers` (session.go:189) | 83.3% | keeping current session, revoking none/several |

File inventory: `server/internal/auth/session.go` is the only source file
(274 lines); tests live in `server/internal/auth/session_test.go`
(321 lines). Read both fully before writing anything — pick the real
missing branches from the source, don't guess from this table.

Conventions (from `AGENTS.md` + existing tests): stdlib `testing` (no
testify dependency in this package unless already present — check
`session_test.go` imports and match), table-driven tests named
`Test<Function>_<Scenario>`, `t.Helper()` on helpers, `_test` package
suffix only if the existing file uses it. Store test fixtures: look at how
`session_test.go` builds its `*store.Store` today and reuse that exact
helper/pattern.

## Commands you will need

| Purpose | Command | Expected on success |
| --- | --- | --- |
| Tests | `cd server && go test ./internal/auth/` | all pass |
| Coverage | `cd server && go test -count=1 -coverprofile=/tmp/auth.cover ./internal/auth/ && go tool cover -func=/tmp/auth.cover` | total ≥ 90% |
| Lint | `make server-lint` (repo root) | `0 issues.` |

## Scope

**In scope:**
- `server/internal/auth/session_test.go` (extend) **or** a new
  `server/internal/auth/session_error_test.go` — prefer a new file so the
  diff doesn't entangle with any parallel work in the existing test file.
  Match whichever package clause the existing file uses.

**Out of scope:**
- `server/internal/auth/session.go` — characterization only; any *bug*
  found while writing tests is reported, not fixed here
- `server/internal/httpapi/auth*.go` — a different layer (HTTP auth
  handlers), and `httpapi/auth_test.go` currently carries uncommitted WIP
  owned by another task
- Anything outside `server/internal/auth/`

## Git workflow

- Branch: `refactor/audit-2026-11-cleanup`.
- One commit: `test(auth): cover session manager error and boundary paths`
  (style from `git log`: `test(auth): cover RevokeOthers, EnsureCSRFCookie and Owner`).
- Stage only the auth test file(s) by name. Do NOT push or open a PR.

## Steps

### Step 1: map the real gaps

Run `go test -count=1 -coverprofile=/tmp/auth.cover ./internal/auth/ &&
go tool cover -func=/tmp/auth.cover`, then open `session.go` and mark the
exact uncovered statements per function (branches the percentages hide).
List them before writing tests.

**Verify**: you have a concrete list of uncovered lines/branches, not just
function percentages.

### Step 2: write the tests

One table-driven test per gap cluster. Cover, where the source supports
it:

- `NewSessionManager` error returns (secret too short / invalid input —
  read `config`'s contract for what it validates vs. what auth validates)
- `Resolve` with an expired session and an idle-timed-out session (the
  8h idle / 24h max-age constants — `auth.MaxSessionAge`)
- `CSRFToken` on missing/expired session
- `Logout`/`RevokeOthers` on unknown sessions and multi-session sets
- `SetCookie` boundary cases that exist in the source

If a listed gap turns out unreachable from the public API (e.g.
`randomHex` entropy failure needs a fault-injectable reader it doesn't
have), skip it and say so — chasing 100% is out of scope; 90% is the
target.

**Verify**: `cd server && go test -count=1 ./internal/auth/` → all pass,
including every new `Test*`.

### Step 3: coverage gate + lint + commit

**Verify**: `go tool cover -func=/tmp/auth.cover` (fresh profile) →
`internal/auth` total ≥ 90% and each function in the table ≥ 90% *or*
explicitly documented as unreachable-from-API in the commit message.
**Verify**: `make server-lint` → `0 issues.`
Commit only the auth test file(s).

## Test plan

This plan *is* the test plan. Structural model: the existing
`session_test.go` — same fixture helpers, same assertion style.

## Done criteria

- [ ] `cd server && go test -count=1 ./internal/auth/` all pass
- [ ] Fresh `go tool cover -func` shows `internal/auth` ≥ 90% total
- [ ] No production file modified (`git diff --stat` shows only test file(s))
- [ ] `make server-lint` → `0 issues.`
- [ ] `git status` shows no WIP file staged or committed
- [ ] `plans/README.md` status row updated

## STOP conditions

- A test reveals the code doesn't behave as the API contract implies
  (e.g. an expired session still resolves) — that's a bug finding: report
  the `file:line` and the failing case, do not fix `session.go` here.
- Covering a path would require changing production signatures (e.g. to
  inject an entropy source) — report instead of editing production code.
- `session_test.go` uses a package/fixture layout incompatible with a
  second test file (e.g. unexported helpers needed in both) — then extend
  the existing file instead, but keep your additions at the end of the
  file so the diff stays reviewable.

## Maintenance notes

- These are characterization tests: they pin *current* behavior. If a
  later change intentionally alters session semantics, expect to update
  these cases and read them as the spec they replaced.
- `internal/auth` sessions back every authenticated request; if
  `MaxSessionAge` or the idle TTL ever changes, the boundary tests in step
  2 are the ones that must be revisited.
