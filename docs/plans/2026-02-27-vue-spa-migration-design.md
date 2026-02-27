# PVMSS — Vue 3 SPA Migration + Backend API Layer

**Date:** 2026-02-27
**Scope:** Phase 1 — VM cards + action buttons

---

## Context

The current stack is: Go templ (SSR) + Alpine.js (reactivity) + htmx (partial updates) + Bulma CSS. The goal is to migrate to a Vue 3 + TypeScript SPA backed by a clean JSON API, keeping the Go backend as the sole server. Phase 1 establishes the infrastructure and delivers two components: `VmCard` and `VmActionButtons`.

---

## Architecture

### Approach: Split-auth parallel layers

A new `backend/api/v1/` package provides JWT-authenticated JSON endpoints alongside the existing templ handlers. The two layers share `StateManager` (for settings, proxmox client) but use separate auth middleware. Existing templ routes are untouched.

```
Browser
  │
  ├── GET /          → Go templ (session auth, unchanged)
  ├── GET /admin/*   → Go templ (session auth, unchanged)
  │
  ├── POST /api/v1/auth/login    → JWT issue (HttpOnly cookie)
  ├── GET  /api/v1/auth/me       → JWT validate → user info
  ├── GET  /api/v1/vms           → JWT validate → Resty → Proxmox
  ├── GET  /api/v1/vms/:id       → JWT validate → Resty → Proxmox
  ├── POST /api/v1/vms/:id/action → JWT validate → Resty → Proxmox
  │
  └── GET /assets/*  → Vite-built static files (dist/)
```

---

## Backend: `backend/api/v1/`

### New files

```
backend/api/v1/
  router.go       # RegisterAPIRoutes(router, state, restyClient)
  middleware.go   # JWTMiddleware — validates HttpOnly cookie, sets user in ctx
  auth.go         # POST /api/v1/auth/login, POST /api/v1/auth/refresh, GET /api/v1/auth/me, POST /api/v1/auth/logout
  vms.go          # GET /api/v1/vms, GET /api/v1/vms/:id
  vm_actions.go   # POST /api/v1/vms/:id/action
  types.go        # Typed request/response structs with json tags
  errors.go       # JSON error helpers: writeError(w, status, code, message)
```

### JWT design

- Library: `github.com/golang-jwt/jwt/v5`
- Access token: 15 min, stored in `access_token` HttpOnly SameSite=Strict cookie
- Refresh token: 7 days, stored in `refresh_token` HttpOnly SameSite=Strict cookie
- Claims: `{ sub: username, admin: bool, exp, iat }`
- Secret: read from `JWT_SECRET` env var (32+ bytes, validated at startup alongside existing `SESSION_SECRET`)

### Handler struct pattern

```go
type VMHandler struct {
    resty *proxmox.RestyClient  // Resty only — no Telmate
    state state.StateManager    // for settings (limits, storages, etc.)
}
```

New handlers use `RestyClient` directly. This advances the Telmate→Resty migration without touching existing handlers.

### Response types (examples)

```go
// types.go
type VMListResponse struct {
    VMs   []VMSummary `json:"vms"`
    Total int         `json:"total"`
}

type VMSummary struct {
    VMID   int     `json:"vmid"`
    Name   string  `json:"name"`
    Node   string  `json:"node"`
    Status string  `json:"status"`
    CPU    float64 `json:"cpu"`
    CPUs   int     `json:"cpus"`
    MemMB  int64   `json:"mem_mb"`
    MaxMemMB int64 `json:"max_mem_mb"`
}

type VMActionRequest struct {
    Action string `json:"action"` // start|stop|reboot|shutdown|reset
    Node   string `json:"node"`
}

type VMActionResponse struct {
    Success bool   `json:"success"`
    TaskID  string `json:"task_id,omitempty"`
    Message string `json:"message,omitempty"`
}
```

### Route registration

`RegisterAPIRoutes` is called from `main.go` after `handlers.InitHandlers()`. It mounts all `/api/v1/` routes on the existing `httprouter`. The `RestyClient` is constructed in `main.go` from the same env vars as the existing Proxmox client.

### Backend org — what changes

| Item | Action |
|---|---|
| Telmate client | Bypassed by new API handlers (no change to existing handlers) |
| `backend/types.go` | Untouched — new types go in `api/v1/types.go` |
| Existing handlers | Untouched |
| `JWT_SECRET` env var | Added to required env var validation in `security/validation.go` |
| `go.mod` | Add `github.com/golang-jwt/jwt/v5` |

---

## Frontend: Vue 3 + TypeScript + Vite

### Directory layout

