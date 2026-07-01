# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Is

**PVMSS** (Proxmox VM Self-Service) is a lightweight web portal letting users manage Proxmox VMs without direct Proxmox UI access. Stack: Go backend + SvelteKit SPA (Svelte 5 runes, adapter-static), deployed via Docker/Kubernetes/Helm.

## Commands

```bash
# Development
make dev              # Build frontend + Go binary + start Docker container
make qualif           # Full QA pipeline: fmt → lint → test → dev
make frontend-dev     # Run SvelteKit dev server (port 5173, proxies /api → :50000)
make frontend-build   # Build SvelteKit SPA → frontend/build/
make frontend-install # Install frontend npm dependencies

# Testing
make test-offline     # All offline tests (CI standard, no Proxmox needed)
make test-unit        # Unit tests only
make test-integration # Integration tests (-tags=integration)
make test-online      # Requires live Proxmox connection
make coverage         # Generate coverage report (backend/coverage.out)

# Code quality
make go-lint          # golangci-lint (3m timeout)
make go-fmt           # Go formatting

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
| `handlers/`   | HTTP handlers: SPA serving, auth forms (legacy), health, static files    |
| `proxmox/`    | Proxmox API client (go-resty), caching, multi-node aggregation           |
| `security/`   | Session management (alexedwards/scs), CSRF, input validation             |
| `middleware/` | Rate limiting, Proxmox health checks                                     |
| `state/`      | Central `StateManager` — shared session manager, settings, cache         |
| `logger/`     | Structured logging via zerolog                                           |
| `i18n/`       | EN + FR translations                                                     |
| `cloudinit/`  | SFTP upload of cloud-init snippets                                       |
| `tests/`      | Integration tests                                                        |

**Dependency direction:** `api/v1` → `handlers` → `proxmox`, `security`, `state`

### Frontend (`frontend/`)

SvelteKit SPA using Svelte 5 runes, TypeScript, Tailwind CSS, and `adapter-static`. The Go binary serves the build output from `frontend/build/` with a catch-all to `index.html` for client-side routing. Key directories:

- `src/routes/` — SvelteKit pages (`(app)/` group requires auth, `(public)/` is unauthenticated)
- `src/lib/api/` — typed API clients for `/api/v1/*`
- `src/lib/stores/` — Svelte 5 reactive stores
- `src/lib/components/` — reusable Svelte components
- `src/lib/i18n/` — EN + FR translation files
- `static/noVNC-1.6.0/` — noVNC console widget (copied into `build/` at build time)

### Deployment

- **Port**: 50000
- **Image**: `gcr.io/distroless/static-debian13:nonroot` (non-root uid 65532)
- **Binary entrypoint**: `/app/pvmss-backend -templates /app/frontend`
- **SPA**: served from `frontend/build/` (catch-all `index.html`), `/_app/` assets bypass session middleware
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
- `PVMSS_DB_PATH` — path to SQLite database file (default `/data/pvmss.db`)
- `LOG_LEVEL` / `LOG_FORMAT` / `LOG_OUTPUT` — logging config

**Database** stores approved nodes/ISOs/storages/bridges, VM resource limits, per-user VM quotas, and SFTP cloud-init config. The `pvmss` tag is mandatory in `tags`.

## Testing Notes

- Offline tests (`make test-offline`) mock all Proxmox calls via `PVMSS_OFFLINE=true` — these run in CI.
- Integration tests require `-tags=integration` and a live Proxmox endpoint.
- Race detector: `make test-offline-race`.

## MCP Tools: code-review-graph

<!-- Single source of truth for the code-review-graph MCP tool guidance is
     AGENTS.md (tool-agnostic). Included here via Claude Code's @path import. -->
@AGENTS.md
