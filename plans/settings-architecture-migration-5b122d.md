# Settings Architecture Migration Plan (SQLite-First)

Migrate from monolithic settings.json to a 2-tier SQLite-first configuration system: environment variables for secrets + infrastructure, embedded SQLite for all user-editable configuration. Accessible via admin UI with audit trail.

## Current State Analysis

### Problems with Current Architecture

- **Mixed configuration sources**: Secrets (JWT, admin password, Proxmox creds) mixed with runtime config in settings.json
- **Complex file mounting**: Must mount settings.json as volume in containers
- **Large JSON with embedded YAML**: Cloud-init templates store YAML as JSON strings (hard to edit, can get large)
- **Security concerns**: JWT secret in file instead of environment variable
- **Complex path detection**: Settings file path logic is convoluted
- **No clear separation**: Infrastructure config mixed with user-editable runtime config
- **Poor user experience**: Admins must edit JSON files directly
- **No audit trail**: No history of who changed what and when

### Current Configuration Structure

**settings.json** contains:

- Resource limits (VM limits, node-specific limits)
- Network cards, disk per VM, VM per user limits
- Tags, ISOs, VMBRs (network bridges)
- Cloud-init templates (with embedded YAML content as JSON strings)
- VM profiles
- SFTP configuration for cloud-init
- JWT secret (**security concern — must move to env var**)

**Environment variables** (example.env):

- Proxmox connection settings (API token, URL, SSL)
- Admin password hash
- CSRF token TTL
- Logging configuration
- Session secret
- PVMSS_ENV (production/development)
- Timezone
- PVMSS_OFFLINE mode
- PVMSS_SETTINGS_PATH (path to settings.json)

---

## New Architecture Design

### 2-Tier Configuration System

#### Tier 1: Environment Variables (Secrets + Infrastructure — never in DB)

```bash
# Authentication secrets (REQUIRED)
JWT_SECRET=your-32-byte-secret-here        # Moved out of settings.json
SESSION_SECRET=your-session-secret
ADMIN_PASSWORD_HASH=$2y$10$...

# Proxmox infrastructure (REQUIRED — bootstrapped before DB exists)
PROXMOX_URL=https://host:8006/api2/json
PROXMOX_API_TOKEN_NAME=user@pam!tokenid
PROXMOX_API_TOKEN_VALUE=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
PROXMOX_SSL_VERIFY=true

# Runtime infrastructure
PVMSS_ENV=production
PVMSS_OFFLINE=false
LOG_LEVEL=INFO
LOG_OUTPUT=stdout
LOG_FORMAT=console
TZ=Europe/Paris

# Database path
PVMSS_DB_PATH=/app/pvmss.db
```

**Rationale for keeping Proxmox in env vars**: Proxmox credentials are infrastructure-level secrets needed before the database is bootstrapped. They must be available at first-run to verify connectivity. Moving them to SQLite creates a chicken-and-egg problem where you cannot configure the connection through the admin UI without already having a working connection.

#### Tier 2: SQLite Database (All User-Editable Settings)

All settings that an admin can change at runtime via the admin UI.

**SQLite driver**: `modernc.org/sqlite` (pure Go, zero CGO). This maintains compatibility with the current `gcr.io/distroless/static-debian13:nonroot` deployment. No CGO toolchain required, binary stays fully static.

**SQLite configuration** (applied at connection open):

```sql
PRAGMA journal_mode=WAL;          -- Concurrent reads during writes
PRAGMA busy_timeout=5000;         -- 5s timeout instead of immediate SQLITE_BUSY
PRAGMA foreign_keys=OFF;          -- No FK constraints (audit tables use soft references)
PRAGMA synchronous=NORMAL;        -- Safe with WAL, better performance than FULL
PRAGMA cache_size=-64000;         -- 64MB page cache
```

**In-memory cache strategy**: The `StateManager` maintains a typed in-memory cache of all settings (the existing `AppSettings` struct pattern). On startup, settings are loaded from SQLite into memory. On any write, the database is updated first, then the in-memory cache is invalidated and reloaded. All reads go to memory (zero DB latency). This matches the current architecture's access pattern while adding persistence.

