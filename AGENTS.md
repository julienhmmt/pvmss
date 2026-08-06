# AGENTS.md

Guide for AI coding agents and LLM models working in this repo. This file is
tool-agnostic: read it fully before doing anything. `CLAUDE.md` imports it, so
everything here applies to every agent (Claude Code, Devin, Cursor, Codex, ...).

## What This Project Is

**PVMSS** (Proxmox VM Self-Service) = lightweight web portal. Users manage
Proxmox VMs without direct Proxmox UI access. Stack: Go backend + SvelteKit SPA
(Svelte 5 runes, adapter-static). Deploy via Docker/Kubernetes/Helm.

## Repository Layout

| Path        | Role                                                                 |
| ----------- | -------------------------------------------------------------------- |
| `backend/`  | Current production Go REST API (Go module `pvmss`, own `go.mod`)     |
| `frontend/` | Current production SvelteKit SPA                                     |
| `server/`   | Next-gen v0.4 rewrite — separate Go module `pvmss/server` (WIP)      |
| `web/`      | Next-gen v0.4 rewrite — `pvmss-web` SvelteKit app (WIP)              |
| `helm/`     | Helm chart                                                           |
| `docs/`     | Documentation (`docs/plans/` holds the v0.4 task plans)              |
| `specs/`    | Feature specifications                                               |
| `tools/`    | Helper scripts                                                       |

`server/` + `web/` are NOT wired into the root `Makefile`; they have their own
tooling (`go test ./...` in `server/`, `bun`/`vite`/`vitest`/`playwright` in
`web/`). Check which codebase a task targets before writing code.

## Commands

```bash
# Development (current app: backend/ + frontend/)
make dev              # Build frontend + Go binary + start Docker container
make qualif           # Full QA pipeline: fmt → lint → test → dev
make frontend-dev     # SvelteKit dev server (port 5173, proxies /api → :50000)
make frontend-build   # Build SvelteKit SPA → frontend/build/
make frontend-install # Install frontend npm dependencies

# Testing (current app)
make test-offline     # All offline tests (CI standard, no Proxmox needed)
make test-unit        # Unit tests only
make test-integration # Integration tests (-tags=integration)
make test-online      # Requires live Proxmox connection
make coverage         # Coverage report (backend/coverage.out)
make test-offline-race # Offline tests with race detector

# Code quality
make go-lint          # golangci-lint (3m timeout)
make go-fmt           # Go formatting

# Docker lifecycle
make up / down / restart / logs

# BuildKit (multi-arch: arm64 + amd64)
make buildkit-start / buildkit-stop / buildkit-status
make docker-build
```

## MANDATORY: Graph-First Workflow

**Before creating or modifying code in ANY folder, consult the knowledge graph
for that folder first.** Do not explore with Grep/Glob/Read alone, and never
write new code without graph context on the surrounding architecture.

Two complementary graphs exist:

1. **code-review-graph MCP tools** — live graph, auto-updates on file changes
   via hooks. Preferred when the MCP server is available.
2. **graphify static snapshots** — `<folder>/graphify-out/` directories.
   Agent-crawlable, no MCP required, work with any model.

### Procedure (follow in order)

1. **Locate the graph** for the folder you are about to touch:
   - If code-review-graph MCP tools are available → use them (table below).
   - Else look for `<folder>/graphify-out/GRAPH_REPORT.md`.
2. **If a graphify snapshot exists**: read `GRAPH_REPORT.md` first (communities,
   god nodes = core abstractions), then the relevant `wiki/*.md` article, then
   `graph.json` only if you need exact nodes/edges.
3. **If NO snapshot exists for that folder, create one before coding:**
   `/graphify <folder>`. Current coverage: `server/`, `web/`, and a merged
   server+web graph at root `graphify-out/`. **`backend/` and `frontend/` have
   no snapshots yet** — run `/graphify backend` / `/graphify frontend` (or ask
   the user to) before working there.
