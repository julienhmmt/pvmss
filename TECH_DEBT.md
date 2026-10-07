# Tech Debt

Things that are lingering, half-finished, or drifted - each with where to
look and what deciding it would take. Not a backlog of new features.

## Accepted limitations (decided 2026-10-04)

These were debt entries until the project lead ruled on them. They are
documented here so the next reader does not reopen them.

- **`Identity.IsAdmin` is a login-time snapshot.** Read from Proxmox once at
  login (`Permissions.Modify` on `/`, with the user's own ticket), stored in
  the session. A user whose admin rights are revoked keeps them until the
  session ends - at most `auth.MaxSessionAge` (8h). Accepted: re-checking
  live would need the API token to read `/access/permissions?userid=` (extra
  role) on a timer, and 8h is short enough.
- **No garbage collection of published snippet files.** PVMSS cannot write
  or delete snippet files through the Proxmox API (`upload`/`download-url`
  accept `iso`, `vztmpl`, `import` only, and files placed by hand under
  `snippets/` are not API-deletable volumes). Old `pvmss-*-<hash>.yml`
  versions accumulate a few KB each; operators remove them by hand on the
  nodes. Won't-fix while the API stays that way.
- **Logs stay on stdout as JSON.** `traceId`/`spanId` correlate them with
  traces. No OTLP log export - it would need an `otelslog` bridge and a log-
  routing decision nobody asked for.
- **No sshd probe for the SSH card.** PVMSS does not probe port 22 (a
  guest-reported address makes the probe an SSRF vector, and the server may
  not share the VM's network). The honest alternative is guest-agent
  `guest-exec`, Linux-only and intrusive.
- **e2e wants `CI=1` locally.** Specs share one fake cluster; `fullyParallel`
  makes VM-creating specs change the row counts `vm-list.spec.ts` expects.
  `CI=1 bunx playwright test` runs one worker, as CI does, and is green.
- **`ostype` is `l26` at create.** The create form has no OS selector -
  Windows guests are expected to come from Proxmox directly. The Connect
  tab reads `ostype` from the detail DTO and shows an RDP card for `w*`
  types, so hand-created Windows VMs are covered.

## Still open (low priority)

| Item | Note |
| --- | --- |
| ADR practice | `docs/adr/` holds 0001-0003; new decisions with real alternatives take the next number |
| `vm_cloudinit_snippets` | kept for the cleanup of legacy per-VM `pvmss-<vmid>.yml` files only |
| `vm_baseline_state` `override` rows | historical; the state cannot be written anymore |
| Two-column admin grids | only the dashboard uses `xl:grid-cols-2`; no other page clearly gains |
| "Pool at quota" | means the per-user VM quota (one pool per user), not a CPU/RAM pool quota - none exists |
| Browser OTel (traces, web-vitals) | errors now report via `POST /api/v1/client-errors`; a full browser SDK (bundle size, CORS, consent) is still undecided |
| Per-cluster CA certificate | a cluster signed by a private CA can only be reached with TLS skip-verify (`newProxmoxHTTPClient` takes one bool). Deciding it takes: a `clusters.ca_pem` column (and its import exclusion), a PEM field in Admin > Clusters, `RootCAs` from that PEM in the HTTP client, and a rotation story. Parked 2026-10-06 (functional review Q2) |
| `docs/plans/`, `server/internal/recovery/` stale `backend/` refs | historical context only - fix when the surrounding area is touched, no dedicated pass |

## Resolved 2026-10-04

- **`api_tokens` dropped** (schema V37). Personal API tokens were removed
  2026-10-01; the table and its early migrations finally went. The import
  path intersects upload columns with the live table, so old backups still
  import.
- **OIDC stubs deleted** (`d3138f25`). `POST /api/v1/auth/oidc` (501), the
  admin toggle endpoint, the `oidc_enabled` column (schema V36), the web
  stores and the commented markup are gone. OIDC will not be implemented.
- **Shared storage uses the Proxmox `shared` flag** (`6a9a36fa`).
  `cluster.Storage` now carries it; the dashboard dedupes on `Shared ||`
  plugin list, so a shared `dir` counts once instead of once per node.
- **Dashboard storage/policy alerts deep-link** (`f9b22e13`). They land on
  `/admin/storages?cluster=&search=` and `/admin/policy?cluster=` instead of
  the unfiltered pages.
- **Connect tab: RDP card for `w*` ostypes, install detection by boot
  order** (`db90590d`). The detail DTO exposes `bootOrder`; "installing"
  now means "the boot order still leads with the mounted ISO", so finishing
  an install no longer needs the Eject click.
- **`POST /api/v1/client-errors` + SPA reporting** (`c94a8dca`).
  `window.onerror`, unhandled rejections and SvelteKit `handleError` reach
  the server log; unauthenticated, per-IP rate limited.
- **`BaselineInputs.Override` removed** (`823e1529`); the hand-placed
  baseline override has been gone since SSH publishing ended.
- **`clusters.snippet_dir` dropped** (schema V37).
- **`make web-test` `ECONNREFUSED` fixed** (`262366af`): three tests ran
  real fetches; `src/test/setup.ts` now fails loudly on any unstubbed fetch.
- **Constitution X amended** (`2c82e5fb`, v1.2.0): it now names the real
  design system - in-house `src/lib/shared/ui/` kit and icon components,
  no shadcn/bits-ui, no Iconify.

## Fixed in the docs review (2026-09-28)

Docs had drifted from the code they describe:

- `PVMSS_RATE_LIMIT_MAX` was read by `config/load.go` but absent from the
  AGENTS/README variable tables. Added.
- README described v0.3 default profiles and per-profile icon, color and
  node/storage overrides; the v0.4 profile has label, sockets, cores, memory,
  disk and bus only, and no profile is seeded outside the `fake` source.
  README, `docs/FEATURES.md` and the in-app admin guide (EN + FR) corrected.
- `docs/FEATURES.md` lacked `/admin/baseline`. Added.
- `pvmss-deployment.yaml`: the Secret had no namespace and the Service
  selector required labels the pods do not carry (no endpoints). Fixed.
- README compose and `docker run` examples mounted the single file
  `pvmss.db`, which breaks with SQLite WAL; they now mount `/data`. The
  compose hash was not `$$`-escaped. Fixed.
- `helm/Chart.yaml` chart `version` was `0.3.0` for `appVersion 0.4.0`; bumped.
- Fixed earlier (2026-09-20): `gopkg.in/yaml.v3` and the SSH variables were
  missing from AGENTS.md. Added.

## Left behind by the SSH-only cloud-init publishing (2026-09-23)

Spec: `.scratch/cloudinit-admin-ssh/spec.md` (not committed).

- **No garbage collection of published files** - see "Accepted limitations"
  above; this is the same constraint, stated where it originated.
- `vm_cloudinit_snippets` only serves the cleanup of legacy per-VM files
  (`pvmss-<vmid>.yml`) of VMs created before the change.
- `vm_baseline_state` rows with state `override` are historical.

## Left behind by the Connect-tab readiness pass (2026-09-29)

Resolved 2026-10-04 except:

- **No sshd check** - see "Accepted limitations" above.
- **`ostype` hardcoded to `l26` at create** - see "Accepted limitations";
  the Connect-tab half of the entry is done (RDP card, boot order).

## Left behind by the observability pass (2026-09-29)

- **No OTLP log export** - see "Accepted limitations" above.
- **Web-client telemetry** - errors now report via `client-errors`
  (`c94a8dca`); a full browser OTel SDK is still undecided (see table).
- **Lint verified.** `sloglint` was checked on 2026-10-01 with the pinned
  `go tool golangci-lint` (`make server-lint`: 0 issues). A globally installed
  `golangci-lint` built with an older Go refuses the module; use `make server-lint`.
- **Domain log keys outside the vocabulary.** `pool`, `code`, `path`,
  `fingerprint`, `label`, `section`, `step`, `port` are used as extra attrs;
  the fixed vocabulary in `AGENTS.md` permits domain extras, so this is a
  note, not debt.

## How to re-check any of this

```bash
grep -n '^go ' server/go.mod; grep -n 'golang:' Dockerfile
sed -n '/^require (/,/^)/p' server/go.mod | head -8
ls docs/adr
grep -n "ADR 0001" server/internal/httpapi/router.go
```

## Related

`ROADMAP.md` · `AGENTS.md` · `CONTEXT.md`
