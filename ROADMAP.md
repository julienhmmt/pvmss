# Roadmap

What has actually happened to PVMSS, from the first commit to the current
`v0.4` rewrite, and what is left before it is done. History below is built
from `git log` and tags, not from memory - re-run the commands in each
section to check it against a later HEAD.

Current state at time of writing: `v0.3.0` was the last tagged release
(shipped on the original stack); `v0.4` is an in-progress rewrite, not yet
tagged, currently at `HEAD 60467b7f`.

## History

### v0.1 - v0.3.0 (2025-10-13 to 2025-12-07): the original stack

First commit `3296f56d` ("pvmss 0.1"), 2025-10-13. Seven tags across the
original `backend/` + `frontend/` codebase:

| Tag | Date | Shape |
| --- | --- | --- |
| `0.1` | 2025-10-13 | initial release |
| `0.1.1` | 2025-10-17 | first post-install fixes |
| `0.1.2` | 2025-10-19 | VM detail page redesign |
| `0.1.3` | 2025-10-23 | node name shown next to VMBR name, plus fixes |
| `0.2` | 2025-11-11 | 139 commits: VM resource editing, TPM/UEFI support, auto-start on create, cluster detection, app-info page, consolidation of the Proxmox API client (`resty_*.go` files merged into their domain files) |
| `0.2.1` | 2025-11-22 | 32 commits, mostly dependency updates |
| `0.3.0` | 2025-12-07 | 78 commits since `0.2.1` |

### Continued v0.3 development (2025-12-07 to 2026-07-21): 361 commits

The original stack kept growing for over seven months after the `0.3.0` tag,
before the rewrite branched off. Breakdown by commit type in that window:
68 `feat`, 67 `chore`, 54 `refactor`, 23 `docs`, 11 `fix`, 5 `test`. Notable
work in this period: server-side pagination/sorting/facets on the search
page, admin bulk VM actions and owner VM clone, a multi-cluster read-only
foundation, documentation UX (TOC, search, reading time), network card
management UI, and the removal of the legacy Telmate Proxmox client in favor
of a resty-based one.

### The v0.4 rewrite (2026-08-01 to 2026-08-12): 108 commits, 12 days

The `v0.4` branch forked from main on 2026-07-21 (commit `5216ebb0`). The
rewrite itself - the Go REST API (`server/`) and the SvelteKit SPA (`web/`)
replacing `backend/` + `frontend/` - was built in 108 commits over twelve
days, 2026-08-01 to 2026-08-12. In parallel, only 2 more commits landed on
the old stack before the cutover.

### T16 cutover (2026-08-12): the legacy tree deleted

Commit `a7a26f7a` ("cutover v0.4 -> main (T16) - delete legacy
backend/frontend"), 2026-08-12, merged the `v0.4` branch into `main` and
removed `backend/` and `frontend/`. From this point on, `server/` + `web/`
is the only codebase.

### Post-cutover hardening (2026-08-12 to present): 402 commits

Since the cutover: the em-dash ban enforced across the whole tree, the
SonarQube quality gate cleared for both `server` and `web` projects, the
`graphify` / code-review-graph tooling removed in favor of `tools/pq` (a
stateless ripgrep-based locator, no index to go stale), personal API tokens
deactivated (code kept, routes unregistered), and a run of cloud-init work
culminating in the current `HEAD`: SSH snippet delivery, so cloud-init works
without a shared filesystem between PVMSS and the Proxmox node.

## Current state

- Version strings: `appVersion = "0.4.0-dev"` (`server/cmd/pvmss/main.go`),
  `"version": "0.4.0"` (`web/package.json`).
- No `v0.4.0` tag yet - see "Now" below.
- Full technical detail: `docs/FEATURES.md`, `WORKFLOWS.md`, `DESIGN.md`,
  `CONTEXT.md`, `AGENTS.md`.

## Now / next (technical only)

Things structurally implied by the current codebase, not a feature backlog:

- **Tag `v0.4.0`.** The rewrite has been the only codebase since the T16
  cutover a month ago and has not been tagged.
- **Decide on personal API tokens.** The code, table, and e2e spec exist;
  the routes are unregistered and bearer auth is disabled. Either re-enable
  (restore three lines in `registerAuthRoutes` plus the bearer branch in
  `Auth.Principal`) or remove the dead surface. See `TECH_DEBT.md`.
- **OIDC is a stub.** The per-cluster toggle and the login button exist;
  `POST /api/v1/auth/oidc` returns 501. Either implement it or remove the
  toggle so it stops looking finished.
- **Decide on the ADR practice.** `docs/adr/` is empty though `AGENTS.md`
  and `CONTEXT.md` describe it as "created lazily". At least one design
  decision (the batch-status rate limit) currently lives only in a code
  comment. See `TECH_DEBT.md`.

## How this file gets updated

Re-derive the history section from git, not from prose memory:

```bash
git tag --sort=creatordate
git log <tagA>..<tagB> --oneline | wc -l
git log --format='%ad' --date=short <tagA>..<tagB> | tail -1   # earliest date in range
```

Update after a tagged release or another cutover-scale event, not on every
commit.

## Related

`TECH_DEBT.md` · `AGENTS.md` · `docs/FEATURES.md`