4. **If the snapshot is stale** (files modified after the date in
   `GRAPH_REPORT.md`'s header): refresh with `/graphify <folder> --update`
   (incremental, only re-extracts changed files).
5. **Only then** fall back to Grep/Glob/Read for anything the graph could not
   answer (exact string literals, config values, generated files).

### code-review-graph MCP tools

| Tool | Use when |
| ------ | ---------- |
| `detect_changes` | Reviewing code changes — gives risk-scored analysis |
| `get_review_context` | Need source snippets for review — token-efficient |
| `get_impact_radius` | Understanding blast radius of a change |
| `get_affected_flows` | Finding which execution paths are impacted |
| `query_graph` | Tracing callers, callees, imports, tests, dependencies (patterns: callers_of/callees_of/imports_of/tests_for) |
| `semantic_search_nodes` | Finding functions/classes by name or keyword |
| `get_architecture_overview` | Understanding high-level codebase structure |
| `refactor_tool` | Planning renames, finding dead code |

When to prefer them over Grep:

- **Exploring code**: `semantic_search_nodes` or `query_graph` instead of Grep
- **Understanding impact**: `get_impact_radius` instead of manually tracing imports
- **Code review**: `detect_changes` + `get_review_context` instead of reading entire files
- **Architecture questions**: `get_architecture_overview` + `list_communities`
- **Coverage check**: `query_graph` pattern="tests_for"

### graphify CLI (static snapshots)

```text
/graphify <folder>                 # full pipeline → <folder>/graphify-out/
/graphify <folder> --update        # incremental refresh (changed files only)
/graphify <folder> --wiki          # rebuild agent-crawlable wiki articles
/graphify query "<question>"       # ask the graph a question (broad context)
/graphify explain "<NodeName>"     # plain-language explanation of a node
/graphify path "<A>" "<B>"         # shortest path between two concepts
```

Unlike code-review-graph, snapshots do NOT auto-update — refresh them after
significant changes.

## Architecture

### Backend (`backend/`)

Go REST API on Proxmox APIs + SQLite DB for settings, limits, quotas, audit.

| Package       | Role                                                                                                     |
| ------------- | -------------------------------------------------------------------------------------------------------- |
| `main.go`     | Entry point; wires all packages                                                                          |
| `database/`   | SQLite persist: approved nodes/ISOs/storages/bridges, VM limits, per-user quotas, SFTP config, audit log |
| `api/v1/`     | RESTful JSON endpoints (`/api/v1/*`), route registration, JWT middleware                                 |
| `handlers/`   | HTTP handlers: SPA serving, auth forms (legacy), health, static files                                    |
| `proxmox/`    | Proxmox API client (go-resty), caching, multi-node aggregation                                           |
| `security/`   | Session management (alexedwards/scs), CSRF, input validation                                             |
| `middleware/` | Rate limiting, Proxmox health checks                                                                     |
| `state/`      | Central `StateManager` — shared session manager, settings, cache                                         |
| `logger/`     | Structured logging via zerolog                                                                           |
| `i18n/`       | EN + FR translations                                                                                     |
| `cloudinit/`  | SFTP upload of cloud-init snippets                                                                       |
| `tests/`      | Integration tests                                                                                        |

**Dependency direction:** `api/v1` → `handlers` → `proxmox`, `security`, `state`

### Frontend (`frontend/`)

SvelteKit SPA: Svelte 5 runes, TypeScript, Tailwind CSS, `adapter-static`. The
Go binary serves the build output from `frontend/build/`, catch-all to
`index.html` for client routing. Key dirs:

- `src/routes/` — SvelteKit pages (`(app)/` group needs auth, `(public)/` unauthenticated)
- `src/lib/api/` — typed API clients for `/api/v1/*`
- `src/lib/stores/` — Svelte 5 reactive stores
- `src/lib/components/` — reusable Svelte components
- `src/lib/i18n/` — EN + FR translation files
- `static/noVNC-1.6.0/` — noVNC console widget (copied into `build/` at build time)

### Next-gen rewrite (`server/` + `web/`)

- `server/internal/`: `auth`, `cluster`, `config`, `httpapi`, `inventory`,
  `store` (SQLite via modernc.org/sqlite). Entry point: `server/cmd/pvmss`.
- `web/src/`: SvelteKit app (Svelte 5), tested with vitest + playwright.
- Architecture orientation: read `server/graphify-out/GRAPH_REPORT.md` and
  `web/graphify-out/GRAPH_REPORT.md` first — do not scan files blindly.

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

**Database** stores approved nodes/ISOs/storages/bridges, VM resource limits,
per-user VM quotas, SFTP cloud-init config. `pvmss` tag mandatory in `tags`.

## Testing Notes

- Offline tests (`make test-offline`) mock all Proxmox calls via `PVMSS_OFFLINE=true` — run in CI.
- Integration tests need `-tags=integration` + live Proxmox endpoint.
- Race detector: `make test-offline-race`.

## Project Conventions

- Follow `.devin/rules/coding-style.md` for Go and TypeScript style.
- Follow `.devin/rules/ui-quality.md` for admin page and form layouts.
- Use `.devin/skills/todo-planning.md` to track multi-step work.
- Use `.devin/skills/backend-refactor.md` when splitting monolithic backend files.
- Additional skills live in `.devin/skills/` (golang-*, svelte-code-writer,
  tailwind-design-system, speckit.*) — check them before starting matching work.
- Admin features are a SvelteKit SPA under `frontend/src/routes/admin/` backed by REST endpoints in `backend/api/v1/admin_*.go`.
- Admin API handlers must return complete response payloads required by the admin UI.
- English for all code and documentation. UI translations: EN + FR.
