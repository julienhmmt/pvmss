---
description: "Task list for frontend SPA rewrite implementation"
---

# Tasks: Frontend SPA Rewrite

**Input**: Design documents from `/specs/001-frontend-spa-rewrite/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/, quickstart.md

**Tests**: Include backend tests, frontend unit/component tests, contract validation, and end-to-end regression coverage as required by the specification and implementation plan.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g. `US1`, `US2`, `US3`)
- Include exact file paths in descriptions

## Path Conventions

- **Backend**: `backend/api/v1/`, `backend/handlers/`, `backend/middleware/`, `backend/proxmox/`, `backend/state/`, `backend/tests/`
- **Frontend SPA**: `frontend-svelte/src/lib/`, `frontend-svelte/src/routes/`, `frontend-svelte/tests/`
- **Legacy reference**: `frontend-legacy/`
- **Feature docs**: `specs/001-frontend-spa-rewrite/`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the new SPA workspace, static hosting pipeline, and migration-safe repository structure.

- [ ] T001 Create the new SPA project skeleton and package configuration in `frontend-svelte/package.json`, `frontend-svelte/tsconfig.json`, `frontend-svelte/vite.config.ts`, and `frontend-svelte/svelte.config.js`
- [ ] T002 [P] Create the SPA application directories and route placeholders in `frontend-svelte/src/routes/`, `frontend-svelte/src/lib/api/`, `frontend-svelte/src/lib/components/`, `frontend-svelte/src/lib/stores/`, `frontend-svelte/src/lib/types/`, and `frontend-svelte/src/lib/utils/`
- [ ] T003 [P] Add frontend test and lint tooling configuration in `frontend-svelte/eslint.config.js`, `frontend-svelte/prettier.config.cjs`, `frontend-svelte/vitest.config.ts`, and `frontend-svelte/playwright.config.ts`
- [ ] T004 Wire SPA build output serving and static asset hosting through the Go application in `backend/main.go`, `backend/app/app.go`, and `backend/handlers/handlers.go`
- [ ] T005 Preserve the legacy frontend as engineering reference by documenting and enforcing the migration boundary in `frontend-legacy/README.md`, `README.md`, and `README.fr.md`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Build the shared API, auth, routing, and UI foundations required before any story implementation.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T006 Implement versioned SPA API route registration and JSON response conventions in `backend/api/v1/routes.go`, `backend/api/v1/response.go`, and `backend/handlers/helpers.go`
- [ ] T007 [P] Implement SPA authentication/session middleware for bearer access, renewable cookie flow, and admin authorization in `backend/middleware/spa_auth.go`, `backend/middleware/admin_api.go`, and `backend/state/manager.go`
- [ ] T008 [P] Implement the frontend typed API client, request retry hooks, and shared error mapping in `frontend-svelte/src/lib/api/client.ts`, `frontend-svelte/src/lib/api/errors.ts`, and `frontend-svelte/src/lib/types/api.ts`
- [ ] T009 [P] Implement the frontend session store, guarded bootstrap flow, and route protection utilities in `frontend-svelte/src/lib/stores/session-store.ts`, `frontend-svelte/src/lib/utils/auth-bootstrap.ts`, and `frontend-svelte/src/hooks.client.ts`
- [ ] T010 [P] Implement the SPA shell, responsive layout, navigation, and theme switching foundation in `frontend-svelte/src/routes/+layout.svelte`, `frontend-svelte/src/lib/components/app-shell.svelte`, `frontend-svelte/src/lib/components/theme-toggle.svelte`, and `frontend-svelte/src/lib/stores/theme-store.ts`
- [ ] T011 Preserve public health compatibility and SPA catch-all routing behavior in `backend/handlers/health.go`, `backend/handlers/docs.go`, and `backend/handlers/spa_fallback.go`
- [ ] T012 [P] Add shared contract validation and API schema test scaffolding in `backend/tests/contracts/api_contract_test.go`, `frontend-svelte/tests/contracts/openapi-contract.test.ts`, and `specs/001-frontend-spa-rewrite/contracts/openapi.yaml`

**Checkpoint**: Foundation ready - user story implementation can now begin in priority order or in parallel if staffed.

---

## Phase 3: User Story 1 - Access and operate my virtual machines from one modern web app (Priority: P1) 🎯 MVP

**Goal**: Deliver the primary end-user SPA workflows for login, dashboard, VM details, VM creation, VM search, profile management, and VM lifecycle operations.

**Independent Test**: Sign in as a standard user and complete login, dashboard, VM details, VM action, VM creation, search, and profile/password flows using only the new SPA.

### Tests for User Story 1

- [ ] T013 [P] [US1] Add backend contract tests for auth, VM listing, VM detail, VM create, search, and profile endpoints in `backend/tests/contracts/auth_vm_profile_contract_test.go`
- [ ] T014 [P] [US1] Add frontend component and store tests for authenticated shell, VM dashboard, VM details, VM creation, search, and profile flows in `frontend-svelte/tests/unit/dashboard.spec.ts`, `frontend-svelte/tests/unit/vm-details.spec.ts`, `frontend-svelte/tests/unit/vm-create.spec.ts`, and `frontend-svelte/tests/unit/profile.spec.ts`
- [ ] T015 [P] [US1] Add end-to-end regression coverage for standard user SPA workflows in `frontend-svelte/tests/e2e/user-core-workflows.spec.ts`

### Implementation for User Story 1

- [ ] T016 [P] [US1] Implement SPA auth endpoints for login, current user, logout, and password change in `backend/api/v1/auth_handlers.go` and `backend/api/v1/auth_mapper.go`
- [ ] T017 [P] [US1] Implement VM list, detail, create, action, description, tags, resources, network toggle, and profile API handlers in `backend/api/v1/vm_handlers.go`, `backend/api/v1/profile_handlers.go`, and `backend/api/v1/vm_mapper.go`
- [ ] T018 [P] [US1] Implement typed frontend models for user profile, VM summaries, VM details, creation payloads, disks, and network cards in `frontend-svelte/src/lib/types/user.ts`, `frontend-svelte/src/lib/types/vm.ts`, and `frontend-svelte/src/lib/types/vm-create.ts`
- [ ] T019 [P] [US1] Implement frontend API modules for auth, VMs, search, and profile in `frontend-svelte/src/lib/api/auth-api.ts`, `frontend-svelte/src/lib/api/vm-api.ts`, `frontend-svelte/src/lib/api/search-api.ts`, and `frontend-svelte/src/lib/api/profile-api.ts`
- [ ] T020 [P] [US1] Implement login, dashboard, and protected landing routes in `frontend-svelte/src/routes/login/+page.svelte`, `frontend-svelte/src/routes/+page.svelte`, and `frontend-svelte/src/lib/components/dashboard/vm-table.svelte`
- [ ] T021 [P] [US1] Implement VM details route and reusable detail panels for status, resources, disks, network cards, and actions in `frontend-svelte/src/routes/vms/[vmid]/+page.svelte`, `frontend-svelte/src/lib/components/vm/vm-overview-card.svelte`, `frontend-svelte/src/lib/components/vm/vm-disks-panel.svelte`, and `frontend-svelte/src/lib/components/vm/vm-network-panel.svelte`
- [ ] T022 [P] [US1] Implement VM creation route, typed form state, and validation-preserving UX in `frontend-svelte/src/routes/vms/create/+page.svelte`, `frontend-svelte/src/lib/stores/vm-create-store.ts`, and `frontend-svelte/src/lib/components/vm/vm-create-form.svelte`
- [ ] T023 [P] [US1] Implement VM search route and reusable result list experience in `frontend-svelte/src/routes/search/+page.svelte` and `frontend-svelte/src/lib/components/search/vm-search-results.svelte`
- [ ] T024 [P] [US1] Implement profile route and password management UI in `frontend-svelte/src/routes/profile/+page.svelte` and `frontend-svelte/src/lib/components/profile/password-change-form.svelte`
- [ ] T025 [US1] Integrate optimistic/pending status, empty states, and actionable errors across the user VM workflows in `frontend-svelte/src/lib/components/feedback/loading-state.svelte`, `frontend-svelte/src/lib/components/feedback/empty-state.svelte`, `frontend-svelte/src/lib/components/feedback/error-state.svelte`, and `frontend-svelte/src/lib/stores/notification-store.ts`

**Checkpoint**: User Story 1 is fully functional as the MVP and can be validated without the legacy frontend.

---

## Phase 4: User Story 2 - Use secure session continuity without re-entering credentials unnecessarily (Priority: P2)

**Goal**: Ensure protected SPA routes recover cleanly on refresh, renew access transparently, and fail safely when renewal is no longer possible.

**Independent Test**: Sign in, let access expire, verify silent renewal and request replay, refresh protected routes, then verify clean sign-out and expired-session redirect behavior.

### Tests for User Story 2

- [ ] T026 [P] [US2] Add backend contract tests for refresh, unauthorized fallback, and logout invalidation in `backend/tests/contracts/session_continuity_contract_test.go`
- [ ] T027 [P] [US2] Add frontend unit tests for session bootstrap, refresh queueing, and expired-session handling in `frontend-svelte/tests/unit/session-store.spec.ts` and `frontend-svelte/tests/unit/auth-bootstrap.spec.ts`
- [ ] T028 [P] [US2] Add end-to-end coverage for refresh recovery, direct navigation, and forced expiry flows in `frontend-svelte/tests/e2e/session-continuity.spec.ts`

### Implementation for User Story 2

- [ ] T029 [P] [US2] Implement backend refresh and sign-out behavior for renewable session cookies and bearer re-issuance in `backend/api/v1/auth_refresh.go`, `backend/api/v1/auth_logout.go`, and `backend/security/middleware/headers.go`
- [ ] T030 [P] [US2] Implement frontend access-token memory state, refresh orchestration, and replay queue handling in `frontend-svelte/src/lib/stores/session-store.ts` and `frontend-svelte/src/lib/api/token-refresh.ts`
- [ ] T031 [P] [US2] Implement protected-route bootstrap loaders and direct-navigation recovery in `frontend-svelte/src/routes/+layout.ts`, `frontend-svelte/src/lib/utils/guarded-load.ts`, and `frontend-svelte/src/lib/components/auth/session-restoring.svelte`
- [ ] T032 [US2] Implement expired-session cleanup, redirect, and unauthorized fan-out handling in `frontend-svelte/src/lib/utils/unauthorized-handler.ts`, `frontend-svelte/src/lib/stores/session-store.ts`, and `frontend-svelte/src/routes/login/+page.svelte`

**Checkpoint**: Session continuity works independently across refresh, expiry, and sign-out scenarios.

---

## Phase 5: User Story 3 - Administer the platform from the same application (Priority: P3)

**Goal**: Deliver administrator-only SPA routes for infrastructure visibility and settings management while blocking non-admin users from admin capabilities.

**Independent Test**: Sign in as an admin and use each in-scope admin section, then verify a non-admin user is denied both route access and backend API access.

### Tests for User Story 3

- [ ] T033 [P] [US3] Add backend contract tests for admin read and mutation endpoints plus forbidden responses in `backend/tests/contracts/admin_contract_test.go`
- [ ] T034 [P] [US3] Add frontend component and route-guard tests for admin sections and access denial in `frontend-svelte/tests/unit/admin-shell.spec.ts` and `frontend-svelte/tests/unit/admin-guard.spec.ts`
- [ ] T035 [P] [US3] Add end-to-end coverage for admin workflows and non-admin blocking in `frontend-svelte/tests/e2e/admin-workflows.spec.ts`

### Implementation for User Story 3

- [ ] T036 [P] [US3] Implement admin API handlers for nodes, storage, VMs, pools, tags, limits, bridges, cloud-init, ISO, settings, and app diagnostics in `backend/api/v1/admin_handlers.go`, `backend/api/v1/admin_mutations.go`, and `backend/api/v1/admin_mapper.go`
- [ ] T037 [P] [US3] Implement frontend admin resource types and admin API modules in `frontend-svelte/src/lib/types/admin.ts` and `frontend-svelte/src/lib/api/admin-api.ts`
- [ ] T038 [P] [US3] Implement admin shell, navigation, and admin route guard behavior in `frontend-svelte/src/routes/admin/+layout.svelte`, `frontend-svelte/src/routes/admin/+layout.ts`, and `frontend-svelte/src/lib/components/admin/admin-nav.svelte`
- [ ] T039 [P] [US3] Implement admin overview and resource sections for nodes, storage, VMs, pools, tags, and limits in `frontend-svelte/src/routes/admin/+page.svelte`, `frontend-svelte/src/routes/admin/nodes/+page.svelte`, `frontend-svelte/src/routes/admin/storage/+page.svelte`, `frontend-svelte/src/routes/admin/vms/+page.svelte`, `frontend-svelte/src/routes/admin/pools/+page.svelte`, `frontend-svelte/src/routes/admin/tags/+page.svelte`, and `frontend-svelte/src/routes/admin/limits/+page.svelte`
- [ ] T040 [P] [US3] Implement admin resource sections for bridges, cloud-init templates, ISO assets, settings, and application diagnostics in `frontend-svelte/src/routes/admin/vmbr/+page.svelte`, `frontend-svelte/src/routes/admin/cloudinit/+page.svelte`, `frontend-svelte/src/routes/admin/iso/+page.svelte`, `frontend-svelte/src/routes/admin/settings/+page.svelte`, and `frontend-svelte/src/routes/admin/appinfo/+page.svelte`
- [ ] T041 [US3] Integrate mutation feedback, stale-data refresh, and forbidden-route fallback across admin workflows in `frontend-svelte/src/lib/components/admin/admin-resource-table.svelte`, `frontend-svelte/src/lib/components/feedback/action-banner.svelte`, and `frontend-svelte/src/lib/utils/admin-access.ts`

**Checkpoint**: Admin parity is available in the SPA and administrator access control is enforced end-to-end.

---

## Phase 6: User Story 4 - Open an interactive VM console from the new application (Priority: P4)

**Goal**: Provide a dedicated console experience with explicit connection-state handling and clear recovery paths when console access fails.

**Independent Test**: Open a VM console for a running VM, confirm the interactive session connects from the SPA, then verify retryable and terminal failures are surfaced clearly.

### Tests for User Story 4

- [ ] T042 [P] [US4] Add backend contract tests for console ticket issuance and websocket authorization failures in `backend/tests/contracts/console_contract_test.go`
- [ ] T043 [P] [US4] Add frontend unit/component tests for console state transitions and failure messaging in `frontend-svelte/tests/unit/console-store.spec.ts` and `frontend-svelte/tests/unit/console-page.spec.ts`
- [ ] T044 [P] [US4] Add end-to-end coverage for console connect, retry, and failure scenarios in `frontend-svelte/tests/e2e/console-flow.spec.ts`

### Implementation for User Story 4

- [ ] T045 [P] [US4] Implement backend console ticket and websocket endpoints aligned with the versioned API contract in `backend/api/v1/console_handlers.go`, `backend/handlers/vm_console_websocket.go`, and `backend/proxmox/console.go`
- [ ] T046 [P] [US4] Implement typed console session models and API helpers in `frontend-svelte/src/lib/types/console.ts` and `frontend-svelte/src/lib/api/console-api.ts`
- [ ] T047 [P] [US4] Implement frontend console store with requesting, connecting, connected, retryable failure, and terminal failure states in `frontend-svelte/src/lib/stores/console-store.ts`
- [ ] T048 [US4] Implement the console route and noVNC integration with retry/exit UX in `frontend-svelte/src/routes/vms/[vmid]/console/+page.svelte`, `frontend-svelte/src/lib/components/console/console-canvas.svelte`, and `frontend-svelte/src/lib/components/console/console-status-panel.svelte`

**Checkpoint**: Console access works as an independent SPA flow with actionable status reporting.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Finalize cutover, documentation, performance, and validation across all stories.

- [ ] T049 [P] Update cutover and developer workflow documentation in `specs/001-frontend-spa-rewrite/quickstart.md`, `plans/2026-03-06-frontend-rewrite-implementation.md`, and `README.md`
- [ ] T050 [P] Optimize SPA bundle splitting, route-level data loading, and API caching behavior in `frontend-svelte/src/routes/+layout.ts`, `frontend-svelte/src/lib/api/client.ts`, and `frontend-svelte/vite.config.ts`
- [ ] T051 Remove remaining legacy runtime dependencies from active application paths while preserving reference assets in `backend/handlers/handlers.go`, `frontend-legacy/`, and `package.json`
- [ ] T052 [P] Complete accessibility, responsive, and theme verification refinements across the SPA in `frontend-svelte/src/lib/components/`, `frontend-svelte/src/routes/`, and `frontend-svelte/tests/e2e/theme-and-responsive.spec.ts`
- [ ] T053 Execute full quickstart validation and record cutover readiness notes in `specs/001-frontend-spa-rewrite/quickstart.md` and `specs/001-frontend-spa-rewrite/tasks.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately.
- **Foundational (Phase 2)**: Depends on Setup completion - blocks all user stories.
- **User Story 1 (Phase 3)**: Depends on Foundational completion - recommended MVP starting point.
- **User Story 2 (Phase 4)**: Depends on Foundational completion and integrates with US1 auth usage, but remains independently testable.
- **User Story 3 (Phase 5)**: Depends on Foundational completion and shared auth/admin guards.
- **User Story 4 (Phase 6)**: Depends on Foundational completion and benefits from VM detail primitives from US1.
- **Polish (Phase 7)**: Depends on all target stories being complete.

