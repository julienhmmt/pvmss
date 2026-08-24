# AGENTS.md

Guide for AI coding agents and LLM models working in this repo. This file is
tool-agnostic: read it fully before doing anything. `CLAUDE.md` imports it, so
everything here applies to every agent (Claude Code, Devin, Cursor, Codex, ...).

## What This Project Is

**PVMSS** (Proxmox VM Self-Service) = lightweight web portal. Users manage
Proxmox VMs without direct Proxmox UI access. Stack: Go REST API (`server/`) +
SvelteKit SPA (`web/`, Svelte 5 runes, `adapter-static`). Deploy via
Docker/Kubernetes/Helm.

The v0.4 rewrite is now the only codebase. The legacy v0.3 stack (`backend/` +
`frontend/`) was deleted at the T16 cutover (commit `a7a26f7a`). Any doc,
script, or CI job still pointing at `backend/` or `frontend/` is stale — see
"Known stale references" below.

## Repository Layout

| Path              | Role                                                            |
| ----------------- | --------------------------------------------------------------- |
| `server/`         | Go REST API — module `pvmss/server`, own `go.mod`               |
| `web/`            | SvelteKit SPA — app `pvmss-web`, own `package.json` (bun)       |
| `helm/`           | Helm chart                                                      |
| `docs/`           | Documentation (`docs/plans/` holds task plans)                  |
| `specs/`          | Feature specifications (speckit); gitignored but real work      |
| `sonar-projects/` | Per-project SonarScanner `.properties` files                    |
| `tools/`          | Helper scripts (sonar bootstrap/coverage/scan/query, superlint) |
| `.devin/`         | Project rules + skills (see "Project Conventions")              |

`server/` and `web/` are separate build units with separate tooling. The root
`Makefile` exposes them via the `server-*` / `web-*` targets.

Three root documents carry the product context. Read the relevant one before
building a user-facing feature:

| File           | Answers                                                       |
| -------------- | ------------------------------------------------------------- |
| `PRODUCT.md`   | Who the users are, why the product exists, design principles  |
| `DESIGN.md`    | Design tokens — colors, typography, spacing                   |
| `WORKFLOWS.md` | What a user does, end to end, per workflow                    |

`WORKFLOWS.md` opens with a seven-field template (audience, entry, route, API,
steps, states, safety nets). Adding a user-facing workflow means adding its
entry there, filled in completely — the file is the model, not just a list.

## Commands

Targets:

```bash
# server/ (Go)
make server-test      # go test -race -timeout=5m ./...
make server-lint      # golangci-lint (config server/.golangci.yml, 5m timeout)
make server-fmt       # golangci-lint fmt
make server-vet       # go vet (light check, no golangci-lint needed)

# web/ (SvelteKit + TypeScript)
make web-install      # bun install --frozen-lockfile
make web-test         # vitest run
make web-check        # svelte-check (type checking)
make web-lint         # eslint
make web-lint-fix     # eslint --fix

# both
make lint             # server-lint + web-lint

# e2e (no make target — run from web/)
cd web && bun run test:e2e          # playwright
cd web && bun run test:e2e:install  # install the chromium browser first

# Images / deploy
make docker-build     # multi-arch (amd64+arm64) build + push, tag via PVMSS_TAG
make buildkit-start / buildkit-stop / buildkit-status
make helm-package / helm-upgrade
```

`make up` / `down` / `restart` / `logs` drive `docker-compose.dev.yml`, which
runs two services: `pvmss-dev` (the Go server, built from the v0.4 `Dockerfile`)
and `web-dev` (the Vite dev server, bind-mounting `./web`, built from
`Dockerfile.web-dev`). The Vite dev server proxies `/api` to the Go backend.

```bash
# SonarQube (local container, 2 projects: pvmss-server, pvmss-web)
make sonar              # Full pipeline: start, token, coverage, lint, scan both
make sonar-up           # Start the server on http://localhost:9000
make sonar-bootstrap    # Provision both projects + rotate the analysis token
make sonar-coverage     # Generate the Go coverage report for server/
make sonar-lint         # Run ESLint on web/ (including .svelte) → SonarQube
make sonar-scan         # Scan both projects + print a summary table
make sonar-scan-server  # Scan server/ (Go) only
make sonar-scan-web     # Scan web/ (SvelteKit + ESLint on .svelte) only
make sonar-query CMD="summary"   # Also: projects, issues <key>, metrics <key>, gate <key>, file <key> <path>
make sonar-down         # Stop the server
make sonar-clean        # Stop and remove all SonarQube data
```

## MANDATORY: Graph-First Workflow

