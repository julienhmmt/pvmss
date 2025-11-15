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
   - [settings.json](#settingsjson)
   - [Environment variables](#environment-variables)
5. [Deployment options](#deployment-options)
6. [Quick start with Docker run](#quick-start-with-docker-run)
7. [Start with Docker compose](#start-with-docker-compose)
8. [Start with Kubernetes](#start-with-kubernetes)
9. [Operations](#operations)
10. [Limitations & roadmap](#limitations--roadmap)
11. [License](#license)

---

## Overview

PVMSS runs as a stateless web application (Go backend + HTML/CSS frontend) and relies on Proxmox APIs for every action. It is designed to be:

- **Secure by default**: per-user sessions.
- **Operations-friendly**: ready-to-use container image, configurable resource limits, cluster-aware storage selection.
- **User-centric**: clear and guided VM forms, built-in documentation.

> ⚠️ The project is still under active development. Review the [Limitations & roadmap](#limitations--roadmap) section before deploying to production.

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

- **Backend**: Go 1.25+, RESTy client for Proxmox APIs, CSRF-protected HTML templates.
- **Frontend**: Bulma-based, with custom CSS.
- **Authentication**: Proxmox API token for backend actions, user sessions for UI.

## Configuration

### Create an API token on Proxmox

In order to be able to use PVMSS, you have to create a user inside your Proxmox cluster and its API token.

On Proxmox, go to Datacenter > Permissions > Users. Click on “Add” button, set its username, select the realm `Proxmox VE Authentication` and type a strong password.

Next, go to Datacenter > Permissions > API Tokens. Click on “Add” button and select the previous created user. Type the API token name, uncheck the case "Privilege Separations", and get the secret (will be visible only one time).

Finally, go to Datacenter > Permissions, click on the “Add” button and select “User Permissions”. Select the path `/` and select the previous user created. Choose the role `PVEAdmin` and click the case “Propagate”. Save it and you are set. Soon, we will restrict rights and paths to be more secure.

### settings.json

`settings.json` file acts as the source of truth for every option available to users:

```json
{
  "tags": ["pvmss"],
  "isos": [],
  "vmbrs": [],
  "enabled_storages": [],
  "max_network_cards": 1,
  "max_disk_per_vm": 1,
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
| `LOG_LEVEL` | `INFO` or `DEBUG` | ❌ | `INFO` |
| `TZ` | Container timezone | ❌ | `UTC` |

> Tip: `ADMIN_PASSWORD_HASH` can be generated locally with `htpasswd -bnBC 10 "admin" "StrongPassword" | cut -d: -f2`.

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
  -e PROXMOX_API_TOKEN_NAME='tokenName@changeMe!value' \
  -e PROXMOX_API_TOKEN_VALUE="aaaaaaaa-0000-44aa-1111-aaaaaaaaaaa" \
  -e PROXMOX_URL=https://ip-or-name:8006/api2/json \
  -e PROXMOX_VERIFY_SSL=false \
  -e PVMSS_ENV="prod" \
  -e PVMSS_OFFLINE="false" \
  -e PVMSS_SETTINGS_PATH="/app/settings.json" \
  -e SESSION_SECRET="$(openssl rand -hex 32)" \
  -e TZ=Europe/Paris \
  jhmmt/pvmss:0.2.0
```

The application will be available at <http://localhost:50000>.

## Start with Docker compose

1. **Create `settings.json`** (see [settings.json](#settingsjson)) and secure it:

   ```bash
   chmod 600 settings.json
   ```

2. **Create `docker-compose.yml`:**

   ```yaml
   services:
     pvmss:
       image: jhmmt/pvmss:0.2.0
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
         SESSION_SECRET: "changeMeWithSomethingElseUnique"
         PVMSS_ENV: "production"
         PVMSS_OFFLINE: "false"
         PVMSS_SETTINGS_PATH: "/app/settings.json"
         TZ: "Europe/Paris"
       volumes:
         - ./settings.json:/app/settings.json
       deploy:
         resources:
           limits:
             cpus: '1'
             memory: 64M
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

- **Logs**: `docker logs -f pvmss` or `kubectl -n pvmss logs -f deploy/pvmss`. Switch `LOG_LEVEL=DEBUG` for verbose traces.
- **Health**: startup logs include Proxmox connectivity, offline-mode status, and runtime metrics. The admin "Application Info" page shows runtime metrics, environment variables, and Proxmox cluster status.
- **Upgrades**: pull the desired image tag, update the file `settings.json` if new fields appear, and restart the container.

## Limitations & roadmap

- Security hardening is ongoing; no formal penetration test yet.
- Cloud-Init support is planned but not implemented.
- OpenID Connect / SSO integration is planned but not implemented.
- Advanced logging outputs (files, syslog shipping) are planned but not implemented.

Feedback and contributions are welcome through issues or pull requests. Next versions and features will be documented here: <https://github.com/julienhmmt/pvmss/projects?query=is%3Aopen>.

## License

PVMSS by Julien HOMMET is licensed under **Creative Commons Attribution-NonCommercial-NoDerivatives 4.0 International**. See <https://creativecommons.org/licenses/by-nc-nd/4.0/>.
