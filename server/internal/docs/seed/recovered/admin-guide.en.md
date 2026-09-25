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
3. Add or verify your **Clusters** (`/admin/clusters`) and run the connection **Test**.
4. Approve the resources users may use: **Nodes**, **Storages**, **ISOs**, **VM templates**, **Cloud images**, and **Bridges**.
5. Optional but recommended: create **VM profiles** so users can pick pre-approved hardware shapes, define **Tags**, and write **Cloud-init templates**.
6. Set the **Policy** (per-cluster gabarit and quota) and **Node capacity** caps.
7. Optionally enable **cloud-init documents** (see below).
8. Create as many **User pools** as you need from `/admin/pools`.
9. Tell your users the portal is available so they can start creating VMs.
10. Watch the **Audit log** (`/admin/settings`) to trace every VM write back to the acting user.

## Dashboard

`/admin` summarizes the cluster: node status and load, VM counts by state,
the running version, and the time of the last inventory refresh.

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

- `PVMSS_PORT` - TCP port the server listens on (the official image uses `50000`).
- `PVMSS_DB_PATH` - path to the SQLite database file.
- `SESSION_SECRET` - 32+ bytes used to encrypt user sessions.
- `LOG_LEVEL` - `debug`, `info`, `warn`, or `error` (lowercase only).
- `LOG_FORMAT` - `json` or `console`.
- `LOG_OUTPUT` - `stdout`, `stderr`, or a file path.
- `PVMSS_CLUSTER_SOURCE` - `proxmox` or `fake`. There is no default on purpose: `fake` ships with demo credentials and must never be selected by accident.

Required when `PVMSS_CLUSTER_SOURCE=proxmox`:

- `PROXMOX_URL` - for example `https://host:8006/api2/json`.
- `PROXMOX_API_TOKEN_NAME` - the Proxmox API token id (`user@pve!token`).
- `PROXMOX_API_TOKEN_VALUE` - the matching token secret.

These three describe the first cluster; additional clusters are added from `/admin/clusters`.

Optional:

- `PVMSS_HOST` - bind address (the image sets `0.0.0.0`).
- `PVMSS_WEB_DIR` - location of the built SPA (defaults to a path relative to the executable).
- `ADMIN_PASSWORD_HASH` - if set, must be a bcrypt hash (`$2…`); enables the local administrator login.
- `PVMSS_COOKIE_SECURE` - defaults to `true`; set `false` only behind plain HTTP for local trials.
- `PVMSS_TRUSTED_PROXY_HOPS` - number of reverse proxies in front of PVMSS, used to derive the client IP for rate limits and the audit log (default `1`).
- `PVMSS_INVENTORY_REFRESH_INTERVAL` - background inventory refresh interval (default `30s`).
- `PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL` - minimum spacing between manual refreshes (default `5s`).
- `PVMSS_INVENTORY_REFRESH_TIMEOUT` - per-refresh timeout (default `15s`).
- `PVMSS_MAX_LIST_PAGE_SIZE` - maximum list page size (default `100`).
- `PVMSS_SSH_KEY_FILE` - private key PVMSS uses to publish cloud-init documents to the nodes over SSH (see below).

For full deployment instructions (Docker, Kubernetes, Helm), see the project README.

## Clusters (multi-cluster)

PVMSS supports connecting to more than one Proxmox environment at the same time. Each connection is a **cluster** with its own URL, API token, and options.

- Open **Admin > Clusters** (`/admin/clusters`) to add, edit, test, and remove cluster connections.
- A cluster is identified by a name; VMs are always addressed by their `cluster` and `VMID`, so two clusters may reuse the same VMIDs without conflict.
- Use the **Test** action to verify connectivity and credentials before exposing the cluster to users; it reports the Proxmox version and the node and VM counts.
- **TLS verification** can be skipped per cluster for self-signed labs; keep it on in production.
- The **OIDC** toggle is reserved for a future single sign-on integration; enabling it shows a button on the login screen but sign-in is not implemented yet.
- **Snippet storage**, **SSH user/port** and the **pinned host keys** enable cloud-init publishing for this cluster (see the dedicated section below). The cluster badge reads "cloud-init: on" once they are set and `PVMSS_SSH_KEY_FILE` is configured.
- Approved nodes, storages, ISOs, images, templates, bridges, cloud-init templates, and the policy are all managed per cluster.

## Nodes

`/admin/nodes` lists every Proxmox VE host per cluster with live CPU and memory consumption, VM count, and online/offline status. Search, filter by status or enabled state, and sort the table. Toggle a node to approve or hide it for VM creation; disabling a node that runs VMs asks for confirmation and never touches those VMs. A background worker refreshes node metrics on `PVMSS_INVENTORY_REFRESH_INTERVAL`; the admin pages read this cache, so navigation stays instant even on large clusters.