### Database Schema

```sql
-- Core settings: typed columns per domain (not key-value — preserves Go type safety)

-- Resource limits
CREATE TABLE vm_limits (
    id INTEGER PRIMARY KEY CHECK (id = 1),   -- Singleton row
    max_vms INTEGER NOT NULL DEFAULT 10,
    max_vm_per_user INTEGER NOT NULL DEFAULT 2,
    max_network_cards INTEGER NOT NULL DEFAULT 2,
    max_disk_per_vm INTEGER NOT NULL DEFAULT 4,
    allow_custom_yaml BOOLEAN NOT NULL DEFAULT 0,
    max_snapshots INTEGER NOT NULL DEFAULT 3,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Per-node resource limits (JSON blob for node-specific overrides)
CREATE TABLE node_limits (
    node_name TEXT PRIMARY KEY,
    max_vms INTEGER,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Enabled nodes, storages, ISOs, bridges (simple list tables)
CREATE TABLE enabled_nodes (
    name TEXT PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE enabled_storages (
    storage_id TEXT PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE enabled_isos (
    name TEXT PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE enabled_vmbrs (
    name TEXT PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tags (
    name TEXT PRIMARY KEY,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Cloud-init templates (YAML stored as raw text — no embedded JSON)
CREATE TABLE cloudinit_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    storage TEXT,
    filename TEXT,
    yaml_content TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- VM profiles
CREATE TABLE vm_profiles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    config TEXT NOT NULL,   -- JSON blob: cores, memory, disk, etc.
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- SFTP config for cloud-init snippets (singleton)
CREATE TABLE sftp_config (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    enabled BOOLEAN NOT NULL DEFAULT 0,
    host TEXT,
    port INTEGER DEFAULT 22,
    username TEXT,
    private_key_path TEXT,
    remote_path TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Audit trail (no FK — soft reference, survives deletes)
CREATE TABLE audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    table_name TEXT NOT NULL,
    record_id TEXT NOT NULL,
    action TEXT NOT NULL,       -- 'create', 'update', 'delete'
    old_value TEXT,             -- JSON snapshot before change
    new_value TEXT,             -- JSON snapshot after change
    changed_by TEXT NOT NULL,   -- admin username
    changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audit_log_table_record ON audit_log(table_name, record_id);
CREATE INDEX idx_audit_log_changed_at ON audit_log(changed_at);

-- Schema migrations tracking
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- First-run completion marker
CREATE TABLE app_bootstrap (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    completed BOOLEAN NOT NULL DEFAULT 0,
    completed_at TIMESTAMP,
    version TEXT
);
```

**Rationale for typed tables over key-value**: The current `AppSettings` is a typed Go struct. Key-value TEXT pairs lose type safety and make deserialization fragile. Per-domain tables map directly to Go structs, enable proper SQL queries/constraints, and make the schema self-documenting.

### First-Run Workflow

Triggered when: **no settings.json exists AND `app_bootstrap.completed = false` (or DB is newly created)**.

```bash
Startup sequence:
  1. Load env vars (validate required secrets: JWT_SECRET, SESSION_SECRET, ADMIN_PASSWORD_HASH, PROXMOX_*)
  2. Open/create SQLite DB at PVMSS_DB_PATH, apply WAL config
  3. Run schema migrations
  4. Check app_bootstrap.completed
     └─ false → enter FIRST-RUN mode
     └─ true  → normal startup
```

**First-run mode behavior**:

- Backend starts with all Proxmox routes disabled
- Serves a special first-run setup page at `/setup` instead of the main app
- First-run page verifies Proxmox connectivity (using env var credentials)
- Admin configures: approved nodes, ISOs, storages, bridges, tags, limits, VM profiles
- On completion: `app_bootstrap.completed = true`, redirect to admin dashboard
- First-run page is inaccessible once bootstrap is complete (returns 404)

**Offline mode (`PVMSS_OFFLINE=true`) during first-run**:

