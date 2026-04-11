# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Is

**PVMSS** (Proxmox VM Self-Service) is a lightweight web portal letting users manage Proxmox VMs without direct Proxmox UI access. Stack: Go 1.25 backend + Vue 3 SPA (no build step), deployed via Docker/Kubernetes/Helm.

## Commands

```bash
# Development
make dev              # Build + start application
make qualif           # Full QA pipeline: fmt → lint → test → dev

# Testing
make test-offline     # All offline tests (CI standard, no Proxmox needed)
make test-unit        # Unit tests only
make test-integration # Integration tests (-tags=integration)
make test-online      # Requires live Proxmox connection
make coverage         # Generate coverage report (backend/coverage.out)

# Code quality
make go-lint          # golangci-lint (3m timeout)
make go-fmt           # Go formatting
make go-template      # Regenerate templ components

# Docker lifecycle
make up / down / restart / logs

# BuildKit (multi-arch: arm64 + amd64)
make buildkit-start / buildkit-stop / buildkit-status
make docker-build
```

## Architecture

### Backend (`backend/`)

Go stateless REST API — no database, all state from Proxmox APIs.

| Package       | Role                                                                     |
| ------------- | ------------------------------------------------------------------------ |
| `main.go`     | Entry point; wires all packages                                          |
| `api/v1/`     | RESTful JSON endpoints (`/api/v1/*`), route registration, JWT middleware |
| `handlers/`   | HTTP handlers for both API and server-rendered pages                     |
| `proxmox/`    | Proxmox API client (go-resty), caching, multi-node aggregation           |
| `security/`   | Session management (alexedwards/scs), CSRF, input validation             |
| `middleware/` | Rate limiting, Proxmox health checks                                     |
| `state/`      | Central `StateManager` — shared session manager, settings, cache         |
| `logger/`     | Structured logging via zerolog                                           |
| `i18n/`       | EN + FR translations                                                     |
| `components/` | Templ components (server-side templating)                                |
| `cloudinit/`  | SFTP upload of cloud-init snippets                                       |
| `tests/`      | Integration tests                                                        |

**Dependency direction:** `api/v1` → `handlers` → `proxmox`, `security`, `state`

### Frontend (`frontend/`)

Vue 3 SPA with **no build step** (airgap-ready, vendor-bundled ESM). Uses Bulma CSS. Vendored dependencies live in `frontend/vendor/`. noVNC console widget at `frontend/components/noVNC-1.6.0/`.

### Deployment

- **Port**: 50000
- **Image**: `gcr.io/distroless/static-debian13:nonroot` (non-root uid 65532)
- **Binary entrypoint**: `/app/pvmss-backend -templates /app/frontend`
- Kubernetes manifests: `pvmss-deployment.yaml`, `pvmss-httproute.yml`
- Helm chart: `helm/`

## Configuration

**Required environment variables:**

- `ADMIN_PASSWORD_HASH` — bcrypt hash for admin login
- `SESSION_SECRET` — 32+ byte session encryption secret
- `PROXMOX_API_TOKEN_NAME` / `PROXMOX_API_TOKEN_VALUE` — Proxmox service account
- `PROXMOX_URL` — e.g. `https://host:8006/api2/json`

**Key optional variables:**

- `PVMSS_OFFLINE=true` — demo mode, disables all Proxmox calls (used in offline tests)
- `PVMSS_ENV` — `production` (default) or `development`
- `PVMSS_SETTINGS_PATH` — path to `settings.json` (default `/app/settings.json`)
- `LOG_LEVEL` / `LOG_FORMAT` / `LOG_OUTPUT` — logging config

**`settings.json`** controls approved nodes/ISOs/storages/bridges, VM resource limits, per-user VM quotas, and SFTP cloud-init config. The `pvmss` tag is mandatory in `tags`.

## Testing Notes

- Offline tests (`make test-offline`) mock all Proxmox calls via `PVMSS_OFFLINE=true` — these run in CI.
- Integration tests require `-tags=integration` and a live Proxmox endpoint.
- Race detector: `make test-offline-race`.
- Coverage target: 80%+.
