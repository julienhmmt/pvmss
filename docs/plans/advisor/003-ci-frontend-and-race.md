# Plan 003: Add frontend check/lint/test and a race-detector job to CI

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report. When done, update the status row in `docs/plans/advisor/README.md`.
>
> **Drift check (run first)**: `git diff --stat d427838..HEAD -- .github/workflows/go.yml .github/workflows/lint.yml Makefile`
> If any in-scope file changed since this plan was written, compare excerpts
> against live code before proceeding; on mismatch, treat as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW (CI-only; no source change; may surface pre-existing frontend failures — those are bugs to fix separately, not a reason to revert CI)
- **Depends on**: none
- **Category**: dx / tests
- **Planned at**: commit `d427838`, 2026-07-01

## Why this matters

CI only runs `make test-offline` (Go, no race flag). The SvelteKit frontend — the entire user-facing surface — has zero CI validation: no `bun run check` (svelte-check typecheck), no `bun run lint` (eslint), no `bun run test` (vitest). Type errors, lint regressions, and test failures reach `main` undetected. The Go race detector is also absent from CI despite concurrent code in `proxmox/cache.go` and `state/manager_cache.go`. This plan closes both gaps and is a prerequisite for the auth/VM handler test plans (004/005), which need CI to actually run new tests.

## Current state

**Backend CI (`.github/workflows/go.yml`):**
- Triggers: push/PR to `main`.
- Single `build` job: checkout (`actions/checkout@v7`), setup-go (`actions/setup-go@v6`, `go-version-file: backend/go.mod`, `cache: true`), `go mod verify`, `go build -v ./...`, then `make test-offline` with offline env vars. No `-race`.

**Lint CI (`.github/workflows/lint.yml`):**
- `super-linter` job: validates CSS, markdown-prettier, YAML (excludes noVNC/bulma).
- `golangci-lint` job: `golangci/golangci-lint-action@v9`, `working-directory: backend`, `args: -v --timeout=3m`.
- No frontend lint/test job.

**Frontend scripts (`frontend/package.json`):**
- `"check": "svelte-kit sync && svelte-check --tsconfig ./tsconfig.json"`
- `"lint": "eslint ."`
- `"test": "vitest run"`
- Package manager: `bun` (Makefile `frontend-install` uses `bun install --frozen-lockfile`).

**Makefile race target (exists, unused in CI):**
- `Makefile:124-127`: `test-offline-race` runs `go test -race -timeout=10m ./...` with `GO_TEST_ENV` + `PVMSS_OFFLINE=true`.

## Commands you will need

| Purpose   | Command                                  | Expected on success |
|-----------|------------------------------------------|---------------------|
| Frontend check | `cd frontend && bun run check`      | exit 0, no type errors |
| Frontend lint  | `cd frontend && bun run lint`       | exit 0              |
| Frontend test  | `cd frontend && bun run test`       | exit 0, all pass    |
| Backend race   | `make test-offline-race`            | exit 0, no races    |

## Scope

**In scope**:
- `.github/workflows/go.yml` (add a `frontend` job; add a `race` job or extend `build` to run race detector)
- `.github/workflows/lint.yml` (optionally add frontend lint here instead — pick one workflow to own frontend lint to avoid duplication)

**Out of scope**:
- `Makefile` (the targets already exist; CI just needs to call them)
- Frontend source code (if CI reveals failures, fix them in a separate PR — do NOT fix-and-disable)
- `frontend/package.json` scripts (already correct)

## Git workflow

- Branch: `advisor/003-ci-frontend-race`
- Commit: `ci: add frontend check/lint/test and race-detector jobs`
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Add a frontend job to go.yml

In `.github/workflows/go.yml`, add a new job `frontend` (sibling of `build`):
```yaml
  frontend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - name: Set up Bun
        uses: oven-sh/setup-bun@v2
        with:
          bun-version: latest
      - name: Install dependencies
        working-directory: frontend
        run: bun install --frozen-lockfile
      - name: Type check (svelte-check)
        working-directory: frontend
        run: bun run check
      - name: Lint (eslint)
        working-directory: frontend
        run: bun run lint
      - name: Test (vitest)
        working-directory: frontend
        run: bun run test
```
Pin `oven-sh/setup-bun@v2` to a recent stable action version (check the latest tag at write time; prefer one published >7 days ago). The `build` and `frontend` jobs run in parallel by default.

