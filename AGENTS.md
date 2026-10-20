# AGENTS.md

The single guide for every AI agent in this repo (Claude Code, Devin, Cursor,
Codex, ...). `CLAUDE.md` only imports this file. Read it fully before doing
anything.

## What This Project Is

**PVMSS** (Proxmox VM Self-Service) = lightweight web portal. Users manage
Proxmox VMs without direct Proxmox UI access. Stack: Go REST API (`server/`) +
SvelteKit SPA (`web/`, Svelte 5 runes, `adapter-static`). Deploy via
Docker/Kubernetes/Helm.

The legacy v0.3 stack (`backend/` + `frontend/`) is gone. Any reference to
those paths is stale (some comments in `server/internal/recovery/` keep them
for context only).

## Repository Layout

| Path              | Role                                                                     |
| ----------------- | ------------------------------------------------------------------------ |
| `server/`         | Go REST API - module `pvmss/server`, own `go.mod`                        |
| `web/`            | SvelteKit SPA - app `pvmss-web`, own `package.json` (bun)                |
| `helm/`           | Helm chart                                                               |
| `docs/`           | `FEATURES.md`, `cloud-init.md`, `observability.md`, `design/`, `agents/` |
| `specs/`          | Feature specifications (speckit); gitignored but real work               |
| `sonar-projects/` | Per-project SonarScanner `.properties` files                             |
| `tools/`          | Helper scripts (`pq`, sonar, superlint)                                  |
| `.devin/`         | Project rules + skills (gitignored - not in a fresh clone)               |
| `.agents/`        | Agent-local files - `skills/`, `memory/` (gitignored)                    |
| `.scratch/`       | Issues and specs as markdown (see `docs/agents/issue-tracker.md`)        |

`server/` and `web/` are separate build units; the root `Makefile` exposes them
via `server-*` / `web-*` targets.

Product context - read the relevant one before building a user-facing feature:

| File               | Answers                                                      |
| ------------------ | ------------------------------------------------------------ |
| `PRODUCT.md`       | Who the users are, why the product exists, design principles |
| `docs/FEATURES.md` | Route-by-route inventory of shipped features                 |
| `WORKFLOWS.md`     | What a user does, end to end, per workflow                   |
| `DESIGN.md`        | Design tokens - colors, typography, spacing                  |
| `ROADMAP.md`       | History and what's next                                      |
| `TECH_DEBT.md`     | What's lingering or half-finished                            |
| `CONTEXT.md`       | Domain glossary (projection, live status, UPID, lock...)     |
| `CONTRIBUTING.md`  | How to contribute; docs to update with a user-facing change  |

`WORKFLOWS.md` opens with a seven-field template (audience, entry, route, API,
steps, states, safety nets). A new user-facing workflow gets a complete entry
there. Domain decisions go in `CONTEXT.md` + `docs/adr/` (created lazily, see
`docs/agents/domain.md`).

## Commands

```bash
# server/ (Go)
make server-test      # go test -race -timeout=5m ./...
make server-lint      # golangci-lint (config server/.golangci.yml)
make server-fmt       # golangci-lint fmt
make server-vet       # go vet

# web/ (SvelteKit + TypeScript)
make web-install      # bun install --frozen-lockfile
make web-test         # vitest run
make web-check        # svelte-check
make web-lint         # eslint (web-lint-fix to fix)

make lint             # server-lint + web-lint

# e2e (from web/)
cd web && bun run test:e2e:install  # once
cd web && bun run test:e2e

# Images / deploy
make docker-build     # multi-arch build + push, tag via PVMSS_TAG
make buildkit-start / buildkit-stop / buildkit-status
make helm-package / helm-upgrade

# Dev stack (docker-compose.dev.yml: pvmss-dev Go server + web-dev Vite (/api proxied) + SonarQube)
make up / down / restart / logs

# SonarQube (projects pvmss-server, pvmss-web)
make sonar              # full pipeline
make sonar-up / sonar-down / sonar-clean
make sonar-scan         # also sonar-scan-server, sonar-scan-web
make sonar-query CMD="summary"   # also: projects, issues, metrics, gate, file
```

## MANDATORY: Locate Before You Read