**Grep/Glob as a first move is a workflow violation, not a shortcut.** Before
creating or modifying code in ANY folder, consult the knowledge graph for that
folder first. Broad, unscoped Grep/Glob sweeps to "find where X lives" or
"understand how Y works" are exactly what the graph replaces — if you catch
yourself about to grep the whole tree for a symbol or concept, stop and check
the graph instead.

Two complementary graphs exist:

1. **code-review-graph MCP tools** — live graph, auto-updates on file changes
   via hooks. Preferred when the MCP server is available.
2. **graphify static snapshots** — `<folder>/graphify-out/` directories.
   Agent-crawlable, no MCP required, work with any model.

### Procedure (follow in order — do not skip to Grep)

1. **Locate the graph** for the folder you are about to touch:
   - If code-review-graph MCP tools are available → use them (table below).
   - Else look for `<folder>/graphify-out/GRAPH_REPORT.md`.
2. **If a graphify snapshot exists**: start at `graphify-out/wiki/index.md`
   (or `GRAPH_REPORT.md` if no wiki) for communities and god nodes = core
   abstractions, then the relevant `wiki/*.md` article for the node/area in
   question, then `graph.json` only if you need exact edge data.
3. **If NO snapshot exists for that folder, create one before coding:**
   `/graphify <folder>`. Current coverage: `server/`, `web/`, and a merged
   server+web graph at root `graphify-out/`.
