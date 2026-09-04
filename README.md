# Proxmox VM Self-Service (PVMSS)

[![Lint](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml/badge.svg?branch=main&event=push)](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml) [![Go](https://github.com/julienhmmt/pvmss/actions/workflows/go.yml/badge.svg?branch=main&event=push)](https://github.com/julienhmmt/pvmss/actions/workflows/go.yml)

> A lightweight, self-service portal for Proxmox VE that lets users create, operate, and troubleshoot virtual machines without exposing the Proxmox UI.

French version: [README.fr.md](README.fr.md)

---

## Table of contents

1. [Overview](#overview)
2. [Feature highlights](#feature-highlights)
3. [Architecture at a glance](#architecture-at-a-glance)
4. [Configuration](#configuration)
5. [Deployment options](#deployment-options)
6. [Quick start with Docker run](#quick-start-with-docker-run)
7. [Start with Docker compose](#start-with-docker-compose)
8. [Start with Kubernetes](#start-with-kubernetes)
9. [Operations](#operations)
10. [Limitations](#limitations)
11. [License](#license)

---

## Overview

PVMSS runs as a stateless web application (Go REST API + SvelteKit SPA) and relies on Proxmox APIs for every action. It is designed to be:

- **Secure by default**: per-user sessions.
- **Operations-friendly**: ready-to-use container image, configurable resource limits, cluster-aware storage selection.
- **User-centric**: clear and guided VM forms, built-in documentation.

> ⚠️ The project is still under active development. Review the [Limitations](#limitations) section before deploying to production.

## Feature highlights

### End users

- Create VMs with custom CPU/RAM/disk/ISO/network/tag options (EFI, TPM, multi-NIC, disk bus selection, network card model, etc.).
- Launch the Proxmox noVNC console straight from the portal (websocket proxy with session cookies).
- Start, stop, reboot, delete, and resize existing VMs.
- Search VMs by VMID or name and inspect live metrics (CPU, memory, disk, network, uptime).
- Self-service profile: list personal VMs, reset password, view quotas.
- Interface localized in **English** and **French**.

### Administrators

- Approve Proxmox nodes, storages, VMBRs, and ISO repositories shown to users.
- Manage tags and user pools.
- Define global VM limits plus per-node caps (CPU, RAM, disk, number of NICs/disks).
- Admin documentation and application insights page (runtime, environment, cluster status).
- **Unified Settings Panel**: A single interface to manage all configuration (VM limits, node limits, inventory items, cloud-init templates, VM profiles, SFTP configuration) with audit trail and import/export functionality.

## Architecture at a glance

- **Server** (`server/`): Go 1.26, stdlib `net/http` routing, SQLite via `modernc.org/sqlite` (CGO-free). Serves `/api/v1/*` and the SPA.
- **Web** (`web/`): SvelteKit SPA (Svelte 5 runes, TypeScript, Tailwind CSS v4, `adapter-static`).
- **Authentication**: Proxmox API token for cluster actions, user sessions for the UI.

## Configuration

### Proxmox roles and permissions (required)

PVMSS relies on dedicated Proxmox roles and ACLs to work correctly (service account, admin accounts, per-user pools).

Before using PVMSS in production, you **must**:

- Create the `PVMSS_Service` role with the expected privileges.
- Create the `PVMSS_Service` user and its API token.
- Create a `PVMSS_Admin` user (human admin to manage PVMSS app and PVMSS users).
- Create the ACLs for the `PVMSS_Service` and `PVMSS_Admin` users.

In your Proxmox cluster, you can create the roles and ACLs using the `pveum` command-line tool. You can also create them using the Proxmox web interface. As _root_, create the roles and privileges:

```bash
# PVMSS_Service
pveum roleadd PVMSS_Service -privs "Sys.Audit VM.Audit VM.Allocate VM.PowerMgmt VM.Console VM.Config.CDROM VM.Config.CPU VM.Config.HWType VM.Config.Memory VM.Config.Disk VM.Config.Network VM.Config.Options VM.Config.Cloudinit VM.Snapshot VM.Snapshot.Rollback Datastore.Audit Datastore.AllocateSpace Datastore.AllocateTemplate Pool.Allocate Pool.Audit User.Modify Permissions.Modify Realm.AllocateUser SDN.Allocate SDN.Audit SDN.Use"

pveum useradd pvmss-svc@pve -comment "PVMSS service account" \
  -enable 1

pveum user token add pvmss-svc@pve pvmss-service-token --privsep 0

# PVMSS_Admin
pveum roleadd PVMSS_Admin -privs "Sys.Audit VM.Audit VM.PowerMgmt VM.Console VM.Config.CPU VM.Config.Memory VM.Config.Disk VM.Config.Network VM.Config.HWType VM.GuestAgent.Audit VM.Migrate VM.Config.CDROM VM.Config.Options VM.Config.Cloudinit Datastore.Audit Datastore.AllocateSpace Pool.Allocate Pool.Audit User.Modify Permissions.Modify Realm.AllocateUser SDN.Audit Group.Allocate"

pveum useradd pvmss-admin1@pve \
  -comment "PVMSS administrator <name>" -password "strong_password" \
  -enable 1

pveum aclmod / -user pvmss-admin1@pve -role PVMSS_Admin -propagate 1
```

The `pveum` commands and information related to roles and required privileges are detailed in:

- The in-app admin page `/docs/proxmox-permissions` (once PVMSS is running and you are logged in as admin)

### Create an API token for the user root@pam

If you want to use PVMSS in development, you **must** create an API token for the user `root@pam`. It is the simple way to get started, but keep in mind that it is the least secure way.

Go to Datacenter > Permissions > API Tokens. Click on “Add” button and select the user `root@pam`. Type the API token name, uncheck the case "Privilege Separations", and get the secret (will be visible only one time).

You can now use the API token in the environment variables `PROXMOX_API_TOKEN_NAME` and `PROXMOX_API_TOKEN_VALUE`.

### Database configuration

PVMSS uses an embedded SQLite database to store all configuration. The database is automatically initialized on first startup and includes:

- Approved Proxmox nodes, storages, VMBRs, and ISO repositories
- VM resource limits (global and per-node)
- Tags and user pools
- Cloud-init templates and SFTP configuration
- VM profiles

All configuration is managed through the **Admin** section of the web UI, which provides:

- Full CRUD operations for all configuration items
- Audit trail of all changes
- Import/export functionality for backup/restore

The database file must be persisted on a volume to survive container restarts.

#### Tags

The tag `pvmss` is used by default for VMs created via PVMSS, it cannot and should not be removed. Only PVMSS tags created by the admin from this app can be used.

#### VM Profiles

VM profiles are pre-configured templates that simplify VM creation by providing common resource configurations. Users can select a profile when creating a VM, which automatically sets CPU, RAM, disk, and other parameters.

PVMSS provides built-in default profiles:

- **Web Server**: 1 vCPU, 2 GB RAM, 24 GB disk
- **Lightweight API**: 2 vCPU, 2 GB RAM, 24 GB disk
- **Light App Server**: 4 vCPU, 4 GB RAM, 32 GB disk
- **Medium App Server**: 4 vCPU, 6 GB RAM, 32 GB disk
- **Test Environment**: 2 vCPU, 4 GB RAM, 24 GB disk

Admins can manage custom profiles via the **Admin > Profiles** page, where they can:

- Create new profiles with custom resource specifications
- Edit existing profiles
- Enable or disable profiles
- Delete profiles
- Set optional node and storage overrides per profile

Each profile includes:

- `id`: Unique identifier
- `name`: Display name
- `description`: User-friendly description
- `sockets`, `cores`, `ram_gb`, `disk_gb`: Resource specifications
- `disk_bus`: Disk bus type (virtio, scsi, sata, ide)
- `node`, `storage`: Optional node/storage overrides (empty = auto-select)
- `icon`, `color`: Visual customization
- `enabled`: Whether the profile is visible to users

### Environment variables

You can rely on `.env` + `env_file` or inline `environment:` entries, but **not both**. The needed variables are listed below:

| Variable                                      | Description                                                                | Required                 | Default                |
| --------------------------------------------- | -------------------------------------------------------------------------- | ------------------------ | ---------------------- |
| `PVMSS_PORT`                                  | TCP port the HTTP server listens on (1–65535)                              | ✅                       | —                      |
| `PVMSS_DB_PATH`                               | Path to the SQLite database file (must be on a persistent volume)          | ✅                       | —                      |
| `SESSION_SECRET`                              | 32+ byte secret to encrypt sessions/cookies                                | ✅                       | —                      |
| `PVMSS_CLUSTER_SOURCE`                        | `proxmox` for a real cluster, `fake` for demo data (no default, on purpose) | ✅                       | —                      |
| `LOG_LEVEL`                                   | `debug`, `info`, `warn`, `error` — lowercase only                          | ✅                       | —                      |
| `LOG_FORMAT`                                  | `console` (human readable) or `json` (machine/SIEM)                        | ✅                       | —                      |
| `LOG_OUTPUT`                                  | `stdout`, `stderr`, or a writable file path                                | ✅                       | —                      |
| `PROXMOX_URL`                                 | Full API URL (`https://host:8006/api2/json`)                               | when source is `proxmox` | —                      |
| `PROXMOX_API_TOKEN_NAME`                      | Proxmox token name (`user@pve!token`)                                      | when source is `proxmox` | —                      |
| `PROXMOX_API_TOKEN_VALUE`                     | Token secret that matches the name above                                   | when source is `proxmox` | —                      |
| `ADMIN_PASSWORD_HASH`                         | Bcrypt hash for the local admin login; disabled when empty                 | ❌                       | —                      |
| `PVMSS_HOST`                                  | Address to bind (`0.0.0.0` for all interfaces)                             | ❌                       | `127.0.0.1`            |
| `PVMSS_WEB_DIR`                               | Directory holding the built SPA                                            | ❌                       | relative to the binary |
| `PVMSS_COOKIE_SECURE`                         | `Secure` flag on auth cookies (keep `true` in production)                  | ❌                       | `true`                 |
| `PVMSS_INVENTORY_REFRESH_INTERVAL`            | Background inventory refresh period                                        | ❌                       | `30s`                  |
| `PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL` | Minimum delay between user-triggered refreshes                             | ❌                       | `5s`                   |
| `PVMSS_INVENTORY_REFRESH_TIMEOUT`             | Timeout for a single inventory refresh                                     | ❌                       | `15s`                  |
| `PVMSS_MAX_LIST_PAGE_SIZE`                    | Upper bound on list endpoint page size                                     | ❌                       | `100`                  |
| `TZ`                                          | Container timezone                                                         | ❌                       | `UTC`                  |

The Docker image presets `PVMSS_DB_PATH=/data/pvmss.db`, `PVMSS_HOST=0.0.0.0`
and `PVMSS_WEB_DIR=/app/web/build`, so those three can be left unset in a
container deployment.

> Tip: `ADMIN_PASSWORD_HASH` can be generated locally with `htpasswd -bnBC 10 "admin" "StrongPassword" | cut -d: -f2`.

#### Logging configuration

PVMSS uses structured logging with the standard library's `log/slog`. All three
variables are required; `LOG_LEVEL` is matched case-sensitively and only accepts
lowercase values. Typical setups:

- Human-readable logs on stdout (development):

  ```bash
  LOG_LEVEL=debug
  LOG_OUTPUT=stdout
  LOG_FORMAT=console
  ```

- JSON logs on stdout for log aggregation / SIEM:

  ```bash
  LOG_LEVEL=info
  LOG_OUTPUT=stdout
  LOG_FORMAT=json
  ```

- JSON logs into a file inside the container:

  ```bash
  LOG_LEVEL=info
  LOG_OUTPUT=/app/pvmss.log
  LOG_FORMAT=json
  ```

`LOG_OUTPUT` takes `stdout`, `stderr`, or a writable file path — there is no
"both" mode. The JSON format is line-delimited and includes a `component` field
(main, cluster, inventory, ...), making it easy to consume with Fluent Bit,
Filebeat, or any SIEM.

## Deployment options

| Platform            | Notes                                                                                                                                                                                                                                      |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Docker / Podman** | Recommended for quick trials or single-node installs. Mount the database volume and expose port `50000`.                                                                                                                                  |
| **Docker Compose**  | Best experience: one service, reproducible environment, easy env var management.                                                                                                                                                           |
| **Kubernetes**      | Use [`pvmss-deployment.yaml`](pvmss-deployment.yaml) for namespace + secret + configmap + PVC + Deployment + Service. Apply with `kubectl apply -f pvmss-deployment.yaml`. Provide your own ingress/HTTPRoute (see `pvmss-httproute.yml`). |

## Quick start with Docker run

```bash
docker run -d \
  --name pvmss \
  --restart unless-stopped \
  -p 50000:50000 \
  -v $(pwd)/pvmss.db:/data/pvmss.db \
  -e ADMIN_PASSWORD_HASH='$2y$10$Ppg7Wl3sNYrmxZmWgcq4reOyznt7AeqMrQucaH4HY.dBrzavhPP1e' \
  -e LOG_LEVEL=info \
  -e LOG_OUTPUT=stdout \
  -e LOG_FORMAT=console \
  -e PROXMOX_API_TOKEN_NAME='tokenName@changeMe!value' \
  -e PROXMOX_API_TOKEN_VALUE="aaaaaaaa-0000-44aa-1111-aaaaaaaaaaa" \
  -e PROXMOX_URL=https://ip-or-name:8006/api2/json \
  -e PVMSS_CLUSTER_SOURCE=proxmox \
  -e PVMSS_PORT=50000 \
  -e PVMSS_DB_PATH="/data/pvmss.db" \
  -e SESSION_SECRET="$(openssl rand -hex 32)" \
  -e TZ=Europe/Paris \
  jhmmt/pvmss:latest
```

To write JSON logs to a file inside the container instead of stdout, override:

```bash
-e LOG_FORMAT=json \
-e LOG_OUTPUT=/app/pvmss.log \
-v $(pwd)/pvmss.log:/app/pvmss.log \
```

The application will be available at <http://localhost:50000>.

## Start with Docker compose

1. **Create `docker-compose.yml`:**

```yaml
services:
  pvmss:
    image: jhmmt/pvmss:latest
    container_name: pvmss
    restart: unless-stopped
    ports:
      - "50000:50000/tcp"
    environment:
      PROXMOX_API_TOKEN_NAME: "tokenName@changeMe!value"
      PROXMOX_API_TOKEN_VALUE: "aaaaaaaa-0000-44aa-1111-aaaaaaaaaaa"
      PROXMOX_URL: "https://ip-or-name:8006/api2/json"
      PVMSS_CLUSTER_SOURCE: "proxmox"
      ADMIN_PASSWORD_HASH: "$2y$10$Ppg7Wl3sNYrmxZmWgcq4reOyznt7AeqMrQucaH4HY.dBrzavhPP1e"
      LOG_LEVEL: "info"
      LOG_OUTPUT: "stdout"
      LOG_FORMAT: "console"
      SESSION_SECRET: "changeMeWithSomethingElseUniqueMinimum32Chars"
      PVMSS_PORT: "50000"
      PVMSS_DB_PATH: "/data/pvmss.db"
      TZ: "Europe/Paris"
    volumes:
      - ./pvmss.db:/data/pvmss.db
      # - ./pvmss.log:/app/pvmss.log # Uncomment to persist logs to a file inside the container
    deploy:
      resources:
        limits:
          cpus: "1"
          memory: 64M
```

To persist logs to a file inside the container, you can change the environment section to use JSON + file output, for example:

```yaml
LOG_FORMAT: "json"
LOG_OUTPUT: "/app/pvmss.log"
# Add this volume to the volumes section
- ./pvmss.log:/app/pvmss.log
```

1. **Start the stack:**

   ```bash
   docker compose up -d
   ```

2. Browse to **<http://localhost:50000>**.
3. Login with the admin credentials configured earlier, on the page "Login", click on "Administrator login".

## Start with Kubernetes

Use the file [`pvmss-deployment.yaml`](pvmss-deployment.yaml) to create namespace + secret + configmap + PVC + Deployment + Service.

Apply with `kubectl apply -f pvmss-deployment.yaml`. Provide your own ingress/HTTPRoute, an example is provided in `pvmss-httproute.yml` (Gateway API).

## Operations

- **Logs**: `docker logs -f pvmss` or `kubectl -n pvmss logs -f deploy/pvmss`. Switch `LOG_LEVEL=debug` for verbose traces. Use `LOG_FORMAT=json` with `LOG_OUTPUT=stdout` or a file path to emit JSON logs that can be shipped to a SIEM or log aggregator.
- **Health**: startup logs include cluster connectivity and inventory refresh status. The admin "Application Info" page shows runtime metrics, environment variables, and Proxmox cluster status.
- **Upgrades**: pull the desired image tag and restart the container. Configuration is stored in the SQLite database and persists automatically.
- **Static analysis (SonarQube)**: run `make sonar` to start a local SonarQube container, provision tokens, generate Go coverage, and scan the two projects — `pvmss-server` (Go) and `pvmss-web` (SvelteKit TS). Results are at `http://localhost:9000/projects`. Stop with `make sonar-down` and clean data with `make sonar-clean`.

## Limitations

- Security hardening is ongoing; no formal penetration test yet.
- App is not as dynamic as I'd like to. It is a work in progress.

### Next major features

- OpenID Connect / SSO integration.
- Migration of VMs between Proxmox nodes.

Feedback and contributions are welcome through issues or pull requests. Next versions and features will be documented here: <https://github.com/julienhmmt/pvmss/projects?query=is%3Aopen>.

## License

PVMSS by Julien HOMMET is licensed under **GNU AGPL v3**. See <https://www.gnu.org/licenses/agpl-3.0.en.html>.
