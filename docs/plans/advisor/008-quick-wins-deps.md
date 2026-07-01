# Plan 008: Quick wins — remove unused adapter-auto dep, extend Dependabot to gomod + npm

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report. When done, update the status row in `docs/plans/advisor/README.md`.
>
> **Drift check (run first)**: `git diff --stat d427838..HEAD -- frontend/package.json frontend/bun.lockb .github/dependabot.yml frontend/svelte.config.js`
> If any in-scope file changed since this plan was written, compare "Current
> state" excerpts against live code before proceeding; on mismatch, treat as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: deps / dx
- **Planned at**: commit `d427838`, 2026-07-01

## Why this matters

Two trivial cleanups. (1) `@sveltejs/adapter-auto` is a devDependency but `svelte.config.js` imports `adapter-static` only — dead weight and a misleading signal about which adapter is in use. (2) Dependabot is configured only for `github-actions`, so Go and npm dependencies get no automated security/upgrade PRs. Both are minutes of work for real hygiene benefit.

## Current state

**Unused adapter (`frontend/package.json:23-24`):**
```json
"@sveltejs/adapter-auto": "^7.0.1",
"@sveltejs/adapter-static": "^3.0.10",
```
`frontend/svelte.config.js:1` → `import adapter from '@sveltejs/adapter-static';` (only static is used). No reference to `adapter-auto` anywhere in `frontend/`.

**Dependabot scope (`.github/dependabot.yml`):**
```yaml
---
version: 2
updates:
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
```
Only github-actions. No `gomod` (backend) or `npm` (frontend) ecosystem. (Note: stale `dependabot/npm_and_yarn/*` branches exist on origin from a prior config — they are orphaned; this plan re-enables npm updates going forward.)

Package manager: `bun` (lockfile `frontend/bun.lockb`). Dependabot's `npm` ecosystem supports bun lockfiles.

## Commands you will need

| Purpose   | Command                                  | Expected on success |
|-----------|------------------------------------------|---------------------|
| Install   | `cd frontend && bun install --frozen-lockfile` | exit 0 (before lockfile regen) |
| Regen lockfile | `cd frontend && bun install`        | lockfile updated, exit 0 |
| Frontend check | `cd frontend && bun run check`      | exit 0              |
| Validate YAML | (super-linter VALIDATE_YAML in CI)    | no YAML errors      |

## Scope

**In scope**:
- `frontend/package.json` (remove `@sveltejs/adapter-auto` line)
- `frontend/bun.lockb` (regenerate after removal)
- `.github/dependabot.yml` (add `gomod` and `npm` ecosystems)

**Out of scope**:
- `frontend/svelte.config.js` (already correct)
- Any version bumps (Dependabot will propose those after this config lands)
- `backend/go.mod` (Dependabot gomod will maintain it; don't bump manually here)

## Git workflow

- Branch: `advisor/008-quick-wins-deps`
- Commits: `chore(deps): remove unused adapter-auto`, `chore(deps): extend dependabot to gomod and npm`
- Do NOT push unless instructed.

## Steps

### Step 1: Remove @sveltejs/adapter-auto

In `frontend/package.json`, delete the `"@sveltejs/adapter-auto": "^7.0.1",` line (line 23). Keep `adapter-static`. Then regenerate the lockfile:
```bash
cd frontend && bun install
```
This updates `bun.lockb` to drop `adapter-auto` and its transitive deps that were only needed by it.

**Verify**: `grep -rn "adapter-auto" frontend/package.json frontend/svelte.config.js frontend/` (excluding `node_modules`/`build`) → no matches. `cd frontend && bun run check` → exit 0 (svelte-check still passes; adapter-static is present).

### Step 2: Extend Dependabot config

Append two ecosystems to `.github/dependabot.yml`:
```yaml
  - package-ecosystem: "gomod"
    directory: "/backend"
    schedule:
      interval: "weekly"
    open-pull-requests-limit: 10
  - package-ecosystem: "npm"
    directory: "/frontend"
    schedule:
      interval: "weekly"
    open-pull-requests-limit: 10
```
Keep the existing `github-actions` entry. The `directory` paths match where `go.mod` (`/backend`) and `package.json` (`/frontend`) live.

**Verify**: `yamllint .github/dependabot.yml` (or rely on super-linter's VALIDATE_YAML in CI). Visually confirm three `updates` entries: github-actions, gomod, npm.

## Test plan

- `cd frontend && bun run check` → exit 0 (adapter removal didn't break the build).
- `cd frontend && bun run build` (optional) → exit 0, `frontend/build/` produced.
- No backend test impact (no backend change).

## Done criteria

- [ ] `grep -rn "adapter-auto" frontend/package.json` returns no matches
- [ ] `frontend/bun.lockb` regenerated (`git diff --stat` shows it changed)
- [ ] `cd frontend && bun run check` exits 0
- [ ] `.github/dependabot.yml` has three ecosystems: github-actions, gomod, npm
- [ ] No files outside `frontend/package.json`, `frontend/bun.lockb`, `.github/dependabot.yml` modified
- [ ] `docs/plans/advisor/README.md` status row updated

## STOP conditions

Stop and report if:
- `bun run check` fails after removing `adapter-auto` (implies something else imports it — `grep -rn "adapter-auto" frontend/src frontend/svelte.config.js` to find the stray import; report rather than fixing unrelated code).
- `bun install` fails to regenerate the lockfile (bun version mismatch — report the bun version).
- The repo actually intends to keep `adapter-auto` for some fallback path (check `svelte.config.js` for conditional adapter selection — if present, the removal is wrong).

## Maintenance notes

- After this lands, Dependabot will open weekly PRs for go and npm deps. Consider grouping major-version bumps separately from patch bumps (Dependabot grouping config) if PR volume becomes noisy.
- The orphaned `dependabot/npm_and_yarn/*` remote branches can be deleted via `gh api` or the GitHub UI once npm updates resume under the new config — out of scope here, note for the maintainer.
