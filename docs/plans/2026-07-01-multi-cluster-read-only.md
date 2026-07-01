# Multi-Cluster Support — Read-Only Foundation

**Goal:** Let one PVMSS instance connect to and surface data from multiple Proxmox
clusters (endpoints), each with its own API token. This is the read-only foundation
for issue #54: register clusters, keep per-cluster connection/snapshot state, and
show aggregated read views across clusters. VM **creation** and **mutations** on
non-default clusters are explicitly deferred to later phases.

**Architecture:** Introduce a first-class `Cluster` entity. The legacy
`PROXMOX_URL` / `PROXMOX_API_TOKEN_NAME` / `PROXMOX_API_TOKEN_VALUE` env vars
become an implicit **`default`** cluster so existing single-cluster deployments
keep working with zero config change. Additional clusters are stored in a new
`clusters` DB table with their token secret encrypted at rest
(`security.EncryptSecret`, keyed off `SESSION_SECRET`). All node/storage/iso/vmbr
approval lists and node limits gain a `cluster_id` column (default `'default'`)
with composite primary keys, because node names are not unique across clusters.
A `ClusterRegistry` in `state` replaces direct `MakeRestyClientFromEnvConfig`
call sites with `registry.ClientFor(clusterID)`. The state manager's single
connection status / single snapshot / single node cache become per-cluster maps.

The Proxmox client layer already supports this: `MakeRestyClient` caches
`*resty.Client` in a process-wide map keyed by
`(baseURL, tokenID, tokenSecret, insecureSkipVerify)` with a shared
`http.Transport`, so multiple cluster credentials produce multiple cached
clients at no extra cost (`backend/proxmox/resty_client.go:38-74`).

**Tech Stack:** Go (stdlib + existing deps), SQLite (modernc), Svelte 5 runes +
TypeScript, Tailwind. No new dependencies.

---

## Scope

### In scope (this plan)

- Register / edit / delete / enable-disable additional Proxmox clusters via an
  admin UI, with a "test connection" action.
- Per-cluster connection status, per-cluster background snapshot, per-cluster
  node cache.
- `cluster_id` scoping of the admin approval lists (`enabled_nodes`,
  `enabled_storages`, `enabled_isos`, `enabled_vmbrs`) and `node_limits`, so
  admins curate approvals per cluster.
- Admin pages (nodes, vms, storage, iso, vmbr, limits) become cluster-aware:
  a cluster selector, data scoped to the selected cluster.
- User-facing **read** paths aggregate across all enabled clusters the user can
  see: VM list, VM details, console, snapshot list, disk/network config view.
  VM identity becomes `(clusterID, node, vmid)`; frontend VM route becomes
  cluster-scoped to disambiguate VMID collisions across clusters.

### Out of scope (deferred — later phases)

