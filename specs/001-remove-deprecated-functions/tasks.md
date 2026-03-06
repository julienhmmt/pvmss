# Tasks: Remove Deprecated Functions

**Input**: Design documents from `/specs/001-remove-deprecated-functions/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/, quickstart.md

**Tests**: Validation tasks are required by the feature specification and repository maintenance workflow: formatting, linting, offline automated tests, and deprecated reference scanning.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g. `US1`, `US2`, `US3`)
- Include exact file paths in descriptions

## Path Conventions

- **Backend maintenance cleanup**: `backend/handlers/`, `backend/tests/`, `backend/`
- **Feature artifacts**: `specs/001-remove-deprecated-functions/`
- **Repository validation**: repository root `Makefile`, `backend/go.mod`, and existing tooling invoked from repository root

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare the deprecated reference inventory and maintenance validation context for the cleanup.

- [ ] T001 Inventory all maintained `NewHandlerContext` references in `backend/handlers/`, `backend/tests/`, and related Go files under `backend/`
- [ ] T002 Review `specs/001-remove-deprecated-functions/plan.md`, `specs/001-remove-deprecated-functions/research.md`, and `specs/001-remove-deprecated-functions/quickstart.md` to confirm validation scope and the Telmate migration boundary
- [ ] T003 [P] Confirm the repository maintenance commands required by this feature in `Makefile` and `backend/go.mod`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish the shared migration baseline that all user story work depends on.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T004 Document the in-scope call-site inventory and migration order in `specs/001-remove-deprecated-functions/tasks.md`
- [ ] T005 [P] Inspect `backend/handlers/handler_context.go` and any related helpers in `backend/handlers/` to confirm `HandlerContextWith` is behaviorally equivalent for all in-scope usages
- [ ] T006 [P] Identify maintained test files under `backend/` that still rely on `NewHandlerContext` and add them to the migration inventory in `specs/001-remove-deprecated-functions/tasks.md`

**Checkpoint**: The migration inventory and scope guardrails are clear; user story implementation can begin.

---

## Phase 3: User Story 1 - Remove active usage of the deprecated entry point (Priority: P1) 🎯 MVP

**Goal**: Migrate all maintained active handler and helper call sites from `NewHandlerContext` to `HandlerContextWith` without changing runtime behavior.

**Independent Test**: Review all maintained backend handler code paths and run a reference scan to confirm active code paths no longer reference `NewHandlerContext` while behavior and signatures remain unchanged.

### Tests for User Story 1

- [ ] T007 [P] [US1] Run a maintained-source reference scan for `NewHandlerContext` across `backend/handlers/` and related Go files under `backend/` to establish the pre-migration baseline

### Implementation for User Story 1

- [ ] T008 [P] [US1] Replace deprecated handler context initialization in `backend/handlers/auth.go` with `HandlerContextWith` (3 call sites)
- [ ] T009 [P] [US1] Replace deprecated handler context initialization in `backend/handlers/profile.go` with `HandlerContextWith` (2 call sites)
- [ ] T010 [P] [US1] Replace deprecated handler context initialization in `backend/handlers/search.go` with `HandlerContextWith` (1 call site)
- [ ] T011 [P] [US1] Replace deprecated handler context initialization in `backend/handlers/vm_actions_lifecycle.go` with `HandlerContextWith` (8 call sites)
- [ ] T012 [P] [US1] Replace deprecated handler context initialization in `backend/handlers/vm_actions_misc.go` with `HandlerContextWith` (2 call sites)
- [ ] T013 [P] [US1] Replace deprecated handler context initialization in `backend/handlers/vm_actions_resources.go` with `HandlerContextWith` (2 call sites)
- [ ] T014 [P] [US1] Replace deprecated handler context initialization in `backend/handlers/vm_delete.go` with `HandlerContextWith` (4 call sites)
- [ ] T015 [P] [US1] Replace deprecated handler context initialization in `backend/handlers/vm_details_info.go` with `HandlerContextWith` (1 call site)
- [ ] T015a [P] [US1] Replace deprecated handler context initialization in `backend/handlers/admin_cloudinit.go` with `HandlerContextWith` (1 call site)
- [ ] T015b [P] [US1] Replace deprecated handler context initialization in `backend/handlers/disks.go` with `HandlerContextWith` (1 call site)
- [ ] T015c [P] [US1] Replace deprecated handler context initialization in `backend/handlers/common.go` with `HandlerContextWith` (1 call site)
- [ ] T016 [US1] Migrate any additional newly discovered maintained production call sites under `backend/handlers/` or `backend/` from `NewHandlerContext` to `HandlerContextWith`
- [ ] T017 [US1] Re-run the maintained-source reference scan to confirm active backend code paths no longer reference `NewHandlerContext`

**Checkpoint**: User Story 1 is complete when maintained production code uses only `HandlerContextWith` and the reference scan shows no active handler references.

---

## Phase 4: User Story 2 - Remove the deprecated wrapper safely (Priority: P2)

**Goal**: Eliminate the deprecated wrapper definition and any remaining maintained test or helper dependencies after all in-scope references have been migrated.

**Independent Test**: Confirm the deprecated wrapper definition is absent and repository validation passes after migrating maintained tests and helper code.

### Tests for User Story 2

- [ ] T018 [P] [US2] Run a full maintained-source reference scan for `NewHandlerContext` across `backend/` to identify any remaining test or helper dependencies before removing the wrapper

### Implementation for User Story 2

- [ ] T019 [P] [US2] Replace deprecated handler context usage in maintained tests under `backend/tests/` or other in-scope `_test.go` files with `HandlerContextWith` — confirmed call site: `backend/handlers/security_test.go` (1 call site)
- [ ] T020 [P] [US2] Replace deprecated handler context usage in maintained helper code under `backend/handlers/` or other in-scope Go files under `backend/` with `HandlerContextWith`
- [ ] T021 [US2] Remove the deprecated `NewHandlerContext` wrapper from `backend/handlers/helpers.go` (lines 149–153 — the function is defined there, not in `handler_context.go` which contains the replacement `HandlerContextWith`)
- [ ] T022 [US2] Update any compilation fallout from the wrapper removal in affected files under `backend/handlers/` and `backend/tests/`
- [ ] T023 [US2] Verify the wrapper removal by confirming no maintained Go source under `backend/` still references `NewHandlerContext`

**Checkpoint**: User Story 2 is complete when `NewHandlerContext` is removed from the codebase and no maintained backend source depends on it.

---

## Phase 5: User Story 3 - Preserve scope boundaries for larger migration work (Priority: P3)

**Goal**: Keep the deprecated-function cleanup strictly limited to the handler context migration and avoid unrelated Telmate migration cleanup.

**Independent Test**: Review the resulting diff and touched files to confirm Telmate migration TODO areas remain unchanged unless they were directly required by the deprecated wrapper removal.

### Tests for User Story 3

- [ ] T024 [P] [US3] Review the touched-file list under `backend/` to flag any edits outside the handler context cleanup scope before final validation

### Implementation for User Story 3

- [ ] T025 [US3] Compare the final changes against `specs/001-remove-deprecated-functions/spec.md` and `specs/001-remove-deprecated-functions/research.md` to confirm unrelated Telmate migration cleanup was not introduced
- [ ] T026 [US3] Document any newly discovered out-of-scope dependency or blocker in `specs/001-remove-deprecated-functions/tasks.md` without expanding implementation into Telmate migration work

**Checkpoint**: User Story 3 is complete when the final scope review confirms the cleanup stayed limited to deprecated handler context removal.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Complete repository-wide validation and final maintainer verification for the feature.

- [ ] T027 [P] Run formatting validation for the cleanup using repository commands from `Makefile` and Go tooling at the repository root
- [ ] T028 [P] Run lint validation for the cleanup using repository commands from `Makefile` and `golangci-lint` configuration in `backend/`
- [ ] T029 [P] Run offline automated tests for the cleanup using the repository maintenance workflow from the repository root
- [ ] T030 Run the final maintained-source reference scan for `NewHandlerContext` across `backend/` and record zero remaining references in `specs/001-remove-deprecated-functions/tasks.md`
- [ ] T031 Review the final diff in `backend/` and `specs/001-remove-deprecated-functions/` to confirm behavior-preserving scope and merge readiness

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately.
- **Foundational (Phase 2)**: Depends on Setup completion; blocks all user story work until the migration inventory and scope guardrails are confirmed.
- **User Story 1 (Phase 3)**: Depends on Foundational completion.
- **User Story 2 (Phase 4)**: Depends on User Story 1 completion because the wrapper cannot be removed until all maintained active references are migrated.
- **User Story 3 (Phase 5)**: Depends on User Stories 1 and 2 completion so the scope review covers the final intended changeset.
- **Polish (Phase 6)**: Depends on all desired user stories being complete.

### User Story Dependencies

- **US1 (P1)**: Starts after Phase 2; delivers the MVP by removing active maintained production references.
- **US2 (P2)**: Starts after US1; removes the deprecated wrapper and remaining maintained test/helper references.
- **US3 (P3)**: Starts after US1 and US2; validates that the cleanup did not expand into Telmate migration work.

### Within Each User Story

- Reference scan tasks establish or confirm the migration state before and after implementation.
- Parallel file updates can proceed independently when they touch separate files.
- Wrapper removal must happen only after all maintained references are migrated.
- Final validation must happen after the code changes are complete.

### Parallel Opportunities

- **Phase 1**: T003 can run in parallel with T001-T002.
- **Phase 2**: T005 and T006 can run in parallel after T004 starts the inventory.
- **US1**: T008–T015c can run in parallel because they target separate files (11 files: auth.go, profile.go, search.go, vm_actions_lifecycle.go, vm_actions_misc.go, vm_actions_resources.go, vm_delete.go, vm_details_info.go, admin_cloudinit.go, disks.go, common.go).
- **US2**: T019 and T020 can run in parallel if test and helper references are in different files.
- **Polish**: T027-T029 can run in parallel once implementation is complete.

---

## Parallel Example: User Story 1

```bash
# Migrate independent handler files in parallel (complete inventory — 25 call sites across 11 files):
Task: "T008 [US1] backend/handlers/auth.go (3 call sites)"
Task: "T009 [US1] backend/handlers/profile.go (2 call sites)"
Task: "T010 [US1] backend/handlers/search.go (1 call site)"
Task: "T011 [US1] backend/handlers/vm_actions_lifecycle.go (8 call sites)"
Task: "T012 [US1] backend/handlers/vm_actions_misc.go (2 call sites)"
Task: "T013 [US1] backend/handlers/vm_actions_resources.go (2 call sites)"
Task: "T014 [US1] backend/handlers/vm_delete.go (4 call sites)"
Task: "T015 [US1] backend/handlers/vm_details_info.go (1 call site)"
Task: "T015a [US1] backend/handlers/admin_cloudinit.go (1 call site)"
Task: "T015b [US1] backend/handlers/disks.go (1 call site)"
Task: "T015c [US1] backend/handlers/common.go (1 call site)"
# Test file handled separately in US2:
Task: "T019 [US2] backend/handlers/security_test.go (1 call site)"
```

---

## Parallel Example: User Story 2

```bash
# Migrate remaining maintained non-production references in parallel:
Task: "T019 [US2] Replace deprecated handler context usage in maintained tests under backend/tests/ or other in-scope _test.go files with HandlerContextWith"
Task: "T020 [US2] Replace deprecated handler context usage in maintained helper code under backend/handlers/ or other in-scope Go files under backend/ with HandlerContextWith"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1.
4. Validate the maintained production reference scan for zero active handler usages of `NewHandlerContext`.
5. Pause for review if you want to ship the active-call-site migration before wrapper deletion.

### Incremental Delivery

1. Finish Setup and Foundational work to lock the migration inventory and scope.
2. Deliver US1 to remove active handler references and verify the MVP.
3. Deliver US2 to remove the deprecated wrapper and any remaining maintained test/helper dependencies.
4. Deliver US3 to confirm scope boundaries stayed intact.
5. Finish with repository formatting, lint, offline tests, and final reference-scan validation.

### Parallel Team Strategy

1. One maintainer owns the migration inventory and verification scans.
2. Additional maintainers split independent file migrations in US1 across handler files.
3. After US1, one maintainer handles test/helper cleanup while another prepares wrapper-removal fallout fixes.
4. Complete with a shared validation and scope-review pass.

---

## Notes

- All tasks follow the required checklist format: checkbox, task ID, optional `[P]`, required story labels for user story phases, and exact file paths.
- The feature contract is documentation-only in `specs/001-remove-deprecated-functions/contracts/maintenance-verification.openapi.yaml`; no external HTTP API changes are part of this work.
- The prerequisite script reported that the repository is currently on `main`, while the feature artifacts expect branch `001-remove-deprecated-functions`; complete implementation work on the intended feature branch if branch policy is enforced.
- Avoid expanding the cleanup into unrelated Telmate migration TODO removal.
