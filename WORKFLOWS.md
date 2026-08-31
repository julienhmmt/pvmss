# Workflows

What a user actually does in PVMSS, end to end. Companion to `PRODUCT.md`
(who and why) and `DESIGN.md` (how it looks). This file is the *what*.

Every workflow below follows the same template. **Use it as the model when
adding a new one** — a workflow that cannot fill in all seven fields is not
specified yet.

```markdown
### <Name>

| | |
| --- | --- |
| **Audience** | end user \| admin |
| **Entry** | where the user comes from (nav item, CTA, deep link) |
| **Route** | SPA route(s) |
| **API** | endpoints hit, in call order |
| **Steps** | the happy path, numbered |
| **States** | loading / empty / error / partial |
| **Safety nets** | confirmations, rate limits, policy gates, undo |
```

Rules for the fields:

- **API** lists real registered routes only — check `server/internal/httpapi/router.go`
  and `router_admin.go` before writing a line here.
- **States** must name the concrete component or behaviour (a skeleton, an
  empty state, a toast), not "handles errors".
- **Safety nets** is never empty for a destructive or outward-facing action.
  If it is, the workflow has a hole (design principle 4 in `PRODUCT.md`).

---

## 1. Authentication

### Sign in

| | |
| --- | --- |
| **Audience** | end user |
| **Entry** | `/login`, or any guarded page via `AuthRequired` |
| **Route** | `/login` |
| **API** | `GET /api/v1/auth/clusters` → `POST /api/v1/auth/login` or `POST /api/v1/auth/oidc` → `GET /api/v1/auth/me` |
| **Steps** | 1. Pick a cluster from the pre-login list. 2. Submit Proxmox credentials, or trigger OIDC. 3. Session cookie is set; redirect to the requested page. |
| **States** | Empty cluster list when no cluster is configured; inline credential error; Proxmox sign-in is disabled when every configured cluster is unreachable, while local admin sign-in remains available |
| **Safety nets** | Per-IP rate limit, 10 requests / minute, shared by login, admin-login, cluster list, and OIDC (`authRateLimitMaxRequests`, `router.go:16`) — the cluster list is unauthenticated and discloses cluster names, so it is bounded too; the backend rejects `POST /api/v1/auth/login` with `cluster_unavailable` when the selected cluster is down, so the restriction cannot be bypassed by calling the API directly |

### Sign in as admin

| | |
| --- | --- |
| **Audience** | admin |
| **Entry** | `/login`, separate path from the Proxmox credential form |
| **Route** | `/login` |
| **API** | `POST /api/v1/auth/admin-login` |
| **Steps** | 1. Submit the admin password. 2. Session cookie is set with admin identity. |
| **States** | Disabled when `ADMIN_PASSWORD_HASH` is unset |
| **Safety nets** | bcrypt hash only (validated at startup, `config/load.go`); same per-IP rate limit as sign-in |

### Manage API tokens

| | |
| --- | --- |
| **Audience** | end user |
| **Entry** | Profile menu |
| **Route** | `/profile/tokens` |
| **API** | `GET /api/v1/auth/tokens`, `POST /api/v1/auth/tokens`, `DELETE /api/v1/auth/tokens/{id}` |
| **Steps** | 1. List existing tokens. 2. Create one — the secret is shown once. 3. Revoke by id. |
| **States** | Empty state on first visit |
| **Safety nets** | Secret displayed once, copy button; revoke is immediate |

Password change goes through `POST /api/v1/auth/password`.

---

## 2. VM lifecycle

This is the core of the product. Everything else exists to support it.

### Browse my VMs

