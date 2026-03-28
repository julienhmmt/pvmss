# Frontend Migration Plan: Go templ → SvelteKit

## Current State

**Already in SvelteKit:** all `/admin/*` pages, `/login`
**Still in Go templ + Alpine.js:** `/` (home), `/profile`, `/vm/details/:id`, `/vm/create`, `/search`, `/docs`, `/docs/:type`

**Frontend stack:**

- Old: Go templ + Alpine.js + HTMX + Bulma CSS
- New: SvelteKit 5 runes + shadcn-svelte + Tailwind CSS

---

## Phase 1 — User VM List (Home Page)

**Route:** `/`
**SvelteKit file:** `(app)/+page.svelte` (placeholder already exists, replace content)
**API needed:** `GET /api/v1/vms` ✅ exists

**UI:**

- VM table: VMID, name, node, status badge, CPU bar, RAM bar, uptime
- Action buttons inline: start / stop / reboot
- Empty state when no VMs

**Go API additions:** none

**Backend routing:** `isSPAPath()` already serves `/` ✅

---

## Phase 2 — VM Details

**Route:** `/vm/:id`
**SvelteKit file:** `src/routes/(app)/vm/[id]/+page.svelte`
**API needed:**

- `GET /api/v1/vms/:id` ✅ exists
- `POST /api/v1/vms/:id/action` ✅ exists
- `GET /api/v1/vms/:id/metrics` ❌ need to add
- `GET /api/v1/vms/:id/console` ❌ need to add (returns noVNC URL/ticket)

**Go API additions needed:**

- `GET /api/v1/vms/:id/metrics` — CPU %, RAM used/total, disk used/total, network in/out
- `GET /api/v1/vms/:id/console` — returns noVNC websocket URL + one-time ticket

**UI sections:**

- Status header: VMID, name, node, status badge
- Action buttons: start / stop / shutdown / reboot / reset
- Resource stat cards: CPU %, RAM, disk
- Tabs:
  - Console (embedded noVNC)
  - Disks
  - Network
  - Snapshots

**Backend routing:** add `/vm/` prefix to `isSPAPath()`

---

## Phase 3 — VM Creation

**Route:** `/vm/create`
**SvelteKit file:** `src/routes/(app)/vm/create/+page.svelte`
**API needed:**

- `GET /api/v1/settings` ❌ need to add (allowed nodes, ISOs, storages, bridges, limits for current user)
- `POST /api/v1/vms` ❌ need to add
- `GET /api/v1/vms` ✅ (for quota check)

**Go API additions needed:**

- `GET /api/v1/settings` — user-scoped settings: allowed nodes, ISOs, storages, vmbrs, resource min/max limits, remaining quota
- `POST /api/v1/vms` — create VM with full validation (quota, name, resources)

**Multi-step form:**

1. Base — name, node, OS/ISO, pool
2. Hardware — CPU sockets/cores, RAM
3. Disk — size, storage
4. Network — vmbr bridge
5. Cloud-init (optional) — user, password, SSH key
6. Review + confirm + submit

**Backend routing:** add `/vm/create` to `isSPAPath()`

---

## Phase 4 — VM Search

**Route:** `/search`
**SvelteKit file:** `src/routes/(app)/search/+page.svelte`
**API needed:** `GET /api/v1/vms` ✅ exists (extend with filter params)

**Go API additions needed:**

- Add query params to `GET /api/v1/vms`: `?q=` (name/VMID), `?node=`, `?status=`

**UI:**

- Search input + filter dropdowns (node, status)
- VM table (reuse component from Phase 1)
- URL-synced filters (query string)

**Backend routing:** add `/search` to `isSPAPath()`

---

## Phase 5 — User Profile

**Route:** `/profile`
**SvelteKit file:** `src/routes/(app)/profile/+page.svelte`
**API needed:**

- `GET /api/v1/auth/me` ✅ exists
- `GET /api/v1/vms` ✅ exists
- `POST /api/v1/auth/change-password` ❌ need to add

**Go API additions needed:**

- `POST /api/v1/auth/change-password` — body: `{ current_password, new_password }`
  - Proxmox users: verify current via `/access/ticket`, set new via Proxmox API
  - Local admin: not supported (password is env var hash)

**UI sections:**

- Account info: username, role badge (admin / user)
- VM quota: used / total count
- Change password form (Proxmox users only; hidden for local admin)

**Backend routing:** add `/profile` to `isSPAPath()`

---

## Phase 6 — Documentation

**Route:** `/docs`, `/docs/user`, `/docs/admin`
**SvelteKit files:**

- `src/routes/(app)/docs/+page.svelte` — redirects to `/docs/user`
- `src/routes/(app)/docs/[type]/+page.svelte`

**API needed:** `GET /api/v1/docs/:type` ❌ need to add (or serve as static assets)

**Go API additions needed:**

- `GET /api/v1/docs/:type` — returns markdown string for `user` or `admin` guide
  - Read from embedded FS or file path at startup

**UI:**

- Rendered markdown with prose styling
- Table of contents sidebar (parsed from headings)
- Admin vs user tab/toggle

**Backend routing:** add `/docs` prefix to `isSPAPath()`

---

## Backend Routing Changes (Summary)

Each phase requires updating `isSPAPath()` in `backend/handlers/middleware_utils.go`:

```go
func isSPAPath(p string) bool {
    if isSPAStaticAsset(p) || isSPALoginPath(p) {
        return false
    }
    return p == "/" ||
        strings.HasPrefix(p, "/admin/") || p == "/admin" ||
        strings.HasPrefix(p, "/vm/") ||     // Phase 2 + 3
        p == "/search" ||                   // Phase 4
        p == "/profile" ||                  // Phase 5
        strings.HasPrefix(p, "/docs")       // Phase 6
}
```

Once a route is live in SvelteKit, remove the corresponding Go-templ handler and routes from `backend/handlers/`.

---

## Priority Order

| Phase | Route | Value | Effort | New API endpoints |
|-------|-------|-------|--------|-------------------|
| 1 | `/` — VM list | High | Low | 0 |
| 2 | `/vm/:id` — VM details | High | Medium | 2 |
| 3 | `/vm/create` — VM creation | High | High | 2 |
| 4 | `/search` — VM search | Medium | Low | 0 (extend existing) |
| 5 | `/profile` — User profile | Low | Low | 1 |
| 6 | `/docs` — Documentation | Low | Low | 1 |

---

## Shared Components to Build (reused across phases)

- `VMTable` — paginated VM list with status badges, CPU/RAM bars, action buttons
- `VMStatusBadge` — colored badge for running/stopped/paused/etc.
- `ResourceBar` — usage bar (CPU %, RAM %, disk %)
- `VMActionButtons` — start/stop/shutdown/reboot/reset with loading states
- `ConfirmDialog` — generic confirmation modal (used for destructive actions)
- `MarkdownRenderer` — prose-styled markdown display (Phase 6)
