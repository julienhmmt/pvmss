# Settings Architecture Migration Tasks

Generated from settings-architecture-migration-5b122d.md

## Feature Overview

Migrate from monolithic settings.json to a 2-tier SQLite-first configuration system: environment variables for secrets + infrastructure, embedded SQLite for all user-editable configuration. Accessible via admin UI with audit trail.

## Implementation Strategy

**MVP First**: Phase 0 (Spike) → Phase 1 (Database Package) → Phase 2 (Migration) → Phase 3 (Env Vars) → Phase 4 (StateManager Integration)

**Incremental Delivery**: Each phase is independently testable and can be merged separately.

**Rollout Sequence**:

- Phase 0 → immediate merge if passes
- Phases 1-3 → one PR (backend only)
- Phase 4 → separate PR with integration tests
- Phase 5 → separate PR (frontend + backend)
- Phase 6 → separate PR
- Phase 7 → release PR with migration guide
- Phase 8 → follow-up release

---

## Phase 0: Spike — Driver + Build Verification

**Goal**: Confirm pure Go SQLite compiles and works in distroless/static image before any real work.

**Independent Test Criteria**:

- Binary compiles without CGO (`CGO_ENABLED=0`)
- Docker image builds and runs correctly
- Smoke test passes inside container
- Cache benchmark confirms <1µs memory reads vs ~10-50µs SQLite reads

**Tasks**:

- [x] T001 Add modernc.org/sqlite dependency to backend/go.mod
- [x] T002 Create smoke test file backend/database/spike_test.go with minimal DB operations (open, create table, insert, read)
- [x] T003 Run smoke test with CGO_ENABLED=0 to verify pure Go compilation
- [x] T004 Build Docker image using make docker-build command
- [ ] T005 Verify binary runs in distroless container with smoke test
- [x] T006 Create benchmark test comparing 1000 in-memory cache reads vs 1000 SQLite reads
- [x] T007 Run benchmark and confirm memory reads <1µs and SQLite reads ~10-50µs
- [x] T008 Create concurrent access test to verify WAL mode works with multiple goroutines
- [x] T009 Run concurrent access test with race detector to ensure thread-safety
- [x] T010 Document spike results in backend/database/SPIKE_RESULTS.md with compilation, benchmark, and concurrency findings

---

## Phase 1: Database Package

**Goal**: Create the database layer with schema, migrations, and CRUD operations.

**Independent Test Criteria**:

- All CRUD operations tested (unit tests with in-memory SQLite)
- Audit trail populated on every write
- Migration runner handles version 0 → 1 correctly
- Thread-safe under concurrent access (race detector clean)
- 80%+ test coverage

**Tasks**:

- [ ] T011 Create backend/database/doc.go with package documentation
- [ ] T012 [P] Create backend/database/database.go with DB connection, WAL config, open/close methods
- [ ] T013 [P] Create backend/database/schema.go with all CREATE TABLE SQL strings
- [ ] T014 [P] Create backend/database/migrations.go with migration runner (apply version N → N+1)
- [ ] T015 [P] Create backend/database/bootstrap.go with first-run detection and completion logic
- [ ] T016 [P] Create backend/database/settings.go with vm_limits and sftp_config CRUD operations
- [ ] T017 [P] Create backend/database/lists.go with enabled_nodes/storages/isos/vmbrs/tags CRUD operations
- [ ] T018 [P] Create backend/database/cloudinit.go with cloudinit_templates CRUD operations
- [ ] T019 [P] Create backend/database/profiles.go with vm_profiles CRUD operations
- [ ] T020 [P] Create backend/database/audit.go with audit_log append and query operations
- [ ] T021 Define DB interface in backend/database/database.go with all required methods (IsBootstrapComplete, GetVMLimits, SetVMLimits, etc.)
- [ ] T022 Implement transaction handling in all write methods to include audit_log entries
- [ ] T023 Configure connection pool with MaxOpenConns=1 for writes and read pool for reads
- [ ] T024 Write unit tests for database.go in backend/database/database_test.go
- [ ] T025 Write unit tests for schema.go in backend/database/schema_test.go
- [ ] T026 Write unit tests for migrations.go in backend/database/migrations_test.go
- [ ] T027 Write unit tests for bootstrap.go in backend/database/bootstrap_test.go
- [ ] T028 Write unit tests for settings.go in backend/database/settings_test.go
- [ ] T029 Write unit tests for lists.go in backend/database/lists_test.go
- [ ] T030 Write unit tests for cloudinit.go in backend/database/cloudinit_test.go
- [ ] T031 Write unit tests for profiles.go in backend/database/profiles_test.go
- [ ] T032 Write unit tests for audit.go in backend/database/audit_test.go
- [ ] T033 Run all database tests with race detector to ensure thread-safety
- [ ] T034 Check test coverage for backend/database package and ensure 80%+ coverage
- [ ] T035 Implement LoadAppSettings method in backend/database/settings.go to assemble full AppSettings from all tables

