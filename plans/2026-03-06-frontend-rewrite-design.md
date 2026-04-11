# Frontend Rewrite: Go Templ/HTMX to SvelteKit SPA

**Date:** 2026-03-06
**Status:** Approved

## Summary

Replace the existing Go templ + Alpine.js + HTMX + Vue 3 hybrid frontend with a single SvelteKit SPA using TypeScript, shadcn-svelte (Mira preset), and a full REST API backend.

## Decisions

| Decision        | Choice                      | Rationale                                                  |
| --------------- | --------------------------- | ---------------------------------------------------------- |
| Architecture    | Full SPA (adapter-static)   | Internal tool, no SEO needed, keeps single-binary deploy   |
| API             | Full REST `/api/v1/*`       | Clean SPA needs complete JSON API surface                  |
| Auth            | JWT-only                    | No server-rendered pages need sessions; simplifies backend |
| noVNC           | npm package `@novnc/novnc`  | Same library, properly bundled by Vite                     |
| i18n            | English-only (deferred)     | Simplifies initial migration                               |
| Migration       | Big bang                    | Auth model changes fundamentally; coexistence too complex  |
| Admin           | Same SPA, guarded routes    | One app, one build, route-level protection                 |
| Legacy frontend | Moved to `frontend-legacy/` | Preserved for reference during migration                   |

## Design System

- **Framework:** shadcn-svelte with Mira preset
- **Base color:** Stone
- **Accent color:** Orange
- **Font:** Geist (sans + mono)
- **Icons:** Phosphor
- **Radius:** Small
- **Dark mode:** Built-in via shadcn theme toggle

Reference: <https://ui.shadcn.com/create?base=base&style=mira&baseColor=stone&theme=orange&iconLibrary=phosphor&font=geist&menuAccent=subtle&menuColor=default&radius=small&item=preview>

## Architecture

```bash
┌─────────────────────────────────────┐
│  SvelteKit SPA (adapter-static)     │
│  Vite + TypeScript                  │
│  shadcn-svelte (Mira preset)        │
│  stone/orange · Geist · Phosphor    │
├─────────────────────────────────────┤
│  fetch → /api/v1/* (JWT Bearer)     │
├─────────────────────────────────────┤
│  Go backend (API-only + static)     │
│  Serves built SPA from dist/        │
│  All routes → index.html (SPA)      │
│  /api/v1/* → JSON handlers          │
└─────────────────────────────────────┘
```

### Auth Flow

1. User submits login form → `POST /api/v1/auth/login`
2. Backend returns `{ access_token }` + sets `refresh_token` as httpOnly cookie
3. SPA stores access token in memory (not localStorage)
4. All API calls include `Authorization: Bearer <token>`
5. On 401 → silent refresh via `POST /api/v1/auth/refresh` (sends httpOnly cookie)
6. On refresh failure → redirect to `/login`

### Request Flow

1. Browser loads `index.html` from Go static file server
2. SvelteKit client-side router handles all navigation
3. Each page fetches data from `/api/v1/*` endpoints
4. Go backend validates JWT, calls Proxmox API, returns JSON
5. Catch-all route in Go serves `index.html` for any non-API, non-static path

## Frontend Directory Structure