| | |
| --- | --- |
| **Audience** | end user |
| **Entry** | Sidebar → Machines, or the home dashboard |
| **Route** | `/vms` |
| **API** | `GET /api/v1/vms` (scope `mine`) |
| **Steps** | 1. Pick a cluster scope (`ClusterSelector`) or stay cross-cluster. 2. Filter and search — state is mirrored into the URL query, so a filtered list is linkable. 3. Open a VM, or select several for a bulk action. |
| **States** | `TableSkeleton` while loading; empty state with a create CTA; cluster list falls back to empty on fetch failure rather than blocking the page |
| **Safety nets** | Ownership is enforced server-side by `vm.Resolve()`, not by the list filter |

### Bulk power actions

| | |
| --- | --- |
| **Audience** | end user |
| **Entry** | Selecting rows in `/vms` |
| **Route** | `/vms` |
| **API** | `POST /api/v1/vms/bulk-action` |
| **Steps** | 1. Select VMs across one or more clusters. 2. Pick an action. 3. Read the per-VM result — each entry is `ok` or `error` with its own message. |
| **States** | Per-row result, not a single global success/failure |
| **Safety nets** | Only the valid power actions are accepted (`validActions`, `server/internal/vm/actions.go`): `start`, `stop`, `shutdown`, `reboot`, `reset`, `pause`, `resume`. A partial failure never rolls back the successes — it reports them. |

### Create a VM

| | |
| --- | --- |
| **Audience** | end user |
| **Entry** | Sidebar CTA, home dashboard, or the `/vms` empty state |
| **Route** | `/vms/create` |
| **API** | `GET /api/v1/vm-create/catalog` → `POST /api/v1/vms` → `GET /api/v1/tasks/{upid}` (polled) |
| **Steps** | 1. Base (name, profile, cluster, node). 2. Disk. 3. Hardware. 4. Network. 5. Review — the only place raw JSON is shown, and only on request. 6. Submit; the response is a Proxmox UPID. 7. The task tray polls until done, then refreshes the VM list. |
| **States** | Wizard step validation; task tray shows in-flight work so the user can navigate away |
| **Safety nets** | Every choice comes from the admin-approved catalog — nodes, storages, bridges, ISOs, profiles, cloud-init templates, tags. Quotas and gabarit limits are checked server-side (`policy/`), not in the wizard. |

### Operate a single VM

| | |
| --- | --- |
| **Audience** | end user |
| **Entry** | A row in `/vms` |
| **Route** | `/vms/[cluster]/[vmid]` |
| **API** | `GET /api/v1/vms/{cluster}/{vmid}` plus the per-tab endpoints below |
| **Steps** | Six tabs, each self-contained (`VmDetail.svelte:23`) |
| **States** | Cloud-init and Snapshots tabs mount lazily — their panels only render when active |
| **Safety nets** | Every write is gated by `vm.Resolve()` inside the handler; destructive actions each get their own dialog |

| Tab | Actions | API |
| --- | --- | --- |
| Overview | 7 power actions (shutdown = guest-agent/ACPI only, no auto force-stop; stop = hard stop), rename, description, delete | `POST .../actions`, `PATCH .../{vmid}`, `DELETE .../{vmid}` |
| Disks | add, resize, detach | `POST .../disks`, `PUT .../disks/{diskKey}/resize`, `DELETE .../disks/{diskKey}` |
| Network | edit interface | `PUT .../network` |
| Hardware | CPU/RAM, tags (admin-curated picker), CDROM | `GET .../hardware-options`, `PUT .../hardware`, `PATCH .../cdrom` |
| Cloud-init | form + raw snippet editor | `GET`/`PUT .../cloudinit`, `GET`/`PUT .../cloudinit/snippet` |
| Snapshots | create, rollback, delete | `GET`/`POST .../snapshots`, `POST .../snapshots/{name}/rollback`, `DELETE .../snapshots/{name}` |

Dialogs: `DeleteVmDialog`, `CreateSnapshotDialog`, `RollbackSnapshotDialog`,
`DeleteSnapshotDialog`, `SaveCloudInitDialog`. A destructive action without a
dialog is a bug.

