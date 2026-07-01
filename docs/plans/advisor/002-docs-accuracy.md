# Plan 002: Fix stale/inaccurate docs (README env table, CLAUDE.md "no database", Bulma reference, dead CSRF_TOKEN_TTL)

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report. When done, update the status row in `docs/plans/advisor/README.md`.
>
> **Drift check (run first)**: `git diff --stat d427838..HEAD -- README.md README.fr.md CLAUDE.md example.env helm/values.yaml pvmss-deployment.yaml`
> If any in-scope file changed since this plan was written, compare excerpts
> against live code before proceeding; on mismatch, treat as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW (docs/config-only; no source behavior change)
- **Depends on**: none (but coordinate with plan 001 which also edits `example.env`/`helm/values.yaml`/`pvmss-deployment.yaml` — land 001 first, or merge the PROXMOX_VERIFY_SSL line change with this plan's CSRF removal in one commit per file)
- **Category**: docs
- **Planned at**: commit `d427838`, 2026-07-01

## Why this matters

New users following the README cannot start the app — the env table omits `JWT_SECRET`, which `backend/env/loader.go:54-60` requires (≥32 bytes) or the process exits at startup. Separately, `CLAUDE.md` says "no database" while a full SQLite layer exists, the README calls the frontend "Bulma-based" (it's SvelteKit/Tailwind), the README's `PROXMOX_VERIFY_SSL` default is wrong (`false` vs the code's `true`), and `CSRF_TOKEN_TTL` appears in example/deploy files but is read nowhere in the backend. These are actively-wrong docs (worse than missing).

## Current state

**JWT_SECRET + PORT missing from README env table:**
- `backend/env/loader.go:26` reads `JWT_SECRET`; `:54-60` requires it (≥32 bytes) or fails fast.
- `backend/env/loader.go:40` reads `PORT` with default `50000`.
- `README.md:166-181` env table lists `ADMIN_PASSWORD_HASH`, `SESSION_SECRET`, `PROXMOX_*`, `PVMSS_DB_PATH`, etc. — but NO `JWT_SECRET` and NO `PORT`.

**PROXMOX_VERIFY_SSL default wrong in README:**
- `README.md:173` → `| `PROXMOX_VERIFY_SSL` | ... | ❌ | `false` |`
- `backend/env/loader.go:33` → `parseBoolDefault("PROXMOX_VERIFY_SSL", true)` — actual default is `true`.
- (The deployment artifacts shipping `false` is security plan S1, handled in plan 001. This plan fixes the README's stated default only.)

**CLAUDE.md "no database" contradiction:**
- `CLAUDE.md:42` → `Go stateless REST API — no database, all state from Proxmox APIs.`
- `CLAUDE.md:96` → `**Database** stores approved nodes/ISOs/storages/bridges, VM resource limits, per-user VM quotas, and SFTP cloud-init config.`
- `backend/database/schema.go:5-104` defines 11 tables. The "no database" line is stale.

**README frontend stack stale:**
- `README.md:59` → `**Frontend**: Bulma-based, with custom CSS.`
- `frontend/package.json` has Svelte 5.56, Tailwind 4.3, no Bulma. `CLAUDE.md:62` correctly says "SvelteKit SPA using Svelte 5 runes, TypeScript, Tailwind CSS".

**Dead CSRF_TOKEN_TTL:**
- `example.env:23` → `CSRF_TOKEN_TTL=2h`
- `helm/values.yaml:57` → `CSRF_TOKEN_TTL: "2h"`
- `pvmss-deployment.yaml:132` → `CSRF_TOKEN_TTL` env var
- `grep -rn "CSRF_TOKEN_TTL\|CSRFTokenTTL" backend/` returns NOTHING — the variable is read nowhere.

Conventions: README has an EN (`README.md`) and FR (`README.fr.md`) pair — keep them in sync. The env table uses a 4-column markdown table (Variable | Description | Required | Default) with ✅/❌ for Required.

## Commands you will need

| Purpose   | Command                  | Expected on success |
|-----------|--------------------------|---------------------|
| Lint markdown | (super-linter runs in CI via `.github/workflows/lint.yml` with `VALIDATE_MARKDOWN_PRETTIER: true`) | no markdown prettier violations |
| Verify dead var | `grep -rn "CSRF_TOKEN_TTL" backend/` | no matches (confirms safe to remove) |

## Scope

**In scope**:
- `README.md` (env table + frontend line)
- `README.fr.md` (mirror the same changes)
- `CLAUDE.md` (remove/fix the "no database" line; the backend package table at line ~46 could add a `database/` row but that is optional — at minimum fix line 42)
- `example.env` (remove `CSRF_TOKEN_TTL=2h`)
- `helm/values.yaml` (remove `CSRF_TOKEN_TTL` line)
- `pvmss-deployment.yaml` (remove `CSRF_TOKEN_TTL` env block)

**Out of scope**:
- `backend/env/loader.go` — do NOT add CSRF_TOKEN_TTL support; the decision is to remove the dead var, not implement it.
- The PROXMOX_VERIFY_SSL value in deployment files (S1, plan 001) — unless merging commits, leave the value to plan 001. This plan only fixes the README's *stated default*.
- `backend/docs/*.md` (admin/cloud-init/user/perms docs) — not in scope.

## Git workflow

- Branch: `advisor/002-docs-accuracy`
- One commit per logical fix, or one bundled "docs: fix stale env table, stack, and dead config" commit. Conventional-commit style.
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Add JWT_SECRET and PORT to the README env table

In `README.md`, inside the env table (lines 166-181), add two rows. Place `JWT_SECRET` right after `SESSION_SECRET` (both are auth secrets), and `PORT` near the end (optional):
```
| `JWT_SECRET`              | HS256 signing key for /api/v1/ JWTs (≥ 32 bytes)                             |    ✅    | —                    |
| `PORT`                    | TCP port the HTTP server listens on                                          |    ❌    | `50000`              |
```
Mirror the exact column format (note the padding/alignment of existing rows).

**Verify**: `grep -n "JWT_SECRET\|PORT" README.md` → both appear in the env table.

### Step 2: Fix PROXMOX_VERIFY_SSL default in README

`README.md:173`: change the Default column from `false` to `true`. Keep the description ("`true` for trusted certs, `false` for self-signed labs"). The Required column stays ❌.

**Verify**: `grep -n "PROXMOX_VERIFY_SSL" README.md` → the table row's Default is `true`.

### Step 3: Update the frontend stack line

`README.md:59`: replace `**Frontend**: Bulma-based, with custom CSS.` with `**Frontend**: SvelteKit SPA (Svelte 5 runes, TypeScript, Tailwind CSS).`. Update `README.fr.md` equivalently (the FR line at ~line 59 says `Basé sur Bulma avec CSS personnalisé`).

**Verify**: `grep -in "bulma" README.md README.fr.md` → no matches (Bulma is no longer referenced as the current stack).

### Step 4: Fix CLAUDE.md "no database" line

`CLAUDE.md:42`: remove the clause `— no database, all state from Proxmox APIs` so the line reads `Go stateless REST API.` (or rewrite to `Go REST API backed by Proxmox APIs and a SQLite database for settings/limits/audit.`). Ensure consistency with line 96 which already documents the database. Optionally add a `database/` row to the backend package table (~line 46) with role "SQLite persistence: approved nodes/ISOs/storages/bridges, VM limits, per-user quotas, SFTP config, audit log".

**Verify**: `grep -n "no database" CLAUDE.md` → no matches.

### Step 5: Remove dead CSRF_TOKEN_TTL from example.env, helm, deployment

First confirm it's still dead: `grep -rn "CSRF_TOKEN_TTL\|CSRFTokenTTL" backend/` → must return nothing. Then remove:
- `example.env:23` (the `CSRF_TOKEN_TTL=2h` line)
- `helm/values.yaml:57` (the `CSRF_TOKEN_TTL: "2h"` line)
- `pvmss-deployment.yaml:132` (the `- name: CSRF_TOKEN_TTL` block + its value line)

**Verify**: `grep -rn "CSRF_TOKEN_TTL" example.env helm/values.yaml pvmss-deployment.yaml` → no matches.

### Step 6: Mirror EN/FR consistency

Ensure `README.fr.md` env table and frontend line match the `README.md` changes (JWT_SECRET, PORT, PROXMOX_VERIFY_SSL default, frontend stack).

**Verify**: `diff <(grep -E "JWT_SECRET|PORT|PROXMOX_VERIFY_SSL" README.md) <(grep -E "JWT_SECRET|PORT|PROXMOX_VERIFY_SSL" README.fr.md)` → variable names match (descriptions may differ by language).

## Test plan

- No code tests (docs/config only).
- Run super-linter locally if available: `docker run --rm -e VALIDATE_MARKDOWN_PRETTIER=true ...` or rely on CI. At minimum, preview the markdown table renders correctly (column alignment).

## Done criteria

- [ ] `JWT_SECRET` and `PORT` appear in `README.md` and `README.fr.md` env tables
- [ ] `PROXMOX_VERIFY_SSL` default is `true` in both README env tables
- [ ] `grep -in "bulma" README.md README.fr.md` returns no matches
- [ ] `grep -n "no database" CLAUDE.md` returns no matches
- [ ] `grep -rn "CSRF_TOKEN_TTL" example.env helm/values.yaml pvmss-deployment.yaml backend/` returns no matches
- [ ] No files outside the in-scope list are modified
- [ ] `docs/plans/advisor/README.md` status row updated

## STOP conditions

Stop and report if:
- `grep -rn "CSRF_TOKEN_TTL" backend/` returns matches (the variable IS used — removing it would break something; do not proceed, report so the plan can be revised to implement it instead).
- The README env table structure has changed and no longer matches the 4-column format (drift).
- Plan 001 has already edited `example.env`/`helm/values.yaml`/`pvmss-deployment.yaml` and a merge conflict is likely — coordinate ordering with the maintainer.

## Maintenance notes

- Keep `README.md` and `README.fr.md` env tables in sync on every future env-var addition. Consider a single source-of-truth table later if divergence becomes chronic.
- If `JWT_SECRET` is later renamed or made optional, update both READMEs and `CLAUDE.md` together.
