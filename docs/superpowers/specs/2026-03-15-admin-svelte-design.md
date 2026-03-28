# Design: Admin Frontend Modernization — Phase 1 (Admin-first SvelteKit SPA)

**Date**: 2026-03-15
**Status**: Approved
**Scope**: Admin pages only — Phase 1 of the full frontend rewrite
**Approach**: Full SvelteKit SPA scaffold, admin pages delivered first (Approach B)

---

## Context and objectives

The current frontend is a Go templ + Alpine.js + HTMX + Vue 3 hybrid with Bulma CSS. The goal is to replace it with a modern, fluid, modular SPA starting with the admin section.

**Phase 1 objectives:**

- Build a reusable component library (admin-first, then reused for user pages in a later phase)
- Make the application more reactive and visually modern
- Expose a complete JSON API for all admin operations
- Maintain exactly the same level of functionality as the current templ admin frontend

---

## Tech stack

| Element | Choice | Version |
| ------- | ------ | ------- |
| Framework | SvelteKit (adapter-static, SPA mode) | 2.x latest |
| Build tool | Vite | 5.x latest |
| Language | Svelte 5 + TypeScript | 5.x latest |
| Design system | shadcn-svelte (Mira preset) | latest |
| CSS | Tailwind CSS | v4 latest |
| Font | Geist (sans + mono) | latest |
| Icons | Phosphor | latest |
| Theme | Stone base / Orange accent | — |
| Border radius | Small | — |

### shadcn-svelte preset

```bash
npx shadcn-svelte@latest init --preset a1DMDThI
```

Pin the exact `shadcn-svelte` version used at scaffold time in `package.json` to prevent preset ID changes from breaking setup.

---

## Architecture

### Coexistence during the admin phase

The SvelteKit SPA and the legacy templ frontend coexist on the same Go server during this phase. Routing is handled at the top-level `ServeMux` level (not at the `httprouter` level) to avoid wildcard conflicts.

```bash
Go :50000
  /api/v1/admin/*    → new Go JSON admin handlers (JWT cookie, isAdmin)
  /api/v1/*          → existing JSON handlers (unchanged)
  /admin/assets/*    → SvelteKit static assets (file server)
  /admin/*           → frontend-svelte/build/index.html (SPA catch-all, via ServeMux)
  /css/*, /js/*...   → legacy static assets (unchanged)
  /* others          → existing templ handlers (unchanged)
```

**Important**: The `/admin/*` SPA catch-all is added to the top-level `http.ServeMux` in `handlers.go`, **not** as an `httprouter` wildcard route. This avoids the `httprouter` panic that occurs when a wildcard and concrete routes share the same prefix. The existing `httprouter` routes under `/admin/` (templ handlers) are removed as part of this work, since the SPA replaces them entirely.

### SvelteKit base path

`svelte.config.js` must set `paths.base = '/admin'` so that Vite generates asset references as `/admin/assets/[hash].js` rather than `/assets/[hash].js`. This is required for the file server and SPA fallback to work correctly under the `/admin` prefix.

```javascript
// svelte.config.js
import adapter from '@sveltejs/adapter-static'

export default {
  kit: {
    adapter: adapter({ fallback: 'index.html' }),
    paths: { base: '/admin' }
  }
}
```

### Auth flow (no JWT refactor in this phase)

JWT tokens are issued as **httpOnly cookies** by the existing backend. The SPA does not set `Authorization` headers — cookies are sent automatically with every same-origin request.

1. User logs in via the existing templ login page (`/login` or `/admin/login`)
2. On load of `/admin`, the SPA root layout calls `POST /api/v1/auth/exchange` once (single call at `onMount`, never on each navigation)
3. `Exchange` reads the active SCS session cookie → issues `access_token` cookie (15 min) + `refresh_token` cookie (7 days) as httpOnly cookies
4. All subsequent calls to `/api/v1/admin/*` carry the `access_token` cookie automatically
5. On 401 (access token expired): `client.ts` silently calls `POST /api/v1/auth/refresh`, which uses the `refresh_token` cookie to issue a new `access_token` cookie, then retries the original request
6. If refresh fails (session + refresh token both expired): redirect to `/admin/login`
7. `JWTAdminMiddleware` (already implemented in `backend/api/v1/middleware.go`) validates the `access_token` cookie and checks the `is_admin` claim

**Note**: `JWTAdminMiddleware` is **not a new file** — it already exists in `backend/api/v1/middleware.go`. It only needs to be wired to the new admin API routes in `router.go`.