**An unanchored `rg NAME` across the tree is a workflow violation.** It costs
~230x more tokens than a targeted lookup. Use `tools/pq` (ripgrep with
declaration-aware patterns, no index, never stale) before any grep/glob.

| Question                       | Command                     |
| ------------------------------ | --------------------------- |
| Where is `Foo` defined?        | `pq def Foo`                |
| What does this package expose? | `pq api server/internal/vm` |
| Who uses `Foo`?                | `pq callers Foo`            |
| What is in this file?          | `pq file path/to/x.go`      |
| How is this area organised?    | `pq tree server/internal`   |
| Free-text, last resort         | `pq grep 'pattern' [path]`  |

Procedure: `pq def` first; `pq callers` before any grep, then `pq grep NAME
<path>`; orient top-down (`tree` -> `api` -> `file` -> read one function).
Reading a whole package is a last resort. Do not reintroduce a code index
(graph, embeddings) without measuring it against `pq` first.

## Architecture

### Server (`server/`)

Go REST API over the Proxmox API + SQLite. Module `pvmss/server`, Go 1.27.
Routing is stdlib `net/http`, logging stdlib `log/slog`. Direct deps:
`coder/websocket` (VNC proxy), `golang.org/x/crypto` (bcrypt),
`gopkg.in/yaml.v3`, `modernc.org/sqlite` (CGO-free), OpenTelemetry (otel, sdk,
OTLP/HTTP exporters, `otelhttp`, `runtime`, Prometheus exporter) and
`prometheus/client_golang`. Do not add another logging or metrics library.

Binaries under `server/cmd/`: `pvmss` (the HTTP server), `pvmss-recover`
(recovery CLI), `pvmss-checklist` (checklist CLI).

Packages under `server/internal/`:

| Package      | Role                                                          |
| ------------ | ------------------------------------------------------------- |
| `httpapi/`   | HTTP handlers + routing for `/api/v1/*`, SPA serving, VNC WS  |
| `vm/`        | VM domain logic - resolve, query, actions, cross-cluster      |
| `cluster/`   | Cluster clients (`proxmox` and `fake` sources), multi-cluster |
| `store/`     | SQLite persistence                                            |
| `inventory/` | Background inventory refresh + cache                          |
| `catalog/`   | Approved nodes/ISOs/storages/bridges, cloud-init templates    |
| `policy/`    | Limits, quotas, authorization policy                          |
| `pools/`     | Proxmox pool handling                                         |
| `recovery/`  | Recovery runs and fixtures                                    |
| `checklist/` | Operational checklist walkthroughs                            |
| `auth/`      | Sessions, password hashing, admin auth                        |
| `cloudinit/` | Cloud-init document validation + slug helpers                 |
| `config/`    | Env-based configuration, validation, slog logger, redaction   |
| `telemetry/` | OTel tracing and metrics, Prometheus `/metrics`               |
| `logctx/`    | Request-scoped log attributes (requestId, user, span IDs)     |

#### Cloud-init (paused since 2026-10-20)

Cloud-init is not reliable enough yet. `cloudInitPaused` in
`server/internal/httpapi/router.go` leaves the admin template/baseline routes
and per-VM cloud-init writes unregistered, and `POST /api/v1/vms` refuses
`cloudInitTemplateId`. The web hides every cloud-init control and redirects
`/admin/cloudinit-templates` and `/admin/baseline`.

When enabled: admins author templates per cluster. **PVMSS never writes on the
nodes** (the Proxmox API cannot upload snippets; SSH publishing was removed).
The admin page shows a content-addressed file and a copy-paste command
(`catalog.WriteCommand`) the admin runs on the nodes. Presence is read live via
`cluster.SnippetChecker`; VM creation re-checks `HasSnippet` before setting
`cicustom`. Users cannot write cloud-init YAML. Operator procedure:
`docs/cloud-init.md`.

### Web (`web/`)

SvelteKit SPA: Svelte 5 runes, TypeScript, Tailwind CSS v4, `adapter-static`,
built with bun and served by the Go binary (catch-all to `index.html`).