```bash
frontend/
  src/
    lib/
      api/                    # Typed fetch-based API client
        client.ts             # Base client: JWT headers, refresh logic, error handling
        auth.ts               # login, logout, refresh, me, updatePassword
        vms.ts                # list, get, create, delete, action, updateDesc/tags/resources
        snapshots.ts          # list, create, delete, rollback
        console.ts            # VNC ticket, WebSocket URL builder
        search.ts             # VM search with filters
        admin/
          nodes.ts
          storage.ts
          vms.ts
          pools.ts
          tags.ts
          limits.ts
          vmbr.ts
          cloudinit.ts
          iso.ts
          settings.ts
      components/
        ui/                   # shadcn-svelte generated components
        vm/
          VmCard.svelte       # VM list card (name, node, status, metrics)
          VmStatusBadge.svelte
          VmActionButtons.svelte
          VmConsole.svelte    # noVNC wrapper component
          VmCreateWizard.svelte
          VmDetailsMetrics.svelte
          VmSnapshotManager.svelte
          VmNetworkCards.svelte
          VmDiskList.svelte
        admin/
          AdminSidebar.svelte
          NodeCard.svelte
          StorageCard.svelte
          PoolManager.svelte
          TagManager.svelte
          LimitsEditor.svelte
          CloudInitManager.svelte
          IsoManager.svelte
        layout/
          Navbar.svelte
          Footer.svelte
          PageHeader.svelte
          AuthGuard.svelte    # Redirects to /login if not authenticated
          AdminGuard.svelte   # Redirects to / if not admin
      stores/
        auth.svelte.ts        # Svelte 5 runes: user, isAdmin, token, login/logout
        vms.svelte.ts         # VM list, loading, error, fetch/action methods
        settings.svelte.ts    # App settings cache
      types/
        vm.ts                 # VM, VMStatus, VMAction, VMConfig, Disk, NetworkCard
        auth.ts               # User, LoginRequest, LoginResponse, Token
        admin.ts              # Node, Storage, Pool, Tag, Limits, VMBR, CloudInit, ISO
        settings.ts           # Settings, AppInfo
        api.ts                # ApiError, PaginatedResponse, ApiResponse
      utils/
        format.ts             # Byte/memory/uptime formatters
        validate.ts           # Client-side validation helpers
        constants.ts          # Status colors, action labels
    routes/
      +layout.svelte          # Root: Navbar, Footer, auth init, Sonner toasts
      +layout.ts              # Client-side auth check
      +page.svelte            # Home / VM dashboard
      +error.svelte           # Error page (404, 500)
      login/
        +page.svelte          # Login form
      vm/
        create/+page.svelte   # VM creation wizard (multi-step)
        [id]/
          +page.svelte        # VM details & management
          +page.ts            # Load VM data
          console/
            +page.svelte      # noVNC console (full-screen)
      search/
        +page.svelte          # VM search with filters
      profile/
        +page.svelte          # User profile + password change
      admin/
        +layout.svelte        # Admin layout with sidebar
        +page.svelte          # Admin dashboard / overview
        nodes/+page.svelte
        storage/+page.svelte
        vms/+page.svelte
        pools/+page.svelte
        tags/+page.svelte
        limits/+page.svelte
        vmbr/+page.svelte
        cloudinit/+page.svelte
        iso/+page.svelte
        appinfo/+page.svelte
    app.html
    app.css                   # Tailwind imports + shadcn theme
  static/
    favicon.ico
  svelte.config.js            # adapter-static, SPA fallback
  vite.config.ts
  tailwind.config.ts
  components.json             # shadcn-svelte Mira preset config
  tsconfig.json
  package.json
```

## Backend API Surface

### Auth (`/api/v1/auth/`)

| Method | Path           | Body / Params            | Response                                           | Notes                 |
| ------ | -------------- | ------------------------ | -------------------------------------------------- | --------------------- |
| POST   | `/login`       | `{ username, password }` | `{ access_token, user }` + httpOnly refresh cookie |                       |
| POST   | `/refresh`     | (httpOnly cookie)        | `{ access_token }`                                 | Silent refresh        |
| POST   | `/logout`      | —                        | 204                                                | Clears refresh cookie |
| GET    | `/me`          | —                        | `{ username, isAdmin, pool }`                      | JWT required          |
| PUT    | `/me/password` | `{ current, new }`       | 204                                                | JWT required          |

### VMs (`/api/v1/vms/`)

| Method | Path                             | Body / Params             | Response            | Notes                      |
| ------ | -------------------------------- | ------------------------- | ------------------- | -------------------------- |
| GET    | `/vms`                           | —                         | `[VM]`              | User's VMs                 |
| GET    | `/vms/:id`                       | —                         | `VM` (full details) |                            |
| POST   | `/vms`                           | `VMCreateRequest`         | `{ vmid, task }`    | Create VM                  |
| DELETE | `/vms/:id`                       | —                         | 204                 | Delete VM                  |
| POST   | `/vms/:id/action`                | `{ action }`              | `{ task }`          | start/stop/shutdown/reboot |
| PUT    | `/vms/:id/description`           | `{ description }`         | 204                 |                            |
| PUT    | `/vms/:id/tags`                  | `{ tags[] }`              | 204                 |                            |
| PUT    | `/vms/:id/resources`             | `{ cores, memory, disk }` | 204                 |                            |
| POST   | `/vms/:id/network/:iface/toggle` | —                         | 204                 | Enable/disable NIC         |

### Snapshots (`/api/v1/vms/:id/snapshots/`)

| Method | Path                                | Body / Params           | Response     |
| ------ | ----------------------------------- | ----------------------- | ------------ |
| GET    | `/vms/:id/snapshots`                | —                       | `[Snapshot]` |
| POST   | `/vms/:id/snapshots`                | `{ name, description }` | `{ task }`   |
| DELETE | `/vms/:id/snapshots/:name`          | —                       | `{ task }`   |
| POST   | `/vms/:id/snapshots/:name/rollback` | —                       | `{ task }`   |

### Console (`/api/v1/vms/:id/console/`)

| Method | Path                          | Body / Params         | Response           |
| ------ | ----------------------------- | --------------------- | ------------------ |
| POST   | `/vms/:id/console/vnc-ticket` | —                     | `{ ticket, port }` |
| WS     | `/vms/:id/console/websocket`  | query: `ticket, port` | WebSocket stream   |

