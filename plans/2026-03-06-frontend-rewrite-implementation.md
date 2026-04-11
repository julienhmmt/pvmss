# Frontend Rewrite: Implementation Plan

**Design:** [2026-03-06-frontend-rewrite-design.md](./2026-03-06-frontend-rewrite-design.md)

## Phase 1: Foundation

### 1.1 Scaffold SvelteKit Project

- [ ] `npm create svelte@latest frontend -- --template skeleton --types typescript`
- [ ] Install adapter-static: `npm i -D @sveltejs/adapter-static`
- [ ] Configure `svelte.config.js` with adapter-static and SPA fallback
- [ ] Configure `vite.config.ts` with API proxy to `localhost:50000`

### 1.2 shadcn-svelte + Mira Preset

- [ ] `npx shadcn-svelte@latest init` (stone, orange, geist, phosphor, small radius)
- [ ] Add core components: button, card, dialog, form, input, select, tabs, badge, dropdown-menu, sidebar, sheet, sonner, table, data-table, separator, avatar, tooltip, skeleton
- [ ] Verify Geist font loading (via `@fontsource/geist-sans` + `@fontsource/geist-mono`)
- [ ] Verify Phosphor icons (`phosphor-svelte`)
- [ ] Test dark mode toggle works

### 1.3 API Client

- [ ] Create `src/lib/api/client.ts` — base fetch wrapper with:
  - JWT Bearer header injection
  - 401 → automatic refresh via `/api/v1/auth/refresh`
  - Typed error handling (`ApiError`)
  - Request/response JSON serialization
- [ ] Create `src/lib/api/auth.ts` — login, logout, refresh, me, updatePassword
- [ ] Create `src/lib/types/api.ts` — ApiError, ApiResponse
- [ ] Create `src/lib/types/auth.ts` — User, LoginRequest, LoginResponse

### 1.4 Auth Store + Login Page

- [ ] Create `src/lib/stores/auth.svelte.ts` — Svelte 5 runes store:
  - `$state` for user, token, isAuthenticated, isAdmin
  - `login()`, `logout()`, `refresh()`, `init()` methods
  - Auto-refresh on app init (check refresh cookie)
- [ ] Create `src/routes/login/+page.svelte` — login form with shadcn Form/Input/Button
- [ ] Create `src/lib/components/layout/AuthGuard.svelte` — redirect to /login if not auth'd
- [ ] Create `src/lib/components/layout/AdminGuard.svelte` — redirect to / if not admin
- [ ] Create `src/routes/+layout.svelte` — root layout with auth init

### 1.5 Go Backend: JWT Refactor

- [ ] Add refresh token support to `/api/v1/auth/login` (httpOnly cookie)
- [ ] Add `POST /api/v1/auth/refresh` endpoint (reads httpOnly cookie, returns new access token)
- [ ] Update `POST /api/v1/auth/logout` to clear refresh cookie
- [ ] Remove session-to-JWT exchange endpoint (`/api/v1/auth/exchange`)
- [ ] Add CORS middleware for Vite dev server (`localhost:5173`)

### 1.6 Go Backend: SPA Static Server

- [ ] Add static file handler serving `frontend/build/` (or embedded)
- [ ] Add catch-all route: non-API, non-static paths → `index.html`
- [ ] Ensure `/api/v1/*` routes take priority over catch-all

---

## Phase 2: Core User Pages

### 2.1 Layout Components

- [ ] Create `Navbar.svelte` — app name, navigation links, user menu (profile, logout), dark mode toggle
- [ ] Create `Footer.svelte` — version info
- [ ] Create `PageHeader.svelte` — title, icon, optional action buttons
- [ ] Wire into `+layout.svelte`

### 2.2 Home / VM Dashboard

- [ ] Create `src/lib/types/vm.ts` — VM, VMStatus, VMAction, Disk, NetworkCard interfaces
- [ ] Create `src/lib/api/vms.ts` — listVMs, getVM, createVM, deleteVM, vmAction, update methods
- [ ] Create `src/lib/stores/vms.svelte.ts` — VM list, loading, error, fetch, action methods
- [ ] Create `VmCard.svelte` — card with name, node, status badge, CPU/RAM/Disk bars
- [ ] Create `VmStatusBadge.svelte` — running (green), stopped (red), paused (yellow)
- [ ] Create `VmActionButtons.svelte` — start/stop/shutdown/reboot filtered by status
- [ ] Create `routes/+page.svelte` — grid of VmCards, empty state, loading skeletons
- [ ] Go: ensure `GET /api/v1/vms` returns full VM data (CPU, RAM, Disk metrics)

### 2.3 VM Details Page

- [ ] Create `routes/vm/[id]/+page.svelte` with sections:
  - Status card with auto-refreshing metrics (CPU, RAM, Disk, uptime)
  - Description (inline edit)
  - Tags (inline edit)
  - Disk list
  - Network cards (with toggle)
  - Cloud-Init data viewer
  - Snapshot manager
  - Action buttons (start/stop/shutdown/reboot/delete)
- [ ] Create `VmDetailsMetrics.svelte` — auto-refresh via `$effect` + `setInterval`, pause on visibility change
- [ ] Create `VmNetworkCards.svelte` — NIC list with toggle, MAC, VLAN, IP
- [ ] Create `VmDiskList.svelte` — disk bus, storage, size display
- [ ] Create `VmSnapshotManager.svelte` — list, create, delete, rollback with confirmation dialogs
- [ ] Go: `GET /api/v1/vms/:id` returns full details (disks, networks, cloudinit, snapshots)
- [ ] Go: `PUT /api/v1/vms/:id/description`, `PUT /api/v1/vms/:id/tags`, `PUT /api/v1/vms/:id/resources`
- [ ] Go: `POST /api/v1/vms/:id/network/:iface/toggle`
- [ ] Go: snapshot CRUD endpoints under `/api/v1/vms/:id/snapshots/`