- Connection test step is skipped automatically; UI shows "Offline mode — connection test skipped"
- Node/storage/ISO/bridge selection uses static placeholder data (same as current offline fixtures)
- First-run still completes normally and sets limits/profiles
- This allows development and CI setups to bootstrap without a live Proxmox instance

### Backward Compatibility (settings.json)

**Priority rule**: If both `settings.json` and a completed SQLite DB exist:

1. **SQLite DB wins** — all settings are read from DB
2. A deprecation warning is logged at startup:

   ```bash
   WARN: settings.json detected at /app/settings.json but SQLite database is active.
   settings.json is DEPRECATED and will be ignored. Remove it to suppress this warning.
   Migration endpoint available: POST /api/v1/admin/migrate-from-json
   ```

3. The migration endpoint remains available to import settings.json into the DB manually

**If only settings.json exists** (no DB or DB bootstrap not completed):

- Migrate settings.json into SQLite automatically on startup
- Log: `INFO: Migrating settings.json to SQLite database...`
- After successful migration: mark bootstrap as complete, log deprecation warning

---

## Implementation Plan

### Phase 0: Spike — Driver + Build Verification

**Objective**: Confirm pure Go SQLite compiles and works in distroless/static image before any real work.

**Tasks**:

1. Add `modernc.org/sqlite` to go.mod
2. Write minimal smoke test: open DB, create table, insert row, read row
3. Build Docker image (`make docker-build`) and verify binary runs in distroless container
4. Benchmark: 1000 reads from in-memory cache vs 1000 reads from SQLite (confirm cache strategy)
5. Confirm WAL mode works with concurrent goroutines (simulate background workers)

**Acceptance Criteria**:

- Binary compiles without CGO (`CGO_ENABLED=0`)
- Docker image builds and runs correctly
- Smoke test passes inside container
- Cache benchmark confirms <1µs memory reads vs ~10-50µs SQLite reads

### Phase 1: Database Package

**Objective**: Create the database layer with schema, migrations, and CRUD operations.

**Files to create**:

```bash
backend/database/
  database.go       — DB connection, WAL config, open/close
  schema.go         — Schema SQL strings
  migrations.go     — Migration runner (apply version N → N+1)
  bootstrap.go      — First-run detection and completion
  settings.go       — vm_limits, sftp_config CRUD
  lists.go          — enabled_nodes/storages/isos/vmbrs/tags CRUD
  cloudinit.go      — cloudinit_templates CRUD
  profiles.go       — vm_profiles CRUD
  audit.go          — audit_log append + query
```

**Database interface**:

```go
type DB interface {
    IsBootstrapComplete() (bool, error)
    CompleteBootstrap(version string) error

    // Resource limits
    GetVMLimits() (*VMLimits, error)
    SetVMLimits(limits *VMLimits, changedBy string) error

    // Per-node limits
    GetNodeLimits() (map[string]int, error)          // node name → max_vms
    SetNodeLimit(node string, maxVMs int, changedBy string) error
    DeleteNodeLimit(node string, changedBy string) error

    // List-based settings (nodes, storages, ISOs, bridges, tags)
    GetEnabledNodes() ([]string, error)
    SetEnabledNodes(nodes []string, changedBy string) error
    GetEnabledStorages() ([]string, error)
    SetEnabledStorages(storages []string, changedBy string) error
    GetEnabledISOs() ([]string, error)
    SetEnabledISOs(isos []string, changedBy string) error
    GetEnabledVMBRs() ([]string, error)
    SetEnabledVMBRs(vmbrs []string, changedBy string) error
    GetTags() ([]string, error)
    SetTags(tags []string, changedBy string) error

    // Cloud-init templates
    ListCloudInitTemplates() ([]CloudInitTemplate, error)
    GetCloudInitTemplate(id string) (*CloudInitTemplate, error)
    CreateCloudInitTemplate(t *CloudInitTemplate, changedBy string) error
    UpdateCloudInitTemplate(t *CloudInitTemplate, changedBy string) error
    DeleteCloudInitTemplate(id string, changedBy string) error

    // VM profiles
    ListVMProfiles() ([]VMProfile, error)
    GetVMProfile(id string) (*VMProfile, error)
    CreateVMProfile(p *VMProfile, changedBy string) error
    UpdateVMProfile(p *VMProfile, changedBy string) error
    DeleteVMProfile(id string, changedBy string) error

    // SFTP config
    GetSFTPConfig() (*SFTPConfig, error)
    SetSFTPConfig(cfg *SFTPConfig, changedBy string) error

    // Convenience: assemble full AppSettings from all tables (used to warm cache)
    LoadAppSettings() (*AppSettings, error)

    // Audit
    ListAuditLog(tableFilter string, limit int, offset int) ([]AuditEntry, error)

    // Backup: uses VACUUM INTO for a consistent point-in-time snapshot
    Backup(destPath string) error

    Close() error
}
```