---

## Phase 2: Settings Migration from JSON to SQLite

**Goal**: Implement migration of existing settings.json to SQLite.

**Independent Test Criteria**:

- Full settings.json migrated in single transaction
- JWT secret never touches DB
- Post-migration validation passes
- Rollback on any error (DB unchanged)
- Unit tests for all field mappings including edge cases

**Tasks**:

- [ ] T036 Create backend/database/migrate_json.go with migration logic
- [ ] T037 Implement read AppSettings from settings.json in migrate_json.go
- [ ] T038 Implement populate all DB tables transactionally in migrate_json.go
- [ ] T039 Add audit_log entry for migration with changed_by = "migration:settings.json"
- [ ] T040 Implement mark bootstrap complete on success in migrate_json.go
- [ ] T041 Implement rollback transaction on any failure in migrate_json.go
- [ ] T042 Map EnabledNodes field to enabled_nodes table
- [ ] T043 Map EnabledStorages field to enabled_storages table
- [ ] T044 Map ISOs field to enabled_isos table
- [ ] T045 Map VMBRs field to enabled_vmbrs table
- [ ] T046 Map Tags field to tags table
- [ ] T047 Map Limits field to vm_limits table
- [ ] T048 Map MaxNetworkCards to vm_limits.max_network_cards
- [ ] T049 Map MaxDiskPerVM to vm_limits.max_disk_per_vm
- [ ] T050 Map MaxVMPerUser to vm_limits.max_vm_per_user
- [ ] T051 Map AllowCustomYAML to vm_limits.allow_custom_yaml
- [ ] T052 Map MaxSnapshots to vm_limits.max_snapshots
- [ ] T053 Map CloudInitTemplates field to cloudinit_templates table
- [ ] T054 Map VMProfiles field to vm_profiles table
- [ ] T055 Map CloudInitSFTP field to sftp_config table
- [ ] T056 Add JWT secret detection and warning in migrate_json.go (do NOT migrate to DB)
- [ ] T057 Implement post-migration validation (count records in each table)
- [ ] T058 Add migration summary logging (INFO: Migration complete: X nodes, Y ISOs, etc.)
- [ ] T059 Write unit tests for migrate_json.go with sample settings.json fixtures
- [ ] T060 Write unit tests for JWT secret warning logic
- [ ] T061 Write unit tests for rollback behavior on migration errors
- [ ] T062 Write unit tests for edge cases (nil slices, empty arrays, missing fields)
- [ ] T063 Test migration with real settings.json from example.env

---

## Phase 3: Environment Variables Layer

**Goal**: Formalize env var loading with validation; remove JWT from settings.

**Independent Test Criteria**:

- Startup fails with clear error if required env vars missing
- JWT middleware reads only from env vars
- No secrets in AppSettings struct
- Tests cover all validation scenarios

**Tasks**:

