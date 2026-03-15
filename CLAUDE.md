# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

PVMSS (Proxmox VM Self-Service) is a stateless web portal that lets users create and manage Proxmox VMs without exposing the Proxmox UI. It runs as a Go binary serving HTML/CSS templates and proxying Proxmox API calls.

## Commands

All development commands are in the `Makefile`. Run from the repo root.

### Build & Run

```bash
make build          # Build Go binary + Docker container (dev)
make dev            # Build + start Docker dev container
make up             # Start existing Docker dev container
make down           # Stop dev container
make logs           # Follow Docker dev container logs
```

The dev container uses `docker-compose.dev.yml` and runs at <http://localhost:50000>.

### Testing

Tests require a settings file; the Makefile copies `backend/settings.dev.json` to `/tmp/settings.test.json` automatically.

```bash
make test-offline          # Run all tests in offline mode (no Proxmox needed) — used in CI
make test-offline-verbose  # Same with -v flag
make test-offline-race     # Same with -race detector
make test-unit             # Unit tests only (offline)
make test-integration      # Integration tests only (offline)
make test-routes           # Route accessibility tests only
make coverage              # Generate coverage.out in backend/
```

To run a single Go test directly:

```bash
cd backend && PVMSS_SETTINGS_PATH=/tmp/settings.test.json GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -v -run TestFunctionName ./...
```

### Code Quality

```bash
make go-fmt     # Format Go code (gofmt)
make go-lint    # Run golangci-lint from backend/
make qualif     # fmt + lint + test-offline + dev (full quality gate)
```

For templates (a-h/templ):

```bash
make go-template   # Regenerate templ files
```

## Architecture

### Directory Layout

```bash
backend/          # Go application (module: pvmss)
  main.go         # Entry point: logger init, Proxmox client, templates, HTTP server
  api/v1/         # JWT-authenticated JSON API (/api/v1/auth/*, /api/v1/vms/*)
  handlers/       # HTTP handlers, one file per feature area
  proxmox/        # Proxmox API client layer (dual: Telmate legacy + Resty)
  state/          # Centralized app state via StateManager interface
  middleware/     # Rate limiting, CSRF, session middleware
  security/       # Session setup, CSRF token management, env validation
  i18n/           # TOML translation files (en/fr per feature)
  templates/      # Template loading utilities
  logger/         # zerolog wrapper
  constants/      # App-wide constants
  tests/          # Integration and route tests (build tag: integration)
frontend/         # Static assets + HTML templates
  components/     # Partial HTML templates rendered by Go backend
  css/            # Compiled CSS (Bulma-based + custom)
  js/             # JavaScript
  src/            # Vue 3 SPA (no build step — plain ES modules, vendored libs)
    api/          # Axios-based API client for /api/v1/
    components/   # VmCard.js, VmActionButtons.js, AppButton.js
    stores/       # Pinia stores (auth, vms)
  vendor/         # Vendored ESM bundles: vue, pinia, axios (no npm needed)
  webfonts/       # Font Awesome webfonts
```

### Request Flow

1. `main.go` initializes `StateManager`, Proxmox client, templates, session manager
2. `handlers.InitHandlers()` creates all handler structs, wires routes via `httprouter`
3. Two middleware stacks: **public** (static/health, no session) and **app** (full: session, CSRF, rate limit)
4. Handlers read/write through `StateManager` interface — never hold state themselves
5. Templates are Go `html/template` files in `frontend/`, parsed at startup

### StateManager

The `StateManager` interface (`backend/state/interface.go`) is the central dependency container. It manages: templates, sessions, Proxmox client, settings (`settings.json`), CSRF tokens, and the frontend path. All handlers receive it via constructor injection.

### Proxmox API Layer

**Resty** (`proxmox/resty_client.go`): new REST client being adopted incrementally

### Offline Mode

Setting `PVMSS_OFFLINE=true` skips all Proxmox API calls. All tests use offline mode. Use `PVMSS_OFFLINE=true` when working without a Proxmox cluster.

### Settings

`settings.json` (runtime) and `backend/settings.dev.json` (development/tests) configure available storages, ISOs, VMBRs, tags, VM resource limits, cloud-init SFTP, and the JWT signing key. All keys are mandatory.

| Key | Purpose |
| --- | ------- |
| `jwt_secret` | 32+ byte signing key for `/api/v1/` JWT tokens. Stored in `settings.json`, **not** an env var. |

### Internationalization

Translation files in `backend/i18n/` are TOML, one file per language per feature area (e.g., `vm_create.en.toml`, `vm_create.fr.toml`). The `T` template function localizes strings. Supported languages: English and French.

### Test Environment Variables

```bash
GO_TEST_ENVIRONMENT=1    # Disables rate limiting, relaxes some security checks
PVMSS_SETTINGS_PATH=...  # Path to settings JSON for tests
PVMSS_OFFLINE=true       # Skip Proxmox API calls
```

### Key Environment Variables (Runtime)

| Variable | Purpose |
| -------- | ------- |
| `PROXMOX_URL` | Full Proxmox API URL |
| `PROXMOX_API_TOKEN_NAME` | `user@pve!token` |
| `PROXMOX_API_TOKEN_VALUE` | Token secret |
| `ADMIN_PASSWORD_HASH` | Bcrypt hash for admin login |
| `SESSION_SECRET` | 32+ byte cookie encryption secret |
| `PVMSS_ENV` | `production` or `development` |
| `PVMSS_OFFLINE` | `true` for demo/test mode |