4. **If the snapshot is stale** (files modified after the date in
   `GRAPH_REPORT.md`'s header): refresh with `/graphify <folder> --update`
   (incremental, only re-extracts changed files).
5. **Grep/Glob/Read are last resort, scoped, and justified** — use them only
   for what the graph cannot answer (exact string literals, config values,
   generated files, or a specific path the graph already pointed you to).
   Never use them as the first exploration step in a folder that has a graph.

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

### Server (`server/`)

Go REST API over the Proxmox API + SQLite for persistence. Module
`pvmss/server`, Go 1.26. Deliberately dependency-light: routing is stdlib
`net/http`, and the only direct deps are `coder/websocket` (VNC console proxy),
`golang.org/x/crypto` (bcrypt), and `modernc.org/sqlite` (pure-Go, CGO-free).

Entry points under `server/cmd/`:

| Binary            | Role                                  |
| ----------------- | ------------------------------------- |
| `pvmss`           | The HTTP server — the deployed binary |
| `pvmss-recover`   | Recovery CLI                          |
| `pvmss-checklist` | Checklist CLI                         |

Packages under `server/internal/`:

| Package      | Role                                                          |
| ------------ | ------------------------------------------------------------- |
| `httpapi/`   | HTTP handlers + routing for `/api/v1/*`, SPA serving, VNC WS  |
| `vm/`        | VM domain logic — resolve, query, actions, cross-cluster      |
| `cluster/`   | Cluster clients (`proxmox` and `fake` sources), multi-cluster |
| `store/`     | SQLite persistence (modernc.org/sqlite)                       |
| `inventory/` | Background inventory refresh + cache                          |
| `catalog/`   | Approved nodes/ISOs/storages/bridges, cloud-init templates    |
| `policy/`    | Limits, quotas, authorization policy                          |
| `pools/`     | Proxmox pool handling                                         |
| `recovery/`  | Recovery runs and fixtures                                    |
| `checklist/` | Operational checklist walkthroughs                            |
| `auth/`      | Sessions, password hashing, admin auth                        |
| `cloudinit/` | Cloud-init snippet generation                                 |
| `config/`    | Env-based configuration, validation, slog logger, redaction   |

### Web (`web/`)

SvelteKit SPA: Svelte 5 runes, TypeScript, Tailwind CSS v4, `adapter-static`.
Built with bun; the Go binary serves the build output (catch-all to
`index.html` for client routing). Key dirs:

- `src/routes/` — pages: `vms/`, `nodes/`, `admin/`, `profile/`, `login/`
- `src/lib/features/` — feature modules (stores + components per domain)
- `src/lib/shared/` — shared API client and utilities
- `src/lib/i18n/` + `messages/` + `project.inlang/` — i18n via Paraglide (EN + FR)
- `src/lib/paraglide/` — generated Paraglide output
- `src/test/` — vitest setup and helpers
- `e2e/` — Playwright specs (auth, vms, nodes, admin, console, multi-cluster)

### Deployment

- **Port**: 50000 (`PVMSS_PORT`; the Dockerfile `EXPOSE`s 50000)
- **Image**: `gcr.io/distroless/static-debian13:nonroot` (non-root uid 65532)
- **Entrypoint**: `/app/pvmss` — no flags; the web dir comes from
  `PVMSS_WEB_DIR` (default `/app/web/build` in the image) or a path relative
  to the executable
- **Build**: multi-stage — `golang:1.26-alpine` builds a static CGO-free
  binary, `oven/bun:1-alpine` builds the SPA
- Kubernetes manifests: `pvmss-deployment.yaml`, `pvmss-httproute.yml`
- Helm chart: `helm/`

## Configuration

All configuration is environment variables, loaded and validated at startup by
`server/internal/config/load.go`. Startup fails fast on a missing or malformed
required value.

**Required — the server refuses to boot without them:**

| Variable               | Notes                                                       |
| ---------------------- | ----------------------------------------------------------- |
| `PVMSS_PORT`           | Integer 1–65535                                             |
| `PVMSS_DB_PATH`        | SQLite file path (image default `/data/pvmss.db`)           |
| `SESSION_SECRET`       | 32+ bytes                                                   |
| `LOG_LEVEL`            | `debug` \| `info` \| `warn` \| `error` — **lowercase only** |
| `LOG_FORMAT`           | `json` \| `console`                                         |
| `LOG_OUTPUT`           | `stdout` \| `stderr` \| a file path                         |
| `PVMSS_CLUSTER_SOURCE` | `fake` \| `proxmox` — no default, on purpose (see below)    |

`PVMSS_CLUSTER_SOURCE` has no default because `fake` ships hardcoded demo
credentials (`admin@pve` / `pvmss-admin`); it must never be selected by an
operator who simply forgot to set the variable.

**Required when `PVMSS_CLUSTER_SOURCE=proxmox`:**

- `PROXMOX_URL` — e.g. `https://host:8006/api2/json`
- `PROXMOX_API_TOKEN_NAME` / `PROXMOX_API_TOKEN_VALUE`

**Optional:**

| Variable                                      | Default                            |
| --------------------------------------------- | ---------------------------------- |
| `PVMSS_HOST`                                  | `127.0.0.1` (image sets `0.0.0.0`) |
| `PVMSS_WEB_DIR`                               | relative to the executable         |
| `ADMIN_PASSWORD_HASH`                         | empty; if set, must be `$2…`       |
| `PVMSS_COOKIE_SECURE`                         | `true`                             |
| `PVMSS_INVENTORY_REFRESH_INTERVAL`            | `30s`                              |
| `PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL` | `5s`                               |
| `PVMSS_INVENTORY_REFRESH_TIMEOUT`             | `15s`                              |
| `PVMSS_MAX_LIST_PAGE_SIZE`                    | `100`                              |

`PVMSS_OFFLINE`, `PVMSS_ENV`, `JWT_SECRET`, `PROXMOX_VERIFY_SSL` and
`LOG_FILE_PATH` belonged to the v0.3 backend and are **no longer read**. Demo
mode is now `PVMSS_CLUSTER_SOURCE=fake`.

## Testing Notes

- `make server-test` runs the whole Go suite with `-race`, no Proxmox needed —
  tests use the `fake` cluster source.
- `make web-test` runs vitest; `cd web && bun run test:coverage` for coverage.
- Playwright e2e lives in `web/e2e/`; run `bun run test:e2e:install` once, then
  `bun run test:e2e`.
- There is no `-tags=integration` build tag and no separate offline/online test
  split any more — both were v0.3 concepts.

## Known stale references

Files still pointing at the deleted `backend/` or `frontend/`. Fix them when
you touch the surrounding area; do not treat them as documentation of reality:

- `docs/plans/` — historical task plans reference v0.3 paths (read-only history)
- `server/internal/recovery/` — comments reference `backend/` for context only

Stale references already cleaned up:

- `README.md` / `README.fr.md` — already link the in-app page `/docs/proxmox-permissions`
  (seeded from `server/internal/docs/seed/recovered/`), not the deleted
  `backend/docs/proxmox-permissions.*.md`.
- `tools/superlinter.sh` — the stale `frontend/` exclude regex and broken volume
  mount (`$(pwd)../.`) have been fixed.

## Project Conventions

- Follow `.devin/rules/coding-style.md` for Go and TypeScript style.
- Follow `.devin/rules/ui-quality.md` for admin page and form layouts.
- Use `.devin/skills/todo-planning.md` to track multi-step work.
- Additional skills live in `.devin/skills/` (golang-*, svelte-code-writer,
  tailwind-design-system, typescript-advanced-types, speckit.*) — check them
  before starting matching work.
- Admin features are SvelteKit routes under `web/src/routes/admin/`, backed by
  the `admin_*.go` handlers in `server/internal/httpapi/`.
- Admin API handlers must return complete response payloads required by the
  admin UI.
- English for all code and documentation. UI translations: EN + FR.