Rename validates as a hostname (lowercase, ≤63 chars, `hostnameRe` in
`vm/actions.go`) because the name becomes a DNS label via cloud-init.

### Open the console

| | |
| --- | --- |
| **Audience** | end user |
| **Entry** | `ConsoleBanner` on the VM detail page |
| **Route** | `/vms/[cluster]/[vmid]/console` |
| **API** | `POST /api/v1/vms/{cluster}/{vmid}/vnc-ticket` → `GET /api/v1/vms/{cluster}/{vmid}/console/websocket` |
| **Steps** | 1. Request a short-lived VNC ticket. 2. Upgrade to a WebSocket proxied to Proxmox. |
| **States** | Connection failure surfaces as a console error, not a blank frame |
| **Safety nets** | Ticket is per-VM and issued only after the ownership check; dev note — the Vite dev server must run under Node, not bun, or the WebSocket breaks with a 1006 |

### View VM metrics history

| | |
| --- | --- |
| **Audience** | end user |
| **Entry** | `VmMetricsRow` on the VM detail page, below the Overview stat cards |
| **Route** | `/vms/[cluster]/[vmid]` |
| **API** | `GET /api/v1/vms/{cluster}/{vmid}/metrics/history?range=hour\|day\|week` |
| **Steps** | 1. Row loads history for the default "hour" range on mount. 2. User toggles hour/day/week; each toggle re-fetches and re-renders only the four charts. |
| **States** | Skeleton cards while loading; an inline error banner on fetch failure; charts render via hand-rolled SVG (`LineChart.svelte`), no charting library |
| **Safety nets** | Resolved and ownership-checked the same way as every other VM read (`vm.Resolve()`); a stale in-flight response from a prior range switch is discarded, never overwrites the current one |

---

## 3. Cluster visibility

### Browse nodes

| | |
| --- | --- |
| **Audience** | end user |
| **Entry** | Sidebar → Nodes |
| **Route** | `/nodes` |
| **API** | `GET /api/v1/cluster/nodes`, `POST /api/v1/cluster/refresh` |
| **Steps** | 1. Read node capacity and status from the cached inventory. 2. Optionally force a refresh. |
| **States** | Data is served from the inventory cache, refreshed in the background every `PVMSS_INVENTORY_REFRESH_INTERVAL` (default 30s) |
| **Safety nets** | Manual refresh is throttled by `PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL` (default 5s) so a click loop cannot hammer Proxmox |

---

## 4. Documentation

| | |
| --- | --- |
| **Audience** | end user and admin — same pages, different visibility |
| **Entry** | Header link, available signed out |
| **Route** | `/docs`, `/docs/[id]` |
| **API** | `GET /api/v1/docs`, `GET /api/v1/docs/{id}` |
| **Steps** | 1. List pages filtered by audience. 2. Open one; markdown is rendered server-side. |
| **States** | Bilingual (EN + FR) per page |
| **Safety nets** | Admin-audience pages are hidden from the list *and* return 401/403 on direct access — the handler resolves the caller itself rather than relying on the list filter |

---

## 5. Administration

All admin routes sit behind `Auth.RequireAdmin` (401 unauthenticated, 403
non-admin), wired in `router_admin.go`. Nav grouping comes from
`web/src/lib/features/chrome/admin-nav-items.svelte.ts`.

| Group | Routes | What it does |
| --- | --- | --- |
| Dashboard | `/admin` | Overview (`GET /api/v1/admin/dashboard`) |
| Infrastructure | `/admin/nodes`, `/admin/clusters`, `/admin/pools` | Approve nodes; cluster CRUD with connection test and OIDC config; pool create and cascade delete |
| Catalog | `/admin/storages`, `/admin/isos`, `/admin/bridges`, `/admin/cloudinit-templates`, `/admin/docs`, `/admin/profiles`, `/admin/tags` | Toggle what users may pick; CRUD for profiles, tags, cloud-init templates, docs |
| Policy | `/admin/policy`, `/admin/policy/nodes` | Quotas and gabarit limits; per-node capacity |
| System | `/admin/appinfo`, `/admin/settings` | App info, audit log, DB export/import |