---

## Component library

### Principle

- Components contain **no fetching logic** — pages handle data fetching, components handle display
- Props are **strictly typed TypeScript interfaces**
- Svelte 5 syntax: `$props()`, `$state()`, `$derived()`, `$effect()`, snippets — no legacy `export let`
- Two levels: UI primitives (shadcn-svelte) and reusable business components

### Level 1 — UI primitives (shadcn-svelte CLI generated)

```bash
src/lib/components/ui/
  button, card, dialog, table, form, input, select,
  badge, sidebar, sheet, sonner, skeleton, separator,
  tooltip, dropdown-menu, tabs, switch
```

### Level 2 — Reusable business components

```bash
src/lib/components/
  layout/
    AppShell.svelte         ← navbar + sidebar + main content slot
    AdminSidebar.svelte     ← admin navigation, auto active state from $page.url.pathname
    PageHeader.svelte       ← title + Phosphor icon + optional action button slot
    ThemeToggle.svelte      ← dark/light mode toggle
  data/
    DataTable.svelte        ← generic sortable/filterable table (TypeScript generics)
    ResourceCard.svelte     ← stat card (title, value, icon, sub-text)
    StatusBadge.svelte      ← colored badge for status (running/stopped/error/ok)
    EmptyState.svelte       ← empty state with icon + message + optional CTA
    LoadingSkeleton.svelte  ← loading skeleton
  forms/
    ConfirmDialog.svelte    ← confirmation dialog for destructive actions
    InlineEdit.svelte       ← click-to-edit inline field
    TagInput.svelte         ← multi-tag input with chips
  feedback/
    ErrorBanner.svelte      ← API error with message + retry button
```

### DataTable TypeScript contract

The `render` field uses a **Svelte 5 snippet** (not a string) to allow rendering components (StatusBadge, progress bars, action buttons) inside cells without `{@html}`:

```typescript
import type { Snippet } from 'svelte'

interface Column<T> {
  key: keyof T
  label: string
  sortable?: boolean
  render?: Snippet<[T[keyof T], T]>  // Svelte 5 snippet, not string
}

interface DataTableProps<T> {
  data: T[]
  columns: Column<T>[]
  loading?: boolean
  emptyMessage?: string
  onRowClick?: (row: T) => void
}
```

### Error response contract

All API error responses use the existing `ErrorResponse` envelope from `backend/types.go`:

```typescript
interface ApiError {
  code: string     // machine-readable error code
  message: string  // human-readable message
}
```

`client.ts` must parse this shape on non-2xx responses.

---

## Admin pages

### Shared layout

`AppShell` + `AdminSidebar` + `PageHeader`. Sidebar with Phosphor icons, auto active state. Dark mode toggle in the top navbar.

### 11 pages

| Page | Route | Display | Actions |
| ---- | ----- | ------- | ------- |
| Dashboard | `/admin` | `ResourceCard` aggregate: node count, VM count, total storage used, Proxmox status — data fetched in parallel from nodes + vms + storage endpoints | — |
| Nodes | `/admin/nodes` | Grid of `ResourceCard`: name, CPU%, RAM%, uptime, status | — (read-only) |
| Storage | `/admin/storage` | `DataTable`: name, type, total/used/free + progress bar (snippet) | — (read-only) |
| VMs | `/admin/vms` | `DataTable`: VMID, name, node, status badge (snippet), owner, pool | start/stop/reboot per row |
| User Pool | `/admin/userpool` | List of existing pools with users. Create form: pool name + Proxmox username + password (creates Proxmox user + assigns to pool simultaneously) | Create (dialog with name+user+password), delete (ConfirmDialog) |
| Tags | `/admin/tags` | Chip list + `TagInput` | Create, delete (with confirmation) |
| Limits | `/admin/limits` | Form: CPU/RAM/disk min-max, max VMs/user, max snapshots | Save (PUT) |
| VMBR | `/admin/vmbr` | `DataTable`: bridge name, VLAN-aware, ports | — (read-only) |
| Cloud-Init | `/admin/cloudinit` | `DataTable` + enabled/disabled toggle | Create, edit (dialog), delete, toggle |
| ISO | `/admin/iso` | `DataTable`: filename, size, storage | — (read-only) |
| App Info | `/admin/appinfo` | Diagnostic cards: version, env, Proxmox connection state, active settings | — (read-only) |

