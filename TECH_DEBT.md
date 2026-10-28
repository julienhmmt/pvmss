# Tech Debt

Things that are lingering, half-finished, or drifted - each with where to
look and what deciding it would take. Not a backlog of new features.

## Accepted limitations (decided 2026-10-04)

These were debt entries until the project lead ruled on them. They are
documented here so the next reader does not reopen them.

- **`Identity.IsAdmin` is a login-time snapshot.** Read from Proxmox once at
  login (`Permissions.Modify` on `/`, with the user's own ticket), stored in
  the session. A user whose admin rights are revoked keeps them until the
  session ends - at most `auth.MaxSessionAge` (24h; the idle TTL is 8h). Accepted: re-checking
  live would need the API token to read `/access/permissions?userid=` (extra
  role) on a timer, and 8h is short enough.
- **No Proxmox OIDC realm sign-in.** The realm picker and the API accept `pve`
  and `pam` only; anything else answers `realm_unsupported`. Supporting OIDC
  means a different Proxmox flow (the browser is redirected to the identity
  provider, so PVMSS cannot collect a password) plus a callback route.
  Future work; PVMSS local administrators cover the "no password in Proxmox"
  case meanwhile. See `normalizePVEUsername` in
  `server/internal/httpapi/auth.go`.
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
- **Template clones keep secondary volumes where the template has them.**
  A clone of a template whose extra disk (e.g. `efidisk0`) sits on a storage
  not approved in PVMSS keeps it there: the clone writes the volume on the
  template's storage, PVMSS only places the boot disk on approved storage.
  Accepted (decided 2026-10-12): operators keep templates on approved
  storage. Revisit if a real template breaks it (`vm/create_template.go`).
- **`ostype` is `l26` at create.** The create form has no OS selector -
  Windows guests are expected to come from Proxmox directly. The Connect
  tab reads `ostype` from the detail DTO and shows an RDP card for `w*`
  types, so hand-created Windows VMs are covered.

## Cloud-init paused (2026-10-20)

Cloud-init is not reliable or viable yet and too complex to set up. Paused, not removed: `cloudInitPaused` in
`server/internal/httpapi/router.go`, two `+page.ts` redirects under
`web/src/routes/admin/{cloudinit-templates,baseline}`, commented-out UI blocks
marked "Cloud-init is disabled for now", and skipped tests/e2e specs. The VM
cloud-init read (`GET …/cloudinit`, SSH user on the Connect tab) and the
console password stay. Consequence for users: PVMSS gives a VM no SSH key
or password, so template clones rely on the sign-in built into the template
(documented in README and the in-app guides) and the `/profile` SSH-key
section is hidden. The `cloud-init-howto` and `cloud-init-setup` built-in
docs are seeded disabled (`pausedPages` in `docs/seed/seed.go`); databases
seeded before that keep them enabled with the old texts until an admin edits
or disables them. Deciding it: rework the feature, or delete the code,
the tables and `docs/cloud-init.md`.

## Still open (low priority)

| Item | Note |
| --- | --- |
| ADR practice | `docs/adr/` holds 0001-0003; new decisions with real alternatives take the next number |
| `vm_cloudinit_snippets` | kept for the cleanup of legacy per-VM `pvmss-<vmid>.yml` files only |
| `vm_baseline_state` `override` rows | historical; the state cannot be written anymore |
| Two-column admin grids | only the dashboard uses `xl:grid-cols-2`; no other page clearly gains |
| "Pool at quota" | means the per-user VM quota (one pool per user), not a CPU/RAM pool quota - none exists |
| Browser OTel (traces, web-vitals) | errors now report via `POST /api/v1/client-errors`; a full browser SDK (bundle size, CORS, consent) is still undecided |
| Node capacity uses the default cluster projection | `main.go` wires `policy.New(st, projection, nil)` with the default cluster's inventory projection only, and `policy/nodes.go` falls back to it when no live snapshot is wired, so node capacity usage on a non-default cluster reads the default cluster's nodes. Fix: pick the projection per cluster |
| `docs/plans/`, `server/internal/recovery/` stale `backend/` refs | historical context only - fix when the surrounding area is touched, no dedicated pass |

## Resolved 2026-10-22 (full app review)

- **`PVMSS_TRUSTED_PROXY_HOPS` defaults to `0`.** With `1` and no proxy (the
  compose example), a client chose its own IP through `X-Forwarded-For` and
  dodged the login rate limit. Helm keeps `1` explicitly.
- **Authenticated writes outlive the 10s `WriteTimeout`.** `protect` and the
  admin guard wrap writes in `withLongWriteDeadline` (2 min): a power action
  or snapshot retrying on a VM lock (30s), bulk actions and the manual
  refresh no longer answer with a dropped connection.
- **`maxDiskPerVmGb` caps the VM's total disk**, not each disk (`vm/disks.go`);
  the add/resize disk dialogs cap their input at the room left (`disk-limit.ts`).
- **The example `SESSION_SECRET` is refused at startup**, and the Helm chart
  requires `secrets.sessionSecret`. An instance already running with the
  example value must get a new secret, then re-enter its cluster tokens in
  Admin > Clusters (they are encrypted with a key derived from it).
- **Markdown links survive underscores**: opening `<a>` tags sit behind
  placeholders until emphasis is done.
- **Session cookies last `auth.MaxSessionAge` (24h)** in the browser; the
  server still enforces the 8h idle expiry.

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

## Cluster id rename not supported (2026-10-14)

The cluster id (`clusters.name`, e.g. `default`) cannot be renamed; only the display label is editable (edit form and `displayName`). The id is a plain `cluster TEXT` key in about 20 tables with no foreign keys, used as the `:cluster` route param, as the client and inventory registry key, and as the persisted active-cluster selection. Deciding it means one transaction that rewrites every `cluster` column (catalog, policy, pools, VM and audit tables), re-keying the in-memory registries, and a decision on whether audit history keeps the old id. Workaround: create a cluster with the wanted id and remove the old one.

## How to re-check any of this

```bash
grep -n '^go ' server/go.mod; grep -n 'golang:' Dockerfile
sed -n '/^require (/,/^)/p' server/go.mod | head -8
ls docs/adr
grep -n "ADR 0001" server/internal/httpapi/router.go
```

## Related

`ROADMAP.md` · `AGENTS.md` · `CONTEXT.md`
