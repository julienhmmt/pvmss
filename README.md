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

PVMSS runs as a stateless web application (Go backend + HTML/CSS frontend) and relies on Proxmox APIs for every action. It is designed to be:

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

## Architecture at a glance

- **Backend**: Go 1.25+, RESTy client for Proxmox APIs, basic HTML templates.
- **Frontend**: Bulma-based, with custom CSS.
- **Authentication**: Proxmox API token for backend actions, user sessions for UI.

## Configuration

### Proxmox roles and permissions (required)

PVMSS relies on dedicated Proxmox roles and ACLs to work correctly (service account, admin accounts, per-user pools).

Before using PVMSS in production, you **must**:

- Create the `PVMSS_Service` role with the expected privileges.
- Create the `PVMSS_Service` user and its API token.
- Create a `PVMSS_Admin` user (human admin to manage PVMSS app and PVMSS users).
- Create the ACLs for the `PVMSS_Service` and `PVMSS_Admin` users.

In your Proxmox cluster, you can create the roles and ACLs using the `pveum` command-line tool. You can also create them using the Proxmox web interface. As *root*, create the roles and privileges:

```bash
# PVMSS_Service
pveum roleadd PVMSS_Service -privs "Sys.Audit VM.Audit VM.Allocate VM.PowerMgmt VM.Console VM.Config.CDROM VM.Config.CPU VM.Config.HWType VM.Config.Memory VM.Config.Disk VM.Config.Network VM.Config.Options VM.Config.Cloudinit Datastore.Audit Datastore.AllocateSpace Pool.Allocate Pool.Audit User.Modify Permissions.Modify Realm.AllocateUser SDN.Allocate SDN.Audit SDN.Use"

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

- `backend/docs/proxmox-permissions.en.md` (in this repository)
- The in-app admin page `/docs/proxmox-permissions` (once PVMSS is running and you are logged in as admin)

### Create an API token for the user root@pam

If you want to use PVMSS in development, you **must** create an API token for the user `root@pam`. It is the simple way to get started, but keep in mind that it is the least secure way.

Go to Datacenter > Permissions > API Tokens. Click on “Add” button and select the user `root@pam`. Type the API token name, uncheck the case "Privilege Separations", and get the secret (will be visible only one time).

You can now use the API token in the environment variables `PROXMOX_API_TOKEN_NAME` and `PROXMOX_API_TOKEN_VALUE`.

### The configuration file settings.json

`settings.json` file acts as the source of truth for every option available to users:

```json
{
  "tags": ["pvmss"],
  "isos": [],
  "vmbrs": [],
  "cloudinit_sftp": {
    "enabled": true,
    "host": "your-proxmox-ip-or-hostname",
    "port": 22,
    "username": "pvmss-snippets",
    "privateKeyPath": "/app/pvmss_snippets_ed25519",
    "snippetBaseDir": "/snippets"
  },
  "enabled_storages": [],
  "max_network_cards": 1,
  "max_disk_per_vm": 1,
  "max_vm_per_user": 3,
  "limits": {
    "nodes": {},
    "vm": {
      "sockets": {"min": 1, "max": 1},
      "cores":   {"min": 1, "max": 2},
      "ram":     {"min": 1, "max": 4},
      "disk":    {"min": 6, "max": 12}
    }
  }
}
```

All keys are mandatory. Next versions of PVMSS can add new keys to this file, so it is recommended to keep it up to date.

#### Tags

The tag `pvmss` is used by default for VMs created via PVMSS, it cannot and should not be removed. Only PVMSS tags created by the admin from this app can be used.

### Environment variables

You can rely on `.env` + `env_file` or inline `environment:` entries, but **not both**. The needed variables are listed below:

| Variable | Description | Required | Default |
| --- | --- | :---: | --- |
| `ADMIN_PASSWORD_HASH` | Bcrypt hash for the admin UI login | ✅ | — |
| `SESSION_SECRET` | 32+ byte secret to encrypt sessions/cookies | ✅ | — |
| `PROXMOX_API_TOKEN_NAME` | Proxmox token name (`user@pve!token`) used by the backend | ✅ | — |
| `PROXMOX_API_TOKEN_VALUE` | Token secret that matches the name above | ✅ | — |
| `PROXMOX_URL` | Full API URL (`https://host:8006/api2/json`) | ✅ | — |
| `PROXMOX_VERIFY_SSL` | `true` for trusted certs, `false` for self-signed labs | ❌ | `false` |
| `PVMSS_ENV` | `production/prod` (secure cookies + HSTS) or `development/dev/developpement` | ❌ | `production` |
| `PVMSS_OFFLINE` | `true` disables all Proxmox calls (demo mode) | ❌ | `false` |
| `PVMSS_SETTINGS_PATH` | Inside-container path to `settings.json` | ❌ | `/app/settings.json` |
| `LOG_LEVEL` | Log verbosity (`debug`, `info`, `warn`, `error`) | ❌ | `INFO` |
| `LOG_OUTPUT` | Log destination: `stdout`, `file`, or `both` | ❌ | `stdout` |
| `LOG_FILE_PATH` | File path when `LOG_OUTPUT` is `file` or `both` | ❌ | — |
| `LOG_FORMAT` | `console` (human readable) or `json` (machine/SIEM) | ❌ | `console` |
| `TZ` | Container timezone | ❌ | `UTC` |