### User Story Dependencies

- **US1 (P1)**: No dependency on other stories after Foundational.
- **US2 (P2)**: Uses the shared auth foundation and should be validated against US1 protected routes.
- **US3 (P3)**: Uses the shared auth foundation and admin authorization primitives, but does not require US1 feature completion to begin.
- **US4 (P4)**: Uses shared auth foundation and VM identity/routing patterns established in US1.

### Within Each User Story

- Contract, unit, and end-to-end tests should be created before or alongside implementation and should fail before the feature is considered complete.
- Backend contract and mapper work should precede frontend data consumption for the same domain.
- Shared models/types should precede route and component assembly.
- User-facing loading/error/empty-state integration should be completed before declaring a story done.

### Parallel Opportunities

- T002 and T003 can run in parallel after T001.
- T007 through T010 and T012 can run in parallel after T006 starts the API baseline.
- In US1, T013 through T015 can run in parallel, then T016 through T024 can be split by backend/frontend domain.
- In US3, T036 through T040 can be distributed across backend and multiple admin routes in parallel.
- In US4, T045 through T047 can run in parallel before T048 integrates the full console route.

---

## Parallel Example: User Story 1

```bash
# Launch backend and frontend verification tasks for US1 together:
Task: "T013 [US1] Add backend contract tests in backend/tests/contracts/auth_vm_profile_contract_test.go"
Task: "T014 [US1] Add frontend component and store tests in frontend-svelte/tests/unit/dashboard.spec.ts and related files"
Task: "T015 [US1] Add end-to-end regression coverage in frontend-svelte/tests/e2e/user-core-workflows.spec.ts"

# Launch domain implementation tasks for US1 together after contracts are defined:
Task: "T016 [US1] Implement SPA auth endpoints in backend/api/v1/auth_handlers.go and backend/api/v1/auth_mapper.go"
Task: "T017 [US1] Implement VM and profile API handlers in backend/api/v1/vm_handlers.go and backend/api/v1/profile_handlers.go"
Task: "T018 [US1] Implement typed frontend models in frontend-svelte/src/lib/types/user.ts and frontend-svelte/src/lib/types/vm.ts"
Task: "T019 [US1] Implement frontend API modules in frontend-svelte/src/lib/api/auth-api.ts and related files"
```

