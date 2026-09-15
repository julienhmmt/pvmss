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
script, or CI job still pointing at `backend/` or `frontend/` is stale - see
"Known stale references" below.

## Repository Layout

| Path              | Role                                                            |
| ----------------- | --------------------------------------------------------------- |
| `server/`         | Go REST API - module `pvmss/server`, own `go.mod`               |
| `web/`            | SvelteKit SPA - app `pvmss-web`, own `package.json` (bun)       |
| `helm/`           | Helm chart                                                      |
| `docs/`           | Documentation (`docs/plans/` holds task plans)                  |
| `specs/`          | Feature specifications (speckit); gitignored but real work      |
| `sonar-projects/` | Per-project SonarScanner `.properties` files                    |
| `tools/`          | Helper scripts (`pq`, sonar bootstrap/coverage/scan/query, superlint) |
| `.devin/`         | Project rules + skills (see "Project Conventions")              |
| `.agents/`        | Agent-local working files - `skills/`, `memory/` (gitignored)   |

`server/` and `web/` are separate build units with separate tooling. The root
`Makefile` exposes them via the `server-*` / `web-*` targets.

Three root documents carry the product context. Read the relevant one before
building a user-facing feature:

| File           | Answers                                                       |
| -------------- | ------------------------------------------------------------- |
| `PRODUCT.md`   | Who the users are, why the product exists, design principles  |
| `docs/FEATURES.md` | Route-by-route inventory of every shipped feature and its status |
| `DESIGN.md`    | Design tokens - colors, typography, spacing                   |
| `WORKFLOWS.md` | What a user does, end to end, per workflow                    |

`WORKFLOWS.md` opens with a seven-field template (audience, entry, route, API,
steps, states, safety nets). Adding a user-facing workflow means adding its
entry there, filled in completely - the file is the model, not just a list.

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

# e2e (no make target - run from web/)
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

## MANDATORY: Locate Before You Read

**An unanchored `rg NAME` across the tree is a workflow violation.** Measured on
this repo: `rg -n "Snapshot"` returns 1 107 lines (~30 570 tokens); the three
declarations you actually wanted cost ~131. Same 0.2 s, 230x the price. Reading
a whole package to find one function costs ~138 000 tokens.

Use `tools/pq`. It wraps ripgrep with declaration-aware patterns and
excludes `node_modules`, build output and vendor trees. It indexes nothing - 
every answer is computed fresh in ~200 ms, so it is never stale.

| Question | Command | Typical cost |
| --- | --- | --- |
| Where is `Foo` defined? | `pq def Foo` | ~130 tok |
| What does this package expose? | `pq api server/internal/vm` | ~2.5k tok |
| Who uses `Foo`? | `pq callers Foo` | ~190 tok |
| What is in this file? | `pq file path/to/x.go` | varies |
| How is this area organised? | `pq tree server/internal` | ~120 tok |
| Free-text, last resort | `pq grep 'pattern' [path]` | unbounded |

### Procedure

1. `pq def NAME` first. It answers "where is X" outright.
2. `pq callers NAME` before any grep - it returns one line per file with an
   occurrence count, so you pick the two files worth reading instead of paging
   through every hit. Then `pq grep NAME <that-path>`.
3. Orientation is top-down: `pq tree` -> `pq api <dir>` -> `pq file` -> `Read`
   the one function. Stop as soon as you have what you need.
4. `Read` on a whole package is a last resort and must be justified.
5. For *why* the code is shaped this way rather than *where* it is, that is the
   project journal, not the locator.

The `project-query` skill carries the same rules plus raw ripgrep fallbacks.

**Removed 2026-09-11:** the code-review-graph MCP server, its `PostToolUse`
hook (it ran on every Edit/Write/Bash), and 173 MB of `graphify-out/` snapshots
frozen since 30 August. Its leftover `pre-commit` git hook and
`.code-review-graph/graph.db` followed on 2026-09-13. They answered "who calls
X" - which ripgrep answers in 200 ms - at the cost of a permanently stale
index. Do not reintroduce a code index without measuring against `pq` first.

## Architecture

### Server (`server/`)

