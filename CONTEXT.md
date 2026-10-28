# PVMSS

A lightweight web portal that lets users manage Proxmox VMs without direct
Proxmox UI access. Go REST API over the Proxmox API + SQLite, SvelteKit SPA
served by the Go binary.

## Language

**Proxmox administrator**:
A Proxmox VE user whose ACL grants `Permissions.Modify` on `/` - any role
carrying it (`PVEAdmin`, `Administrator`, or a custom one). PVMSS reads that
privilege once, at login, with the user's own ticket and stores it as
`Identity.IsAdmin` for the session. Only the `pve` and `pam` realms can sign
in; OIDC realms are unsupported.
_Avoid_: superuser, root admin, PVE admin (ambiguous with the PVEAdmin role)

**Local administrator**:
A PVMSS administrator whose credentials live in `/data/pvmss.db`
(`local_accounts`), independent of Proxmox. Holds the full admin surface, but
may not manage other local administrators. Created, reset, disabled and
deleted by the owner under Admin > Settings > Accounts.
_Avoid_: local user, DB user, fallback admin

**Bootstrap administrator (owner)**:
The env-seeded administrator (`ADMIN_PASSWORD_HASH`, username `admin`,
`provider = bootstrap`). Its password is rotated by changing the variable and
restarting, never from the UI, and it is the only identity allowed to manage
local administrators (`owner_only`). It is not counted against
`PVMSS_MAX_LOCAL_ADMINS` and cannot be locked.
_Avoid_: root, super admin, emergency admin

**Realm**:
The Proxmox authentication backend after the `@` in a username (`user@pve`,
`root@pam`). PVMSS accepts an explicit picker value or a typed suffix for
`pve` and `pam` only; any other realm answers `realm_unsupported` so an
unsupported backend fails loudly instead of silently falling back.
_Avoid_: domain, auth type

**Provider**:
Where a session's credentials came from: `pve` (a Proxmox cluster account),
`local` (a DB local administrator) or `bootstrap` (the env-seeded owner).
Stored on the session row and exposed to the SPA as `Principal.provider`;
`IsOwner` is derived from it, never persisted.
_Avoid_: login type, source

**Projection**:
The cached inventory snapshot stored in SQLite, refreshed by the background
inventory worker every 30s and by the per-cluster refresher after write
actions. Source for list views and bulk reads - never the source of truth for
an in-flight lifecycle transition.
_Avoid_: cache, inventory state, DB state

