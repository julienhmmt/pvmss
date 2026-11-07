# Proxmox VM Self-Service (PVMSS)

PVMSS is a lightweight, self-service web portal for Proxmox Virtual Environment (PVE). It lets users create, operate, and troubleshoot virtual machines (VMs) without exposing the Proxmox web UI. The application runs as a single container (Go REST API + SvelteKit SPA) backed by an embedded SQLite database.

⚠️ Only PVE 9.x and newer are supported. PVE 8 and earlier are not supported due to breaking role and permission changes introduced in PVE 9.

## Features

The complete, route-by-route inventory lives in [docs/FEATURES.md](https://github.com/julienhmmt/pvmss/blob/main/docs/FEATURES.md).

### For users

- **Sign in**: Authenticate with Proxmox credentials (`pve` or `pam` realm) on the cluster of your choice.
- **My VMs**: VM list of the sign-in cluster, linkable search/status filters, live status refresh, bulk power actions with per-VM results, and quick console access.
- **Create VM wizard**: Simple (single-page) and Detailed (5-step) modes from approved **ISOs** or **Proxmox templates** (linked or full clone). Choose hardware profiles or custom CPU/RAM/disk, auto-placement with capacity scoring, multi-NIC (bridge + model), UEFI (no Secure Boot) / TPM 2.0, curated tags, and CD-ROM boot priority.
- **Operate a VM**: 7 power actions (`start`, `shutdown`, `stop`, `reboot`, `reset`, `pause`, `resume`), rename, Markdown description, delete; disk add/grow/detach; NIC edit (bridge, model, VLAN, rate limit); CPU/RAM/tags/CD-ROM; snapshots (create/rollback/delete, optional RAM) plus a daily/weekly automatic schedule with retention; duplicate a VM into a full clone; lease expiry and extension on lease-enabled clusters; historical (hour/day/week) and live streaming metrics; per-VM activity log.
- **Consoles**: Built-in noVNC graphical console and serial (xterm.js) console, both WebSocket-proxied through PVMSS with single-use tickets.
- **Activity & nodes**: Track running and recent operations across your VMs (`/activity`) and inspect live node capacity (`/nodes`).
- **Profile & preferences**: Manage your password, theme, and language (**English**, **French**, **German** and **Italian**), with built-in user documentation (`/docs`).

### For administrators

- **Dual admin sign-in**: Sign in with a Proxmox account holding `Permissions.Modify` on `/` (`Administrator` / `PVMSS_Admin`; `PVEAdmin` lacks it), or with a PVMSS local administrator account (bootstrap owner via `ADMIN_PASSWORD_HASH` plus DB-managed local admin accounts) that works even when clusters are unreachable.
- **Multi-cluster management**: Connect multiple Proxmox environments, test connectivity, monitor health, rename a cluster id, and adopt pools/tags a previous PVMSS install left on a cluster (**Admin > Recovery**).
- **Catalog curation**: Approve nodes, storages, ISOs, VM templates, and network bridges; manage hardware profiles and curated tags; reconcile stale approvals against live cluster discovery.
- **User pool provisioning**: Create a self-service user (Proxmox user + resource pool + ACL) in one step, with cascade deletion.
- **Live VM migration**: Migrate a PVMSS-managed VM to another approved online node in the same cluster with preflight checks and task tracking.
- **Policies & quotas**: Configure per-cluster gabarit limits (sockets, cores, memory, disk per VM, NICs, snapshots, isolation VLAN), per-pool VM quotas with per-pool overrides, and per-node capacity caps. Approving a user's quota request writes an override for that user's pool.
- **System & operations**: Admin dashboard, runtime app info, runtime log-level switch, audit log with retention and prune preview, SQLite backup export and two-phase import, Prometheus `/metrics` endpoint, and in-app bilingual documentation CMS.

> **Note on cloud-init**: Cloud-init authoring (native forms, cloud-image imports, cloud-init snippet templates) was removed from PVMSS on 2026-10-31; only a read-only cloud-init inspection endpoint remains. Cloned VMs inherit the sign-in baked into their Proxmox VM template, and ISO installs use the account created during installation. See [docs/cloud-init.md](https://github.com/julienhmmt/pvmss/blob/main/docs/cloud-init.md).

## Getting started

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/)
- A Proxmox VE 9.x+ host or cluster

### 1. Configure Proxmox roles and API token

PVMSS uses a dedicated service account and API token for cluster operations, plus an admin role for human administrators. Run the following commands as `root` on a Proxmox node (or configure equivalent objects in the Proxmox UI under **Datacenter > Permissions**):

```bash
# Service role, user, and API token (privilege separation disabled)
pveum roleadd PVMSS_Service -privs "Sys.Audit VM.Audit VM.Allocate VM.PowerMgmt VM.Console VM.Config.CDROM VM.Config.CPU VM.Config.HWType VM.Config.Memory VM.Config.Disk VM.Config.Network VM.Config.Options VM.Config.Cloudinit VM.Snapshot VM.Snapshot.Rollback VM.Migrate VM.GuestAgent.Audit VM.GuestAgent.Unrestricted Datastore.Audit Datastore.AllocateSpace Datastore.AllocateTemplate Pool.Allocate Pool.Audit User.Modify Permissions.Modify Realm.AllocateUser SDN.Allocate SDN.Audit SDN.Use"

pveum useradd pvmss-svc@pve -comment "PVMSS service account" -enable 1
pveum aclmod / -user pvmss-svc@pve -role PVMSS_Service -propagate 1
pveum user token add pvmss-svc@pve pvmss-service-token --privsep 0

# Optional: human Proxmox administrator account for PVMSS
pveum roleadd PVMSS_Admin -privs "Sys.Audit VM.Audit VM.PowerMgmt VM.Console VM.Config.CPU VM.Config.Memory VM.Config.Disk VM.Config.Network VM.Config.HWType VM.GuestAgent.Audit VM.Migrate VM.Config.CDROM VM.Config.Options VM.Config.Cloudinit Datastore.Audit Datastore.AllocateSpace Pool.Allocate Pool.Audit User.Modify Permissions.Modify Realm.AllocateUser SDN.Audit Group.Allocate"

pveum useradd pvmss-admin1@pve -comment "PVMSS administrator" -password "strong_password" -enable 1
pveum aclmod / -user pvmss-admin1@pve -role PVMSS_Admin -propagate 1
```

Save the token name (`pvmss-svc@pve!pvmss-service-token`) and secret value output by `pveum user token add` for `PROXMOX_API_TOKEN_NAME` and `PROXMOX_API_TOKEN_VALUE`.

### 2. Database & configuration storage

All application configuration (approved nodes, storages, ISOs, templates, bridges, hardware profiles, limits, tags, user pools, clusters, docs, and audit logs) is stored in an embedded **SQLite** database inside `/data` and managed from the **Admin** UI - no `settings.json` file is needed.

Always mount a persistent volume at `/data` (not a single file mount) because SQLite runs in WAL mode and creates `-wal` and `-shm` files alongside `pvmss.db`.

### 3. Environment variables

The server validates all environment variables at startup and refuses to start if a required variable is missing or invalid.

| Variable                   | Description                                                                                | Required      | Default          |
| -------------------------- | ------------------------------------------------------------------------------------------ | ------------- | ---------------- |
| `PVMSS_PORT`               | TCP port the HTTP server listens on (`1`–`65535`)                                          | ✅            | -                |
| `PVMSS_DB_PATH`            | Path to the SQLite database file (preset to `/data/pvmss.db` in the Docker image)          | ✅            | `/data/pvmss.db` |
| `SESSION_SECRET`           | 32+ byte secret to sign/encrypt sessions (`openssl rand -hex 32`)                          | ✅            | -                |
| `PVMSS_CLUSTER_SOURCE`     | `proxmox` for a real cluster, `fake` for demo data (no default on purpose)                 | ✅            | -                |
| `LOG_LEVEL`                | `debug`, `info`, `warn`, or `error` (**lowercase only**)                                   | ✅            | -                |
| `LOG_FORMAT`               | `console` (human-readable) or `json` (log aggregators)                                     | ✅            | -                |
| `LOG_OUTPUT`               | `stdout`, `stderr`, or a writable file path                                                | ✅            | -                |
| `PROXMOX_URL`              | Full Proxmox API URL (`https://host:8006/api2/json`) to seed the initial `default` cluster | ❌ (see note) | -                |
| `PROXMOX_API_TOKEN_NAME`   | Proxmox API token name (`user@pve!token`)                                                  | ❌ (see note) | -                |
| `PROXMOX_API_TOKEN_VALUE`  | Proxmox API token secret                                                                   | ❌ (see note) | -                |
| `ADMIN_PASSWORD_HASH`      | Bcrypt hash for the bootstrap local admin (`admin`); disabled when empty                   | ❌            | empty            |
| `PVMSS_COOKIE_SECURE`      | `Secure` attribute on session cookies (`true` behind HTTPS, set `false` for plain HTTP)    | ❌            | `true`           |
| `PVMSS_TRUSTED_PROXY_HOPS` | Number of reverse proxies in front of PVMSS (set it when behind one; Helm sets `1`)        | ❌            | `0`              |
| `TZ`                       | Container timezone                                                                         | ❌            | `UTC`            |

The most common variables are listed above. Syslog/SIEM, TLS and mutual TLS, Prometheus metrics, inventory tuning and rate limits are described in [docs/configuration.md](https://github.com/julienhmmt/pvmss/blob/main/docs/configuration.md).

> The three `PROXMOX_*` variables are all-or-nothing: set all three to seed the
> `default` cluster on first boot (afterwards, clusters are managed from the
> **Admin > Clusters** UI), or leave all three empty and the server boots with
> no cluster and serves the first-run setup at `/setup` until an administrator
> registers one. A partial set refuses to boot.

> **Tip**: Generate `ADMIN_PASSWORD_HASH` with `htpasswd -bnBC 10 "" "StrongPassword" | tr -d ':\n'`. In `docker-compose.yml` or a `.env` file loaded by Compose, escape every `$` as `$$`.

### 4. Run with Docker Compose

Create a `docker-compose.yml` file:

```yaml
services:
  pvmss:
    image: jhmmt/pvmss:0.4.0
    container_name: pvmss
    restart: unless-stopped
    ports:
      - "50000:50000/tcp"
    environment:
      PVMSS_CLUSTER_SOURCE: "proxmox"
      PROXMOX_URL: "https://ip-or-name:8006/api2/json"
      PROXMOX_API_TOKEN_NAME: "tokenName@changeMe!value"
      PROXMOX_API_TOKEN_VALUE: "aaaaaaaa-0000-44aa-1111-aaaaaaaaaaa"
      # Every "$" of the bcrypt hash is doubled for Compose interpolation
      ADMIN_PASSWORD_HASH: "$$2y$$10$$Ppg7Wl3sNYrmxZmWgcq4reOyznt7AeqMrQucaH4HY.dBrzavhPP1e"
      # Plain HTTP without a TLS reverse proxy: disable Secure cookie flag (set "true" behind HTTPS)
      PVMSS_COOKIE_SECURE: "false"
      LOG_LEVEL: "info"
      LOG_OUTPUT: "stdout"
      LOG_FORMAT: "console"
      SESSION_SECRET: "replace-with-output-of-openssl-rand-hex-32" # generate: openssl rand -hex 32
      PVMSS_PORT: "50000"
      PVMSS_DB_PATH: "/data/pvmss.db"
      TZ: "Europe/Paris"
    volumes:
      - pvmss_data:/data
    deploy:
      resources:
        limits:
          cpus: "1"
          memory: 128M

volumes:
  pvmss_data: {}
```

Start the container:

```bash
docker compose up -d
```

### 5. Run with `docker run`

```bash
docker run -d \
  --name pvmss \
  --restart unless-stopped \
  -p 50000:50000 \
  -v pvmss_data:/data \
  -e ADMIN_PASSWORD_HASH='$2y$10$Ppg7Wl3sNYrmxZmWgcq4reOyznt7AeqMrQucaH4HY.dBrzavhPP1e' \
  -e LOG_LEVEL=info \
  -e LOG_OUTPUT=stdout \
  -e LOG_FORMAT=console \
  -e PROXMOX_API_TOKEN_NAME='tokenName@changeMe!value' \
  -e PROXMOX_API_TOKEN_VALUE="aaaaaaaa-0000-44aa-1111-aaaaaaaaaaa" \
  -e PROXMOX_URL=https://ip-or-name:8006/api2/json \
  -e PVMSS_CLUSTER_SOURCE=proxmox \
  -e PVMSS_COOKIE_SECURE=false \
  -e PVMSS_PORT=50000 \
  -e PVMSS_DB_PATH="/data/pvmss.db" \
  -e SESSION_SECRET="$(openssl rand -hex 32)" \
  -e TZ=Europe/Paris \
  jhmmt/pvmss:0.4.0
```

The application will be available at [http://localhost:50000](http://localhost:50000). Sign in with your Proxmox account, or click **Local administrator** on the login page to sign in with `admin` and the password matching `ADMIN_PASSWORD_HASH`.

Lost a local administrator password? The owner resets DB accounts under **Admin > Settings > Accounts**; the bootstrap `admin` is rotated by setting a fresh `ADMIN_PASSWORD_HASH` and restarting - which is also the escape hatch when the setup-created owner locks itself out.

### View logs

```bash
docker logs -f pvmss
```

### Deployment on Kubernetes

PVMSS ships a Helm chart in the repository under [`helm/`](https://github.com/julienhmmt/pvmss/tree/main/helm):

```bash
helm install pvmss ./helm -n pvmss --create-namespace -f my-values.yaml
```

## Architecture

- **Server (`server/`)**: Go 1.27 REST API (`net/http`), embedded SQLite (`modernc.org/sqlite`, CGO-free), structured logging (`log/slog`), OpenTelemetry tracing/metrics, and Prometheus `/metrics`. Serves `/api/v1/*`, WebSocket console proxies, and the static SPA.
- **Web (`web/`)**: SvelteKit SPA built with Svelte 5 runes, TypeScript, Tailwind CSS v4, Paraglide i18n (EN/FR/DE/IT), noVNC, and xterm.js.
- **Container image**: Multi-arch (`linux/amd64`, `linux/arm64`) distroless non-root image (`gcr.io/distroless/static-debian13:nonroot`, UID `65532`).

## Limitations

- Security hardening is ongoing; no formal third-party penetration test has been performed yet.
- No HA-managed VMs, no cross-cluster VM migration, and SDN / firewall rules remain managed in Proxmox.
- Proxmox OIDC realms cannot sign in to PVMSS yet (only `pve` and `pam` realms are supported, plus PVMSS local administrator accounts).
- Cloud-init customization and cloud-image imports were removed on 2026-10-31; build the sign-in into your Proxmox VM templates instead.
- ISO images and VM templates are uploaded/created in Proxmox and then approved in PVMSS.

## Sources

- [GitHub Repository](https://github.com/julienhmmt/pvmss)
- [Issue Tracker & Roadmap](https://github.com/julienhmmt/pvmss/projects?query=is%3Aopen)

## License

PVMSS by Julien HOMMET is licensed under **GNU AGPL v3**. See <https://www.gnu.org/licenses/agpl-3.0.en.html>.