Go REST API over the Proxmox API + SQLite for persistence. Module
`pvmss/server`, Go 1.26. Deliberately dependency-light: routing is stdlib
`net/http`, and the only direct deps are `coder/websocket` (VNC console proxy),
`golang.org/x/crypto` (bcrypt), and `modernc.org/sqlite` (pure-Go, CGO-free).

Entry points under `server/cmd/`:

| Binary            | Role                                  |
| ----------------- | ------------------------------------- |
| `pvmss`           | The HTTP server - the deployed binary |
| `pvmss-recover`   | Recovery CLI                          |
| `pvmss-checklist` | Checklist CLI                         |

Packages under `server/internal/`:

| Package      | Role                                                          |
| ------------ | ------------------------------------------------------------- |
| `httpapi/`   | HTTP handlers + routing for `/api/v1/*`, SPA serving, VNC WS  |
| `vm/`        | VM domain logic - resolve, query, actions, cross-cluster      |
| `cluster/`   | Cluster clients (`proxmox` and `fake` sources), multi-cluster |
| `store/`     | SQLite persistence (modernc.org/sqlite)                       |
| `inventory/` | Background inventory refresh + cache                          |
| `catalog/`   | Approved nodes/ISOs/storages/bridges, cloud-init templates     |
| `policy/`    | Limits, quotas, authorization policy                          |
| `pools/`     | Proxmox pool handling                                         |
| `recovery/`  | Recovery runs and fixtures                                    |
| `checklist/` | Operational checklist walkthroughs                            |
| `auth/`      | Sessions, password hashing, admin auth                        |
| `cloudinit/` | Cloud-init document validation + slug helpers                 |
| `config/`    | Env-based configuration, validation, slog logger, redaction   |

Cloud-init documents are written by PVMSS itself into a bind-mounted storage
`snippets/` directory configured per cluster (`clusters.snippet_dir` /
`clusters.snippet_storage`, admin form in `/admin/clusters`); the Proxmox
REST API cannot write snippets. Sources: `catalog_cloudinit_templates`
(admin, per cluster) and `user_cloudinit_files` (owner-scoped, max 20);
each VM gets its own `pvmss-<vmid>.yml` copy recorded in
`vm_cloudinit_snippets`.

Cloud-init documents are written by PVMSS itself into a bind-mounted storage
`snippets/` directory configured per cluster
(`clusters.snippet_dir/snippet_storage`); the Proxmox API cannot write
snippets. User-owned cloud-init files live in `store/user_cloudinit_files.go`.

### Web (`web/`)

SvelteKit SPA: Svelte 5 runes, TypeScript, Tailwind CSS v4, `adapter-static`.
Built with bun; the Go binary serves the build output (catch-all to
`index.html` for client routing). Key dirs:

- `src/routes/` - pages: `vms/`, `nodes/`, `admin/`, `profile/`, `login/`
- `src/lib/features/` - feature modules (stores + components per domain)
- `src/lib/shared/` - shared API client and utilities
- `src/lib/i18n/` + `messages/` + `project.inlang/` - i18n via Paraglide (EN + FR)
- `src/lib/paraglide/` - generated Paraglide output
- `src/test/` - vitest setup and helpers
- `e2e/` - Playwright specs (auth, vms, nodes, admin, console, multi-cluster)

### Deployment

- **Port**: 50000 (`PVMSS_PORT`; the Dockerfile `EXPOSE`s 50000)
- **Image**: `gcr.io/distroless/static-debian13:nonroot` (non-root uid 65532)
- **Entrypoint**: `/app/pvmss` - no flags; the web dir comes from
  `PVMSS_WEB_DIR` (default `/app/web/build` in the image) or a path relative
  to the executable
- **Build**: multi-stage - `golang:1.26-alpine` builds a static CGO-free
  binary, `oven/bun:1-alpine` builds the SPA
- Kubernetes manifests: `pvmss-deployment.yaml`, `pvmss-httproute.yml`
- Helm chart: `helm/`

## Configuration

All configuration is environment variables, loaded and validated at startup by
`server/internal/config/load.go`. Startup fails fast on a missing or malformed
required value.

**Required - the server refuses to boot without them:**