### 2.4 VM Creation Wizard

- [ ] Create `routes/vm/create/+page.svelte` — multi-step form with shadcn Tabs:
  - Step 1: Basic (name, description, VMID, node, pool)
  - Step 2: Hardware (CPU sockets/cores, memory, EFI, TPM)
  - Step 3: Disks (multi-disk config: bus, storage, size)
  - Step 4: Network (multi-NIC config: bridge, VLAN, MAC, model)
  - Step 5: Boot (ISO or Cloud-Init template selection)
  - Review & Create
- [ ] Create `VmCreateWizard.svelte` — step navigation, validation per step
- [ ] Client-side validation with shadcn Form + superforms or custom
- [ ] Go: `POST /api/v1/vms` — create VM from JSON body
- [ ] Go: `GET /api/v1/settings` — returns available nodes, storages, ISOs, VMBRs, cloud-init templates, limits
- [ ] Go: validation endpoints (VMID availability, name, VLAN)

### 2.5 Search Page

- [ ] Create `routes/search/+page.svelte` — search input with debounce, filter selector (vmid/name/tags)
- [ ] Results as VmCard grid
- [ ] Go: `GET /api/v1/search/vms?q=&filter=`

### 2.6 Profile Page

- [ ] Create `routes/profile/+page.svelte`:
  - User info card (username, pool, VM count)
  - VM list (reuse VmCard)
  - Password change form
- [ ] Go: `GET /api/v1/me` already exists
- [ ] Go: `PUT /api/v1/auth/me/password`

---

## Phase 3: VM Console

### 3.1 noVNC Integration

- [ ] `npm i @novnc/novnc`
- [ ] Create `VmConsole.svelte`:
  - Full-page layout (minimal chrome)
  - Fetch VNC ticket: `POST /api/v1/vms/:id/console/vnc-ticket`
  - Open WebSocket to Go backend proxy
  - Initialize `RFB` instance targeting canvas
  - Connection state UI (connecting, connected, disconnected, error)
  - Toolbar: clipboard sync, scaling, fullscreen, disconnect
- [ ] Create `routes/vm/[id]/console/+page.svelte` — mounts VmConsole
- [ ] Go: `POST /api/v1/vms/:id/console/vnc-ticket` — get Proxmox VNC ticket
- [ ] Go: `WS /api/v1/vms/:id/console/websocket` — proxy WebSocket to Proxmox

---

## Phase 4: Admin Pages

### 4.1 Admin Layout

- [ ] Create `routes/admin/+layout.svelte` — sidebar (shadcn Sidebar) + AdminGuard
- [ ] Sidebar links: Dashboard, Nodes, Storage, VMs, Pools, Tags, Limits, VMBR, Cloud-Init, ISO, App Info
- [ ] Create `src/lib/api/admin/` — all admin API functions

### 4.2 Admin Dashboard

- [ ] `routes/admin/+page.svelte` — overview cards (node count, VM count, storage usage)

### 4.3 Admin Pages (each follows same pattern: fetch data → render table/cards → CRUD dialogs)

- [ ] Nodes — `NodeCard` grid with status, CPU, memory, uptime
- [ ] Storage — DataTable with name, type, total/used/free
- [ ] VMs (all) — DataTable with VMID, name, node, status, owner, actions
- [ ] Pools — PoolManager: list, create dialog, delete with confirmation
- [ ] Tags — TagManager: list, create, delete
- [ ] Limits — LimitsEditor: form with CPU, RAM, disk limits
- [ ] VMBR — DataTable with bridge name, VLAN-aware, ports
- [ ] Cloud-Init — CloudInitManager: list, create/edit dialog, delete, toggle enabled
- [ ] ISO — DataTable with filename, size, storage
- [ ] App Info — diagnostic cards (version, environment, Proxmox connection, settings)

### 4.4 Go: Admin API Endpoints

- [ ] Add admin middleware: verify JWT `isAdmin` claim
- [ ] Implement all `/api/v1/admin/*` endpoints (see design doc)
- [ ] Reuse existing Proxmox client methods, just return JSON instead of rendering templ

---

## Phase 5: Polish & Cutover

### 5.1 UX Polish

- [ ] Loading skeletons on all pages (shadcn Skeleton)
- [ ] Toast notifications for all actions (shadcn Sonner)
- [ ] Error boundaries with friendly messages
- [ ] Responsive design: mobile navbar (sheet), card grid breakpoints
- [ ] Dark mode: verify all pages look correct
- [ ] Keyboard navigation and accessibility audit

### 5.2 Cutover

- [ ] Move `frontend/` → `frontend-legacy/`
- [ ] Update Makefile: `make frontend-build`, `make dev` runs both
- [ ] Update `docker-compose.dev.yml` for new build
- [ ] Update Dockerfile to include `frontend/build/`
- [ ] Remove dead Go code:
  - [ ] All templ components (`backend/components/`)
  - [ ] Template loading (`backend/templates/`)
  - [ ] Session middleware, CSRF middleware
  - [ ] Form-based handlers (replaced by API handlers)
  - [ ] Old static file routes (`/css/*`, `/js/*`, `/webfonts/*`, `/vendor/*`, `/src/*`)
- [ ] Update `CLAUDE.md` with new architecture
- [ ] Update `README.md`

### 5.3 Testing

- [ ] Go: API integration tests for all new endpoints (offline mode)
- [ ] Go: route accessibility tests updated for new API routes
- [ ] Frontend: consider Playwright e2e tests for critical flows (login, VM create, VM details)