```
frontend/
  src/
    api/
      client.ts       # Axios instance, base URL /api/v1, credentials: 'include'
                      # Response interceptor: 401 → POST /api/v1/auth/refresh → retry
      auth.ts         # login(user, pass), logout(), getMe()
      vms.ts          # getVMs(), getVM(id), postAction(id, action, node)
    components/
      VmCard.vue      # VM status card — Tailwind + FA icons
      VmActionButtons.vue  # start/stop/reboot/shutdown/reset/console buttons
      AppButton.vue   # typed button primitive (variant, loading, disabled props)
    stores/
      auth.ts         # Pinia: { username, isAdmin, isAuthenticated }
                      #   actions: login(), logout(), init()
      vms.ts          # Pinia: { vms: VMSummary[], loading, error }
                      #   actions: fetchVMs(), executeAction()
    main.ts           # createApp(App), Pinia, mount('#vue-app')
    App.vue           # root, calls authStore.init() on mount, renders <slot>
  dist/               # Vite output — Go serves these as /assets/*
  index.html          # Vite entry (dev only — Go serves in prod)
  vite.config.ts      # build.outDir: ../dist, base: '/assets/'
  tsconfig.json
  package.json
```

### Mount point

The templ `Layout` component gets a single addition:

```html
<div id="vue-app"
     data-page="vm-list"
     data-csrf="{{ .CSRFToken }}">
</div>
<script type="module" src="/assets/main.js"></script>
```

`App.vue` reads `data-page` to decide which view to render. In phase 1, only `vm-list` is handled.

### CSS

- Tailwind CSS v4 (Vite plugin: `@tailwindcss/vite`)
- Font Awesome icons via existing `/webfonts/` static path (no new dependency)
- Bulma stays in the templ pages — no conflict since Tailwind is scoped to `#vue-app`

### Component API

**`VmCard.vue`**
```typescript
interface Props {
  vm: VMSummary
}
emits: ['action'] // action: { vmid, action, node }
```

**`VmActionButtons.vue`**
```typescript
interface Props {
  vmid: number
  node: string
  status: 'running' | 'stopped' | 'paused'
  loading?: boolean
}
emits: ['action'] // action: string
```

**`AppButton.vue`**
```typescript
interface Props {
  variant?: 'primary' | 'danger' | 'success' | 'warning' | 'ghost'
  size?: 'sm' | 'md' | 'lg'
  loading?: boolean
  disabled?: boolean
  icon?: string  // FA class e.g. 'fa-play'
}
```

### Auth flow in Vue

```
App.vue mount
  └── authStore.init()
        └── GET /api/v1/auth/me
              ├── 200 → set username, isAdmin, isAuthenticated=true
              └── 401 → isAuthenticated=false (user sees login page, served by templ)
```

The templ login page is untouched for phase 1. The Vue app assumes the user is already authenticated (redirected to login by templ if not). Phase 2 will replace the login page with a Vue view.

---

## Go static file serving

The Vite `dist/` output is served by Go:

```go
// In handlers.go setupStaticFiles()
registerStaticHandler(router, "/assets/*filepath",
    http.StripPrefix("/assets/", http.FileServer(http.Dir("../frontend/dist"))))
```

In production Docker image, `frontend/dist/` is built during the Docker build step and copied in. The `Dockerfile` gets a Node build stage before the Go build stage.

---

## Makefile additions

```makefile
frontend-install:   ## Install frontend deps
    cd frontend && npm install

frontend-dev:       ## Start Vite dev server (proxy to Go on :50000)
    cd frontend && npm run dev

frontend-build:     ## Build Vue SPA to frontend/dist/
    cd frontend && npm run build
```

---

## What is NOT in phase 1

- Vue Router (no client-side navigation yet)
- Replacing login, admin, profile, VM create pages
- i18n in Vue (FR translations)
- Removing Alpine.js or htmx
- Full Telmate removal
- noVNC console in Vue

---

## Files changed / created

| File | Change |
|---|---|
| `backend/api/v1/router.go` | New |
| `backend/api/v1/middleware.go` | New |
| `backend/api/v1/auth.go` | New |
| `backend/api/v1/vms.go` | New |
| `backend/api/v1/vm_actions.go` | New |
| `backend/api/v1/types.go` | New |
| `backend/api/v1/errors.go` | New |
| `backend/security/validation.go` | Add JWT_SECRET to required vars |
| `backend/go.mod` | Add golang-jwt/jwt/v5 |
| `backend/main.go` | Call RegisterAPIRoutes, init RestyClient for API |
| `backend/components/layout.templ` | Add `<div id="vue-app">` + assets script tag |
| `frontend/` | New Vite + Vue 3 + TS project |
| `Makefile` | Add frontend-install, frontend-dev, frontend-build |
| `Dockerfile` | Add Node build stage |
| `CLAUDE.md` | Update with new commands and structure |