- [x] T064 Create backend/env/doc.go with package documentation
- [x] T065 [P] Create backend/env/loader.go with LoadAndValidate function
- [x] T066 [P] Create backend/env/config.go with EnvConfig struct definition
- [x] T067 Define EnvConfig struct fields: JWTSecret, SessionSecret, AdminPasswordHash, ProxmoxURL, ProxmoxAPITokenName, ProxmoxAPITokenValue, ProxmoxSSLVerify, DBPath, Environment, Offline, LogLevel, LogOutput, LogFormat, Timezone
- [x] T068 Implement JWT_SECRET validation (min 32 bytes)
- [x] T069 Implement SESSION_SECRET validation (min 32 bytes)
- [x] T070 Implement ADMIN_PASSWORD_HASH validation (must start with $2 for bcrypt)
- [x] T071 Implement PROXMOX_URL validation (must parse as valid HTTPS URL, skipped if PVMSS_OFFLINE=true)
- [x] T072 Implement fail-fast behavior (exit 1 if any required var missing or empty)
- [x] T073 Implement default values for optional env vars (DBPath, Environment, Offline, LogLevel, LogOutput, LogFormat, Timezone)
- [x] T074 Add startup summary logging with secrets redacted
- [x] T075 Write unit tests for env/loader.go covering all validation scenarios
- [x] T076 Write unit tests for missing required env vars
- [x] T077 Write unit tests for invalid env var values
- [x] T078 Write unit tests for default value assignments
- [x] T079 Write unit tests for offline mode (Proxmox vars optional)
- [x] T080 Update backend/main.go to load EnvConfig on startup
- [x] T081 Update backend/main.go startup order: Load EnvConfig → Open SQLite DB → Run migrations → Check bootstrap
- [x] T082 Update JWT middleware in backend/api/v1/ to read JWT_SECRET from EnvConfig only
- [x] T083 Remove fallback to settings struct in JWT middleware
- [x] T084 Remove JWTSecret field from AppSettings struct in backend/state/settings.go
- [x] T085 N/A — ProxmoxURL/APIToken fields were never in AppSettings struct
- [x] T086 N/A — No AppSettings-level JWTSecret in tests (database migration tests use JSONSettings, which is separate)
- [x] T087 N/A — Proxmox credentials were never in AppSettings
- [x] T088 Covered by TestLoadAndValidate_MissingRequiredVars / TestLoadAndValidate_AllErrorsReportedTogether
- [x] T089 Covered by TestLoadAndValidate_AllRequired_Valid
- [x] T090 Covered by JWT middleware using injected jwtSecret string (no settings dependency)

---

## Phase 4: StateManager Integration

**Goal**: Wire the DB layer into StateManager with in-memory cache.

**Independent Test Criteria**:

- Zero reads from SQLite during normal request handling
- Cache invalidated atomically after any write
- All existing StateManager tests pass
- No regression in background worker behavior

**Tasks**:

- [ ] T091 Add db field to appState struct in backend/state/manager.go
- [ ] T092 Add settings field (*AppSettings) to appState struct for in-memory cache
- [ ] T093 Add settingsMu sync.RWMutex to appState struct to protect settings cache
- [ ] T094 Implement GetSettings() method in backend/state/manager.go with read lock and defensive copy
- [ ] T095 Implement SetVMLimits() method in backend/state/manager.go with DB call then cache reload
- [ ] T096 Implement SetNodeLimit() method in backend/state/manager.go with DB call then cache reload
- [ ] T097 Implement DeleteNodeLimit() method in backend/state/manager.go with DB call then cache reload
- [ ] T098 Implement SetEnabledNodes() method in backend/state/manager.go with DB call then cache reload
- [ ] T099 Implement SetEnabledStorages() method in backend/state/manager.go with DB call then cache reload
- [ ] T100 Implement SetEnabledISOs() method in backend/state/manager.go with DB call then cache reload
- [ ] T101 Implement SetEnabledVMBRs() method in backend/state/manager.go with DB call then cache reload
- [ ] T102 Implement SetTags() method in backend/state/manager.go with DB call then cache reload
- [ ] T103 Implement CreateCloudInitTemplate() method in backend/state/manager.go with DB call then cache reload
- [ ] T104 Implement UpdateCloudInitTemplate() method in backend/state/manager.go with DB call then cache reload
- [ ] T105 Implement DeleteCloudInitTemplate() method in backend/state/manager.go with DB call then cache reload
- [ ] T106 Implement CreateVMProfile() method in backend/state/manager.go with DB call then cache reload
- [ ] T107 Implement UpdateVMProfile() method in backend/state/manager.go with DB call then cache reload
- [ ] T108 Implement DeleteVMProfile() method in backend/state/manager.go with DB call then cache reload
- [ ] T109 Implement SetSFTPConfig() method in backend/state/manager.go with DB call then cache reload
- [ ] T110 Implement reloadSettingsCache() method in backend/state/manager.go to assemble AppSettings from DB and replace cache atomically
- [ ] T111 Update all setter methods to extract changedBy from authenticated JWT claims (never hardcoded)
- [ ] T112 Update backend/main.go to initialize StateManager with DB instance
- [ ] T113 Update backend/main.go to load settings from DB into in-memory cache on startup
- [ ] T114 Add auto-migration logic in backend/main.go: if settings.json exists and bootstrap not complete, migrate then normal start
- [ ] T115 Add first-run mode logic in backend/main.go: if bootstrap not complete and no settings.json, enter first-run mode
- [ ] T116 Add deprecation warning logging in backend/main.go when both settings.json and completed DB exist
- [ ] T117 Update all existing StateManager tests to use DB fixtures instead of settings.json
- [ ] T118 Write integration tests for cache invalidation after writes
- [ ] T119 Write integration tests for concurrent read/write access
- [ ] T120 Test that all existing StateManager tests still pass
- [ ] T121 Test background worker behavior with new cache pattern
- [ ] T122 Verify zero DB reads during normal request handling with profiling