**Key implementation details**:

- All writes use transactions + append to `audit_log` in same transaction
- Connection pool: single write connection (`*sql.DB` with `MaxOpenConns=1` for writes) + read pool
- `audit_log` entries include JSON snapshots of before/after state

**Acceptance Criteria**:

- All CRUD operations tested (unit tests with in-memory SQLite)
- Audit trail populated on every write
- Migration runner handles version 0 → 1 correctly
- Thread-safe under concurrent access (race detector clean)
- 80%+ test coverage

### Phase 2: Settings Migration from JSON to SQLite

**Objective**: Implement migration of existing settings.json to SQLite.

**Tasks**:

1. Create `backend/database/migrate_json.go`
   - Read `AppSettings` from settings.json
   - Populate all DB tables transactionally
   - Record migration in audit_log (`changed_by = "migration:settings.json"`)
   - Mark bootstrap complete on success
   - Rollback entire transaction on failure

2. Map all fields:

   ```bash
   settings.json → SQLite table
   ─────────────────────────────────────────────
   EnabledNodes        → enabled_nodes
   EnabledStorages     → enabled_storages
   ISOs                → enabled_isos
   VMBRs               → enabled_vmbrs
   Tags                → tags
   Limits              → vm_limits (+ node_limits)
   MaxNetworkCards     → vm_limits.max_network_cards
   MaxDiskPerVM        → vm_limits.max_disk_per_vm
   MaxVMPerUser        → vm_limits.max_vm_per_user
   AllowCustomYAML     → vm_limits.allow_custom_yaml
   CloudInitTemplates  → cloudinit_templates
   VMProfiles          → vm_profiles
   CloudInitSFTP       → sftp_config
   JWTSecret           → NOT MIGRATED (must be set as JWT_SECRET env var)
   ```bash
   ```

3. JWT secret handling:
   - If `settings.json` contains a `jwt_secret` field, log a clear warning:

     ```bash
     WARN: JWTSecret found in settings.json — this will NOT be migrated.
     Set JWT_SECRET environment variable with this value and remove it from settings.json.
     ```

   - Do not write JWT secret to DB under any circumstance

4. Validation post-migration:
   - Count records in each table, compare with input
   - Log summary: `INFO: Migration complete: 3 nodes, 5 ISOs, 2 templates, 4 profiles`

**Acceptance Criteria**:

- Full settings.json migrated in single transaction
- JWT secret never touches DB
- Post-migration validation passes
- Rollback on any error (DB unchanged)
- Unit tests for all field mappings including edge cases (nil slices, empty arrays)

### Phase 3: Environment Variables Layer

**Objective**: Formalize env var loading with validation; remove JWT from settings.

**Files**:

```bash
backend/env/
  loader.go     — Load and validate all env vars
  config.go     — EnvConfig struct
