# Tasks: Remove Telmate Dependency

**Input**: Design documents from `/specs/001-telmate-removal/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md, contracts/migration-verification.openapi.yaml

**Tests**: Validation tasks are required for this feature because the specification explicitly requires build, lint, and offline test verification after each migration stage and after final dependency removal.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each migration outcome.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (`[US1]`, `[US2]`, `[US3]`)
- Include exact file paths in descriptions

## Path Conventions

- **Web app**: `backend/`, `frontend/`, `specs/` at repository root
- **Feature artifacts**: `specs/001-telmate-removal/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish the migration inventory, phase checkpoints, and execution references before code changes begin.

- [ ] T001 Review `specs/001-telmate-removal/spec.md`, `specs/001-telmate-removal/plan.md`, `specs/001-telmate-removal/research.md`, and `plans/telmate-migration-removal.md` to confirm the phased migration scope
- [ ] T002 Create a maintained Telmate usage inventory in `specs/001-telmate-removal/tasks.md` execution notes or implementation PR notes covering files listed in `plans/telmate-migration-removal.md`
- [ ] T003 [P] Record the per-phase verification commands from `specs/001-telmate-removal/quickstart.md` and `backend/Makefile` targets (`test-offline`, `go-lint`, `go-fmt`, build)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Define the shared migration foundation that all story work depends on.

**⚠️ CRITICAL**: No user story work should start until the remaining Telmate usage map and validation checkpoints are clear.

- [ ] T004 Map `LegacyIntegrationUsage`, `RestyIntegrationCoverage`, `StateManagerSurface`, and `DependencyRemovalCheckpoint` from `specs/001-telmate-removal/data-model.md` to concrete code areas in `backend/handlers/`, `backend/proxmox/`, `backend/state/`, and `backend/tests/`
- [ ] T005 [P] Map the verification contract in `specs/001-telmate-removal/contracts/migration-verification.openapi.yaml` to concrete phase gates for simple swaps, new Resty helpers, state cleanup, and dependency removal
- [ ] T006 Define the phase completion checklist in `specs/001-telmate-removal/tasks.md` notes so each migration phase requires usage scan, build, lint, and offline test evidence before destructive cleanup

**Checkpoint**: Foundation ready - user story implementation can now begin.

---

## Phase 3: User Story 1 - Run all Proxmox operations through one maintained client path (Priority: P1) 🎯 MVP

**Goal**: Migrate the remaining maintained Telmate-backed Proxmox operations to Resty-backed equivalents or new Resty helpers without changing user-visible behavior.

**Independent Test**: Review the affected handlers and helpers to confirm remaining Telmate-backed operational flows now use `*proxmox.RestyClient` or dedicated Resty helper functions, then run offline validation for the migrated files.

### Tests for User Story 1

- [ ] T007 [US1] Run phase verification for simple swap candidates after each migration batch using repository commands from `specs/001-telmate-removal/quickstart.md`
- [ ] T008 [US1] Run phase verification for newly added Resty helper coverage after helper and caller migration in `backend/proxmox/`, `backend/handlers/auth.go`, `backend/handlers/profile.go`, and `backend/handlers/user_pool.go`

### Implementation for User Story 1

- [ ] T009 [P] [US1] Replace remaining VM configuration Telmate calls in `backend/handlers/search.go` and `backend/handlers/vm_actions_misc.go` with `proxmox.GetVMConfigResty` and `proxmox.UpdateVMConfigResty`
- [ ] T010 [P] [US1] Replace remaining node and cluster Telmate calls in `backend/main.go`, `backend/handlers/vm_details_info.go`, `backend/handlers/storage.go`, `backend/handlers/vmbr.go`, `backend/handlers/admin.go`, and `backend/state/manager_proxmox.go` with Resty equivalents
- [ ] T011 [P] [US1] Implement VNC proxy coverage in `backend/proxmox/vnc_resty.go` and migrate `backend/handlers/vm_console_helpers.go` to `proxmox.GetVNCProxyResty`
- [ ] T012 [P] [US1] Implement ticket and password Resty helpers in `backend/proxmox/auth_resty.go` for `CreateTicketResty`, `UpdateUserPasswordResty`, and maintained capability parsing logic
- [ ] T013 [US1] Migrate authentication and password update flows in `backend/handlers/auth.go` and `backend/handlers/profile.go` to the Resty ticket and password helpers while preserving cookie and CSRF behavior
- [ ] T014 [P] [US1] Implement admin access Resty helpers in `backend/proxmox/access_resty.go` for user, role, pool, and ACL provisioning
- [ ] T015 [US1] Migrate administrative pool and user provisioning call sites in `backend/handlers/user_pool.go` to the new Resty admin access helpers
- [ ] T016 [US1] Scan `backend/handlers/`, `backend/proxmox/`, and `backend/state/` for any newly discovered maintained Telmate operational usage and either migrate it or record it as a blocker in `specs/001-telmate-removal/spec.md` follow-up notes