- `src/routes/` - pages: `vms/`, `nodes/`, `activity/`, `search/`, `docs/`, `about/`, `admin/`, `profile/`, `login/`
- `src/lib/features/` - feature modules (stores + components per domain)
- `src/lib/shared/` - API client and utilities; `shared/ui/` - in-house UI components (no shadcn/bits-ui)
- `messages/` (`en.json`, `fr.json`) + `project.inlang/` - i18n via Paraglide; `src/lib/paraglide/` is generated
- `src/test/` - vitest setup; `e2e/` - Playwright specs

### Deployment

- Port 50000 (`PVMSS_PORT`)
- Image `gcr.io/distroless/static-debian13:nonroot` (uid 65532), entrypoint
  `/app/pvmss`, web dir from `PVMSS_WEB_DIR` (default `/app/web/build`)
- Multi-stage build: `golang:1.27.2-alpine` + `oven/bun:1-alpine`
- Helm chart: `helm/`

## Configuration

Environment variables only, validated at startup by
`server/internal/config/load.go` (fails fast).

**Required:**

| Variable               | Notes                                                       |
| ---------------------- | ----------------------------------------------------------- |
| `PVMSS_PORT`           | Integer 1-65535                                             |
| `PVMSS_DB_PATH`        | SQLite file path (image default `/data/pvmss.db`)           |
| `SESSION_SECRET`       | 32+ bytes                                                   |
| `LOG_LEVEL`            | `debug` \| `info` \| `warn` \| `error` - **lowercase only** |
| `LOG_FORMAT`           | `json` \| `console`                                         |
| `LOG_OUTPUT`           | `stdout` \| `stderr` \| a file path                         |
| `PVMSS_CLUSTER_SOURCE` | `fake` \| `proxmox` - no default on purpose                 |

`PVMSS_CLUSTER_SOURCE` has no default because `fake` ships hardcoded demo
credentials (`admin@pve` / `pvmss-admin`).

**Required when `PVMSS_CLUSTER_SOURCE=proxmox`:** `PROXMOX_URL`,
`PROXMOX_API_TOKEN_NAME`, `PROXMOX_API_TOKEN_VALUE`. They seed the `default`
cluster on first boot only; afterwards the admin UI owns clusters.

**Optional:**

| Variable                                      | Default                                                         |
| --------------------------------------------- | --------------------------------------------------------------- |
| `PVMSS_HOST`                                  | `127.0.0.1` (image sets `0.0.0.0`)                              |
| `PVMSS_WEB_DIR`                               | relative to the executable                                      |
| `ADMIN_PASSWORD_HASH`                         | empty; if set, must be `$2...`                                  |
| `PVMSS_COOKIE_SECURE`                         | `true`                                                          |
| `PVMSS_INVENTORY_REFRESH_INTERVAL`            | `30s`                                                           |
| `PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL` | `5s`                                                            |
| `PVMSS_INVENTORY_REFRESH_TIMEOUT`             | `15s`                                                           |
| `PVMSS_MAX_LIST_PAGE_SIZE`                    | `100`                                                           |
| `PVMSS_DB_RESET_ON_INCOMPATIBLE`              | `false` (dev compose: `true`; old DB is renamed, never deleted) |
| `PVMSS_TRUSTED_PROXY_HOPS`                    | `1`                                                             |
| `PVMSS_RATE_LIMIT_MAX`                        | `0` (built-in ceilings; positive raises them, used by e2e)      |
| `PVMSS_METRICS_PORT`                          | empty (off); a port != `PVMSS_PORT` enables `GET /metrics`      |

Tracing/OTLP use the standard `OTEL_*` variables read by the SDK (see
`docs/observability.md`). `LOG_LEVEL` is only the startup level; admins can
change it at runtime (Admin > Settings).

No longer read (startup warns for the SSH ones): `PVMSS_OFFLINE`, `PVMSS_ENV`,
`JWT_SECRET`, `PROXMOX_VERIFY_SSL`, `LOG_FILE_PATH`, `PVMSS_SSH_KEY_FILE`,
`PVMSS_SSH_USER`, `PVMSS_SSH_PORT`.

## Logging and observability

Binding for all server code. Operator detail lives in `docs/observability.md`.