**Note**: The User Pool route uses `/admin/userpool` to match existing backend routes and sidebar links.

### Common UX patterns on every page

- **Loading**: `LoadingSkeleton` during initial fetch
- **Error**: `ErrorBanner` with retry
- **Empty**: `EmptyState` with contextual CTA
- **Destructive actions**: always via `ConfirmDialog` before execution
- **Feedback**: Sonner toast on every action (success + error)
- **Refresh**: automatic re-fetch of the list after every mutation

---

## Backend Go — new admin API endpoints

### Existing middleware (do not recreate)

`JWTAdminMiddleware` already exists in `backend/api/v1/middleware.go`. Wire it to the new admin routes in `router.go` using the existing `adminJWTWrap` helper pattern.

### New endpoints

```bash
GET  /api/v1/admin/nodes
GET  /api/v1/admin/storage
GET  /api/v1/admin/vms
POST /api/v1/admin/vms/:id/action        body: { action: "start"|"stop"|"reboot" }

GET    /api/v1/admin/userpool
POST   /api/v1/admin/userpool            body: { pool, username, password }
DELETE /api/v1/admin/userpool/:name      ← :name is the pool name (string), not a numeric ID — matches Proxmox pool key

GET    /api/v1/admin/tags
POST   /api/v1/admin/tags                body: { name }
DELETE /api/v1/admin/tags/:name

GET  /api/v1/admin/limits
PUT  /api/v1/admin/limits                body: Limits

GET  /api/v1/admin/vmbr

GET    /api/v1/admin/cloudinit
POST   /api/v1/admin/cloudinit
PUT    /api/v1/admin/cloudinit/:id
DELETE /api/v1/admin/cloudinit/:id
POST   /api/v1/admin/cloudinit/:id/toggle

GET  /api/v1/admin/iso
GET  /api/v1/admin/appinfo               ← version, env, Proxmox connection, active settings
GET  /api/v1/admin/settings              ← alias of appinfo for settings subset (kept for completeness)
```

### New Go files

```bash
backend/api/v1/
  admin_handlers.go     ← read-only handlers: nodes, storage, vmbr, iso, appinfo, settings
  admin_mutations.go    ← write handlers: userpool, tags, limits, cloudinit CRUD
  admin_vms.go          ← all-VMs list + per-VM action (admin scope)
  admin_mapper.go       ← Proxmox types → JSON response types
```

Existing handlers in `backend/handlers/` (the templ-based admin handlers) are **removed** in this phase since the SPA replaces them. Their underlying Proxmox client calls (in `backend/proxmox/`) are reused directly by the new API handlers.

### SPA serving in Go

Added to the top-level mux in `handlers.go` (not as httprouter routes):

```go
// Serve SvelteKit static assets
mux.Handle("/admin/assets/", http.StripPrefix("/admin/assets/", http.FileServer(http.Dir("frontend-svelte/build/assets"))))

// SPA fallback: all /admin/* paths → index.html
mux.HandleFunc("/admin/", func(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, "frontend-svelte/build/index.html")
})
```

`http.ServeMux` matches the most specific prefix first, so `/admin/assets/` and `/admin/` registrations win over the existing `"/"` catch-all handler without any changes to the internal `isStaticPath` or `isAPIPath` dispatch functions. No modifications to those helpers are required.

**`main.go` update required**: the hardcoded `"frontend"` static path constant must be updated or made configurable so it points to `frontend-svelte/build` in addition to (or instead of) the legacy `frontend/` path. The exact line to update is in `initTemplates()` — search for `filepath.Join(rootDir, "frontend")`.

---

## frontend-svelte/ structure