**Checkpoint**: User Story 1 is complete when maintained operational Proxmox flows no longer depend on Telmate-backed calls.

---

## Phase 4: User Story 2 - Remove obsolete Telmate-specific code safely (Priority: P2)

**Goal**: Remove Telmate-specific state manager access, dead code, interfaces, and module references after all maintained consumers have been migrated.

**Independent Test**: Verify the Telmate-oriented state manager surface, helper files, and module references are absent while the repository still builds and tests cleanly.

### Tests for User Story 2

- [ ] T017 [US2] Run build, lint, offline tests, and usage scans after state manager cleanup in `backend/state/`, `backend/handlers/`, and test mocks
- [ ] T018 [US2] Run final post-removal validation after deleting Telmate-specific files and removing the module from `backend/go.mod` and `backend/go.sum`

### Implementation for User Story 2

- [ ] T019 [US2] Remove `GetProxmoxClient` and `SetProxmoxClient` from `backend/state/interface.go`, `backend/state/manager.go`, and `backend/state/manager_proxmox.go`
- [ ] T020 [US2] Remove leftover `GetProxmoxClient()` consumers and unused Telmate client variables across remaining handler files listed in `plans/telmate-migration-removal.md`
- [ ] T021 [P] [US2] Update Telmate-aware mocks and test scaffolding in `backend/middleware/middleware_test.go`, `backend/handlers/vm_actions_test.go`, `backend/handlers/auth_guard_test.go`, and `backend/api/v1/middleware_test.go`
- [ ] T022 [US2] Remove Telmate bootstrap and health-check wiring from `backend/main.go` so application startup uses only Resty-based Proxmox access
- [ ] T023 [P] [US2] Delete obsolete Telmate-backed functions from `backend/proxmox/vms.go`, `backend/proxmox/nodes.go`, `backend/proxmox/vnc.go`, `backend/proxmox/cluster.go`, and move or retain only maintained pure utility logic
- [ ] T024 [P] [US2] Delete obsolete Telmate-specific files `backend/proxmox/telmate_client.go`, `backend/proxmox/access.go`, `backend/proxmox/cache.go`, and remove `ClientInterface` from `backend/proxmox/interfaces.go`
- [ ] T025 [US2] Remove `github.com/Telmate/proxmox-api-go` from `backend/go.mod` and `backend/go.sum`, then reconcile dependencies with module tidy and usage scans

**Checkpoint**: User Story 2 is complete when the maintained codebase contains no Telmate-specific state surface or dependency declarations.

---

## Phase 5: User Story 3 - Keep the migration staged and verifiable (Priority: P3)

**Goal**: Preserve a reviewable, phase-based migration flow with explicit validation evidence before destructive cleanup proceeds.

**Independent Test**: Confirm that each migration phase has documented verification evidence and that cleanup only proceeds after the corresponding phase passes.

### Tests for User Story 3

- [ ] T026 [US3] Validate that each migration phase records pass or fail outcomes for build, lint, offline tests, and usage scans according to `specs/001-telmate-removal/contracts/migration-verification.openapi.yaml`

### Implementation for User Story 3

- [ ] T027 [US3] Document phase-by-phase execution and verification evidence in `specs/001-telmate-removal/tasks.md` and maintain alignment with `specs/001-telmate-removal/quickstart.md`
- [ ] T028 [US3] Verify the final implementation sequence in `specs/001-telmate-removal/tasks.md` preserves the ordered progression defined in `specs/001-telmate-removal/research.md` and `plans/telmate-migration-removal.md`