### Search (`/api/v1/search/`)

| Method | Path          | Params                        | Response |
| ------ | ------------- | ----------------------------- | -------- |
| GET    | `/search/vms` | `?q=&filter=vmid\|name\|tags` | `[VM]`   |

### Admin (`/api/v1/admin/`) — all require `isAdmin` JWT claim

| Method | Path                          | Body / Params       | Response              |
| ------ | ----------------------------- | ------------------- | --------------------- |
| GET    | `/admin/nodes`                | —                   | `[Node]`              |
| GET    | `/admin/storage`              | —                   | `[Storage]`           |
| GET    | `/admin/vms`                  | —                   | `[VM]` (all VMs)      |
| GET    | `/admin/pools`                | —                   | `[Pool]`              |
| POST   | `/admin/pools`                | `{ name, ... }`     | `Pool`                |
| DELETE | `/admin/pools/:id`            | —                   | 204                   |
| GET    | `/admin/tags`                 | —                   | `[Tag]`               |
| POST   | `/admin/tags`                 | `{ name }`          | `Tag`                 |
| DELETE | `/admin/tags/:name`           | —                   | 204                   |
| GET    | `/admin/limits`               | —                   | `Limits`              |
| PUT    | `/admin/limits`               | `Limits`            | 204                   |
| GET    | `/admin/vmbr`                 | —                   | `[VMBR]`              |
| GET    | `/admin/cloudinit`            | —                   | `[CloudInitTemplate]` |
| POST   | `/admin/cloudinit`            | `CloudInitTemplate` | `CloudInitTemplate`   |
| PUT    | `/admin/cloudinit/:id`        | `CloudInitTemplate` | 204                   |
| DELETE | `/admin/cloudinit/:id`        | —                   | 204                   |
| POST   | `/admin/cloudinit/:id/toggle` | —                   | 204                   |
| GET    | `/admin/iso`                  | —                   | `[ISO]`               |
| GET    | `/admin/settings`             | —                   | `Settings`            |
| GET    | `/admin/appinfo`              | —                   | `AppInfo`             |

### Health (public, no auth)

| Method | Path                     | Response              |
| ------ | ------------------------ | --------------------- |
| GET    | `/api/v1/health`         | `{ status, version }` |
| GET    | `/api/v1/health/proxmox` | `{ connected, url }`  |

## Pages & Features Mapping

### User Pages

| Page           | Current (templ)                | New (Svelte)                          | Key Components                                      |
| -------------- | ------------------------------ | ------------------------------------- | --------------------------------------------------- |
| Login          | `login.templ`                  | `routes/login/+page.svelte`           | shadcn Form, Input, Button                          |
| Home/Dashboard | `home.templ` + Vue SPA         | `routes/+page.svelte`                 | VmCard grid, VmActionButtons                        |
| VM Create      | `vm_create.templ` (giant form) | `routes/vm/create/+page.svelte`       | Multi-step wizard (Tabs/Stepper), Form validation   |
| VM Details     | `vm_details.templ` + Alpine    | `routes/vm/[id]/+page.svelte`         | Metrics auto-refresh, inline edit, snapshot manager |
| VM Console     | `vm-console.js` + noVNC        | `routes/vm/[id]/console/+page.svelte` | VmConsole component wrapping @novnc/novnc           |
| Search         | `search.templ` + Alpine        | `routes/search/+page.svelte`          | Debounced search, filter chips                      |
| Profile        | `profile.templ` + Alpine       | `routes/profile/+page.svelte`         | User info card, password form, VM list              |

### Admin Pages

| Page       | Current (templ)         | New (Svelte)                          | Key Components                   |
| ---------- | ----------------------- | ------------------------------------- | -------------------------------- |
| Dashboard  | `admin_layout.templ`    | `routes/admin/+page.svelte`           | Overview cards                   |
| Nodes      | `admin_nodes.templ`     | `routes/admin/nodes/+page.svelte`     | NodeCard grid, status indicators |
| Storage    | `admin_storage.templ`   | `routes/admin/storage/+page.svelte`   | DataTable                        |
| VMs (all)  | `admin_vms.templ`       | `routes/admin/vms/+page.svelte`       | DataTable with actions           |
| Pools      | `admin_userpool.templ`  | `routes/admin/pools/+page.svelte`     | PoolManager (CRUD)               |
| Tags       | `admin_tags.templ`      | `routes/admin/tags/+page.svelte`      | TagManager (create/delete)       |
| Limits     | `admin_limits.templ`    | `routes/admin/limits/+page.svelte`    | LimitsEditor form                |
| VMBR       | `admin_vmbr.templ`      | `routes/admin/vmbr/+page.svelte`      | DataTable                        |
| Cloud-Init | `admin_cloudinit.templ` | `routes/admin/cloudinit/+page.svelte` | CloudInitManager (CRUD + toggle) |
| ISO        | `admin_iso.templ`       | `routes/admin/iso/+page.svelte`       | DataTable                        |
| App Info   | `admin_appinfo.templ`   | `routes/admin/appinfo/+page.svelte`   | Diagnostic cards                 |