```

**EnvConfig struct**:

```go
type EnvConfig struct {
    // Required secrets
    JWTSecret         string  // JWT_SECRET (min 32 bytes)
    SessionSecret     string  // SESSION_SECRET (min 32 bytes)
    AdminPasswordHash string  // ADMIN_PASSWORD_HASH (bcrypt)
    
    // Required infrastructure
    ProxmoxURL            string  // PROXMOX_URL
    ProxmoxAPITokenName   string  // PROXMOX_API_TOKEN_NAME
    ProxmoxAPITokenValue  string  // PROXMOX_API_TOKEN_VALUE
    ProxmoxSSLVerify      bool    // PROXMOX_SSL_VERIFY (default true)
    
    // Optional with defaults
    DBPath      string  // PVMSS_DB_PATH (default /app/pvmss.db)
    Environment string  // PVMSS_ENV (default "production")
    Offline     bool    // PVMSS_OFFLINE (default false)
    LogLevel    string  // LOG_LEVEL (default "INFO")
    LogOutput   string  // LOG_OUTPUT (default "stdout")
    LogFormat   string  // LOG_FORMAT (default "console")
    Timezone    string  // TZ (default "UTC")
}
```

**Validation rules**:

- Fail fast (exit 1) if any required var is missing or empty
- `JWT_SECRET` must be ≥ 32 bytes
- `SESSION_SECRET` must be ≥ 32 bytes
- `ADMIN_PASSWORD_HASH` must start with `$2` (bcrypt)
- `PROXMOX_URL` must parse as valid HTTPS URL — **skipped if `PVMSS_OFFLINE=true`** (Proxmox vars become optional in offline mode)
- Log a startup summary of loaded config (secrets redacted)

**StateManager update**:

```bash
Startup order:
  1. Load EnvConfig → fail fast if invalid
  2. Open SQLite DB → apply WAL pragma
  3. Run schema migrations
  4. Check bootstrap status
     ├─ Not complete + settings.json exists → auto-migrate then normal start
     ├─ Not complete + no settings.json → first-run mode
     └─ Complete → normal startup
  5. Load settings from DB into in-memory AppSettings cache
```

**JWT middleware update**:

- Read `JWT_SECRET` exclusively from `EnvConfig.JWTSecret`
- Remove any fallback to settings struct

**AppSettings struct update**:

- Remove `JWTSecret` field
- Remove `ProxmoxURL`, `ProxmoxAPITokenName`, `ProxmoxAPITokenValue` (now in env vars)

**Acceptance Criteria**:

- Startup fails with clear error if required env vars missing
- JWT middleware reads only from env vars
- No secrets in AppSettings struct
- Tests cover all validation scenarios

### Phase 4: StateManager Integration

**Objective**: Wire the DB layer into StateManager with in-memory cache.

**Cache pattern**:

```go
type appState struct {
    // existing fields...
    db         database.DB
    settings   *AppSettings     // in-memory cache
    settingsMu sync.RWMutex    // protects settings cache
}

// Read: always from memory (zero DB latency)
func (s *appState) GetSettings() *AppSettings {
    s.settingsMu.RLock()
    defer s.settingsMu.RUnlock()
    return s.settings.clone()  // defensive copy — never return pointer to cache
}

// Write: typed DB call first, then refresh cache from DB
// changedBy must be the authenticated username extracted from the JWT, never hardcoded.
func (s *appState) SetVMLimits(limits *VMLimits, changedBy string) error {
    if err := s.db.SetVMLimits(limits, changedBy); err != nil {
        return err
    }
    return s.reloadSettingsCache()
}