**Verify**: `yq` or visual inspection confirms the job is valid YAML and triggers on push/PR to main (it inherits the workflow-level `on:`).

### Step 2: Add a race-detector job to go.yml

Add a `race` job (or extend `build` — a separate job is cleaner so a race finding doesn't block the plain build):
```yaml
  race:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v6
        with:
          go-version-file: backend/go.mod
          cache: true
      - name: Race detector
        run: make test-offline-race
        env:
          ADMIN_PASSWORD_HASH: "$2y$10$Ppg7Wl3sNYrmxZmWgcq4reOyznt7AeqMrQucaH4HY.dBrzavhPP1e"
          GO_TEST_ENVIRONMENT: 1
          LOG_LEVEL: INFO
          PROXMOX_API_TOKEN_NAME: "tokenName@changeMe!value"
          PROXMOX_API_TOKEN_VALUE: "aaaaaaaaaa-0000-44aa-1111-aaaaaaaaaaa"
          PROXMOX_URL: "https://localhost:8006/api2/json"
          PROXMOX_VERIFY_SSL: false
          PVMSS_OFFLINE: true
          SESSION_SECRET: "changeMeWithSomethingElseUnique"
          JWT_SECRET: "changeMeWithSomethingElseUniqueAtLeast32Bytes!!"
```
Note: add `JWT_SECRET` (≥32 bytes) to the race job env — `make test-offline-race` invokes `env.LoadAndValidate` which requires it. Verify whether the existing `build` job already includes `JWT_SECRET`; if not, add it there too (the build job's `make test-offline` will need it for the same reason — if tests currently pass without it, the loader may be bypassed in some test paths, but add it for safety/consistency).

**Verify**: the env block includes `JWT_SECRET` with ≥32 bytes; `make test-offline-race` is the command.

### Step 3: Decide lint ownership (avoid duplicate frontend lint)

If you add `bun run lint` to the `frontend` job in go.yml (Step 1), do NOT also add eslint to `lint.yml`'s super-linter (super-linter doesn't run eslint for Svelte by default here). Keep frontend lint in the `frontend` job. Leave `lint.yml` as-is. Document this choice in a one-line comment in go.yml if helpful.

**Verify**: `grep -rn "eslint\|VALIDATE_JAVASCRIPT\|VALIDATE_TYPESCRIPT" .github/workflows/` → frontend lint lives only in the `frontend` job, not duplicated.

## Test plan

- Trigger the workflow on a branch (push) and confirm: `build` passes, `frontend` passes, `race` passes.
- If `frontend` or `race` fails on pre-existing issues, those are bugs — file them separately and (with maintainer approval) make the new jobs `continue-on-error: false` but track the failures as follow-ups. Do NOT silently disable the jobs.

## Done criteria

- [ ] `.github/workflows/go.yml` has a `frontend` job running `bun install --frozen-lockfile`, `bun run check`, `bun run lint`, `bun run test`
- [ ] `.github/workflows/go.yml` has a `race` job running `make test-offline-race` with all required env vars including `JWT_SECRET` (≥32 bytes)
- [ ] Both new jobs trigger on push/PR to `main`
- [ ] `yamllint` (or super-linter's YAML validation in lint.yml) passes on the modified workflow
- [ ] No files outside `.github/workflows/go.yml` are modified
- [ ] `docs/plans/advisor/README.md` status row updated

## STOP conditions

Stop and report if:
- `make test-offline-race` fails locally with a real data race (do not suppress it — report the race so it can be fixed before CI enforces it).
- `bun run check`/`lint`/`test` fails locally on `main` (pre-existing frontend breakage — report so the plan can decide whether to make the job non-blocking initially).
- `oven-sh/setup-bun@v2` is unavailable or a newer major is required (pin a stable version published >7 days ago).
- The existing `build` job's env block is missing `JWT_SECRET` and `make test-offline` fails because of it — report whether the build job is currently passing without it (implies tests bypass the loader).

## Maintenance notes

- Once this lands, every later test plan (004, 005) is automatically verified in CI.
- If frontend tests grow slow, consider splitting `bun run test` into a separate job with caching.
- The race job runs the full offline suite with `-race` (~2-3× slower); if it becomes a bottleneck, scope it to packages with concurrent code (`./proxmox/... ./state/...`).