## noVNC Console Integration

The `VmConsole.svelte` component:

1. Calls `POST /api/v1/vms/:id/console/vnc-ticket` to get a ticket
2. Opens WebSocket to `/api/v1/vms/:id/console/websocket?ticket=...&port=...`
3. Initializes `@novnc/novnc` `RFB` class with the WebSocket
4. Renders to a canvas element filling the page
5. Handles connect/disconnect/error states with UI feedback
6. Supports clipboard sharing, keyboard grab, scaling options

## What Gets Removed

- `frontend/` → renamed to `frontend-legacy/` (preserved for reference)
- All `.templ` files in `backend/components/`
- Alpine.js, HTMX, Vue 3, Pinia, Axios (vendored ESM)
- Bulma CSS + 4,400 lines custom CSS
- Font Awesome webfonts (replaced by Phosphor)
- Session middleware (`backend/middleware/session.go`)
- CSRF middleware (`backend/middleware/csrf.go`, `backend/security/csrf.go`)
- Template loading (`backend/templates/`)
- All form-based handlers (replaced by API handlers)
- `frontend/vendor/` directory

## What Gets Added

- `frontend/` — full SvelteKit project
- Expanded `/api/v1/` handlers in `backend/api/v1/`
- Admin API middleware (JWT isAdmin check)
- Go static file server for SPA (`dist/` directory)
- SPA catch-all route handler in Go

## Build & Deploy

```bash
# Development
cd frontend && npm run dev    # Vite dev server with proxy to Go API
cd backend && go run main.go         # API server on :50000

# Production build
cd frontend && npm run build  # Outputs to dist/
# Go binary embeds dist/ or serves from filesystem
make build                           # Builds Go + copies dist/ into Docker
```

### Makefile Updates

- `make frontend-dev` — run Vite dev server
- `make frontend-build` — build SvelteKit SPA
- `make build` — build frontend + Go binary + Docker
- `make dev` — run both Vite dev + Go API (concurrent)

## Implementation Phases

### Phase 1: Foundation

- Scaffold SvelteKit project with shadcn-svelte Mira preset
- Set up Tailwind, Geist font, Phosphor icons
- Create API client with JWT auth flow
- Implement auth store + login page
- Go: refactor JWT auth (remove session exchange, add refresh token cookie)
- Go: SPA static file server + catch-all route

### Phase 2: Core User Pages

- Home/Dashboard with VM cards
- VM details page (metrics, inline edit, actions)
- VM creation wizard (multi-step form)
- Search page
- Profile page
- Go: expand `/api/v1/` for VM CRUD, details, search, profile

### Phase 3: VM Console

- VmConsole component with @novnc/novnc
- Go: console API endpoints (VNC ticket, WebSocket proxy)

### Phase 4: Admin Pages

- Admin layout with sidebar
- All admin pages (nodes, storage, pools, tags, limits, vmbr, cloudinit, iso, appinfo)
- Go: full admin API surface

### Phase 5: Polish & Cutover

- Error handling, loading states, toast notifications
- Dark mode toggle
- Responsive design pass
- Move `frontend/` → `frontend-legacy/`
- Update Docker build, CI, Makefile
- Remove dead Go code (templ handlers, session/CSRF middleware)

## TypeScript Types (Key Interfaces)

```typescript
interface VM {
  vmid: number;
  name: string;
  node: string;
  status: "running" | "stopped" | "paused";
  cpu: number; // usage fraction 0-1
  maxcpu: number; // core count
  mem: number; // bytes used
  maxmem: number; // bytes total
  disk: number; // bytes used
  maxdisk: number; // bytes total
  uptime: number; // seconds
  tags: string[];
  description: string;
  pool: string;
  disks: Disk[];
  networks: NetworkCard[];
  cloudinit?: CloudInitConfig;
}

interface VMCreateRequest {
  name: string;
  vmid?: number; // auto-assign if omitted
  node: string;
  pool: string;
  cores: number;
  sockets: number;
  memory: number; // MB
  disks: DiskConfig[];
  networks: NetworkConfig[];
  iso?: string;
  cloudinit?: CloudInitConfig;
  start: boolean;
  efi: boolean;
  tpm: boolean;
}

type VMAction = "start" | "stop" | "shutdown" | "reboot";

interface User {
  username: string;
  isAdmin: boolean;
  pool?: string;
  vmCount?: number;
}
```
