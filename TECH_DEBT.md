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

### ADR practice: started

`docs/adr/` now holds the three decisions the code already referenced or
explained only in comments: 0001 (live status reads and the 120/min batch
limit), 0002 (Proxmox rejections with a machine code), 0003 (404 not 403 for a
VM outside scope). New decisions with real alternatives get the next number.

## Stale references (known, deliberate, low priority)

| Location | Note |
| --- | --- |
| `docs/plans/` | historical task plans referencing `backend/` and `frontend/` - read-only history, not current design |
| `server/internal/recovery/` | comments reference `backend/` for context only (it maps the deleted v0.3 tree into the v0.4 schema, so the reference is accurate for what the package does) |

Repo convention: fix these when the surrounding area is touched anyway, do
not do a dedicated pass, and do not read them as documentation of current
reality.

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

## Left behind by the calm-workspace UI pass (2026-09-27)

Plan: `PLAN-ui-v0.4.md` (not committed), branches `fix/web-offline-links`,
`feat/web-wide-screens`, `feat/admin-dashboard-attention`,
`feat/vms-list-counts`.

- **Two-column admin grids only on the dashboard.** The other admin pages
  are single tables or forms; none clearly gains from `xl:grid-cols-2` yet.
- **Only node alerts deep-link.** Dashboard node alerts open the node detail
  page; storage and policy alerts still open `/admin/storages` and
  `/admin/policy` unfiltered (neither reads a cluster or node from the query
  string).
- **Shared-storage detection is a type list.** `sharedStoragePlugins` in
  `httpapi/admin_dashboard.go` (rbd, cephfs, nfs, cifs, glusterfs, iscsi,
  iscsidirect, pbs). Proxmox exposes a `shared` flag per storage; the
  cluster client does not read it. A `dir` storage marked shared is listed
  once per node.
- **"Pool at quota" means the per-user VM quota.** One pool per user, so a
  pool at `MaxVMPerUser` is a user who cannot create. There is no pool-level
  CPU/RAM quota to alert on.
- **e2e shares one server and one fake cluster across specs.** The Playwright
  config now starts from an empty database, and `profile-ssh-keys.spec.ts`
  approves its own bridge, so a full run is green (111 passed, 1 skipped) and
  repeatable. Locally `fullyParallel` still runs specs against the same fake
  state, so VM-creating specs change the row counts `vm-list.spec.ts` expects;
  run with `CI=1 bunx playwright test` (one worker, as CI does) for a clean
  result.
- **`make web-test` prints `ECONNREFUSED 127.0.0.1:3000`** on a clean
  `v0.4`: some test reaches a real fetch. Tests pass; the noise hides real
  errors.

## Found in the docs review (2026-09-28), not fixed

- **`docs/constitution.md` principle X names shadcn / bits-ui**; `web/` ships
  in-house components under `src/lib/shared/ui/` and neither dependency.
  Amending a constitution needs the project lead's decision (version bump).

## Left behind by the Connect-tab readiness pass (2026-09-29)

- **Windows and non-Linux guests.** `ostype` is hardcoded to `l26` at
  create (`cluster/proxmox_create.go`) and is not exposed in the detail DTO,
  so the Connect tab is SSH-only. A Windows OS installed by hand on such a
  VM still gets an SSH card once the agent reports an address. Deciding it
  takes: exposing `ostype`, then choosing console-only or an RDP card.
- **"Installing" is "an ISO is mounted".** A user who finishes the install
  but leaves the ISO mounted sees "Installation in progress" until they
  press "Eject the ISO". Deliberate (one explicit gesture, no stored flag);
  a boot-order or first-boot signal would remove the click.
- **No sshd check.** PVMSS does not probe port 22 (guest-reported address =
  SSRF vector; the server may not share the VM's network). The only honest
  alternative is a guest-agent `guest-exec` check, Linux-only and intrusive.

## Left behind by the observability pass (2026-09-29)

- **No OTLP log export.** Logs stay on stdout as JSON and are correlated with
  traces by `traceId`/`spanId`. Shipping them over OTLP would need a log
  bridge (`otelslog`) and a decision on whether the collector or the platform
  owns log routing. Deciding it takes: an operator who wants logs and traces
  in one backend without a log shipper.
- **No web-client telemetry.** The SPA sends no traces, errors or web-vitals;
  the server only sees the requests it receives. Deciding it takes: a browser
  OTel SDK (bundle size, CORS to the collector, consent) or a small error-report
  endpoint.
- **Lint verified.** `sloglint` was checked on 2026-10-01 with the pinned
  `go tool golangci-lint` (`make server-lint`: 0 issues). A globally installed
  `golangci-lint` built with an older Go refuses the module; use `make server-lint`.
- **Domain log keys outside the vocabulary.** `pool`, `code`, `path`,
  `fingerprint`, `label`, `section`, `step`, `port` are used as extra attrs;
  the fixed vocabulary in `AGENTS.md` has no equivalent for them.

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
