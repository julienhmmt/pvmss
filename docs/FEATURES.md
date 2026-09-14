# PVMSS feature inventory

Every user-facing capability of PVMSS v0.4, grouped by audience. Each line
names the SPA route and the API it relies on so the list can be checked
against `server/internal/httpapi/router.go` / `router_admin.go`.

Legend: ✅ shipped · 🧪 partial / behind a toggle · 🚧 planned (toggle
exists, backend not implemented).

---

## 1. Authentication & identity

| Feature | Route | API | Status |
| --- | --- | --- | --- |
| Sign in with Proxmox credentials, per cluster | `/login` | `GET /api/v1/auth/clusters`, `POST /api/v1/auth/login` | ✅ |
| Local administrator sign-in (`ADMIN_PASSWORD_HASH`, bcrypt) | `/login` | `POST /api/v1/auth/admin-login` | ✅ |
| Session cookie (`SESSION_SECRET`), CSRF token, secure cookie flag | - | all writes | ✅ |
| Sign out | header menu | `POST /api/v1/auth/logout` | ✅ |
| Change own Proxmox password | API only (no page yet) | `POST /api/v1/auth/password` | 🧪 |
| Personal API tokens (create - secret shown once - list, revoke) | `/profile/tokens` (sidebar → API tokens) | `GET/POST/DELETE /api/v1/auth/tokens` | ⛔ deactivated - routes unregistered, bearer resolution disabled in `Auth.Principal`; code kept |
| Proxmox sign-in blocked while the selected cluster is unreachable; admin sign-in stays available | `/login` | `cluster_unavailable` error | ✅ |
| Per-IP rate limit on auth endpoints (10 req/min) | - | `router.go` | ✅ |
| OIDC / SSO sign-in | `/login` (button appears when enabled on a cluster) | `POST /api/v1/auth/oidc` → **501** | 🚧 |

## 2. My VMs

| Feature | Route | API | Status |
| --- | --- | --- | --- |
| Home dashboard: my VM counts, quota meter, in-flight tasks | `/` | `GET /api/v1/vms`, `GET /api/v1/tasks/{upid}` | ✅ |
| VM list across all clusters or scoped to one (`ClusterSelector`) | `/vms` | `GET /api/v1/vms` | ✅ |
| Search / filter / sort mirrored into the URL (linkable views) | `/vms`, `/search` | `GET /api/v1/vms` | ✅ |
| Live status refresh for the visible rows | `/vms` | `POST /api/v1/vms/status` | ✅ |
| Bulk power actions with per-VM result (`start`, `stop`, `shutdown`, `reboot`, `reset`, `pause`, `resume`) | `/vms` | `POST /api/v1/vms/bulk-action` | ✅ |
| Console button straight from the list | `/vms` | - | ✅ |
| Just-deleted VM hidden until inventory catches up | `/vms` | client side | ✅ |
| Ownership enforced server-side (`vm.Resolve()`), not by the list filter | - | every VM route | ✅ |

## 3. Create a VM

Wizard at `/vms/create` - **Simple** and **Detailed** modes, five steps
(Base, Disk, Hardware, Network, Review). Catalog from
`GET /api/v1/vm-create/catalog`, submit with `POST /api/v1/vms`, progress via
`GET /api/v1/tasks/{upid}` in the task tray.

| Feature | Status |
| --- | --- |
| Three sources: **ISO** (admin-approved), **Proxmox template** clone (linked or full - the wizard says which), **cloud image** import (`import-from`, requires cloud-init user/SSH keys/network) | ✅ |
| Hardware profiles (admin-curated CPU/RAM/disk shapes) or custom values | ✅ |
| Node auto-placement with capacity scoring + live storage free-space check; node fixed to the template's node for clones | ✅ |
| Disk: storage + size; minimum raised to the template/image size | ✅ |
| Network: one or more NICs, bridge + model (VirtIO, E1000, E1000E, RTL8139, VMXNet3); Proxmox firewall always on; optional admin-wide isolation VLAN tag | ✅ |
| Firmware: UEFI (default on), Secure Boot toggle (default off), TPM 2.0 | ✅ |
| Cloud-init document picker: admin templates **or** my own files, one grouped select (hidden when the cluster has no snippet write target) | ✅ |
| Boot from CD-ROM first when an ISO is selected | ✅ |
| Tags from the admin-curated list (`pvmss` tag always added) | ✅ |
| Start after create (image source: starts only after cloud-init is applied) | ✅ |
| VM name validated as a hostname and unique in the pool; VMID collision retry; rollback on failure | ✅ |
| Quotas and gabarit limits checked server-side before any Proxmox call | ✅ |
| Draft auto-saved in the browser | ✅ |