## Catalog: resources exposed to users

The **Catalog** area of the admin nav controls what VM creation may reference. Discovered resources appear automatically; toggle the enabled switch to control what users see. Every catalog page has a cluster selector, search, filters, and sortable columns.

- **Storages** (`/admin/storages`) - approve the storage backends that may host VM disks, grouped by node, with usage bars.
- **ISOs** (`/admin/isos`) - approve the ISO images users may boot from.
- **VM templates** (`/admin/templates`) - approve the Proxmox templates users may clone, with optional per-template overrides. A clone stays on the template's node; the wizard warns when the target storage forces a full copy.
- **Cloud images** (`/admin/images`) - approve cloud images discovered under a storage's `import/` content directory. PVMSS never downloads images from the internet: place them on the storage yourself.
- **Bridges** (`/admin/bridges`) - approve the network bridges (VMBR) available for VM network cards. Open vSwitch bridges are not listed.
- **Cloud-init templates** (`/admin/cloudinit-templates`) - create, enable, disable, and edit admin-curated `#cloud-config` documents users can pick at creation time.
- **Profiles** (`/admin/profiles`) - define pre-approved hardware profiles (sockets, cores, memory, disk, bus) with optional node/storage overrides, an icon and a color, so users can pick a known-good shape instead of free-typing values.
- **Tags** (`/admin/tags`) - manage the labels users can attach to VMs, with a color each. A tag is immutable once created (only its color changes); the `pvmss` tag is reserved and cannot be deleted.

### Stale approvals

Approvals are reconciled with live discovery. When a resource (node, storage, ISO, image, template, bridge) disappears from Proxmox, its row is flagged as missing and offers a **Remove** action so the catalog never references something that no longer exists.

## Network considerations

At creation, users pick a bridge and a card model per NIC; the Proxmox per-VM firewall is always enabled. After creation, they can edit each NIC on the VM's Network tab:

- **Network speed** (Proxmox `rate`, in Mbps): leaving it empty grants unlimited speed.
- **VLAN tag** (1-4094): appended to the interface as `,tag=X`. Ensure your physical switches and Proxmox bridges are configured for the VLAN IDs you allow.

The policy's **Isolation VLAN tag** applies one VLAN to every NIC created through PVMSS on that cluster; leave it at 0 to disable.

## User pools

`/admin/pools` is where you create self-service users. Each pool provisions:

- a dedicated Proxmox user,
- a dedicated Proxmox pool for that user, and
- an ACL binding the user to the shared `PVMSSUser` role on that pool.

You enter a short name (1-32 lowercase alphanumeric characters with internal hyphens); PVMSS prefixes it with `pvmss-`, so the Proxmox pool is `pvmss-<name>` and the user `pvmss-<name>@pve`. The login password is generated for you, shown once in the create response, and never stored - communicate it to the user securely. Users only ever see the VMs inside their own pool. Deleting a pool cascades to the Proxmox user and ACL; pools not created by PVMSS are refused.

## Policy (limits)

`/admin/policy` is per cluster and has two parts:

- **Gabarit** - the ceiling for a single VM: max sockets, max cores, max memory, max disk per VM, max network cards, max snapshots, and the isolation VLAN tag.
- **Quota** - max VMs per user.

`/admin/policy/nodes` caps how much of a single node PVMSS may allocate in total (VMs, vCPUs, RAM, disk) and shows the current usage against the physical capacity. Everything is enforced server-side before any Proxmox call, so requests above a limit are rejected early with a clear message.

## Platform limits

Beyond the policy knobs, the application itself enforces:

- **Rate limits** - 10 requests/minute per IP on authentication endpoints; 30 writes/minute per user on VM routes; 120 status polls/minute per user; 60 writes/minute per user on admin routes; 10 cluster tests/minute.
- **Bulk power actions** - at most 100 VMs per request.
- **VM name** - a lowercase hostname, at most 63 characters, unique in the owner's pool; **description** - at most 512 characters.
- **Snapshot name** - a leading letter, then letters, digits, hyphens or underscores, 2 to 40 characters; `current` is reserved.
- **Pool name** - 1-32 lowercase alphanumeric characters with internal hyphens (stored as `pvmss-<name>`).
- **List pages** - capped at `PVMSS_MAX_LIST_PAGE_SIZE` entries per page (default 100).

## Enabling cloud-init documents

Only administrators write cloud-init documents (**Admin › Cloud-init templates**); users pick one when they create a VM or switch a VM to another one from its Cloud-init tab. Proxmox's REST API cannot write `snippets` files, so PVMSS publishes each template, merged with the qemu-guest-agent baseline, as an immutable `pvmss-tpl-<id>-<hash>.yml` file into the snippet storage of **every node**, over SSH, and checks through the API that each node lists it. Creating a VM never writes a file: the VM points at the published file. Editing a template publishes a new file, so existing VMs keep their version.

