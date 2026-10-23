# Proxmox VM Self-Service (PVMSS)

[![Lint](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml/badge.svg?branch=main&event=push)](https://github.com/julienhmmt/pvmss/actions/workflows/lint.yml) [![CI](https://github.com/julienhmmt/pvmss/actions/workflows/v0.4.yml/badge.svg?branch=main&event=push)](https://github.com/julienhmmt/pvmss/actions/workflows/v0.4.yml)

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
9. [Coming from v0.3](#coming-from-v03)
10. [Operations](#operations)
11. [Limitations](#limitations)
12. [License](#license)

---

## Overview

PVMSS runs as a stateless web application (Go REST API + SvelteKit SPA) and relies on Proxmox APIs for every action. It is designed to be:

- **Secure by default**: per-user sessions.
- **Operations-friendly**: ready-to-use container image, configurable resource limits, cluster-aware storage selection.
- **User-centric**: clear and guided VM forms, built-in documentation.

> ⚠️ The project is still under active development. Review the [Limitations](#limitations) section before deploying to production.

## Feature highlights

The complete, route-by-route inventory lives in [docs/FEATURES.md](docs/FEATURES.md).

### End users

- Sign in with Proxmox credentials on the cluster of your choice.
- **My VMs**: list of the sign-in cluster's VMs, search/filter/sort mirrored in the URL, live status, bulk power actions with per-VM results, console button in every row.
- **Create a VM** wizard (Simple / Detailed) from three sources: an approved **ISO**, a Proxmox **template** (linked or full clone), or a **cloud image** (`import-from` + cloud-init) (cloud images are temporarily disabled: they do not work reliably yet). Hardware profiles or custom CPU/RAM/disk, auto-placement with capacity scoring, multi-NIC (bridge + model), UEFI (no Secure Boot) / TPM 2.0, curated tags, boot from CD-ROM.
- **Sign-in on VMs**: PVMSS adds no SSH key or password (cloud-init is paused). A template clone keeps the sign-in built into its Proxmox template; an ISO install gets the account the user creates during installation. See [VM templates carry the sign-in](#vm-templates-carry-the-sign-in).
- **Operate a VM**: 7 power actions, rename, Markdown description, delete; disks (add / grow / detach); NIC edit (bridge, model, VLAN, rate); CPU/RAM/tags/CD-ROM; snapshots (create / rollback / delete, RAM optional); metrics history (hour/day/week) and live stream; per-VM activity log.
- **Consoles**: noVNC and serial (xterm.js), both proxied through PVMSS with single-use tickets; power actions from the console page.
- **Cloud-init** (paused for now): native form (user, password via guest agent, SSH keys, IP/DNS), "add key now" injection, administrator **cloud-init templates** picked at creation or switched later on the VM (users never write YAML).
- Nodes page with live capacity, in-app documentation, EN + FR, keyboard-first and WCAG 2.1 AA target.

### Administrators

- **Sign-in**: Proxmox users granted `Permissions.Modify` on `/` (the `PVEAdmin` or `Administrator` role, `pve` or `pam` realm), plus PVMSS local administrators that work even when no cluster is reachable. The bootstrap local administrator is the only one allowed to create and manage further local accounts.
- **Clusters**: connect several Proxmox environments, test connectivity, per-cluster snippet storage for cloud-init templates (paused).
- **Catalog**: approve nodes, storages, ISOs, cloud images (temporarily disabled), VM templates, bridges; CRUD for hardware profiles, tags, cloud-init templates (published to every node, per-node status, "publish all"); stale approvals reconciled against live discovery.
- **Pools**: create a self-service user = Proxmox user + pool + ACL in one step; cascade delete.
- **Migration**: move a PVMSS-managed VM to another approved, online node of the same cluster from the node page (preflight, explicit confirmation, progress in the task tray).
- **Policy**: per-cluster gabarit (sockets, cores, memory, disk per VM, NICs, snapshots, isolation VLAN) and quota (VMs per user); per-node capacity caps with live usage.
- **System**: dashboard, app info, audit log with retention + prune preview, SQLite export and two-phase import, in-app documentation CMS (EN/FR, audience-scoped).

## Architecture at a glance

- **Server** (`server/`): Go 1.27, stdlib `net/http` routing, SQLite via `modernc.org/sqlite` (CGO-free). Serves `/api/v1/*` and the SPA.
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
pveum roleadd PVMSS_Service -privs "Sys.Audit VM.Audit VM.Allocate VM.PowerMgmt VM.Console VM.Config.CDROM VM.Config.CPU VM.Config.HWType VM.Config.Memory VM.Config.Disk VM.Config.Network VM.Config.Options VM.Config.Cloudinit VM.Snapshot VM.Snapshot.Rollback VM.Migrate VM.GuestAgent.Audit VM.GuestAgent.Unrestricted Datastore.Audit Datastore.AllocateSpace Datastore.AllocateTemplate Pool.Allocate Pool.Audit User.Modify Permissions.Modify Realm.AllocateUser SDN.Allocate SDN.Audit SDN.Use"

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

### Administrator sign-in

PVMSS accepts two kinds of administrator, both landing on the same admin surface:

- **Proxmox administrator** - sign in on `/login` with a Proxmox account whose ACL grants `Permissions.Modify` on `/` (the built-in `PVEAdmin` or `Administrator` role, or a custom role carrying that privilege). Choose the `pve` (Proxmox VE) or `pam` (Linux PAM) realm; typing an explicit `user@realm` overrides the picker. The admin flag is read at login and kept for the session (at most 24 hours), so a permission granted later needs a fresh sign-in. OIDC realms are not supported yet.
- **Local administrator** - independent of Proxmox; sign in from the same page with the "Local administrator" toggle. `ADMIN_PASSWORD_HASH` seeds the bootstrap account (`admin`), the only owner allowed to create further accounts under **Admin > Settings > Accounts**. Those DB accounts are capped by `PVMSS_MAX_LOCAL_ADMINS`, have 8+ character passwords (72 bytes maximum, the bcrypt limit), and can be reset, disabled and deleted; each of those actions revokes that account's sessions. Five consecutive failed sign-ins lock an account for 15 minutes.

### Create an API token for the user root@pam

If you want to use PVMSS in development, you **must** create an API token for the user `root@pam`. It is the simple way to get started, but keep in mind that it is the least secure way.

Go to Datacenter > Permissions > API Tokens. Click on “Add” button and select the user `root@pam`. Type the API token name, uncheck the case "Privilege Separations", and get the secret (will be visible only one time).

You can now use the API token in the environment variables `PROXMOX_API_TOKEN_NAME` and `PROXMOX_API_TOKEN_VALUE`.

### Database configuration

PVMSS uses an embedded SQLite database to store all configuration. The database is automatically initialized on first startup and includes:

- Approved Proxmox nodes, storages, VMBRs, and ISO repositories
- VM resource limits (global and per-node)
- Tags and user pools
- Cloud-init templates and their publications on the nodes
- VM profiles, cluster connections, audit log

All configuration is managed through the **Admin** section of the web UI, which provides:

- Full CRUD operations for all configuration items
- Audit trail of all changes
- Import/export functionality for backup/restore

Persist the whole `/data` directory (a volume), not the single `pvmss.db` file: SQLite runs in WAL mode and keeps `-wal`/`-shm` files next to it. For a backup, use **Admin > Settings > Export Database**.

#### Tags

The tag `pvmss` is used by default for VMs created via PVMSS, it cannot and should not be removed. Only PVMSS tags created by the admin from this app can be used.

#### VM profiles

VM profiles are hardware shapes (sockets, cores, memory, disk, disk bus) that
administrators curate in **Admin > Profiles**. Users pick a profile in the
create wizard instead of typing CPU/RAM/disk values. PVMSS ships no default
profile: create the ones your users need. Each profile has a label, the
resource values above, and an enabled flag (only enabled profiles are offered).
Per-cluster limits (gabarit) in **Admin > Policy** still cap every value.

### VM templates carry the sign-in

PVMSS adds no SSH key and no password to the VMs it creates while cloud-init
is paused. A clone keeps exactly what its Proxmox template carries, so build
every template you approve with the sign-in users need:

1. In the template's **Cloud-Init** tab in Proxmox, set the user and an SSH
   public key or a password (or bake an account into the image).
2. Install and enable `qemu-guest-agent`, so VMs report their address and the
   **Connect** tab can show the SSH command.
3. Convert it (`qm template <vmid>`) and approve it in **Admin > VM templates**.

Every VM cloned from one template shares that sign-in: hand it to the users
of that template, or keep one template per team. ISO installs are unaffected:
users create their own account during installation, from the browser console.

### Cloud-init templates (optional)

> **Paused since 2026-10-20**: cloud-init is not reliable or viable yet and too complex to set up. Hidden in the UI and refused by the API; see
> [docs/cloud-init.md](docs/cloud-init.md).

Administrators write cloud-init templates in **Admin › Cloud-init**; users
pick one when they create a VM (or switch later on the VM's cloud-init tab).
Users never write YAML. The Proxmox REST API cannot write `snippets` files,
so **PVMSS never writes on the nodes**: for each template it shows a command
that you paste, as root, on the nodes that must offer it. No key, no SSH.

1. Enable the Snippets content type on a storage (one node, once; or GUI:
   Datacenter > Storage):

   ```sh
   STORAGE=local
   CUR=$(pvesh get /storage/$STORAGE --output-format json | perl -MJSON -0ne 'print decode_json($_)->{content}')
   case ",$CUR," in *,snippets,*) ;; *) pvesm set "$STORAGE" --content "$CUR,snippets" ;; esac
   ```

2. **Infrastructure › Clusters › Edit**: select that snippet storage.
3. **Admin › Cloud-init**: write the template, copy its **Command to paste**,
   run it on the chosen nodes, click **Verify**.

A template is offered only on the nodes that have its file. Each file is
immutable (`pvmss-tpl-<id>-<hash>.yml`, the PVMSS baseline merged in):
editing a template yields a new file and a new command, existing VMs keep
theirs. Full procedure and troubleshooting: [docs/cloud-init.md](docs/cloud-init.md).

### Environment variables

You can rely on `.env` + `env_file` or inline `environment:` entries, but **not both**. The needed variables are listed below:

| Variable                                      | Description                                                                                                  | Required                 | Default                |
| --------------------------------------------- | ------------------------------------------------------------------------------------------------------------ | ------------------------ | ---------------------- |
| `PVMSS_PORT`                                  | TCP port the HTTP server listens on (1–65535)                                                                | ✅                       | -                      |
| `PVMSS_DB_PATH`                               | Path to the SQLite database file (must be on a persistent volume)                                            | ✅                       | -                      |
| `SESSION_SECRET`                              | 32+ byte secret to encrypt sessions/cookies                                                                  | ✅                       | -                      |
| `PVMSS_CLUSTER_SOURCE`                        | `proxmox` for a real cluster, `fake` for demo data (no default, on purpose)                                  | ✅                       | -                      |
| `LOG_LEVEL`                                   | `debug`, `info`, `warn`, `error` - lowercase only                                                            | ✅                       | -                      |
| `LOG_FORMAT`                                  | `console` (human readable) or `json` (machine/SIEM)                                                          | ✅                       | -                      |
| `LOG_OUTPUT`                                  | `stdout`, `stderr`, or a writable file path                                                                  | ✅                       | -                      |
| `PROXMOX_URL`                                 | Full API URL (`https://host:8006/api2/json`)                                                                 | when source is `proxmox` | -                      |
| `PROXMOX_API_TOKEN_NAME`                      | Proxmox token name (`user@pve!token`)                                                                        | when source is `proxmox` | -                      |
| `PROXMOX_API_TOKEN_VALUE`                     | Token secret that matches the name above                                                                     | when source is `proxmox` | -                      |
| `ADMIN_PASSWORD_HASH`                         | Bcrypt hash of the bootstrap local administrator (`admin`), the only account allowed to manage other local accounts; disabled when empty | ❌                       | -                      |
| `PVMSS_HOST`                                  | Address to bind (`0.0.0.0` for all interfaces)                                                               | ❌                       | `127.0.0.1`            |
| `PVMSS_WEB_DIR`                               | Directory holding the built SPA                                                                              | ❌                       | relative to the binary |
| `PVMSS_COOKIE_SECURE`                         | `Secure` flag on auth cookies (keep `true` in production)                                                    | ❌                       | `true`                 |
| `PVMSS_INVENTORY_REFRESH_INTERVAL`            | Background inventory refresh period                                                                          | ❌                       | `30s`                  |
| `PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL` | Minimum delay between user-triggered refreshes                                                               | ❌                       | `5s`                   |
| `PVMSS_INVENTORY_REFRESH_TIMEOUT`             | Timeout for a single inventory refresh                                                                       | ❌                       | `15s`                  |
| `PVMSS_MAX_LIST_PAGE_SIZE`                    | Upper bound on list endpoint page size                                                                       | ❌                       | `100`                  |
| `PVMSS_MAX_LOCAL_ADMINS`                      | Maximum number of DB local administrator accounts (1-50); the bootstrap administrator is not counted          | ❌                       | `5`                    |
| `PVMSS_TRUSTED_PROXY_HOPS`                    | Number of reverse proxies in front of PVMSS (for client IP / rate limits); set it when behind one (Helm sets `1`)           | ❌                       | `0`                    |
| `PVMSS_RATE_LIMIT_MAX`                        | Raises every built-in rate-limit ceiling (per-IP auth, per-user writes); `0` keeps the defaults              | ❌                       | `0`                    |
| `PVMSS_METRICS_PORT`                          | Port of the `GET /metrics` listener on `PVMSS_HOST` (1-65535, different from `PVMSS_PORT`); empty disables it | ❌                       | empty                  |
| `TZ`                                          | Container timezone                                                                                           | ❌                       | `UTC`                  |

The Docker image presets `PVMSS_DB_PATH=/data/pvmss.db`, `PVMSS_HOST=0.0.0.0`
and `PVMSS_WEB_DIR=/app/web/build`, so those three can be left unset in a
container deployment.

> Tip: generate `ADMIN_PASSWORD_HASH` with `htpasswd -bnBC 10 "" "StrongPassword" | tr -d ':\n'`. In a Compose file or a `.env` used by Compose, escape every `$` as `$$`.

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

`LOG_OUTPUT` takes `stdout`, `stderr`, or a writable file path - there is no
"both" mode. The JSON format is line-delimited and includes a `component` field
(main, cluster, inventory, ...), making it easy to consume with Fluent Bit,
Filebeat, or any SIEM.

## Deployment options

| Platform            | Notes                                                                                                    |
| ------------------- | -------------------------------------------------------------------------------------------------------- |
| **Docker / Podman** | Recommended for quick trials or single-node installs. Mount the database volume and expose port `50000`. |
| **Docker Compose**  | Best experience: one service, reproducible environment, easy env var management.                         |
| **Kubernetes**      | Use the Helm chart in [`helm/`](helm/): `helm install pvmss ./helm -f my-values.yaml`.                   |

## Quick start with Docker run

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
  -e PVMSS_PORT=50000 \
  -e PVMSS_DB_PATH="/data/pvmss.db" \
  -e SESSION_SECRET="$(openssl rand -hex 32)" \
  -e TZ=Europe/Paris \
  jhmmt/pvmss:0.4
```

To write JSON logs to a file inside the container instead of stdout, override:

```bash
-e LOG_FORMAT=json \
-e LOG_OUTPUT=/app/pvmss.log \
-v $(pwd)/pvmss.log:/app/pvmss.log \
```

The application will be available at <http://localhost:50000>. Over plain HTTP on anything other than `localhost`, add `-e PVMSS_COOKIE_SECURE=false` (keep the default behind HTTPS).

## Start with Docker compose

1. **Create `docker-compose.yml`:**

```yaml
services:
  pvmss:
    image: jhmmt/pvmss:0.4
    container_name: pvmss
    restart: unless-stopped
    ports:
      - "50000:50000/tcp"
    environment:
      PROXMOX_API_TOKEN_NAME: "tokenName@changeMe!value"
      PROXMOX_API_TOKEN_VALUE: "aaaaaaaa-0000-44aa-1111-aaaaaaaaaaa"
      PROXMOX_URL: "https://ip-or-name:8006/api2/json"
      PVMSS_CLUSTER_SOURCE: "proxmox"
      # Every "$" of the bcrypt hash is doubled for Compose
      ADMIN_PASSWORD_HASH: "$$2y$$10$$Ppg7Wl3sNYrmxZmWgcq4reOyznt7AeqMrQucaH4HY.dBrzavhPP1e"
      # Plain HTTP with no TLS in front: drop the Secure cookie flag (keep it on behind HTTPS)
      PVMSS_COOKIE_SECURE: "false"
      LOG_LEVEL: "info"
      LOG_OUTPUT: "stdout"
      LOG_FORMAT: "console"
      SESSION_SECRET: "changeMeWithSomethingElseUniqueMinimum32Chars"
      PVMSS_PORT: "50000"
      PVMSS_DB_PATH: "/data/pvmss.db"
      TZ: "Europe/Paris"
    volumes:
      - pvmss_data:/data
      # - ./pvmss.log:/app/pvmss.log # Uncomment to persist logs to a file inside the container
    deploy:
      resources:
        limits:
          cpus: "1"
          memory: 128M

volumes:
  pvmss_data: {}
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

A Helm chart lives in [`helm/`](helm/): `helm install pvmss ./helm -f my-values.yaml`.
`values.yaml` documents every variable.

## Coming from v0.3

There is no migration from v0.3 to v0.4. v0.4 is a rewrite (Go API + SvelteKit) and imports nothing from v0.3: deploy it as a new installation, then configure it again from the Admin UI. PVMSS never deletes VMs, so the VMs themselves stay in Proxmox.

- **Data**: v0.4 does not read `settings.json` (v0.3.0). A SQLite database written by a later v0.3 build is refused at startup (`incompatible database schema`). Point `PVMSS_DB_PATH` at a new file on a persistent `/data` volume.
- **New required variables**: `PVMSS_PORT`, `PVMSS_CLUSTER_SOURCE` (`proxmox` for a real cluster), `LOG_LEVEL`, `LOG_FORMAT`, `LOG_OUTPUT`.
- **Changed variables**:
  - `LOG_LEVEL` is lowercase only: `INFO` stops startup, use `info`.
  - `LOG_OUTPUT` takes `stdout`, `stderr` or a file path; `file` / `both` and `LOG_FILE_PATH` are gone.
  - `SESSION_SECRET` (32+ bytes) is refused if it is still the example value.
  - `ADMIN_PASSWORD_HASH` is now optional; it creates the bootstrap administrator `admin`.
  - `PROXMOX_URL` and `PROXMOX_API_TOKEN_*` only seed the first cluster on the first boot; afterwards **Admin › Clusters** owns them.
- **Removed variables** (no longer read): `JWT_SECRET`, `PVMSS_SETTINGS_PATH`, `CSRF_TOKEN_TTL`, `PVMSS_OFFLINE`, `PVMSS_ENV`, `LOG_FILE_PATH`, `PROXMOX_VERIFY_SSL` (now a per-cluster TLS option in **Admin › Clusters**), `PVMSS_SSH_KEY_FILE`, `PVMSS_SSH_USER`, `PVMSS_SSH_PORT` (startup warns about the SSH ones).
- **To configure again**: clusters, approved nodes, storages, ISOs, bridges and templates, hardware profiles, limits and quotas, tags, user pools and documentation pages. Check the Proxmox roles against [Proxmox roles and permissions](#proxmox-roles-and-permissions-required).

## Operations

- **Logs**: `docker logs -f pvmss` or `kubectl -n pvmss logs -f deploy/pvmss`. Switch `LOG_LEVEL=debug` for verbose traces. Use `LOG_FORMAT=json` with `LOG_OUTPUT=stdout` or a file path to emit JSON logs that can be shipped to a SIEM or log aggregator.
- **Health**: startup logs include cluster connectivity and inventory refresh status. The admin "Application Info" page shows runtime metrics, environment variables, and Proxmox cluster status.
- **Upgrades** (within v0.4.x): pull the desired image tag and restart the container. Configuration is stored in the SQLite database and persists automatically.
- **Built-in documentation**: the in-app pages (`/docs`) are seeded once into the database and never overwritten, so admin edits survive upgrades. To pick up a newer built-in text after an upgrade, delete the page in **Admin › Documentation** and restart, or paste the new content from `server/internal/docs/seed/`.
- **Static analysis (SonarQube)**: run `make sonar` to start a local SonarQube container, provision tokens, generate Go coverage, and scan the two projects - `pvmss-server` (Go) and `pvmss-web` (SvelteKit TS). Results are at `http://localhost:9000/projects`. Stop with `make sonar-down` and clean data with `make sonar-clean`.

## Limitations

- Security hardening is ongoing; no formal penetration test yet.
- No HA-managed VMs, no cross-cluster migration, SDN and firewall rules stay in Proxmox.
- Proxmox OIDC realms cannot sign in to PVMSS: only the `pve` and `pam` realms are supported (future work). Local PVMSS administrators remain available.

Feedback and contributions are welcome through issues or pull requests. Next versions and features will be documented here: <https://github.com/julienhmmt/pvmss/projects?query=is%3Aopen>.

## License

PVMSS by Julien HOMMET is licensed under **GNU AGPL v3**. See <https://www.gnu.org/licenses/agpl-3.0.en.html>.