> **Decision recorded 2026-07-01:** the read-only foundation below is the first
> slice only. The end goal of issue #54 (admin grants users the right to create
> VMs/pools on specific clusters) is covered by Phase 3 + Phase 4 follow-up
> plans, to be written after this foundation lands. The chosen ACL model for
> Phase 4 is **per-user / per-pool grants** (issue-literal: "user X may create
> on cluster A"), which is *more granular* than today's node model — note that
> node access today is **global** enablement + capacity limits
> (`enabled_nodes`, `node_limits`, `max_vm_per_user`), not per-user grants.

- VM **creation** on non-default clusters (Phase 3). The create wizard is
  unchanged here and targets the `default` cluster; `vm-create/settings` keeps
  serving the default cluster's options. Phase 3 adds a cluster selector step,
  per-cluster `vm-create/settings`, and `clusterId` on `VMCreateRequest`.
- VM **mutations** (start/stop/restart/delete, config edits, snapshot
  create/rollback/delete, disk/network add/edit) on non-default clusters. The
  plumbing routes these to the right cluster client, but the UI hides/disables
  action controls for non-default-cluster VMs in this iteration with a tooltip
  explaining management on that cluster is not yet enabled. (Default-cluster
  mutations are unchanged.) Lifted in Phase 3 alongside creation.
- Per-cluster ACL — **per-user/per-pool grants** (Phase 4): a `cluster_acl`
  table mapping `(cluster_id, principal, principal_type)` to
  `can_create_pool` / `can_create_vm`, an admin form, and enforcement in
  vm-create and pool-create. In this read-only iteration all authenticated
  users see all enabled clusters for read views; admins manage clusters.
- Cluster selector in the VM-create wizard + cluster "documentation" cards
  (issue points #2/#3) — Phase 3, once creation is cluster-scoped.
- Pool creation scoped to a chosen cluster — Phase 4, with the ACL.

### Backwards compatibility

- If only the `default` cluster exists (no `clusters` rows), behavior is
  identical to today. All `cluster_id` columns default to `'default'`.
- `PROXMOX_*` env vars remain required (unless `PVMSS_OFFLINE=true`) and back
  the `default` cluster; they are NOT moved to the DB.
- Existing tests (`make test-offline`) must pass unchanged.

---

## Key design decisions

1. **`cluster_id` column, not composite string keys.** A real column on each
   list table with composite PKs `(cluster_id, name)`. Queryable, indexable,
   avoids string-parsing bugs. Slightly larger migration than `'cluster:node'`
   strings, chosen deliberately.
2. **`default` cluster is synthetic.** It is not stored in the `clusters` table;
   it is derived from `EnvConfig` and represented in the `ClusterRegistry` with a
   reserved id `constants.DefaultClusterID = "default"`. The `cluster_id`
   columns use `'default'` to reference it. Deleting the default cluster is
   forbidden.
3. **Token secrets encrypted at rest** with `security.EncryptSecret(tokenValue,
   sessionSecret)`. The `SESSION_SECRET` (already mandatory, ≥32 bytes) is the
   key. Read paths decrypt lazily inside `ClusterRegistry`; the plaintext never
   leaves the registry / never goes to the frontend.
4. **Cluster identity is stable.** `clusters.id` is an admin-chosen slug
   (lowercase, `[a-z0-9-]`), immutable after creation. Renaming the display
   `name` is allowed; the `id` is not.
5. **VM identity is `(clusterID, vmid)`** in API and frontend. The frontend VM
   route changes from `/vm/[id]` to `/vm/[cluster]/[vmid]`.

---

## Database schema (migrations v4 and v5)

### Migration v4 — `clusters` table

Add to `backend/database/schema.go`:

```sql
CREATE TABLE IF NOT EXISTS clusters (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    description  TEXT,
    location     TEXT,
    url          TEXT NOT NULL,
    token_name   TEXT NOT NULL,
    token_value  TEXT NOT NULL,            -- encv1:… (AES-256-GCM, key = SESSION_SECRET)
    verify_ssl   BOOLEAN NOT NULL DEFAULT 1,
    enabled      BOOLEAN NOT NULL DEFAULT 1,
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

Bump `currentSchemaVersion` to 4 in `backend/database/migrations.go` and register
`4: schemaV4`.

### Migration v5 — `cluster_id` columns + composite PKs

SQLite cannot alter PKs in place, so v5 recreates the affected tables with new
composite PKs and copies data over (with `cluster_id='default'`), inside one
transaction per table. Affected tables:

- `enabled_nodes`      → PK `(cluster_id, name)`
- `enabled_storages`   → PK `(cluster_id, storage_id)`
- `enabled_isos`       → PK `(cluster_id, name)`
- `enabled_vmbrs`      → PK `(cluster_id, name)`
- `node_limits`        → PK `(cluster_id, node_name)`

Pattern per table (example for `enabled_nodes`):

```sql
CREATE TABLE enabled_nodes_new (
    cluster_id TEXT NOT NULL DEFAULT 'default',
    name       TEXT NOT NULL,
    enabled    BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (cluster_id, name)
);
INSERT INTO enabled_nodes_new (cluster_id, name, enabled, created_at)
  SELECT 'default', name, enabled, created_at FROM enabled_nodes;
DROP TABLE enabled_nodes;
ALTER TABLE enabled_nodes_new RENAME TO enabled_nodes;
```

Register `5: schemaV5`, bump `currentSchemaVersion` to 5. Add a migration test in
`backend/database/migrations_test.go` that loads v1, runs to v5, and asserts the
new PKs + that existing rows carry `cluster_id='default'`.

---

## Task 1 — `Cluster` types and constants

**Files:**

- New: `backend/proxmox/cluster_config.go`
- Modify: `backend/constants/proxmox.go`

**Steps:**

1. Add `constants.DefaultClusterID = "default"` and `constants.ClusterIDPattern`
   regex `^[a-z0-9-]{1,40}$`.
2. In `backend/proxmox/cluster_config.go` define:

   ```go
   type ClusterConfig struct {
       ID          string
       Name        string
       Description string
       Location    string
       URL         string
       TokenName   string
       TokenValue  string // plaintext, only in-memory
       VerifySSL   bool
       Enabled     bool
       Source      ClusterSource // env | db
   }
   type ClusterSource int // const: ClusterSourceEnv, ClusterSourceDB
   ```

3. Add `func (c ClusterConfig) IsDefault() bool { return c.ID == constants.DefaultClusterID }`.

---

## Task 2 — `ClusterRegistry` in state

**Files:**

- New: `backend/state/cluster_registry.go`
- New: `backend/state/cluster_registry_test.go`
- Modify: `backend/state/manager.go`, `backend/state/interface.go`

**Steps:**

1. Add a `clusterRegistry` struct held by `appState`, holding
   `map[string]*proxmox.ClusterConfig` keyed by cluster id, guarded by an RWMutex.
2. `appState` gains `clusterRegistry *clusterRegistry`, initialized in
   `newAppState`. The default cluster is registered from `EnvConfig` in
   `SetEnvConfig`.
3. Registry API:
   - `Register(c *proxmox.ClusterConfig) error` — validates id pattern, rejects
     duplicate ids and the reserved `default` id for DB clusters.
   - `Get(id string) (*proxmox.ClusterConfig, bool)`
   - `All() []*proxmox.ClusterConfig` — sorted by id.
   - `Enabled() []*proxmox.ClusterConfig` — default always included if env vars
     are set.
   - `Replace(dbClusters []*proxmox.ClusterConfig)` — used after a DB reload;
     always preserves the env-backed default.
   - `ClientFor(ctx, id string, timeout) (*proxmox.RestyClient, error)` — looks
     up the config and calls `proxmox.MakeRestyClient(url, tokenName, tokenValue,
     !verifySSL, timeout)`. Reuses the existing singleton cache.
4. Expose on `StateManager` interface: `GetClusterRegistry()` (or finer-grained
   `GetCluster(id)`, `EnabledClusters()`, `ClusterClientFor(ctx, id, timeout)`).
   Keep `GetEnvConfig()` for the default cluster and for non-Proxmox config.
5. Unit tests: default-only registry, register/get, replace preserves default,
   invalid ids rejected, `ClientFor` returns the cached singleton for the same
   config (assert same `*resty.Client` pointer via a test-only accessor).

---

## Task 3 — DB layer for clusters

**Files:**

- New: `backend/database/clusters.go`
- New: `backend/database/clusters_test.go`
- Modify: `backend/database/lists.go`, `backend/database/settings.go` (node_limits),
  `backend/database/database.go` (interface + open)

**Steps:**

1. `database/clusters.go`:
   - `type ClusterRow struct { ID, Name, Description, Location, URL, TokenName, TokenValue (ciphertext), VerifySSL bool, Enabled bool }`
   - `GetClusters() ([]ClusterRow, error)` — all rows.
   - `GetCluster(id string) (ClusterRow, bool, error)`.
   - `InsertCluster(row ClusterRow, changedBy string) error` — audit via
     `appendAudit`; rejects id `default`.
   - `UpdateCluster(row ClusterRow, changedBy string) error` — updates mutable
     fields (name, description, location, url, token_name, token_value,
     verify_ssl, enabled); id immutable.
   - `DeleteCluster(id string, changedBy string) error` — rejects `default`;
     refuses if `enabled_*`/`node_limits` rows reference it (or cascades — see
     decision below; **recommend: refuse with a clear error** so admins
     consciously clean up approvals).
2. Extend the `database.DB` interface (`backend/database/database.go`) with the
   cluster methods.
3. Update `lists.go` read/write functions to take/return `clusterID`:
   - `GetEnabledNodes(clusterID)`, `SetEnabledNodes(clusterID, nodes, changedBy)`,
     and likewise for storages/isos/vmbrs. SQL gains `WHERE cluster_id = ?` and
     inserts `cluster_id`.
   - Keep the old signatures as thin wrappers that pass `constants.DefaultClusterID`
     ONLY where a caller has not yet been migrated (aim: migrate all callers in
     Task 6; leave no wrappers by end of plan).
4. Update `node_limits` accessors in `settings.go` to be cluster-scoped:
   `GetNodeLimit(clusterID, node)`, `SetNodeLimit(clusterID, limit, changedBy)`,
   `DeleteNodeLimit(clusterID, node, changedBy)`, `SetEnabledNodes` etc.
5. Tests (table-driven, `_test` package): CRUD round-trip, ciphertext stored
   (value has `encv1:` prefix), `default` protected from insert/delete, composite
   PK enforced (duplicate `(cluster_id, name)` rejected).

---

## Task 4 — Load clusters into the registry at startup

**Files:**

- Modify: `backend/main.go` (`initializeApp`)
- Modify: `backend/state/manager.go` / new `manager_clusters.go`

**Steps:**

1. After `LoadSettingsFromDB()`, load DB clusters, decrypt each `token_value`
   with `security.DecryptSecret(tokenValue, envCfg.SessionSecret)`, build
   `[]*proxmox.ClusterConfig`, and call `stateManager.ReplaceClusters(...)`.
2. The registry merges these with the env-backed default.
3. If a DB cluster fails to decrypt (e.g. SESSION_SECRET changed), log a warning
   and mark that cluster disabled in-memory; do not abort startup.
4. On any cluster mutation via the admin API (Task 5), reload the affected entry
   into the registry (decrypt + `Register`/`Replace` of that one row).

---

## Task 5 — Admin cluster CRUD + test-connection API

**Files:**

- New: `backend/api/v1/admin_clusters.go`
- New: `backend/api/v1/admin_clusters_test.go`
- Modify: `backend/api/v1/admin_db.go` (route registration), `backend/api/v1/admin_types.go`,
  `backend/api/v1/validation.go`

**Routes (admin-only, behind `JWTAdminMiddleware`):**

- `GET    /api/v1/admin/clusters`                  → list (token value NEVER sent;
  return `{id, name, description, location, url, token_name, verify_ssl, enabled,
  is_default, source, connected, error}`)
- `POST   /api/v1/admin/clusters`                  → create
- `PUT    /api/v1/admin/clusters/:id`              → update (id immutable)
- `DELETE /api/v1/admin/clusters/:id`              → delete (refuse `default`)
- `POST   /api/v1/admin/clusters/:id/test`         → test connection
  (calls `proxmox.GetNodeNamesResty` with a 10s timeout client; returns
  `{connected, node_count, error}`)
- `GET    /api/v1/admin/clusters/:id/status`       → cached per-cluster connection
  status (Task 7)

**Request validation (`validation.go`):**

- `id`: required on create, matches `constants.ClusterIDPattern`, not `default`.
- `url`: required, https, non-empty host (reuse `env.validateHTTPSURL` — extract
  it to a shared validator).
- `token_name`, `token_value`: required on create; on update, empty `token_value`
  means "keep existing" (do not overwrite). This avoids leaking/requiring the
  secret on every edit.
- `name`: required, 1–80 chars.

**Encryption:** on write, `security.EncryptSecret(req.TokenValue, sessionSecret)`
before `InsertCluster`/`UpdateCluster`. `sessionSecret` comes from
`state.GetEnvConfig().SessionSecret`.

**Tests:** offline-mode handlers return empty/`errOffline`; validation rejects
bad ids/urls; `default` is protected; test-connection handler returns
`connected=false` in offline mode.

---

## Task 6 — Per-cluster state: status, snapshot, node cache

**Files:**

- Modify: `backend/state/manager.go`, `backend/state/manager_proxmox.go`,
  `backend/state/manager_cache.go`, `backend/state/proxmox_snapshot.go`,
  `backend/state/interface.go`

**Steps:**

1. Replace the single connection-status fields with
   `map[string]clusterStatus` (struct of connected/errorMsg/lostTime/failureCount,
   with its own mutex). `GetProxmoxStatus()` returns the **default** cluster's
   status (backwards compat for the existing health endpoint and frontend status
   store). Add `GetClusterStatus(id string) (bool, string)`.
2. Replace `clusterSnapshot *ProxmoxClusterSnapshot` with
   `map[string]*ProxmoxClusterSnapshot` + mutex. `GetProxmoxSnapshot()` returns
   the default cluster's snapshot (backwards compat). Add
   `GetClusterSnapshot(id string) *ProxmoxClusterSnapshot` and
   `AllSnapshots() map[string]*ProxmoxClusterSnapshot`.
3. Replace `nodeCache []*proxmox.NodeDetails` with
   `map[string][]*proxmox.NodeDetails` + per-cluster update timestamps. Add
   `GetClusterNodeCache(id string)`.
4. The monitor/snapshot/node-cache workers iterate `registry.Enabled()`. Each
   cluster is checked/refreshed independently; one cluster's failure does not
   affect others. Keep the existing auto-offline behavior per cluster.
5. `buildProxmoxSnapshot` is unchanged in shape but is called once per cluster
   with that cluster's client. `SnapshotVM` gains a `ClusterID string` field.
6. `StartOnlineMode()` starts workers for the default cluster (and any clusters
   already registered). A newly enabled cluster triggers a one-off refresh +
   joins the worker rotation.

**Interface additions:** `GetClusterStatus(id) (bool,string)`,
`GetClusterSnapshot(id) *ProxmoxClusterSnapshot`, `AllSnapshots()`,
`GetClusterNodeCache(id) ([]*proxmox.NodeDetails, time.Time)`,
`RefreshClusterSnapshot(ctx, id)`.

---

## Task 7 — Admin pages become cluster-aware (backend)

**Files:**

- Modify: `backend/api/v1/admin_handlers.go` (nodes/vms/storage/iso/vmbr/limits
  list endpoints), `backend/api/v1/admin_types.go`, `backend/api/v1/admin_limits.go`,
  `backend/api/v1/admin_db.go`

**Steps:**

1. Admin list endpoints accept a `cluster` query param (default `default`).
   They read from `GetClusterSnapshot(cluster)` / `GetClusterNodeCache(cluster)`
   and the cluster-scoped DB lists.
2. The "available nodes/storages/isos/vmbrs to approve" endpoints
  (`GET /api/v1/admin/.../available`) query the selected cluster's live/snapshot
   data and diff against the cluster-scoped approved list.
3. Mutation endpoints (`SetEnabledNodes`, `SetEnabledStorages`, …,
  `SetNodeLimit`) take `cluster` and call the cluster-scoped DB setters from
   Task 3.
4. `admin/settings-overview` includes a clusters summary (count, ids, per-cluster
   connected flags) so the admin dashboard reflects multi-cluster state.

No change to non-admin endpoints here except the read aggregation in Task 8.

---

## Task 8 — User-facing read paths aggregate across clusters

**Files:**

- Modify: `backend/api/v1/vms.go` (list, get, ownership), `backend/api/v1/vm_details.go`,
  `backend/api/v1/vm_details_snapshot.go`, `backend/api/v1/vm_disks.go`,
  `backend/api/v1/vm_network.go`, `backend/api/v1/vnc.go`, `backend/api/v1/types.go`
- Modify: `backend/proxmox/vms.go` (VM identity carries cluster)

**Steps:**

1. `VMInfo` and the VM list response gain a `clusterId` field. The list handler
   iterates `registry.Enabled()` and merges per-cluster VM lists, tagged with
   each VM's `clusterId`. Non-admin users: filter per cluster by pool membership
   using **that cluster's** client (`fetchPoolVMIDs` becomes cluster-scoped).
2. VM detail / console / snapshot-list / disk / network **read** endpoints
   accept `cluster` (path segment or query). They obtain the client via
   `registry.ClientFor(cluster, ...)` instead of `MakeRestyClientFromEnvConfig`.
   Ownership checks use the cluster's client.
3. **Mutation** endpoints (start/stop/delete/config/snapshot-ops/disk-net edits)
   also accept `cluster` and route to the right client, BUT for non-default
   clusters they return `501 Not Implemented` with code
   `cluster_management_not_enabled` in this iteration. This keeps the plumbing
   complete and the boundary explicit. Default-cluster mutations are unchanged.
4. API doc (`backend/api/v1/docs.go`) updated to document the `cluster` param on
   read endpoints and the 501 on non-default mutations.

**Frontend (read-only):**

- `frontend/src/lib/types/vm-create.ts` and the VM list/detail types gain
  `clusterId`. The VM route changes from `/vm/[id]` to `/vm/[cluster]/[vmid]`
  (`frontend/src/routes/(app)/vm/[id]/` → `[cluster]/[vmid]/`). Update internal
  links and the back-navigation.
- VM list shows a `cluster` column/badge. Detail/console/snapshot/disk/network
  views pass `clusterId` to the API.
- Action buttons (start/stop/delete/snapshot/etc.) are hidden when
  `clusterId !== 'default'`, with an info tooltip "Management on this cluster is
  not yet enabled." The create button is unchanged (targets default).
- `frontend/src/lib/api/vms.ts`, `vm-details.ts`, `vnc.ts`, etc. updated to send
  `cluster` and to use the new route shape.

---

## Task 9 — Admin cluster management frontend

**Files:**

- New: `frontend/src/routes/admin/clusters/+page.svelte`
- New: `frontend/src/lib/api/admin/clusters.ts`
- Modify: `frontend/src/lib/stores/settings.svelte.ts` (or a new
  `clusters.svelte.ts`), admin sidebar (`frontend/src/lib/components/layout/`),
  i18n (`frontend/src/lib/i18n/*.ts`)

**Steps:**

1. API client `clusters.ts`: `listClusters`, `createCluster`, `updateCluster`,
   `deleteCluster`, `testCluster(id)` — never sends `token_value` on read; sends
   it only on create, and omits it on update unless the user explicitly retypes
   it (password-style input with "leave blank to keep").
2. `admin/clusters` page: a table of clusters (name, location, url, token name,
   enabled toggle, connected badge, default badge). Buttons: Add, Edit, Delete,
   Test connection. A form (sheet/dialog) for create/edit with fields per Task 5
   validation. The `default` cluster row is shown but not editable/deletable
   (annotated "from environment variables").
3. Admin sidebar adds "Clusters" entry.
4. Other admin pages (nodes/vms/storage/iso/vmbr/limits) get a cluster selector
   (defaulting to `default`) that scopes their data and mutations.
5. i18n keys added in EN and FR.

---

## Task 10 — Tests, docs, config

**Files:**

- Modify: `backend/docs/admin.en.md`, `backend/docs/admin.fr.md`,
  `backend/docs/proxmox-permissions.en.md`, `backend/docs/proxmox-permissions.fr.md`,
  `backend/docs/user.en.md`, `backend/docs/user.fr.md`
- Modify: `backend/env/doc.go`
- Tests across the changed packages

**Steps:**

1. **Offline tests (`make test-offline`):** extend the offline path so the
   registry can hold a second synthetic cluster without making real calls. Add
   fixtures for a second cluster's snapshot in the offline data. All existing
   offline tests pass unchanged.
2. **Unit tests:** registry, cluster DB CRUD + encryption round-trip, migration
   v5 (PK + `cluster_id='default'` backfill), cluster-scoped list setters,
   admin cluster handlers (validation + `default` protection + offline), per-
   cluster snapshot/status map.
3. **Docs:**
   - `admin.*.md`: new "Clusters" section — how to register a cluster, token
     requirements per cluster, encryption note (keyed off SESSION_SECRET), the
     `default` cluster concept, the read-only boundary.
   - `proxmox-permissions.*.md`: per-cluster token & rights requirements (issue
     point #1).
   - `user.*.md`: the cluster column/badge in the VM list and that management on
     non-default clusters is not yet enabled.
   - `env/doc.go`: note that `PROXMOX_*` now back the `default` cluster and that
     additional clusters are DB-managed.
4. **Helm/K8s:** no manifest change required (still one instance). Add a docs
   note that only one PVMSS instance is now needed to manage multiple clusters.

---

## Open decisions to confirm before implementation

1. **Delete-cluster behavior:** refuse if approvals/limits reference it
   (recommended) vs. cascade-delete those rows. Plan currently assumes *refuse*.
2. **Non-default mutation boundary:** return `501` from the backend (explicit)
   vs. only hide the UI. Plan currently assumes *both* (UI hidden + backend 501
   as a hard guard).
3. **User VM route change** `/vm/[id]` → `/vm/[cluster]/[vmid]` is the largest
   frontend risk in this scope. If an even smaller first cut is desired, defer
   Task 8's user-facing aggregation and keep user views default-only (admin
   multi-cluster only). Flag if you want that trim.

---

## Suggested commit order

1. Task 1 + Task 2 (types + registry, default-only) — no behavior change.
2. Task 3 + migrations v4/v5 + tests — schema in place, default rows backfilled.
3. Task 4 — startup loads DB clusters.
4. Task 5 — admin cluster API.
5. Task 6 — per-cluster status/snapshot/node-cache.
6. Task 7 — admin pages cluster-aware.
7. Task 9 — admin cluster UI.
8. Task 8 — user read aggregation + frontend route change.
9. Task 10 — docs + final test sweep.

Each commit keeps `make test-offline` green and single-cluster behavior identical.

---

## Task tracker

Progress checklist for this plan. Check items off as commits land.

### Pre-implementation — open decisions

- [ ] Confirm delete-cluster behavior (refuse vs cascade) — plan assumes *refuse*.
- [ ] Confirm non-default mutation boundary (501 + UI hidden vs UI-only) — plan assumes *both*.
- [ ] Confirm whether to trim Task 8 to admin-only multi-cluster for the first cut.

### Tracker: Task 1 — `Cluster` types and constants

- [ ] Add `constants.DefaultClusterID` + `constants.ClusterIDPattern`.
- [ ] Create `backend/proxmox/cluster_config.go` with `ClusterConfig`, `ClusterSource`, `IsDefault()`.

### Tracker: Task 2 — `ClusterRegistry` in state

- [ ] Create `backend/state/cluster_registry.go` (Register/Get/All/Enabled/Replace/ClientFor).
- [ ] Wire `clusterRegistry` into `appState`; register default cluster from `EnvConfig` in `SetEnvConfig`.
- [ ] Expose cluster accessors on the `StateManager` interface.
- [ ] Unit tests: default-only, register/get, replace preserves default, invalid ids, `ClientFor` singleton reuse.

### Tracker: Task 3 — DB layer for clusters

- [ ] Migration v4: `clusters` table; bump `currentSchemaVersion` to 4.
- [ ] Migration v5: `cluster_id` columns + composite PKs on `enabled_nodes/storages/isos/vmbrs`, `node_limits`; bump to 5.
- [ ] Create `backend/database/clusters.go` (CRUD, `default` protected, audit).
- [ ] Extend `database.DB` interface with cluster methods.
- [ ] Make `lists.go` + `settings.go` accessors cluster-scoped; migrate all callers (no wrappers left).
- [ ] Tests: CRUD round-trip, `encv1:` ciphertext, `default` protected, composite PK enforced.
- [ ] Migration test: load v1 → run to v5, assert new PKs + `cluster_id='default'` backfill.

### Tracker: Task 4 — Load clusters into the registry at startup

- [ ] In `initializeApp`, load DB clusters, decrypt tokens, call `ReplaceClusters`.
- [ ] Handle decrypt failure (SESSION_SECRET changed): warn + disable cluster in-memory, don't abort.
- [ ] Reload registry entry on admin API cluster mutations (Task 5).

### Tracker: Task 5 — Admin cluster CRUD + test-connection API

- [ ] Create `backend/api/v1/admin_clusters.go` (list/create/update/delete/test/status).
- [ ] Register admin routes in `admin_db.go` behind `JWTAdminMiddleware`.
- [ ] Validation in `validation.go` (id pattern, https url, token required-on-create/optional-on-update, name).
- [ ] Encrypt `token_value` via `security.EncryptSecret` on write; never return it on read.
- [ ] Tests: offline returns empty/`errOffline`, validation, `default` protected, test-connection offline.

### Tracker: Task 6 — Per-cluster state: status, snapshot, node cache

- [ ] Per-cluster connection status map + `GetClusterStatus(id)`.
- [ ] Per-cluster snapshot map + `GetClusterSnapshot(id)`, `AllSnapshots()`.
- [ ] Per-cluster node cache map + `GetClusterNodeCache(id)`.
- [ ] Workers iterate `registry.Enabled()`; per-cluster auto-offline.
- [ ] `SnapshotVM` gains `ClusterID`; `GetProxmoxStatus/Snapshot` stay default-backed (backwards compat).
- [ ] Newly enabled cluster triggers one-off refresh + joins rotation.

### Tracker: Task 7 — Admin pages become cluster-aware (backend)

- [ ] Admin list endpoints accept `cluster` query param (default `default`).
- [ ] "Available to approve" endpoints diff against cluster-scoped approved list.
- [ ] Mutation endpoints take `cluster`, call cluster-scoped DB setters (Task 3).
- [ ] `admin/settings-overview` includes clusters summary (count, ids, connected flags).

### Tracker: Task 8 — User-facing read paths aggregate across clusters

- [ ] `VMInfo`/list response gains `clusterId`; list iterates `registry.Enabled()`.
- [ ] Non-admin pool membership filtered per cluster with that cluster's client.
- [ ] Read endpoints (detail/console/snapshot/disk/network) accept `cluster`, use `registry.ClientFor`.
- [ ] Mutation endpoints accept `cluster`, return 501 for non-default (boundary guard).
- [ ] Update `backend/api/v1/docs.go` for the `cluster` param + 501.
- [ ] Frontend: VM route `/vm/[id]` → `/vm/[cluster]/[vmid]`; types gain `clusterId`.
- [ ] Frontend: VM list cluster column/badge; detail views send `clusterId`.
- [ ] Frontend: hide action controls for non-default cluster VMs with tooltip.

### Tracker: Task 9 — Admin cluster management frontend

- [ ] Create `frontend/src/lib/api/admin/clusters.ts` (token never sent on read).
- [ ] Create `frontend/src/routes/admin/clusters/+page.svelte` (table + form sheet/dialog).
- [ ] `default` row shown read-only ("from environment variables").
- [ ] Admin sidebar "Clusters" entry.
- [ ] Cluster selector on other admin pages (nodes/vms/storage/iso/vmbr/limits).
- [ ] i18n keys in EN + FR.

### Tracker: Task 10 — Tests, docs, config

- [ ] Offline fixtures: support a second synthetic cluster; existing offline tests unchanged.
- [ ] Unit tests: registry, cluster DB CRUD + encryption round-trip, migration v5, cluster-scoped setters, admin handlers, per-cluster snapshot/status.
- [ ] Docs: `admin.*.md` Clusters section; `proxmox-permissions.*.md` per-cluster tokens; `user.*.md` cluster column; `env/doc.go` default-cluster note.
- [ ] Docs: Helm/K8s note (single instance now manages multiple clusters).
- [ ] Final `make test-offline` green sweep.
