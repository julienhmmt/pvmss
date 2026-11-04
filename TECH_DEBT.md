# Tech Debt

Things that are lingering, half-finished, or drifted - each with where to
look and what deciding it would take. Not a backlog of new features.

## Reopened and funded (decided 2026-11-02)

Former limitations promoted to tracked work. All but one delivered
2026-11-04:

- **OIDC realm sign-in** - still open; full spec + tickets already written:
  `.scratch/oidc-login/` (GitHub #159). Browser-redirect flow through the
  Proxmox OIDC realm; Proxmox stays the identity source.

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
- **Pool password reset recreates the Proxmox user** (2026-11-02).
  `PUT /access/password` refuses API tokens (403), so `SetUserPassword`
  deletes and recreates the pool user with the new password and re-grants
  the ACL (`cluster/proxmox_pools.go`, `pools/reset.go`). Accepted: the
  managed pool user carries no settings worth preserving.
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
| VM name reservations live in process memory | `policy/reserve.go` tracks in-flight creates in a 15-min map; correct for the single-replica deployment (move to SQLite if replicas > 1). A VM deleted or renamed outside the app holds its name until the TTL (see the `shortcut:` comments) |
| ~~ADR practice~~ | **Resolved** 2026-11-04: `docs/adr/` recreated; `0002-browser-telemetry.md` is the live ADR (0001 was removed once implemented); ADRs 0001-0003 from before the 2026-10-15 removal live in git history |
| ~~Legacy single projection in `policy`~~ | **Resolved** (`refactor/audit-2026-11-cleanup`): `policy.New` no longer takes a projection; every caller shares the one service wired with `SetProjectionSource`, and the variadic `services` parameters are required `*policy.Policy` fields |
| ~~Smalls in `.scratch/housekeeping/`~~ | **Resolved** 2026-11-04: two-column admin grids ruled per-page (`xl:grid-cols-2` on dashboard + appinfo, no blanket rule); `recovery/` stale `backend/` refs now flagged as removed paths in comments |

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

## Left behind by the Connect-tab readiness pass (2026-09-29)

Resolved 2026-10-04 except:

- **No sshd check** - final ruling 2026-11-02: info only, no probe; see
  "Accepted limitations".

## Left behind by the observability pass (2026-09-29)

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