| Variable               | Notes                                                       |
| ---------------------- | ----------------------------------------------------------- |
| `PVMSS_PORT`           | Integer 1–65535                                             |
| `PVMSS_DB_PATH`        | SQLite file path (image default `/data/pvmss.db`)           |
| `SESSION_SECRET`       | 32+ bytes                                                   |
| `LOG_LEVEL`            | `debug` \| `info` \| `warn` \| `error` - **lowercase only** |
| `LOG_FORMAT`           | `json` \| `console`                                         |
| `LOG_OUTPUT`           | `stdout` \| `stderr` \| a file path                         |
| `PVMSS_CLUSTER_SOURCE` | `fake` \| `proxmox` - no default, on purpose (see below)    |

`PVMSS_CLUSTER_SOURCE` has no default because `fake` ships hardcoded demo
credentials (`admin@pve` / `pvmss-admin`); it must never be selected by an
operator who simply forgot to set the variable.

**Required when `PVMSS_CLUSTER_SOURCE=proxmox`:**

- `PROXMOX_URL` - e.g. `https://host:8006/api2/json`
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
| `PVMSS_TRUSTED_PROXY_HOPS`                    | `1`                                |

`PVMSS_OFFLINE`, `PVMSS_ENV`, `JWT_SECRET`, `PROXMOX_VERIFY_SSL` and
`LOG_FILE_PATH` belonged to the v0.3 backend and are **no longer read**. Demo
mode is now `PVMSS_CLUSTER_SOURCE=fake`.

## Testing Notes

- `make server-test` runs the whole Go suite with `-race`, no Proxmox needed - 
  tests use the `fake` cluster source.
- `make web-test` runs vitest; `cd web && bun run test:coverage` for coverage.
- Playwright e2e lives in `web/e2e/`; run `bun run test:e2e:install` once, then
  `bun run test:e2e`.
- There is no `-tags=integration` build tag and no separate offline/online test
  split any more - both were v0.3 concepts.

## Known stale references

Files still pointing at the deleted `backend/` or `frontend/`. Fix them when
you touch the surrounding area; do not treat them as documentation of reality:

- `docs/plans/` - historical task plans reference v0.3 paths (read-only history)
- `server/internal/recovery/` - comments reference `backend/` for context only

Stale references already cleaned up:

- `README.md` / `README.fr.md` - already link the in-app page `/docs/proxmox-permissions`
  (seeded from `server/internal/docs/seed/recovered/`), not the deleted
  `backend/docs/proxmox-permissions.*.md`.
- `tools/superlinter.sh` - the stale `frontend/` exclude regex and broken volume
  mount (`$(pwd)../.`) have been fixed.

## Agent Memory Journal (`.agents/memory/`)

`.agents/memory/` is a **gitignored daily journal** (see `.gitignore`: `.agents/`)
where agents record what they did, where, and a short summary so future agents
can find the information fast. It is never committed.

Convention:

- One file per work session, named `YYYY-MM-DD-<short-slug>.md`.
- Sections: **What** (one-line summary), **Where** (files touched, with paths),
  **Why** (the problem being solved), **Key insight** (any non-obvious fact
  worth remembering), **Verification performed** (commands run + results),
  and **Verdict** (`<ACCEPT>` / `<REJECT>` / outcome).
- Append-only - never edit or delete past entries; start a new file for a new
  session. If two sessions happen on the same day, suffix `-2`, `-3`, etc.
- Read recent entries before starting work in an area to avoid redoing
  investigation.

## Project Conventions

- Follow `.devin/rules/coding-style.md` for Go and TypeScript style.
- Follow `.devin/rules/ui-quality.md` for admin page and form layouts.
- Use `.devin/skills/todo-planning.md` to track multi-step work.
- Additional skills live in `.devin/skills/` (golang-*, svelte-code-writer,
  tailwind-design-system, typescript-advanced-types, speckit.*) - check them
  before starting matching work.
- Admin features are SvelteKit routes under `web/src/routes/admin/`, backed by
  the `admin_*.go` handlers in `server/internal/httpapi/`.
- Admin API handlers must return complete response payloads required by the
  admin UI.
- English for all code and documentation. UI translations: EN + FR.
- **No em-dash (U+2014) in authored source or docs.** Use a spaced ASCII
  hyphen ` - ` instead. Em-dash is an AI-typography tell; the repo is
  lint-clean against it. Generated/vendored trees (node_modules, paraglide,
  build, .svelte-kit) are exempt. Lint: `rg '—'` returns nothing outside
  exempt dirs.