---

## Phase 5: First-Run UI

**Goal**: Build the first-run setup flow.

**Independent Test Criteria**:

- Fresh install with env vars set → first-run page shows automatically
- After completion → normal app accessible
- First-run page inaccessible once complete
- Connection test shows real Proxmox status

**Tasks**:

- [ ] T123 Create backend/handlers/setup.go with setup route handlers
- [ ] T124 Register /setup/* route group in backend/handlers/handlers.go
- [ ] T125 Implement GET /setup/status endpoint returning {complete: bool, proxmox_ok: bool}
- [ ] T126 Implement POST /setup/test-connection endpoint to test Proxmox connection using env var creds
- [ ] T127 Implement POST /setup/complete endpoint to save initial config and mark bootstrap complete
- [ ] T128 Add middleware to return 404 for /setup/* requests when bootstrap complete
- [ ] T129 Add middleware to redirect to /setup when bootstrap not complete and request hits non-setup route
- [ ] T130 Write unit tests for setup.go handlers
- [ ] T131 Create frontend/src/routes/(public)/setup/+page.svelte — SvelteKit first-run wizard root
- [ ] T132 Create frontend/src/lib/api/setup.ts — typed API client for /setup/* endpoints
- [ ] T133 Implement Step 1 in +page.svelte: Connection test UI (shows Proxmox connectivity status from env vars)
- [ ] T134 Implement Step 2 in +page.svelte: Approve nodes UI (checkboxes from live Proxmox data)
- [ ] T135 Implement Step 3 in +page.svelte: Approve storages, ISOs, bridges UI
- [ ] T136 Implement Step 4 in +page.svelte: Set resource limits UI
- [ ] T137 Implement Step 5 in +page.svelte: Review + confirm UI
- [ ] T138 Add offline mode handling in +page.svelte (skip connection test step, show placeholder data)
- [ ] T139 Add redirect to /login on setup completion using SvelteKit goto()
- [ ] T140 Test first-run flow with fresh install and env vars set
- [ ] T141 Test first-run flow in offline mode
- [ ] T142 Test that first-run page returns 404 after bootstrap complete
- [ ] T143 Test that non-setup routes redirect to /setup when bootstrap not complete

---

## Phase 6: Admin UI Updates

**Goal**: Update all admin pages to read/write from SQLite (via StateManager).

**Independent Test Criteria**:

- All existing admin UI flows work unchanged
- Audit log visible in UI
- DB export/import functional
- No regression in existing frontend tests

**Tasks**:

- [ ] T144 Update backend/handlers/limits_helpers.go to call db.SetVMLimits instead of file I/O
- [ ] T145 Update backend/handlers/tags.go to call db.SetTags instead of file I/O
- [ ] T146 Update backend/handlers/settings_iso.go to call db.SetEnabledISOs instead of file I/O
- [ ] T147 Update backend/handlers/vmbr.go to call db.SetEnabledVMBRs instead of file I/O
- [ ] T148 Update backend/handlers/storage.go to call db.SetEnabledStorages instead of file I/O
- [ ] T149 Update backend/handlers/user_pool.go to call db.SetEnabledNodes instead of file I/O
- [ ] T150 Update backend/handlers/cloudinit.go to call db CRUD methods for templates
- [ ] T151 Update backend/handlers/vm_profiles.go to call db CRUD methods for profiles
- [ ] T152 Update backend/handlers/sftp.go to call db.SetSFTPConfig instead of file I/O
- [ ] T153 Implement GET /api/v1/admin/audit endpoint with table, limit, offset query params
- [ ] T154 Implement POST /api/v1/admin/migrate-from-json endpoint to manually import settings.json
- [ ] T155 Implement GET /api/v1/admin/db/export endpoint using db.Backup with VACUUM INTO
- [ ] T156 Implement POST /api/v1/admin/db/import endpoint with DB validation and replacement
- [ ] T157 Add restart of settings cache after DB import
- [ ] T158 Add audit log viewer section to frontend/src/routes/admin/settings/+page.svelte (create page if it doesn't exist)
- [ ] T159 Add "Export DB backup" button to admin settings page with download handler
- [ ] T160 Add "Import DB backup" button to admin settings page with confirmation modal
- [ ] T161 Create frontend/src/lib/api/admin/audit.ts — typed API client for GET /api/v1/admin/audit
- [ ] T162 Create frontend/src/lib/api/admin/db.ts — typed API client for DB export/import endpoints
- [ ] T163 Create frontend/src/lib/components/AuditLog.svelte — reusable audit log table component
- [ ] T164 Write unit tests for new admin API endpoints
- [ ] T165 Test all existing admin UI flows (limits, tags, ISOs, VMBRs, storage, user pools)
- [ ] T166 Test audit log viewer in UI
- [ ] T167 Test DB export functionality
- [ ] T168 Test DB import functionality
- [ ] T169 Test manual migration endpoint with sample settings.json
- [ ] T170 Run existing frontend tests to ensure no regression

---

## Phase 7: Deployment Updates

**Goal**: Update all deployment artifacts.

**Independent Test Criteria**:

- Docker Compose starts cleanly with new config
- Kubernetes deploys correctly
- Migration guide is complete and tested

**Tasks**:

- [x] T171 Add PVMSS_DB_PATH default to Dockerfile
- [x] T172 Update docker-compose.example.yml with JWT_SECRET env var
- [x] T173 Update docker-compose.example.yml with SESSION_SECRET env var
- [x] T174 Update docker-compose.example.yml with ADMIN_PASSWORD_HASH env var
- [x] T175 Update docker-compose.example.yml with PROXMOX_URL env var
- [x] T176 Update docker-compose.example.yml with PROXMOX_API_TOKEN_NAME env var
- [x] T177 Update docker-compose.example.yml with PROXMOX_API_TOKEN_VALUE env var
- [x] T178 Update docker-compose.example.yml with PROXMOX_SSL_VERIFY env var
- [x] T179 Update docker-compose.example.yml with pvmss.db volume mount
- [x] T180 Remove settings.json volume mount from docker-compose.example.yml
- [x] T181 Remove PVMSS_SETTINGS_PATH from docker-compose.example.yml
- [x] T182 Update Kubernetes manifests to add JWT_SECRET to Secret
- [x] T183 Update Kubernetes manifests to add PVC for SQLite
- [x] T184 Update Kubernetes deployment mounts for pvmss.db
- [x] T185 Update Helm chart to add DB PVC template
- [x] T186 Update Helm chart values.yaml with new env vars
- [x] T187 Update example.env with JWT_SECRET documentation
- [x] T188 Update example.env with SESSION_SECRET documentation
- [x] T189 Update example.env with ADMIN_PASSWORD_HASH documentation
- [x] T190 Update example.env with PROXMOX_* env var documentation
- [x] T191 Update example.env with PVMSS_DB_PATH documentation
- [x] T192 Create docs/migration-v1-v2.md migration guide
- [x] T193 Document extracting JWT secret from settings.json in migration guide
- [x] T194 Document setting Proxmox credentials as env vars in migration guide
- [x] T195 Document deploying new version and auto-migration in migration guide
- [x] T196 Document verifying admin UI works in migration guide
- [x] T197 Document removing settings.json in migration guide
- [x] T198 Document rollback procedure (restore .db backup + old binary) in migration guide
- [ ] T199 Test Docker Compose startup with new configuration
- [ ] T200 Test Kubernetes deployment with new manifests
- [ ] T201 Test migration guide instructions with fresh deployment

---

## Phase 9: Unified Settings Panel (View / Add / Edit, No Delete)

**Goal**: Extend the `/admin/settings` page so every DB-backed setting is visible in one place, with inline add/edit capability. Deletion of settings records is explicitly forbidden (disabling is allowed where the schema already supports it, e.g. `enabled` flag).

**Rationale**: After Phase 7, configuration lives in SQLite. Admins today must navigate through multiple pages (limits, tags, ISOs, VMBRs, storage, nodes, cloudinit, profiles, SFTP) to inspect state. A single consolidated panel makes the full configuration auditable at a glance while keeping existing per-resource pages as power-user editors.

**Independent Test Criteria**:

- All DB tables covered by Phase 1 are represented in the panel (vm_limits, node_limits, enabled_nodes, enabled_storages, enabled_isos, enabled_vmbrs, tags, cloudinit_templates, vm_profiles, sftp_config).
- Every row shows its current value and `updated_at` (where available).
- Add + edit flows succeed end-to-end and appear in `audit_log`.
- No delete button/endpoint is exposed from this panel; backend rejects DELETE requests routed through the unified endpoint.
- Disable-via-flag (e.g. `enabled=false`) works and is reversible.
- Existing per-resource admin pages continue to function unchanged.

**Backend Tasks**:

- [ ] T221 Create backend/handlers/admin_settings_overview.go with an aggregated read handler
- [ ] T222 Implement GET /api/v1/admin/settings/overview returning a typed snapshot of every DB table (vm_limits, node_limits, enabled_nodes, enabled_storages, enabled_isos, enabled_vmbrs, tags, cloudinit_templates, vm_profiles, sftp_config, bootstrap status)
- [ ] T223 Add per-section metadata to the overview response (schema version, row count, `updated_at`, whether section supports add/edit)
- [ ] T224 Create POST /api/v1/admin/settings/upsert endpoint dispatching to the right `state.SetX` / `state.CreateX` / `state.UpdateX` method based on a `{table, record}` payload
- [ ] T225 Explicitly reject DELETE verbs and any `action: "delete"` payload in the upsert endpoint with HTTP 405 + audit-safe log line
- [ ] T226 Reuse existing validation helpers from backend/handlers/limits_helpers.go, tags.go, vmbr.go, etc. — no duplicated validation logic
- [ ] T227 Ensure every mutation goes through StateManager so cache + audit invariants from Phase 4 remain intact
- [ ] T228 Add admin-only middleware + CSRF protection on the new endpoints (reuse existing `AdminOnly`, `CSRFProtect`)
- [ ] T229 Write unit tests for admin_settings_overview.go covering: full snapshot shape, empty DB, each section present, delete rejection, audit-log side-effect
- [ ] T230 Write integration test: upsert → overview reflects change → audit_log contains entry with `changed_by` from JWT

**Frontend Tasks**:

- [ ] T231 Create frontend/src/lib/api/admin/settings-overview.ts — typed client for GET /overview and POST /upsert
- [ ] T232 Create frontend/src/lib/types/admin-settings.ts — mirror of the backend snapshot structure (no `any`, one export per file)
- [ ] T233 Extend frontend/src/routes/admin/settings/+page.svelte with a new "Configuration overview" section above the existing DB management block
- [ ] T234 Create frontend/src/lib/components/admin/SettingsOverview.svelte — accordion/tab layout with one panel per table
- [ ] T235 Create frontend/src/lib/components/admin/SettingsSection.svelte — reusable section component (title, row count, `updated_at`, rows table)
- [ ] T236 Render scalar singletons (vm_limits, sftp_config) as a read-only summary + "Edit" button opening an inline form
- [ ] T237 Render list tables (nodes, storages, ISOs, VMBRs, tags) as a table with inline "Add" row and per-row "Edit" action (toggle `enabled`, rename where schema allows)
- [ ] T238 Render keyed tables (cloudinit_templates, vm_profiles, node_limits) with "Add" button + per-row "Edit" modal; reuse existing forms from per-resource pages when possible
- [ ] T239 Hide/omit any delete affordance in every sub-component; rely on disable flags where supported
- [ ] T240 Wire add/edit actions to POST /api/v1/admin/settings/upsert and invalidate the overview query on success
- [ ] T241 Surface audit entries inline (last change per section via join with audit_log) using the existing AuditLog component as reference
- [ ] T242 Add i18n keys under `admin.settings.overview.*` in both EN and FR translation files
- [ ] T243 Add empty-state + error-state UI for each section (loading skeleton, empty message, retry on failure)
- [ ] T244 Verify keyboard navigation + focus order across accordion sections (accessibility)

**Documentation & Verification**:

- [ ] T245 Document the "no delete" policy in docs/migration-v1-v2.md ("Why deletions are disabled in the unified panel")
- [ ] T246 Add screenshot / description of the unified panel to the admin section of README.md and README.fr.md
- [ ] T247 Manual test: edit vm_limits → overview refreshes → audit row visible with correct `changed_by`
- [ ] T248 Manual test: attempt DELETE via curl on /api/v1/admin/settings/upsert returns 405
- [ ] T249 Manual test: toggle `enabled=false` on an ISO → VM creation UI no longer lists it
- [ ] T250 Run existing frontend tests to ensure no regression on `/admin/settings`

---

## Phase 8: Cleanup

**Goal**: Remove all legacy code once migration is stable (after one release cycle).

**Independent Test Criteria**:

- No references to settings.json in codebase (except migration guide)
- All tests pass
- No deprecated routes

**Tasks**:

- [x] T202 Remove LoadSettings() function from backend/state/settings.go
- [x] T203 Remove WriteSettings() function from backend/state/settings.go
- [x] T204 Remove all file-based I/O code from backend/state/settings.go
- [x] T205 Remove JWTSecret from all struct definitions
- [x] T206 Remove JWTSecret from all test fixtures
- [x] T207 Remove PVMSS_SETTINGS_PATH env var support from backend/main.go
- [x] T208 Remove PVMSS_SETTINGS_PATH from example.env
- [x] T209 Remove /api/settings legacy endpoint
- [x] T210 Remove /api/settings/all legacy endpoint
- [x] T211 Remove /api/vmbr/all legacy endpoint
- [x] T212 Remove POST /api/v1/admin/migrate-from-json endpoint
- [x] T213 Update all tests to use DB fixtures instead of settings.json test files
- [x] T214 Delete any remaining settings.json test fixtures
- [x] T215 Search codebase for any remaining references to settings.json and remove
- [x] T216 Search codebase for any remaining references to JWTSecret in structs and remove
- [x] T217 Run full test suite to ensure all tests pass
- [x] T218 Verify no deprecated routes remain in route registration
- [x] T219 Update README.md to reflect new configuration system
- [x] T220 Update README.fr.md to reflect new configuration system

---

## Dependencies

### Phase Prerequisites

- **Phase 0**: No prerequisites (spike)
- **Phase 1**: Requires Phase 0 completion (SQLite driver verified)
- **Phase 2**: Requires Phase 1 completion (database package exists)
- **Phase 3**: Requires Phase 1 completion (database package exists)
- **Phase 4**: Requires Phase 1, 2, 3 completion (DB + migration + env vars ready)
- **Phase 5**: Requires Phase 4 completion (StateManager integrated with DB)
- **Phase 6**: Requires Phase 4 completion (StateManager integrated with DB)
- **Phase 7**: Requires Phase 6 completion (admin UI updated)
- **Phase 8**: Requires Phase 7 completion + one release cycle (stable in production)
- **Phase 9**: Requires Phase 6 completion (DB-backed admin handlers + audit log endpoint)

### Parallel Execution Opportunities

**Phase 0**: No parallel opportunities (sequential verification steps)

**Phase 1**:

- T012-T020 can be executed in parallel (different files)
- T024-T032 can be executed in parallel (different test files)

**Phase 2**:

- T042-T055 can be executed in parallel (different field mappings)

**Phase 3**:

- T065-T066 can be executed in parallel (different files)
- T075-T079 can be executed in parallel (different test scenarios)

**Phase 4**:

- T095-T109 can be executed in parallel (different setter methods)

**Phase 5**:

- T133-T137 can be executed in parallel (different wizard steps)

**Phase 6**:

- T144-T152 can be executed in parallel (different admin handlers)

**Phase 7**:

- T172-T181 can be executed in parallel (different env var updates)
- T182-T186 can be executed in parallel (different deployment manifests)

**Phase 8**:

- T209-T211 can be executed in parallel (different legacy endpoints)

---

## Testing Strategy

### Unit Tests

- All database package functions (80%+ coverage required)
- Environment variable validation logic
- Migration logic with edge cases
- Setup handlers
- Admin API endpoints

### Integration Tests

- StateManager cache invalidation
- Concurrent read/write access
- First-run flow end-to-end
- Admin UI flows end-to-end
- DB export/import end-to-end

### Manual Testing

- Docker Compose startup with new config
- Kubernetes deployment
- Migration guide instructions
- First-run wizard in offline mode
- Audit log viewer in production

---

## Rollback Plan

At any phase:

1. DB file is the only state — `cp pvmss.db pvmss.db.bak` before upgrade
2. Export endpoint available from Phase 6 onward
3. Previous binary + settings.json works until Phase 8 cleanup

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