## 4. Operate a VM - `/vms/[cluster]/[vmid]`

| Tab | Actions | API | Status |
| --- | --- | --- | --- |
| Overview | 7 power actions (shutdown = guest/ACPI only, stop = hard), rename, Markdown description, delete (dialog), one-time boot from CD-ROM | `POST …/actions`, `PATCH …/{vmid}`, `DELETE …/{vmid}`, `POST …/boot-cdrom` | ✅ |
| Overview | Metrics history (hour / day / week, CPU/RAM/disk/net SVG charts) and live stream | `GET …/metrics/history`, `GET …/metrics/stream` | ✅ |
| Disks | add, resize (grow), detach | `POST …/disks`, `PUT …/disks/{key}/resize`, `DELETE …/disks/{key}` | ✅ |
| Network | edit each NIC: bridge, model, VLAN tag, rate limit (Mbps) | `PUT …/network` | ✅ |
| Hardware | sockets/cores, memory, tags (curated picker), CD-ROM load/eject | `GET …/hardware-options`, `PUT …/hardware`, `PATCH …/cdrom` | ✅ |
| Cloud-init | native form (user, password via guest agent, SSH keys, IP/gateway/DNS); "Add key now" injection; per-VM document editor (when the admin allows custom YAML **and** the cluster has a snippet write target) | `GET/PUT …/cloudinit`, `POST …/cloudinit/ssh-keys`, `GET/PUT …/cloudinit/snippet` | ✅ |
| Snapshots | create (with/without RAM), rollback, delete, view a snapshot's config; per-VM max enforced by policy | `GET/POST …/snapshots`, `POST …/snapshots/{name}/rollback`, `DELETE …/snapshots/{name}`, `GET …/snapshots/{name}/config` | ✅ |
| Activity | per-VM audit trail (who did what, when) | `GET …/audit` | ✅ |
| Status | polled status/lock/uptime | `GET …/status` | ✅ |

## 5. Consoles

| Feature | Route | API | Status |
| --- | --- | --- | --- |
| noVNC graphical console, WebSocket proxied by PVMSS with a single-use ticket | `/vms/[cluster]/[vmid]/console` | `POST …/vnc-ticket`, `GET …/console/websocket` | ✅ |
| Power actions on the console page | same | `POST …/actions` | ✅ |
| Serial (xterm.js) console; enable a serial port on a VM that has none | same page | `POST …/serial`, `POST …/serial-ticket`, `GET …/serial/websocket` | ✅ |

## 6. Cloud-init documents

| Feature | Route | API | Status |
| --- | --- | --- | --- |
| Personal cloud-init files (max 20 per user, private to the owner) | `/cloud-init` | `GET/POST/PUT/DELETE /api/v1/cloudinit/files` | ✅ |
| Admin cloud-init templates, per cluster, enable/disable | `/admin/cloudinit-templates` | `/api/v1/admin/cloudinit-templates` | ✅ |
| Per-VM copy written by PVMSS as `pvmss-<vmid>.yml` into the cluster's mounted `snippets/` dir, attached as `vendor=`; later edits to the source never touch the VM | at creation | `POST /api/v1/vms` | ✅ |
| Feature off = loud: cluster without snippet dir → picker hidden, create with a document → 409 `cloudinit_write_unavailable` | - | - | ✅ |
| Baseline snippet `pvmss-baseline.yml` auto-attached for cloud-image VMs when present | - | - | ✅ |
| Snippet file and its row removed when the VM is deleted (best effort, never blocks the delete) | - | `DELETE /api/v1/vms/{cluster}/{vmid}` | ✅ |

## 7. Cluster visibility

| Feature | Route | API | Status |
| --- | --- | --- | --- |
| Node list with capacity/status from the inventory cache; manual refresh throttled | `/nodes` | `GET /api/v1/cluster/nodes`, `POST /api/v1/cluster/refresh` | ✅ |
| Graceful degradation when a cluster is down (banner, actions disabled, no crash) | everywhere | - | ✅ |
| Multi-cluster: every VM addressed by `cluster` + `vmid`; same VMID may exist on two clusters | everywhere | - | ✅ |

## 8. Documentation (in-app CMS)

| Feature | Route | API | Status |
| --- | --- | --- | --- |
| Public docs index + reader, Markdown rendered server-side, EN + FR per page | `/docs`, `/docs/[id]` | `GET /api/v1/docs`, `GET /api/v1/docs/{id}` | ✅ |
| Admin-audience pages hidden from users and 401/403 on direct access | - | - | ✅ |
| Seeded system pages (getting-started, user guide, VM guidelines, cloud-init how-to, admin guide, cloud-init setup, Proxmox permissions) - inserted once, admin edits never clobbered | `/admin/docs` | `/api/v1/admin/docs` | ✅ |
| About page | `/about` | `GET /api/v1/public/version` | ✅ |