**Live status**:
An on-demand read of a single VM's current power state (`VMLiveStatus{Status,
Lock, Uptime}`), fetched from Proxmox `/nodes/{n}/qemu/{id}/status/current`.
The source of truth for lifecycle transitions: the front converge loop,
shutdown escalation, idempotence pre-checks, and lock-retry all consume it.
_Avoid_: real status, current status, fresh status

**Convergence**:
The process by which an optimistic status (posed by the front immediately on
click) reaches the live status via polling, bounded by a timeout. The
optimistic value survives until the live read confirms or contradicts it.
_Avoid_: sync, refresh, reconciliation

**UPID**:
The Proxmox task identifier returned by async POST endpoints (create, clone,
delete). PVMSS discards it on interactive power actions (start/stop/shutdown/
reboot/reset) - matching both reference implementations - and waits on it only
where post-task configuration is required (create → cloud-init attach, delete).
_Avoid_: task id, job id

**Cluster id**:
The immutable logical name of a Proxmox connection (`clusters.name`), chosen at creation; `default` for the env-seeded cluster. It is the key in URLs, the API, audit history and per-cluster settings, and it cannot be renamed. Workaround: create a cluster with the wanted id.
_Avoid_: cluster name, slug

**Cluster display label**:
The editable name shown in the interface (`clusters.display_name`). Empty means the id is shown. A connection test fills it from Proxmox only while it is empty, so an admin label is never overwritten.
_Avoid_: display name, alias

**Cluster TLS trust**:
How PVMSS validates the certificate of a cluster's Proxmox API: a CA
certificate chain (PEM), or pinned SHA-256 node fingerprints that replace the
chain and host name checks, or skip-verify. An optional client certificate and
key (mTLS) is stored with it; the key is encrypted and never returned.
_Avoid_: TLS settings (too vague), SSL

**VMID range**:
The per-cluster window (`vmidMin` / `vmidMax` in the policy gabarit) that new VM
ids are taken from: the lowest id not in use on the cluster. Both 0 means
Proxmox's next free id. A full range answers 409 `vmid_range_exhausted`.
_Avoid_: VMID prefix, id pool

**Shared storage**:
A datastore every node of a cluster sees (Proxmox `shared` flag, such as Ceph,
NFS or CephFS). Approved once per cluster rather than per node; enabled when any
of its node rows is.
_Avoid_: global storage, cluster storage

**Tâche**:
An asynchronous Proxmox operation tracked by UPID and polled by the task tray
(create, snapshot, rollback, snapshot delete). Terminal states are
running / ok / error. A task that ends in `WARNINGS: N` is a **success** - the
warning is an attribute (`Warnings`), never a state: `Warnings` non-empty
implies state `ok`. When the tray stops following a task (deadline, repeated
errors) it says "we stopped following", never "it failed".
_Avoid_: job, workflow, "task with warnings" as a state

**Verrou Proxmox (lock)**:
A per-VM mutex PVE holds while an operation runs (snapshot, rollback, backup,
migrate, clone, create…). While set, PVE refuses other operations on the VM.
Snapshot operations retry on it (bounded, then fail naming the lock);
`lock = snapshot-delete` left behind by a failed delete is the NFS/ESTALE
signature. PVMSS reads it (`LiveStatus.Lock`, error parsing via
`extractLockName`) but cannot clear it under an API token - the operator runs
`qm unlock <vmid>` on the node.
_Avoid_: treating the lock as a VM power state

**Migration**:
Moving a PVMSS-managed VM to another node of the same cluster, started by an
administrator (`POST /api/v1/admin/vms/{cluster}/{vmid}/migrate`) and run as a
Proxmox task tracked by its UPID. A running VM migrates live, a stopped one
offline; local disks travel with Proxmox's `with-local-disks`. Cross-cluster
migration and HA-managed VMs are out of scope. Recorded as the `vm.migrate`
audit event once Proxmox accepts the dispatch.
_Avoid_: move, relocate, vMotion

**Migration preflight**:
The read-only check behind the Migrate dialog
(`GET /api/v1/admin/vms/{cluster}/{vmid}/migrate`): live status and lock,
Proxmox's migration precheck, and the approved nodes, split into candidates
(online and allowed, each with non-blocking capacity warnings) and excluded
nodes (offline, or refused by Proxmox with its reason). It also reports
blockers (local resources on a running VM). Start-migration runs the same
check again, so the UI holds no rules of its own.
_Avoid_: dry run, validation

**Rollback**:
Restoring a VM to a snapshot. Proxmox stops the VM, reverts the disks, and
starts it again - with RAM state the VM resumes live at the snapshot point,
without it the VM boots fresh from the restored disks. The confirmation UI
must say « arrête puis redémarre la VM ».
_Avoid_: silent restore, "revert" (no PVMSS usage)

**Snapshot avec état RAM (vmstate)**:
A snapshot that also captures the VM's RAM, enabling resume-at-point
rollback. Requires the VM to be running and every disk on storage able to hold
the RAM state. The UI offers it as a checkbox and greys it out with a reason,
never silently.
_Avoid_: RAM snapshot (ambiguous with a memory dump)

**Request ID (requestId)**:
The identifier of one HTTP request, generated by the outermost middleware (or
taken from a valid inbound `X-Request-Id`), echoed in the `X-Request-Id`
response header and carried on every log line written through the request
logger (`logctx.From(ctx)`). It is how a support ticket ("it failed at 14:03,
here is the id") maps to the server's logs.
_Avoid_: correlation id, trace id (the `traceId` is the OpenTelemetry trace, a
separate identifier that is only present when tracing is on)

**Audit mirror**:
The log line `insertAuditRow` emits for every `audit_log` row (`event=audit`).
The table stays the business source of truth for `/activity` and retention; the
mirror puts the same actions in the log stream for operators. There is one
emission point and no per-handler log call.
_Avoid_: audit log (that is the SQLite table)
