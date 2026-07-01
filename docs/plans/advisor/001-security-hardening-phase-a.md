# Plan 001: Ship security-hardening Phase A (PROXMOX_VERIFY_SSL, pool-name regex, tag regex on all mutators)

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `docs/plans/advisor/README.md`.
>
> **Drift check (run first)**: `git diff --stat d427838..HEAD -- helm/values.yaml pvmss-deployment.yaml example.env backend/api/v1/admin_mutations.go backend/api/v1/admin_tags.go backend/api/v1/validation.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: LOW (targeted config + validation changes; existing plan already verified against code)
- **Depends on**: none
- **Category**: security
- **Planned at**: commit `d427838`, 2026-07-01

## Why this matters

A prior security audit (`docs/plans/2026-05-22-backend-security-hardening.md`) identified production blockers that are still OPEN two months later. S1 ships `PROXMOX_VERIFY_SSL=false` as the default in all deployment artifacts, exposing the Proxmox API token to MITM. S2 lets unvalidated pool names reach the Proxmox API via string concatenation. S3 validates tag names only on create, not on delete/set-color. These are the "must ship before next deploy" Phase A items. S4/S5 (ISO/bridge allowlist) are already DONE.

## Current state

The existing plan is the authoritative spec. Read `docs/plans/2026-05-22-backend-security-hardening.md` lines 99-109 (Phase A) before starting. Key excerpts inlined here so this plan is standalone:

**S1 — PROXMOX_VERIFY_SSL shipped false (HIGH, Transport):**

- `example.env:6` → `PROXMOX_VERIFY_SSL=false`
- `helm/values.yaml:52` → `PROXMOX_VERIFY_SSL: "false"`
- `pvmss-deployment.yaml:112-113` → `- name: PROXMOX_VERIFY_SSL` / `value: "false"`
- Code default is already secure: `backend/env/loader.go:33` → `parseBoolDefault("PROXMOX_VERIFY_SSL", true)`. Only the deployment artifacts ship the insecure value.

**S2 — Pool name not validated (HIGH, Validation):**

- `backend/api/v1/admin_mutations.go:161` (per existing plan) concatenates `constants.PoolPrefix + req.Pool` without validation. Pool name reaches the Proxmox API unvalidated.

**S3 — Tag regex only on create (HIGH, Validation):**

- `backend/api/v1/admin_tags.go:15` defines `tagNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,50}$`)`.
- `validateTagName` is applied in `CreateTag` at `admin_tags.go:71` (call shown in Current-state excerpt below).
- `DeleteTag` (`admin_tags.go:163-204`) and `SetTagColor` (`admin_tags.go:108-160`) do NOT call `validateTagName` — they read `name` from the URL param (`ps.ByName("name")`) and proceed without regex validation.

Repo conventions to match:

- Validation helpers live in `backend/api/v1/validation.go` (e.g. `validateTagName`). Add `validatePoolName` there per the existing plan's "Critical files to modify" table.
- Errors use `errBadRequest(w, "message")` / `writeAppError(w, err)` — see `admin_tags.go:71-73` for the pattern.
- Regexes declared at package scope with `var nameRegex = regexp.MustCompile(...)` — see `admin_tags.go:15`.

## Commands you will need

| Purpose   | Command                                                                  | Expected on success |
|-----------|--------------------------------------------------------------------------|---------------------|
| Test      | `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -timeout=5m ./...` | exit 0, all pass    |
| Lint      | `cd backend && golangci-lint run -v --timeout=3m`                        | exit 0              |
| Format    | `cd backend && go fmt ./...`                                             | exit 0              |

## Scope

**In scope** (the only files you should modify):

- `helm/values.yaml`
- `pvmss-deployment.yaml`
- `example.env`
- `backend/api/v1/validation.go` (add `validatePoolName`)
- `backend/api/v1/admin_mutations.go` (call `validatePoolName` before pool concat)
- `backend/api/v1/admin_tags.go` (apply `validateTagName` in `DeleteTag` and `SetTagColor`)
- New/updated tests: `backend/api/v1/validation_test.go` (if not present, create; else extend)

**Out of scope** (do NOT touch):

- `backend/env/loader.go` — code default is already `true`; only deployment artifacts are wrong.
- S6/S10/S11 and Phase B/C items — separate effort. Do not bundle.
- `vm_create.go` allowlist (S4/S5) — already DONE per the status table.

## Git workflow

- Branch: `advisor/001-security-phase-a`
- Commit per logical unit (S1 config, S2 pool regex, S3 tag regex). Message style: conventional commits, e.g. `fix(security): validate pool name before Proxmox API concat (S2)`.
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Flip PROXMOX_VERIFY_SSL to true in deployment artifacts (S1)

Change the three artifacts:

- `example.env:6`: `PROXMOX_VERIFY_SSL=true`
- `helm/values.yaml:52`: `PROXMOX_VERIFY_SSL: "true"`
- `pvmss-deployment.yaml:113`: `value: "true"`

