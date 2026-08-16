# Administrator guide

Welcome to the PVMSS administrator guide. PVMSS (Proxmox Virtual Machine
Self-Service) is a self-service portal that lets your users create, operate,
and troubleshoot Proxmox VE virtual machines without exposing the Proxmox UI.

The administrator has full access to every application feature. There is no
separate auditor or observer role: signing in with an administrator account
unlocks both the standard user surface and everything under `/admin`.

## First steps

1. Open the administration panel on `/admin` (an administrator account is required).
2. Check **Application Info** (`/admin/appinfo`) to confirm the instance is connected to the right Proxmox environment.
3. Approve the resources users may use: **Nodes**, **Storages**, **ISOs**, **Bridges**, and **Cloud-init templates**.
4. Optional but recommended: create **VM profiles** so users can pick pre-approved hardware shapes, and define **Tags** to organize VMs.
5. Set the **Policy** (per-user limits) and **Node capacity** caps.
6. Create as many **User pools** as you need from `/admin/pools`.
7. Tell your users the portal is available so they can start creating VMs.
8. Watch the **Audit log** (`/admin/settings`) to trace every VM write back to the acting user.

## Application information (App Info)

`/admin/appinfo` is a read-only overview of the running instance:

- **Build information**: application version, Go version, operating system, and architecture.
- **Environment**: whether PVMSS is running against a real Proxmox cluster (`PVMSS_CLUSTER_SOURCE=proxmox`) or the built-in trial cluster (`PVMSS_CLUSTER_SOURCE=fake`).
- **Proxmox cluster status**: cluster name and node count when connected to a cluster, or standalone mode for a single node.
- **Environment variables (safe subset)**: non-sensitive configuration such as `PROXMOX_URL`, `PVMSS_PORT`, `PVMSS_DB_PATH`.

Use this page to confirm connectivity after a deployment or a configuration change. If the instance cannot reach Proxmox, review the server logs and the environment variables below.

## Configuration (environment variables)

PVMSS is configured entirely through environment variables, validated at startup. The server refuses to boot if a required value is missing or malformed.

Required:

- `PVMSS_PORT` — TCP port the server listens on (the official image uses `50000`).
- `PVMSS_DB_PATH` — path to the SQLite database file.
- `SESSION_SECRET` — 32+ bytes used to encrypt user sessions.
- `LOG_LEVEL` — `debug`, `info`, `warn`, or `error` (lowercase only).
- `LOG_FORMAT` — `json` or `console`.
- `LOG_OUTPUT` — `stdout`, `stderr`, or a file path.
- `PVMSS_CLUSTER_SOURCE` — `proxmox` or `fake`. There is no default on purpose: `fake` ships with demo credentials and must never be selected by accident.

Required when `PVMSS_CLUSTER_SOURCE=proxmox`:

- `PROXMOX_URL` — for example `https://host:8006/api2/json`.
- `PROXMOX_API_TOKEN_NAME` — the Proxmox API token id (`user@pve!token`).
- `PROXMOX_API_TOKEN_VALUE` — the matching token secret.

Optional:

- `PVMSS_HOST` — bind address (the image sets `0.0.0.0`).
- `PVMSS_WEB_DIR` — location of the built SPA (defaults to a path relative to the executable).
- `ADMIN_PASSWORD_HASH` — if set, must be a bcrypt hash (`$2…`); lets you pin the administrator password.
- `PVMSS_COOKIE_SECURE` — defaults to `true`; set `false` only behind plain HTTP for local trials.
- `PVMSS_INVENTORY_REFRESH_INTERVAL` — background inventory refresh interval (default `30s`).
- `PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL` — minimum spacing between manual refreshes (default `5s`).
- `PVMSS_INVENTORY_REFRESH_TIMEOUT` — per-refresh timeout (default `15s`).
- `PVMSS_MAX_LIST_PAGE_SIZE` — maximum list page size (default `100`).

For full deployment instructions (Docker, Kubernetes, Helm), see the project README.

## Nodes

`/admin/nodes` lists every Proxmox VE host with live CPU and memory consumption, plus online/offline status. A background worker refreshes node metrics on the interval configured by `PVMSS_INVENTORY_REFRESH_INTERVAL`; the admin pages read this local cache, so navigation stays instant even on large clusters. Each node card also exposes a per-node refresh button and the last update timestamp. When Proxmox reports a node offline, PVMSS keeps showing the last known resource values while clearly marking the node as offline.

## Clusters (multi-cluster)

PVMSS supports connecting to more than one Proxmox environment at the same time. Each connection is a **cluster** with its own URL, API token, and optional OIDC provider.