Three admin workflows deserve the full template because they are the risky ones.

### Approve or disable a node

|| | |
| --- | --- |
| **Audience** | admin |
| **Entry** | Sidebar → Administration → Infrastructure → Nodes |
| **Route** | `/admin/nodes` |
| **API** | `GET /api/v1/admin/nodes` → `POST /api/v1/admin/nodes/toggle` |
| **Steps** | 1. Pick a cluster from the selector. 2. Search, filter by status or enabled state, and sort by name, status, usage, or VM count. 3. Toggle a node on or off. 4. If disabling a node that has running VMs, confirm the action. 5. A toast confirms the change. |
| **States** | `TableSkeleton` while loading; `EmptyState` with a link to `/admin/clusters` when no nodes are discovered; a second `EmptyState` with a reset-filters action when filters exclude every row; `ConfirmDialog` when disabling a node with running VMs; usage bars and sortable columns in the table. |
| **Safety nets** | Disabling a node with running VMs requires confirmation and explains that existing VMs are unaffected; success and error toasts give feedback after the API call; toggling a node off only removes it from the create-VM catalog, never from existing VMs. |

### Approve a catalog item

| | |
| --- | --- |
| **Audience** | admin |
| **Entry** | Any catalog page under `/admin` |
| **Route** | `/admin/nodes`, `/admin/storages`, `/admin/isos`, `/admin/bridges` |
| **API** | `GET /api/v1/admin/{kind}` → `POST /api/v1/admin/{kind}/toggle` |
| **Steps** | 1. Pick a cluster from the selector. 2. Search, filter by node/state/type, and sort by the available columns. 3. Toggle an item on or off. 4. A toast confirms the change. 5. It appears in, or disappears from, the create-VM catalog. |
| **States** | `TableSkeleton` while loading; `EmptyState` with a link to `/admin/clusters` when no items are discovered; a second `EmptyState` with a reset-filters action when filters exclude every row; sortable columns, usage bars for storages, and active-state dots for bridges. |
| **Safety nets** | Toggling off does not touch existing VMs — it only removes the item from future choices; success and error toasts give feedback after the API call. |

### Export / import the database

| | |
| --- | --- |
| **Audience** | admin |
| **Entry** | `/admin/settings` |
| **Route** | `/admin/settings` |
| **API** | `GET /api/v1/admin/db/export`; `POST /api/v1/admin/db/import` → `POST /api/v1/admin/db/import/confirm` |
| **Steps** | 1. Export downloads the current SQLite state. 2. Import uploads a candidate and returns a preview. 3. A second, explicit confirm call applies it. |
| **States** | Preview between upload and apply |
| **Safety nets** | Two-phase import — nothing is written until the confirm call. This is the most destructive action in the app. |

`GET /api/v1/public/version` is deliberately outside the admin guard so the
version is readable without an account.

---

## Cross-cutting

These apply to every workflow above; a new workflow inherits them.

| Concern | Where |
| --- | --- |
| i18n | Paraglide, EN + FR, `m['...']()` message keys — no bare user-facing strings |
| Background work | Task tray polls `GET /api/v1/tasks/{upid}` and refreshes the affected list on completion |
| Loading | `TableSkeleton` and per-feature skeletons, never a blank page |
| Quotas | `Meter` / `quota-meter` components fed by `policy/` |
| Accessibility | `role="tabpanel"` panels, focus trap in dialogs, skip-to-content link, visible focus rings |
| Security headers | `withSecurityHeaders` wraps the whole mux — CSP, HSTS, frame and content-type options, cache-control on `/api` |
| Unknown API paths | Return a JSON `{"detail": "unknown API path"}` 404, never the SPA shell |