1. In Proxmox, enable the **Snippets** content type on a storage available on every node (Datacenter › Storage › Edit; `local` works).
2. Generate a key pair (`ssh-keygen -t ed25519 -N '' -f pvmss_ed25519`) and give PVMSS the private key with `PVMSS_SSH_KEY_FILE`. Compose: mount it read-only. Helm: a Secret named in `cloudInit.sshKeySecret`.
3. On every node, as root: `sh tools/pvmss-node-setup.sh --storage <storage> --key '<PVMSS public key>'`. It installs the `pvmss-snippet` helper, a dedicated `pvmss` user that can only write the storage's `snippets/` directory, and the key with a forced command (no shell). It prints the node's host key.
4. **Admin › Clusters › Edit**: snippet storage, SSH user (`pvmss`), port and pinned host keys (paste `ssh-keyscan -t ed25519 <node ip>` lines, or save without the SSH user first, reopen and **Scan host keys**), compare the fingerprints, save. Host keys are always verified. The badge turns "cloud-init: on" and PVMSS republishes everything in the background.
5. Verify: create a cloud-init template (the "Published" column must read n/n nodes), create a VM with it, then on the node run `qm config <vmid> | grep cicustom`.
6. After adding or reinstalling a node, click **Publish to all nodes** on the templates page.

Without SSH publishing, the wizard hides the document picker and a create request carrying a template is refused before any VMID is spent. A template missing from the VM's node is refused with `cloudinit_not_published`.

Cloud-image VMs created without a template get the baseline alone (`pvmss-baseline-<hash>.yml`); when it is not on the node, the VM still boots on its native keys.

Old published versions stay on the nodes (a few KB each). Per-VM files from earlier versions (`pvmss-<vmid>.yml`) are removed when their VM is deleted through PVMSS.

See the [cloud-init setup guide](/docs/cloud-init-setup) for troubleshooting.

## Documentation (this CMS)

This page is one of several managed under **Documentation** (`/admin/docs`). Administrators can author, edit, toggle, and delete Markdown pages in English and French. Built-in pages are marked **system** and cannot be deleted, but their content may be edited. Each page has an audience of `user` (public) or `admin` (admin-only); admin-only pages are hidden from non-administrators in the public docs list and refused on direct access.

Built-in pages are seeded once, when missing, and never overwritten on restart, so your edits survive upgrades. To pick up a newer built-in text after an upgrade, delete the page and restart, or paste the new content from the release.

## Settings, audit, and maintenance

`/admin/settings` exposes operational controls:

- **Audit log** - every write (VM create, power action, edit, delete, cloud-init change, catalog and policy changes, sign-ins) is recorded with the acting user, IP, severity, and target. Filter and page through it; each VM also shows its own entries on its Activity tab.
- **Retention** - set the number of days to keep audit rows and preview how many rows a prune would delete before applying.
- **Database export / import** - back up the SQLite database or restore one. Import is two-phase: upload returns a table-by-table preview, and nothing is written until you confirm.

## Security recommendations

- Serve PVMSS only over HTTPS (typically behind a reverse proxy) and restrict access to trusted networks. Set `PVMSS_TRUSTED_PROXY_HOPS` to the number of proxies so rate limits and the audit log see the real client IP.
- Use dedicated Proxmox accounts for PVMSS; avoid sharing the built-in administrator password.
- Keep Proxmox permissions simple: a service API token with the `PVMSS_Service` role for the backend, human administrators with the `PVMSS_Admin` role, and end users confined to their `PVMSSUser`-scoped pool. See `/docs/proxmox-permissions` for the exact `pveum` commands.
- Do not grant broad Proxmox privileges to regular self-service users.
- Periodically review user pools and disable or delete unused accounts.
- Cloud-init documents are stored in plain text; tell users never to put secrets in them.

## Known limitations

- PVMSS targets Proxmox VE 8.x/9.x clusters (and standalone nodes).
- OIDC/SSO sign-in is not implemented yet; user accounts are provisioned through `/admin/pools` on the Proxmox side.
- Advanced cluster operations (live migration, HA, backup orchestration) are performed directly in Proxmox.
- Administrators create VMs through the same self-service UI as users, or directly in Proxmox.
- Backups and LXC containers are managed in Proxmox, not in PVMSS.
- Password change is API-only for now (`POST /api/v1/auth/password`).
- Personal API tokens are deactivated in this version: their routes are not registered, so the API tokens page cannot create or list tokens.