| Level   | Meaning                                                       | Examples                                                |
| ------- | ------------------------------------------------------------- | ------------------------------------------------------- |
| `Error` | Broken, needs a human                                         | a 5xx, a DB failure, a cluster becoming unreachable     |
| `Warn`  | Degraded, or an expected refusal worth noticing               | failed login, 403, CSRF reject, 429, deprecated env var |
| `Info`  | App lifecycle, user actions (audit mirror), state transitions | startup banner, login, VM action, cluster recovered     |
| `Debug` | Diagnostics                                                   | each Proxmox call, each successful refresh, GET requests |

- **Log once, where the error is handled.** A handler that returns a 4xx/5xx
  calls `httpapi.SetError(w, err)` / `SetErrorMsg(w, msg, err)` and does not
  log; the access log carries `error`
  (`TestHandlersDoNotLogErrorThenFailTheRequest` enforces it). A handler that
  swallows an error and still answers 2xx logs it at `Warn`.
- **In request code use `logctx.From(ctx)`** (or `logctx.FromOr`) with the
  `*Context` log methods.
- **Never log** request bodies, headers, raw URLs or query strings. Secret keys
  are masked centrally as a safety net only.
- **Messages**: lowercase, no trailing period, object + verb, static; variable
  data goes in attrs.
- **Fields (camelCase):** `component`, `event`, `cluster`, `node`, `vmid`,
  `user`, `clientIp`, `requestId`, `traceId`, `spanId`, `method`, `route`,
  `status`, `bytes`, `durationMs`, `error`, `action`, `result` (domain extras
  allowed).
- **Audit rows are logged automatically** by `store.insertAuditRow`; don't add
  a second line.
- **Metric labels**: only `route`, `method`, `status_class`, `cluster`,
  `action`, `result`, `status` (+ otelhttp semconv). Never `vmid`, `user`,
  `node`.
- Enforced by `sloglint` in `server/.golangci.yml`.

## CI

`.github/workflows/`: `v0.4.yml` (server build + race tests, web
check/lint/build/vitest, Playwright; pushes to `main`/`v0.4`, PRs to `main`),
`lint.yml` (Super-Linter + golangci-lint, every push), `codeql.yml`. Go version
from `server/go.mod`, bun pinned to `1.3.13`.

## Testing Notes

- `make server-test` needs no Proxmox: tests use the `fake` cluster source.
- `cd web && bun run test:coverage` for web coverage.
- Run `make web-check` and `bun run build` **sequentially**: both regenerate
  SvelteKit/Paraglide artifacts, and concurrent runs can produce a blank SPA.

## Workflow

- **Planning/TDD/review**: use the skills flow `/grill-with-docs` ->
  `/to-spec` -> `/to-tickets` -> `/implement` (which drives `/tdd` and
  `/code-review` per ticket). Do not also run generic planner / tdd-guide /
  code-reviewer agents on top. Use `/diagnosing-bugs` for bug reports.
- **Skills** live in `.devin/skills/` (golang-*, svelte-code-writer,
  tailwind-design-system, typescript-advanced-types, todo-planning, ...).
  Invoke the matching one at the start of matching work, e.g.
  `svelte-code-writer` for any `.svelte` file.
- **Rules**: `.devin/rules/coding-style.md` (Go/TS) and
  `.devin/rules/ui-quality.md` (admin pages, forms).

## Project Conventions

- Admin features are SvelteKit routes under `web/src/routes/admin/`, backed by
  the `admin_*.go` handlers in `server/internal/httpapi/`, which must return
  the complete payload the admin UI needs.
- English for code and docs. UI translations: EN + FR.
- **No em-dash (U+2014)** in authored source or docs; use ` - `. Generated
  trees (node_modules, paraglide, build, .svelte-kit) are exempt.

## Agent Memory

- `.agents/memory/` (gitignored): one append-only file per work session,
  `YYYY-MM-DD-<slug>.md` (suffix `-2`, `-3` for same day), with sections
  **What**, **Where**, **Why**, **Key insight**, **Verification performed**,
  **Verdict**. Read recent entries before working in an area.
- Obsidian vault (outside the repo):
  `/Users/jh/Library/Mobile Documents/iCloud~md~obsidian/Documents/JhoBoxes/Projets/PVMSS`
  (start at `PVMSS Home.md`). Mirrors architecture, API, data model and
  workflows; cheaper than re-deriving an unfamiliar area.