**Checkpoint**: All migration phases are reviewable, ordered, and backed by explicit verification evidence.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final repository-level cleanup and merge readiness checks spanning all user stories.

- [ ] T029 [P] Run formatting and code cleanup across modified backend files using the repository Go formatting workflow
- [ ] T030 Run the full repository validation flow referenced by `specs/001-telmate-removal/quickstart.md` and capture final evidence for build, lint, offline tests, and Telmate usage elimination
- [ ] T031 [P] Review documentation and maintenance notes in `specs/001-telmate-removal/quickstart.md`, `specs/001-telmate-removal/plan.md`, and `plans/telmate-migration-removal.md` for consistency with the final implementation state

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - blocks all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational completion
- **User Story 2 (Phase 4)**: Depends on User Story 1 completion because Telmate cleanup cannot begin until maintained flows are migrated
- **User Story 3 (Phase 5)**: Depends on Foundational completion and should be updated throughout Phases 3 and 4 as verification evidence accumulates
- **Polish (Phase 6)**: Depends on all selected user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Starts after Foundational and establishes the maintained Resty path for all in-scope operational flows
- **User Story 2 (P2)**: Starts only after User Story 1 is validated, because code and dependency removal depend on completed migration
- **User Story 3 (P3)**: Runs alongside story completion tracking but validates ordered phase evidence across User Story 1 and User Story 2

### Within Each User Story

- Validation tasks must execute after each migration batch and before destructive cleanup
- Existing Resty-equivalent swaps should land before new helper creation is considered complete
- New Resty helpers should be implemented before migrating their consuming handlers
- State manager cleanup must complete before dead file and dependency deletion
- Final dependency removal must complete before repository-wide validation and documentation reconciliation

### Parallel Opportunities

- `T003`, `T005`, and some inventory work can run in parallel during setup and foundation
- In User Story 1, `T009`, `T010`, `T011`, `T012`, and `T014` can be split across different files in parallel
- In User Story 2, `T021`, `T023`, and `T024` can proceed in parallel once state-surface removal is ready
- Polish tasks `T029` and `T031` can run in parallel before the final validation task `T030`

---

## Parallel Example: User Story 1

```bash
# Launch direct Resty swaps in parallel:
Task: "Replace remaining VM configuration Telmate calls in backend/handlers/search.go and backend/handlers/vm_actions_misc.go with Resty equivalents"
Task: "Replace remaining node and cluster Telmate calls in backend/main.go, backend/handlers/vm_details_info.go, backend/handlers/storage.go, backend/handlers/vmbr.go, backend/handlers/admin.go, and backend/state/manager_proxmox.go"

# Launch new helper creation in parallel:
Task: "Implement VNC proxy coverage in backend/proxmox/vnc_resty.go and migrate backend/handlers/vm_console_helpers.go"
Task: "Implement ticket and password Resty helpers in backend/proxmox/auth_resty.go"
Task: "Implement admin access Resty helpers in backend/proxmox/access_resty.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Setup and Foundational phases
2. Complete User Story 1 to eliminate maintained Telmate-backed operational usage
3. Stop and validate the migrated operational flows with offline verification
4. Use that checkpoint as the mergeable MVP before destructive cleanup begins

### Incremental Delivery

1. Complete Setup + Foundational to lock scope and verification rules
2. Deliver User Story 1 and validate the maintained Resty path
3. Deliver User Story 2 to remove obsolete state surface, dead code, and dependency declarations
4. Deliver User Story 3 evidence updates to prove each phase stayed verifiable
5. Finish with Polish and repository-wide validation

### Parallel Team Strategy

1. One maintainer handles direct call-site swaps while another implements new Resty helpers
2. Once User Story 1 is validated, state manager cleanup and test mock updates can proceed in parallel
3. Dead-code deletion and dependency cleanup can be split by file group while one maintainer drives the final validation run

---

## Notes

- All tasks follow the required checkbox, ID, label, and file path format
- Validation is mandatory for this feature because the specification explicitly requires build, lint, and offline test proof
- User Story 2 intentionally depends on User Story 1 because safe deletion is blocked until migration is validated
- The prerequisite script currently reports that the repository is not on the expected `001-telmate-removal` branch; switch to that branch before executing the implementation workflow if branch-specific guards are required