```bash
frontend-svelte/
  src/
    lib/
      api/
        client.ts               ← fetch wrapper: cookie-based auth, 401 → refresh → retry
        auth.ts                 ← exchange(), refresh(), me()
        admin/
          nodes.ts
          storage.ts
          vms.ts
          userpool.ts
          tags.ts
          limits.ts
          vmbr.ts
          cloudinit.ts
          iso.ts
          appinfo.ts
      components/
        ui/                     ← shadcn-svelte generated
        layout/
          AppShell.svelte
          AdminSidebar.svelte
          PageHeader.svelte
          ThemeToggle.svelte
        data/
          DataTable.svelte
          ResourceCard.svelte
          StatusBadge.svelte
          EmptyState.svelte
          LoadingSkeleton.svelte
        forms/
          ConfirmDialog.svelte
          InlineEdit.svelte
          TagInput.svelte
        feedback/
          ErrorBanner.svelte
      stores/
        auth.svelte.ts          ← Svelte 5 runes: username, isAdmin, initialized
        theme.svelte.ts         ← dark/light persisted in localStorage
      types/
        admin.ts                ← Node, Storage, VM, Pool, Tag, Limits, VMBR, CloudInit, ISO
        api.ts                  ← ApiError { code, message }, ApiResponse<T>
      utils/
        format.ts               ← formatBytes, formatCpu, formatUptime
    routes/
      +layout.svelte            ← root layout: exchange() once in onMount, theme init
      +layout.ts                ← no exchange call here (SPA mode: runs on every navigation)
      +page.svelte              ← stub (user pages, later phase)
      admin/
        +layout.svelte          ← AdminGuard (redirect /admin/login if not isAdmin) + AppShell
        +page.svelte            ← Admin dashboard
        nodes/+page.svelte
        storage/+page.svelte
        vms/+page.svelte
        userpool/+page.svelte
        tags/+page.svelte
        limits/+page.svelte
        vmbr/+page.svelte
        cloudinit/+page.svelte
        iso/+page.svelte
        appinfo/+page.svelte
    app.html
    app.css
  static/
    favicon.ico
  svelte.config.js              ← adapter-static, fallback: 'index.html', paths.base: '/admin'
  vite.config.ts                ← proxy /api/* → localhost:50000
  tailwind.config.ts
  components.json               ← shadcn-svelte Mira preset (pinned version)
  tsconfig.json
  package.json
```

### Auth store — single exchange at app init

```typescript
// src/routes/+layout.svelte
import { onMount } from 'svelte'
import { auth } from '$lib/stores/auth.svelte'

onMount(async () => {
  // Called once at app init, not on every navigation
  await auth.exchange()
  // If exchange fails → auth.exchange() redirects to /admin/login
})
```

---

## Build & development

### Local dev

```bash
# Terminal 1 — Go API
make dev-api              # go run ./backend — API on :50000

# Terminal 2 — Svelte
cd frontend-svelte
npm run dev               # Vite dev server on :5173
                          # proxy /api/* → localhost:50000
```

### Makefile — new targets

```makefile
frontend-install:
 cd frontend-svelte && npm ci

frontend-build:
 cd frontend-svelte && npm run build

frontend-dev:
 cd frontend-svelte && npm run dev

dev-api:
 go run ./backend

dev:
 # Use overmind or concurrently — not bare & (orphans processes on Ctrl+C)
 npx concurrently "make dev-api" "make frontend-dev"

build: frontend-build
 go build -o pvmss-backend ./backend
 docker build -t pvmss .
```

### Dockerfile

```dockerfile
FROM node:24-alpine AS frontend
WORKDIR /app
COPY frontend-svelte/package*.json ./
RUN npm ci
COPY frontend-svelte/ .
RUN npm run build

# ... existing Go builder stage (unchanged) ...

FROM gcr.io/distroless/static-debian13:nonroot AS final
# ... existing COPY for Go binary ...
COPY --from=frontend /app/build /app/frontend-svelte/build
```

### CORS

No CORS configuration is needed. In production, the SPA and the API are served from the same origin (`:50000`). The Vite dev proxy (`/api/* → :50000`) handles CORS during development without requiring backend CORS headers.

---

## What does not change in this phase

- `frontend/` remains intact and functional for user-facing pages (rename to `frontend-legacy/` is deferred to the user-pages phase to avoid breaking the hardcoded `"frontend"` path in `main.go`)
- All existing templ handlers for user pages (login, home, VM details, search, profile)
- Session auth middleware, CSRF, session cookies
- Existing `/api/v1/auth/*` endpoints (login, exchange, refresh, logout, me)
- Existing `/api/v1/vms` and `/api/v1/vms/:id` endpoints
- All existing Go tests (offline + integration)

**Existing templ-based admin handlers in `backend/handlers/`** (`admin_cloudinit.go`, `admin_nodes.go`, etc.) **are removed** since the SPA fully replaces them.

---

## Success criteria

- 100% of current admin pages available in the Svelte SPA with equivalent functionality
- User Pool page includes full user creation flow (pool name + Proxmox username + password)
- `DataTable`, `ResourceCard`, `ConfirmDialog`, `EmptyState` are generic enough to be reused for user pages without modification
- No regression on existing user-facing templ pages
- Dark mode works on all admin pages
- Responsive on desktop and tablet