> Tip: `ADMIN_PASSWORD_HASH` can be generated locally with `htpasswd -bnBC 10 "admin" "StrongPassword" | cut -d: -f2`.

#### Logging configuration

PVMSS uses structured logging with [zerolog](https://github.com/rs/zerolog). Typical setups:

- Human-readable logs on stdout (development):

  ```bash
  LOG_LEVEL=DEBUG
  LOG_OUTPUT=stdout
  LOG_FORMAT=console
  ```

- JSON logs on stdout for log aggregation / SIEM:

  ```bash
  LOG_LEVEL=INFO
  LOG_OUTPUT=stdout
  LOG_FORMAT=json
  ```

- JSON logs on stdout **and** in a file inside the container:

  ```bash
  LOG_LEVEL=INFO
  LOG_OUTPUT=both
  LOG_FORMAT=json
  LOG_FILE_PATH=/app/pvmss.log
  ```

The JSON format is line-delimited and includes fields such as `component`, `operation`, `reason`, and `event_category` (for auth, VM, admin, security, console, Proxmox events), making it easy to consume with Fluent Bit, Filebeat, or any SIEM.

## Deployment options

| Platform | Notes |
| --- | --- |
| **Docker / Podman** | Recommended for quick trials or single-node installs. Mount `settings.json` and expose port `50000`. |
| **Docker Compose** | Best experience: one service, reproducible environment, easy env var management. |
| **Kubernetes** | Use [`pvmss-deployment.yaml`](pvmss-deployment.yaml) for namespace + secret + configmap + PVC + Deployment + Service. Apply with `kubectl apply -f pvmss-deployment.yaml`. Provide your own ingress/HTTPRoute (see `pvmss-httproute.yml`). |

## Quick start with Docker run

```bash
docker run -d \
  --name pvmss \
  --restart unless-stopped \
  -p 50000:50000 \
  -v $(pwd)/settings.json:/app/settings.json \
  -e ADMIN_PASSWORD_HASH='$2y$10$Ppg7Wl3sNYrmxZmWgcq4reOyznt7AeqMrQucaH4HY.dBrzavhPP1e' \
  -e LOG_LEVEL=INFO \
  -e LOG_OUTPUT=stdout \
  -e LOG_FORMAT=console \
  -e PROXMOX_API_TOKEN_NAME='tokenName@changeMe!value' \
  -e PROXMOX_API_TOKEN_VALUE="aaaaaaaa-0000-44aa-1111-aaaaaaaaaaa" \
  -e PROXMOX_URL=https://ip-or-name:8006/api2/json \
  -e PROXMOX_VERIFY_SSL=false \
  -e PVMSS_ENV="prod" \
  -e PVMSS_OFFLINE="false" \
  -e PVMSS_SETTINGS_PATH="/app/settings.json" \
  -e SESSION_SECRET="$(openssl rand -hex 32)" \
  -e TZ=Europe/Paris \
  jhmmt/pvmss:0.2.1
```

To also write JSON logs to a file inside the container (and keep stdout), override:

```bash
-e LOG_OUTPUT=both \
-e LOG_FORMAT=json \
-e LOG_FILE_PATH=/app/pvmss.log \
-v $(pwd)/pvmss.log:/app/pvmss.log \
```

The application will be available at <http://localhost:50000>.

## Start with Docker compose

1. **Create `settings.json`** (see [the configuration file](#the-configuration-file-settingsjson)) and secure it:

   ```bash
   chmod 600 settings.json
   ```

2. **Create `docker-compose.yml`:**

```yaml
services:
  pvmss:
    image: jhmmt/pvmss:0.2.1
    container_name: pvmss
    restart: unless-stopped
    ports:
      - "50000:50000/tcp"
    environment:
      PROXMOX_API_TOKEN_NAME: "tokenName@changeMe!value"
      PROXMOX_API_TOKEN_VALUE: "aaaaaaaa-0000-44aa-1111-aaaaaaaaaaa"
      PROXMOX_URL: "https://ip-or-name:8006/api2/json"
      PROXMOX_VERIFY_SSL: "false"
      ADMIN_PASSWORD_HASH: "$2y$10$Ppg7Wl3sNYrmxZmWgcq4reOyznt7AeqMrQucaH4HY.dBrzavhPP1e"
      LOG_LEVEL: "INFO"
      LOG_OUTPUT: "stdout"
      LOG_FORMAT: "console"
      SESSION_SECRET: "changeMeWithSomethingElseUnique"
      PVMSS_ENV: "production"
      PVMSS_OFFLINE: "false"
      PVMSS_SETTINGS_PATH: "/app/settings.json"
      TZ: "Europe/Paris"
    volumes:
      - ./settings.json:/app/settings.json
      # - ./pvmss.log:/app/pvmss.log # Uncomment to persist logs to a file inside the container
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 64M
```

To persist logs to a file inside the container, you can change the environment section to use JSON + file output, for example:

```yaml
LOG_OUTPUT: "both"
LOG_FORMAT: "json"
LOG_FILE_PATH: "/app/pvmss.log"
# Add this volume to the volumes section
- ./pvmss.log:/app/pvmss.log
```

3. **Start the stack:**

   ```bash
   docker compose up -d
   ```

4. Browse to **<http://localhost:50000>**.
5. Login with the admin credentials configured earlier, on the page "Login", click on "Administrator login".

## Start with Kubernetes

Use the file [`pvmss-deployment.yaml`](pvmss-deployment.yaml) to create namespace + secret + configmap + PVC + Deployment + Service.

Apply with `kubectl apply -f pvmss-deployment.yaml`. Provide your own ingress/HTTPRoute, an example is provided in `pvmss-httproute.yml` (Gateway API).

## Operations

- **Logs**: `docker logs -f pvmss` or `kubectl -n pvmss logs -f deploy/pvmss`. Switch `LOG_LEVEL=DEBUG` for verbose traces. Use `LOG_FORMAT=json` and `LOG_OUTPUT=stdout` or `file` to emit JSON logs that can be shipped to a SIEM or log aggregator.
- **Health**: startup logs include Proxmox connectivity, offline-mode status, and runtime metrics. The admin "Application Info" page shows runtime metrics, environment variables, and Proxmox cluster status.
- **Upgrades**: pull the desired image tag, update the file `settings.json` if new fields appear, and restart the container.

## Limitations

- Security hardening is ongoing; no formal penetration test yet.
- App is not as dynamic as I'd like to. It is a work in progress.

### Next major features

- OpenID Connect / SSO integration.
- Migration of VMs between Proxmox nodes.

Feedback and contributions are welcome through issues or pull requests. Next versions and features will be documented here: <https://github.com/julienhmmt/pvmss/projects?query=is%3Aopen>.

## License

PVMSS by Julien HOMMET is licensed under **Creative Commons Attribution-NonCommercial-NoDerivatives 4.0 International**. See <https://creativecommons.org/licenses/by-nc-nd/4.0/>.