---

## Parallel Example: User Story 3

```bash
# Split admin implementation by domain once shared admin guard work is ready:
Task: "T036 [US3] Implement backend admin handlers in backend/api/v1/admin_handlers.go and backend/api/v1/admin_mutations.go"
Task: "T038 [US3] Implement admin shell and route guard in frontend-svelte/src/routes/admin/+layout.svelte and +layout.ts"
Task: "T039 [US3] Implement admin overview, nodes, storage, VMs, pools, tags, and limits routes"
Task: "T040 [US3] Implement admin vmbr, cloudinit, iso, settings, and appinfo routes"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1.
4. Validate login, dashboard, VM details, VM creation, search, and profile flows without using the legacy UI.
5. Demo the SPA MVP before moving to later stories.

### Incremental Delivery

1. Setup the SPA workspace and backend hosting integration.
2. Build the auth/session and route foundations.
3. Deliver US1 as the first usable cutover increment.
4. Add US2 for secure continuity and refresh resilience.
5. Add US3 for admin parity.
6. Add US4 for console parity.
7. Finish with polish, cutover cleanup, and quickstart validation.

### Parallel Team Strategy

1. One developer owns backend API foundation and auth middleware.
2. One developer owns SPA shell, session store, and client infrastructure.
3. After Foundation:
   - Developer A: US1 user workflows
   - Developer B: US2 session continuity
   - Developer C: US3 admin area
   - Developer D: US4 console flow

---

## Notes

- All tasks follow the required checklist format with task ID, optional `[P]`, optional story label, and exact file paths.
- Tasks are organized so each story remains independently testable.
- Public health endpoints stay outside auth requirements throughout the migration.
- Legacy frontend assets remain repository reference material until final cleanup.
- The suggested MVP scope is **User Story 1 only** after Setup and Foundational phases.