// reloadSettingsCache assembles AppSettings from all DB tables and replaces the cache atomically.
func (s *appState) reloadSettingsCache() error {
    loaded, err := s.db.LoadAppSettings()
    if err != nil {
        return err
    }
    s.settingsMu.Lock()
    s.settings = loaded
    s.settingsMu.Unlock()
    return nil
}
```

**Notes**:

- Each domain setter (limits, tags, profiles, etc.) follows the same pattern: typed DB call → `reloadSettingsCache()`.
- `changedBy` is extracted from the authenticated JWT claims in the HTTP handler, never hardcoded, ensuring audit entries reflect the actual admin user.
- `db.LoadAppSettings()` assembles `*AppSettings` from all tables in a single read transaction.

**Acceptance Criteria**:

- Zero reads from SQLite during normal request handling
- Cache invalidated atomically after any write
- All existing StateManager tests pass
- No regression in background worker behavior

### Phase 5: First-Run UI

**Objective**: Build the first-run setup flow.

**Backend tasks**:

1. New route group `/setup/*` (only active when bootstrap not complete)
2. Endpoints:
   - `GET  /setup/status` — returns `{complete: bool, proxmox_ok: bool}`
   - `POST /setup/test-connection` — test Proxmox connection using env var creds
   - `POST /setup/complete` — save initial config + mark bootstrap complete
3. Middleware: if bootstrap complete and request hits `/setup/*` → 404
4. If bootstrap not complete and request hits any non-`/setup` route → redirect to `/setup`

**Frontend tasks**:

1. New route `/setup` (public, no auth required)
2. Multi-step wizard:
   - Step 1: Connection test (shows Proxmox connectivity status from env vars)
   - Step 2: Approve nodes (checkboxes from live Proxmox data)
   - Step 3: Approve storages, ISOs, bridges
   - Step 4: Set resource limits
   - Step 5: Review + confirm
3. On completion: redirect to `/login`

**Acceptance Criteria**:

- Fresh install with env vars set → first-run page shows automatically
- After completion → normal app accessible
- First-run page inaccessible once complete
- Connection test shows real Proxmox status

### Phase 6: Admin UI Updates

**Objective**: Update all admin pages to read/write from SQLite (via StateManager).

**Tasks**:

1. All existing admin API endpoints continue to work (same URLs, same response shapes)
2. Backend handlers now call `db.*` methods instead of file I/O
3. Add audit log endpoint: `GET /api/v1/admin/audit?table=&limit=&offset=`
4. Add migration endpoint: `POST /api/v1/admin/migrate-from-json` (imports settings.json manually)
5. Frontend: add audit log viewer to admin settings page
6. Frontend: add "Export DB backup" button → `GET /api/v1/admin/db/export`
   - Backend uses `db.Backup(tempPath)` which calls SQLite `VACUUM INTO` for a consistent point-in-time snapshot (safe during live writes, unlike streaming the raw .db file)
   - Serves the resulting file then deletes the temp copy
7. Frontend: add "Import DB backup" button → `POST /api/v1/admin/db/import`
   - Validates uploaded file is a valid SQLite DB before replacing current DB
   - Requires confirmation prompt in UI
   - Restarts settings cache after import

**Acceptance Criteria**:

- All existing admin UI flows work unchanged
- Audit log visible in UI
- DB export/import functional
- No regression in existing frontend tests

### Phase 7: Deployment Updates

**Objective**: Update all deployment artifacts.

**Tasks**:

1. **Dockerfile**: No changes needed (pure Go SQLite, static binary unchanged). Add `PVMSS_DB_PATH` default.
2. **docker-compose.example.yml**:
   - Add `JWT_SECRET`, `PROXMOX_*` env vars
   - Add `pvmss.db` volume mount
   - Remove `settings.json` volume mount
   - Remove `PVMSS_SETTINGS_PATH`
3. **Kubernetes manifests**: Add `JWT_SECRET` to Secret, add PVC for SQLite, update deployment mounts
4. **Helm chart**: Add DB PVC template, update values.yaml with new env vars
5. **example.env**: Add all new required vars with documentation
6. **Migration guide** (`docs/migration-v1-v2.md`):
   - Extract JWT secret from settings.json, set as env var
   - Set Proxmox credentials as env vars
   - Deploy new version → auto-migration runs
   - Verify admin UI works
   - Remove settings.json
   - Rollback: restore .db backup + old binary

**Acceptance Criteria**:

- Docker Compose starts cleanly with new config
- Kubernetes deploys correctly
- Migration guide is complete and tested

### Phase 8: Cleanup

**Objective**: Remove all legacy code once migration is stable (after one release cycle).

**Tasks**:

1. Remove `backend/state/settings.go` file-based I/O (`LoadSettings()`, `WriteSettings()`)
2. Remove `JWTSecret` from all struct definitions and tests
3. Remove `PVMSS_SETTINGS_PATH` env var support
4. Remove legacy API endpoints (`/api/settings`, `/api/settings/all`, `/api/vmbr/all`)
5. Remove migration endpoint (`/api/v1/admin/migrate-from-json`) after 2 release cycle
6. Update all tests to use DB fixtures instead of settings.json test files

**Acceptance Criteria**:

- No references to settings.json in codebase (except migration guide)
- All tests pass
- No deprecated routes

---

## Architecture Decisions Record

| Decision | Choice | Rationale |
| -------- | ------ | --------- |
| SQLite driver | `modernc.org/sqlite` (pure Go) | Stays CGO-free, compatible with distroless/static image, zero attack surface increase |
| Proxmox creds location | Tier 1 env vars (not DB) | Needed before DB bootstrap; infrastructure-level secrets |
| JWT secret location | Tier 1 env vars only | Never in file, never in DB; rotate without app restart |
| Schema design | Typed tables per domain | Type safety, Go struct mapping, SQL constraints, queryable |
| Audit table FK | None (soft reference) | Deleting a template should not prevent audit history from existing |
| Cache strategy | In-memory write-through | Zero read latency; atomic invalidation on write |
| SQLite WAL mode | Enabled | Concurrent readers during admin writes; background workers unblocked |
| First-run trigger | `app_bootstrap.completed = false` AND no valid settings.json | Clean detection, covers fresh install and DB-only deployments |
| settings.json priority | DB always wins if bootstrap complete | Clear precedence; deprecated path removed cleanly |
| Audit writer | Same transaction as the data write | Audit is always consistent with data; no partial records |

---

## Migration Strategy

### Rollout Sequence

1. Phase 0 (spike) → merge immediately if passes
2. Phases 1–3 (DB + env) → one PR, all backend, no frontend changes
3. Phase 4 (StateManager) → separate PR, integration tests required
4. Phase 5 (first-run UI) → separate PR, frontend + backend
5. Phase 6 (admin UI updates) → separate PR
6. Phase 7 (deployment) → release PR with migration guide
7. Phase 8 (cleanup) → follow-up release

### Rollback Plan

At any phase:

1. DB file is the only state — `cp pvmss.db pvmss.db.bak` before upgrade
2. Export endpoint available from Phase 6 onward
3. Previous binary + settings.json works until Phase 8 cleanup

---

## Risk Mitigation

| Risk | Mitigation |
| ---- | ---------- |
| `modernc.org/sqlite` performance vs mattn | Phase 0 benchmark; WAL + cache config closes most gaps |
| DB corruption | WAL mode (ACID), regular export endpoint, single-file backup |
| First-run never triggered on existing install | Only triggers if `app_bootstrap.completed = false`; migration from settings.json sets it to true |
| JWT secret accidentally ends up in DB | `JWTSecret` removed from `AppSettings` struct; migration explicitly skips it with a warning |
| settings.json left in place after migration | Startup warning on every boot; deprecation doc; removal instructions in migration guide |
| Concurrent write contention | Single write connection (`MaxOpenConns=1`), WAL allows concurrent reads, `busy_timeout=5000ms` |
| First-run page accessible post-setup | Middleware checks `app_bootstrap.completed` on every `/setup/*` request |

---

## Success Criteria

### Technical

- ✅ Pure Go binary, zero CGO — distroless/static image unchanged
- ✅ JWT secret exclusively in env var — never in file or DB
- ✅ Proxmox creds in env vars — DB-independent
- ✅ All user-editable settings in typed SQLite tables
- ✅ In-memory cache — zero DB reads on hot path
- ✅ Audit trail on every write in same transaction
- ✅ First-run workflow for fresh installs
- ✅ Backward compat: settings.json auto-migrated, DB wins if both exist
- ✅ 80%+ test coverage on all new packages

### User Experience

- ✅ First install guided via setup wizard
- ✅ All admin UI flows unchanged post-migration
- ✅ Audit log visible in admin UI
- ✅ DB export/import for backup/restore
- ✅ Clear deprecation messaging for settings.json

### Security

- ✅ No secrets in any configuration file
- ✅ JWT + session secrets in env vars only
- ✅ Audit trail captures all setting changes
- ✅ DB file permissions: 0600 (same as current settings.json)
- ✅ First-run page inaccessible after bootstrap
- ✅ Minimal attack surface (no CGO, no dynamic linking)
