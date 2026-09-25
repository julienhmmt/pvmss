# Tech Debt

Things that are lingering, half-finished, or drifted - each with where to
look and what deciding it would take. Not a backlog of new features.

## Needs a decision

### Personal API tokens: half-present

| Artifact | State |
| --- | --- |
| `auth/tokens.go`, `TokenService` | present |
| `api_tokens` table, migrations | present |
| `/profile/tokens` page | present |
| Sidebar entry | removed |
| `POST/GET/DELETE /api/v1/auth/tokens` | unregistered - the catch-all 404 answers |
| Bearer resolution in `Auth.Principal` | disabled |
| `web/e2e/tokens.spec.ts` | present |

The feature was deliberately switched off, but the code, the table, and a
test file all remain. Re-enabling means restoring three lines in
`registerAuthRoutes` and the bearer branch in `Auth.Principal`. Worth
checking what `tokens.spec.ts` actually asserts now before trusting it as
coverage for anything. Either finish re-enabling it or remove the dead
surface - the current half-state is the worst of both.

### OIDC: a stub with a working front door

The per-cluster toggle exists, the login button appears when a cluster has
it enabled, but `POST /api/v1/auth/oidc` returns `501`. A user who enables
the toggle gets a button that does nothing useful. Either implement it or
pull the toggle until it is real.

### ADR practice: described, not followed

`AGENTS.md` and `CONTEXT.md` describe a single-context domain model of
`CONTEXT.md` + `docs/adr/`, "created lazily". `docs/adr/` currently has no
files. There is a reference to "ADR 0001" in `httpapi/router.go` (the batch
live-status read design) that points at an ADR which does not exist in the
repo. Either start writing them or stop describing a practice that is not
happening - and if it stays informal, at least promote the two decisions
below out of comments.

## Design decisions that live only in code comments

Worth promoting to `CONTEXT.md` or an ADR, because they are good reasoning
that is currently invisible unless you happen to read the right file:

1. **Why the batch status endpoint has a 120/min limit**, not the 30/min the
   write path uses. The reason (the convergence loop polls every 1.5s for up
   to 30s, 20 polls per action, plus headroom for concurrent actions) is a
   comment in `router.go`.
2. **Why `Resolve` returns 404, not 403, for a VM without the `pvmss` tag.**
   The security rationale (a VM outside PVMSS scope must be indistinguishable
   from a nonexistent one) is a doc comment on `ErrNotFound` in
   `server/internal/vm/resolve.go`.

## Stale references (known, deliberate, low priority)

| Location | Note |
| --- | --- |
| `docs/plans/` | historical task plans referencing `backend/` and `frontend/` - read-only history, not current design |
| `server/internal/recovery/` | comments reference `backend/` for context only (it maps the deleted v0.3 tree into the v0.4 schema, so the reference is accurate for what the package does) |

Repo convention: fix these when the surrounding area is touched anyway, do
not do a dedicated pass, and do not read them as documentation of current
reality.

## Fixed in this pass (2026-09-20)

`AGENTS.md` had drifted from the code it describes:

- Listed three direct server dependencies; a fourth, `gopkg.in/yaml.v3`
  (used by `cloudinit/validate.go` to reject malformed cloud-init YAML), was
  missing. Added.
- `PVMSS_SSH_USER` / `PVMSS_SSH_KEY_FILE` / `PVMSS_SSH_PORT` (the SSH
  snippet-delivery path, the newest feature at HEAD) were read by
  `config/load.go` but absent from the configuration table. Added.

## Left behind by the SSH-only cloud-init publishing (2026-09-23)

Spec: `.scratch/cloudinit-admin-ssh/spec.md` (not committed).

- **No garbage collection of published files.** Every template edit and
  every baseline change publishes a new immutable `pvmss-*-<hash>.yml` on
  each node; old versions stay (a few KB each). A GC needs the list of files
  still referenced by live VM configs (`cicustom`), not only
  `vm_cloudinit_documents`, before removing anything.
- **Dead schema kept on purpose:** `clusters.snippet_dir` (the helper owns
  the directory now), `policy.allow_custom_yaml` / `Gabarit.AllowCustomYAML`
  (still accepted by the admin policy API, no longer enforced or shown),
  `cloudinit.BaselineInputs.Override` (the hand-placed `pvmss-baseline.yml`
  override is gone). Dropping them touches recovery fixtures and the import
  allowlist.
- `vm_cloudinit_snippets` only serves the cleanup of legacy per-VM files
  (`pvmss-<vmid>.yml`) of VMs created before the change.
- `vm_baseline_state` rows with state `override` are historical.

## How to re-check any of this

```bash
grep -n '^go ' server/go.mod; grep -n 'golang:' Dockerfile
sed -n '/^require (/,/^)/p' server/go.mod | head -8
ls docs/adr
grep -n "ADR 0001" server/internal/httpapi/router.go
grep -rn "registerAuthRoutes\|auth/tokens" server/internal/httpapi/router.go
```

## Related

`ROADMAP.md` · `AGENTS.md` · `CONTEXT.md`