- Open **Admin > Clusters** (`/admin/clusters`) to add, edit, test, and remove cluster connections.
- A cluster is identified by a name; VMs are always addressed by their `cluster` and `VMID`, so two clusters may reuse the same VMIDs without conflict.
- Use the **Test** action to verify connectivity and credentials before exposing the cluster to users.
- When a cluster's Proxmox supports OIDC, you can **enable OIDC** on that cluster so its users may sign in through the cluster's identity provider from the login screen.
- Approved nodes, storages, ISOs, bridges, and tags are managed per cluster; the catalog surfaces let you scope what users see on each cluster.

The **Application Info** page reports the connected cluster(s), name, and node count so you can confirm the expected topology.

## Catalog: resources exposed to users

The **Catalog** area of the admin nav controls what VM creation may reference. Discovered resources appear here automatically; toggle the enabled switch to control what users see.

- **Storages** (`/admin/storages`) — approve the storage backends that may host VM disks, grouped by node when connected to a cluster.
- **ISOs** (`/admin/isos`) — approve the ISO images users may boot from.
- **Bridges** (`/admin/bridges`) — approve the network bridges (VMBR) available for VM network cards. Open vSwitch bridges are not listed.
- **Cloud-init templates** (`/admin/cloudinit-templates`) — create, enable, disable, and edit admin-curated `#cloud-config` templates users can pick at creation time.
- **Profiles** (`/admin/profiles`) — define pre-approved hardware profiles (CPU, memory, disk, bus) so users can pick a known-good shape instead of free-typing values.
- **Tags** (`/admin/tags`) — manage the labels users can attach to VMs for filtering and search. A tag is immutable once created; the `pvmss` tag is reserved and cannot be deleted.

## Network considerations

End users can specify per network card, on the Create VM and Edit resources pages:

- **Network speed** (Proxmox `rate`, in MB/s): enforced between 1 and 10240 MB/s. Leaving it empty grants unlimited speed.
- **VLAN tag** (1-4096): appended to the interface as `,tag=X`. Ensure your physical switches and Proxmox bridges are configured for the VLAN IDs you allow.
- **MTU** (576-9000 bytes): leaving it empty uses the Proxmox default of 1500. Only use custom MTUs on carefully controlled networks.

As an administrator, document the VLAN IDs your users should use and monitor creation logs for VLAN-related issues.

## User pools

`/admin/pools` is where you create self-service users. Each pool provisions:

- a dedicated Proxmox user,
- a dedicated Proxmox pool named after the user, and
- an ACL binding the user to the shared `PVMSSUser` role on that pool.

PVMSS enforces a pool name pattern (1-32 lowercase alphanumeric characters with internal hyphens) and a minimum password length of 8 characters. Users only ever see the VMs inside their own pool.

## Policy (limits)

`/admin/policy` sets per-user quotas: maximum number of VMs, CPU, memory, and disk. `/admin/policy/nodes` caps how much of a single node's resources one VM may consume. Both are enforced server-side before any Proxmox call, so requests above the quota or the node capacity are rejected early. Snapshot limits (maximum snapshots per VM) are also enforced through policy.

## Documentation (this CMS)

This page is one of several managed under **Documentation** (`/admin/docs`). Administrators can author, edit, toggle, and delete Markdown pages here. Built-in pages are marked **system** and cannot be deleted, but their content may be edited. Each page has an audience of `user` (public) or `admin` (admin-only); admin-only pages are hidden from non-administrators in the public docs list.

## Settings, audit, and maintenance

`/admin/settings` exposes operational controls:

- **Audit log** — every VM write (create, start, stop, edit, delete, cloud-init change) is recorded with the acting user, so you can trace activity.
- **Database export / import** — back up or restore the SQLite database used by PVMSS for all its configuration and audit history.

## Security recommendations

- Serve PVMSS only over HTTPS (typically behind a reverse proxy) and restrict access to trusted networks.
- Use dedicated Proxmox accounts for PVMSS; avoid sharing the built-in administrator password.
- Keep Proxmox permissions simple: a service API token with the `PVMSS_Service` role for the backend, human administrators with the `PVMSS_Admin` role, and end users confined to their `PVMSSUser`-scoped pool. See `/docs/proxmox-permissions` for the exact `pveum` commands.
- Do not grant broad Proxmox privileges to regular self-service users.
- Periodically review user pools and disable or delete unused accounts.

## Known limitations

- PVMSS targets Proxmox VE 8.x/9.x clusters (and standalone nodes).
- There is no external authentication integration (OIDC/SAML) wired into PVMSS accounts beyond what the login screen may offer; user accounts are provisioned through `/admin/pools` on the Proxmox side.
- PVMSS supports standalone servers and clusters, but advanced cluster operations (live migration, HA, backup orchestration) are performed directly in Proxmox.
- Administrators cannot create VMs from the admin interface; VM creation happens through the user self-service UI or directly in Proxmox.
- Backups and LXC containers are managed in Proxmox, not in PVMSS.