Add a short comment near each (where the format allows) noting self-signed Proxmox deployments must mount the CA cert and may set this to `false` explicitly. For `example.env`, add a comment line above: `# Set to false ONLY for self-signed labs; mount the Proxmox CA cert for production`.

**Verify**: `grep -rn "PROXMOX_VERIFY_SSL" example.env helm/values.yaml pvmss-deployment.yaml` → all non-comment occurrences show `true` (or `"true"`).

### Step 2: Add validatePoolName and call it before pool concat (S2)

In `backend/api/v1/validation.go`, add:

```go
var poolNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,50}$`)

// validatePoolName rejects pool names that could inject unexpected characters
// into the Proxmox API path or pool-id concatenation.
func validatePoolName(name string) error {
	if !poolNameRegex.MatchString(name) {
		return fmt.Errorf("invalid pool name: must match %s", poolNameRegex.String())
	}
	return nil
}
```

Match the existing `validateTagName` signature/style in the same file. Add the `regexp`/`fmt` imports only if not already present.

In `backend/api/v1/admin_mutations.go`, locate the pool-create handler that does `constants.PoolPrefix + req.Pool` (around line 161 per the existing plan). Before the concatenation, call `validatePoolName(req.Pool)` and return `errBadRequest` on error:

```go
if err := validatePoolName(req.Pool); err != nil {
    errBadRequest(w, err.Error())
    return
}
```

**Verify**: `cd backend && go build ./...` → exit 0. `cd backend && go vet ./...` → exit 0.

### Step 3: Apply validateTagName to DeleteTag and SetTagColor (S3)

In `backend/api/v1/admin_tags.go`:

- In `SetTagColor` (line 108+): after `name := ps.ByName("name")` and the empty check (line 115), add `if err := validateTagName(name); err != nil { writeAppError(w, err); return }` before the "known tag" lookup.
- In `DeleteTag` (line 163+): after `name := ps.ByName("name")` and the empty check, and before the `constants.RequiredTag` check, add the same `validateTagName(name)` guard.

**Verify**: `grep -n "validateTagName" backend/api/v1/admin_tags.go` → appears in CreateTag, SetTagColor, and DeleteTag.

### Step 4: Add tests for the validators

In `backend/api/v1/validation_test.go` (create if absent, package `apiv1` or `apiv1_test` per existing convention — check `admin_db_test.go` uses `package apiv1_test`), add table-driven tests:
- `TestValidateTagName_<Scenario>`: valid names, empty, too long (>50), disallowed chars (`/`, `..`, space).
- `TestValidatePoolName_<Scenario>`: valid, empty, disallowed chars, length boundary.

Follow the table-driven style from `backend/constants/constants_test.go` or existing validation tests.

**Verify**: `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -run 'TestValidate(Tag|Pool)Name' ./api/v1/ -v` → all pass.

## Test plan

- New validator unit tests (Step 4) covering valid/invalid/edge cases for both `validateTagName` and `validatePoolName`.
- Existing offline suite must still pass (no regression in tag/pool handlers).
- Verification: `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -timeout=5m ./...` → all pass.

## Done criteria

- [ ] `grep -rn "PROXMOX_VERIFY_SSL" example.env helm/values.yaml pvmss-deployment.yaml` shows only `true`/`"true"` for non-comment occurrences
- [ ] `validatePoolName` exists in `validation.go` and is called before `constants.PoolPrefix + req.Pool` in `admin_mutations.go`
- [ ] `validateTagName` is called in `CreateTag`, `SetTagColor`, and `DeleteTag`
- [ ] `cd backend && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -timeout=5m ./...` exits 0
- [ ] `cd backend && golangci-lint run --timeout=3m` exits 0
- [ ] No files outside the in-scope list are modified (`git status`)
- [ ] `docs/plans/advisor/README.md` status row updated

## STOP conditions

Stop and report back (do not improvise) if:
- The pool-create concatenation site in `admin_mutations.go` is not at/near line 161, or the surrounding code does not match `constants.PoolPrefix + req.Pool` (codebase has drifted).
- `validateTagName` does not exist in `validation.go` (the plan assumes it does — if it must be created, confirm the intended location before proceeding).
- S4/S5 (ISO/bridge allowlist) appear NOT done in `vm_create.go` (the plan assumes they are DONE; if they're open too, the scope needs maintainer input before bundling).
- A step's verification fails twice after a reasonable fix attempt.

## Maintenance notes

- After S1 lands, any deployment using a self-signed Proxmox CA must mount the CA cert into the container and keep `PROXMOX_VERIFY_SSL=true`, or explicitly set `false` with documented justification. Add a deploy-doc note if a `docs/deploy` section exists.
- When `validatePoolName` is later reused (e.g. pool delete), it lives in `validation.go` — import it, don't re-declare.
- A reviewer should confirm the regex is applied to EVERY code path that builds a pool-id from user input, not just the one site in scope.
