# Tech Debt

Things that are lingering, half-finished, or drifted - each with where to
look and what deciding it would take. Not a backlog of new features.

## Reopened and funded (decided 2026-11-02)

Former limitations promoted to tracked work - specs and tickets in `.scratch/`:

- **OIDC realm sign-in** - full spec + tickets already written:
  `.scratch/oidc-login/` (GitHub #159). Browser-redirect flow through the
  Proxmox OIDC realm; Proxmox stays the identity source.
- **`ostype` selector at create** - `.scratch/ostype-selector/`. Windows
  demand exists; the create form gets an OS field instead of hardcoding `l26`.
- **OTLP log export** - `.scratch/otlp-logs/`. A collector now exists;
  research ticket first (filelog scrape and the syslog sink are zero-code
  routes) - `otelslog` is the last resort.
- **"Pool at quota" wording** - `.scratch/reword-pool-quota/`.

## Accepted limitations (decided 2026-10-04, amended 2026-11-02)

These were debt entries until the project lead ruled on them. They are
documented here so the next reader does not reopen them.

- **`Identity.IsAdmin` is a login-time snapshot.** Read from Proxmox once at
  login (`Permissions.Modify` on `/`, with the user's own ticket), stored in
  the session. A user whose admin rights are revoked keeps them until the
  session ends - at most `auth.MaxSessionAge` (24h; the idle TTL is 8h). Accepted: re-checking
  live would need the API token to read `/access/permissions?userid=` (extra
  role) on a timer, and 8h is short enough.
- **The SSH card shows agent-reported info only - no probe.** (Amended
  2026-11-02.) Neither a TCP probe (a guest-reported address makes it an
  SSRF vector) nor guest-agent `guest-exec` (Linux-only, intrusive, exec
  perms). The card already shows the agent-reported address and user
  honestly without claiming liveness - that is the final design.
- **e2e wants `CI=1` locally.** Specs share one fake cluster; `fullyParallel`
  makes VM-creating specs change the row counts `vm-list.spec.ts` expects.
  `CI=1 bunx playwright test` runs one worker, as CI does, and is green.
  `bun run test:e2e` rebuilds `web/build` first: Playwright serves that
  directory, and a stale build fails against a newer server.
- **Template clones keep secondary volumes where the template has them.**
  A clone of a template whose extra disk (e.g. `efidisk0`) sits on a storage
  not approved in PVMSS keeps it there: the clone writes the volume on the
  template's storage, PVMSS only places the boot disk on approved storage.
  Accepted (decided 2026-10-12): operators keep templates on approved
  storage. Revisit if a real template breaks it (`vm/create_template.go`).

## Cloud-init removed (2026-10-31)

The paused cloud-init feature is deleted end to end (web + server + schema
V5): admin cloud-init templates, the generated baseline, per-VM cloud-init
writes, cloud-image creation and the cluster snippet-storage setting are
gone; recoverable from git history. Kept on purpose: `GET …/cloudinit` (the
VM page reads the SSH user, from live Proxmox config). The
`vm_cloudinit_snippets`/`vm_cloudinit_documents` tables were kept so the
VM-delete cleanup could warn about files a pre-removal deployment left on
the nodes; that warning and the whole table surface were removed
2026-11-04 (`.scratch/cloudinit-table-drop/`). The DDL left the squashed
baseline, so fresh databases never create the tables; databases that already
have them keep them inert, per the no-drop precedent (`profile_ssh_keys`).
Databases
seeded before keep the `cloud-init-howto`/`cloud-init-setup` doc pages until
an admin disables or deletes them.

## Still open (low priority)

| Item | Note |
| --- | --- |
| ~~ADR practice~~ | **Resolved** 2026-11-04: `docs/adr/` recreated by `0001-cluster-id-rename.md`; ADRs 0001-0003 from before the 2026-10-15 removal live in git history |
| ~~Legacy single projection in `policy`~~ | **Resolved** (`refactor/audit-2026-11-cleanup`): `policy.New` no longer takes a projection; every caller shares the one service wired with `SetProjectionSource`, and the variadic `services` parameters are required `*policy.Policy` fields |
| ~~Smalls in `.scratch/housekeeping/`~~ | **Resolved** 2026-11-04: two-column admin grids ruled per-page (`xl:grid-cols-2` on dashboard + appinfo, no blanket rule); `recovery/` stale `backend/` refs now flagged as removed paths in comments |

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
  stores and the commented markup are gone. OIDC sign-in was reopened on 2026-11-02 (see "Reopened and funded").
- **Shared storage uses the Proxmox `shared` flag** (`6a9a36fa`).
  `cluster.Storage` now carries it; the dashboard dedupes on `Shared ||`
  plugin list, so a shared `dir` counts once instead of once per node.
- **Dashboard storage/policy alerts deep-link** (`f9b22e13`). They land on
  `/admin/storages?cluster=&search=` and `/admin/policy?cluster=` instead of
  the unfiltered pages.
- **General information tab: RDP card for `w*` ostypes, install detection by boot
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
  no shadcn/bits-ui, no Iconify. bits-ui was tried and removed (2026-11-04).

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

Superseded by the full cloud-init removal (2026-10-31).

- **No garbage collection of published files** - superseded by the cloudinit
  table drop (`.scratch/cloudinit-table-drop/`, 2026-11-02); legacy
  `pvmss-<vmid>.yml` files are removed by hand.
- `vm_cloudinit_snippets` - removed 2026-11-04, see "Cloud-init removed"
  above.

## Left behind by the Connect-tab readiness pass (2026-09-29)

Resolved 2026-10-04 except:

- **No sshd check** - final ruling 2026-11-02: info only, no probe; see
  "Accepted limitations".
- **`ostype` hardcoded to `l26` at create** - reopened 2026-11-02; selector
  funded (`.scratch/ostype-selector/`). The Connect-tab half of the entry
  is done (RDP card, boot order).

## Left behind by the observability pass (2026-09-29)

- **No OTLP log export** - reopened 2026-11-02 (a collector now exists);
  investigation at `.scratch/otlp-logs/`.
- **Web-client telemetry** - errors now report via `client-errors`
  (`c94a8dca`); browser OTel shipped 2026-11-04 (web vitals + traceparent
  propagation, `POST /api/v1/traces` proxy - ADR 0002).
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
```

## Related

`ROADMAP.md` · `AGENTS.md` · `CONTEXT.md`