## 9. Administration - `/admin`

All routes behind `RequireAdmin`.

| Area | Route | What it does | Status |
| --- | --- | --- | --- |
| Dashboard | `/admin` | node summary, VM status counts, version, last refresh | ✅ |
| Clusters | `/admin/clusters` | add / edit / remove connections (URL, token, TLS skip-verify), **Test** connectivity (version, node & VM count), OIDC toggle, **snippet directory + storage id** for cloud-init documents (badge "cloud-init: on") | ✅ |
| Nodes | `/admin/nodes` | approve/disable per cluster; confirm when disabling a node with running VMs; search/filter/sort; orphan cleanup | ✅ |
| Storages | `/admin/storages` | approve per node/cluster; usage bars; orphan cleanup | ✅ |
| ISOs | `/admin/isos` | approve discovered ISOs; orphan cleanup | ✅ |
| Cloud images | `/admin/images` | approve images discovered under a storage's `import/` content; orphan cleanup | ✅ |
| VM templates | `/admin/templates` | approve Proxmox templates for cloning, per-template overrides; orphan cleanup | ✅ |
| Bridges | `/admin/bridges` | approve VMBRs (OVS not listed); orphan cleanup | ✅ |
| Cloud-init templates | `/admin/cloudinit-templates` | CRUD + enable/disable `#cloud-config` documents (header + YAML validated) | ✅ |
| Profiles | `/admin/profiles` | CRUD + enable/disable hardware profiles, optional node/storage override, icon/color | ✅ |
| Tags | `/admin/tags` | create / delete / recolor; `pvmss` reserved | ✅ |
| Pools | `/admin/pools` | create a self-service user = Proxmox user + pool + ACL; cascade delete | ✅ |
| Policy | `/admin/policy` | per-cluster gabarit (max sockets, cores, memory, disk/VM, NICs, snapshots, allow custom cloud-init YAML, isolation VLAN) + quota (max VMs per user) | ✅ |
| Node capacity | `/admin/policy/nodes` | per-node caps (VMs, vCPUs, RAM, disk) with live usage vs physical | ✅ |
| Documentation | `/admin/docs` | CMS for the in-app docs (EN/FR, audience, toggle, system pages protected) | ✅ |
| App info | `/admin/appinfo` | build, runtime, safe env subset, cluster status | ✅ |
| Settings | `/admin/settings` | audit log (paged, filterable, severity), retention days + prune preview, DB export, **two-phase** DB import (upload → preview → confirm) | ✅ |
| Live discovery reconciliation | all catalog pages | stale approvals flagged and removable when the resource vanished from Proxmox | ✅ |

## 10. Platform & operations

| Feature | Detail | Status |
| --- | --- | --- |
| Single static binary + SPA, distroless non-root image, port 50000 | `Dockerfile` | ✅ |
| Env-only configuration validated at boot (fail fast) | `server/internal/config` | ✅ |
| SQLite (pure Go, CGO-free) with migrations; one file to back up | `PVMSS_DB_PATH` | ✅ |
| Structured `slog` logging, console or JSON, stdout/stderr/file | `LOG_*` | ✅ |
| Health endpoint | `GET /health` | ✅ |
| Public version endpoint | `GET /api/v1/public/version` | ✅ |
| Background inventory refresh with configurable interval/timeout | `PVMSS_INVENTORY_*` | ✅ |
| Security headers (CSP, HSTS, frame/content-type options), CSRF, rate limiting, trusted proxy hops for client IP | `security_headers.go`, `csrf.go`, `ratelimit.go`, `PVMSS_TRUSTED_PROXY_HOPS` | ✅ |
| `fake` cluster source for demos/tests (no Proxmox needed) | `PVMSS_CLUSTER_SOURCE=fake` | ✅ |
| Docker, Docker Compose, Kubernetes manifest, Helm chart | repo root, `helm/` | ✅ |
| `pvmss-recover` one-shot v0.3 → v0.4 DB migration CLI | `server/cmd/pvmss-recover` | ✅ |
| i18n EN + FR, WCAG 2.1 AA target, keyboard-first, reduced-motion | `web/messages` | ✅ |

## Not in scope (done in Proxmox)

LXC containers · backups · live migration / HA · SDN and firewall rules ·
Proxmox user management beyond `/admin/pools` · OpenID Connect (planned).
