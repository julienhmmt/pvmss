# Tasks: Constructor Rename to Make

**Input**: Design documents from `/specs/001-constructor-rename/`
**Prerequisites**: `plan.md` and `spec.md`; optional inputs used: `research.md`, `data-model.md`, `quickstart.md`, `contracts/openapi.yaml`

**Tests**: No new feature-specific tests are required. Verification relies on the existing repository formatting, offline test, and lint workflows defined in `Makefile`.

**Organization**: Tasks are grouped by user story so each increment can be implemented and verified independently.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel when the touched files do not overlap.
- **[Story]**: Maps the task to a specific user story from `spec.md`.
- Every task includes exact file paths so the work is immediately executable.

## Phase 1: Setup (Shared Preparation)

**Purpose**: Confirm the approved rename scope and identify the concrete backend files before editing symbols.

- [ ] T001 Review the approved rename scope in `specs/001-constructor-rename/spec.md`, `specs/001-constructor-rename/data-model.md`, and `specs/001-constructor-rename/research.md` and extract the authoritative `New...` -> `Make...` mapping for the implementation session.
- [ ] T002 Inspect the maintainer verification workflow in `specs/001-constructor-rename/quickstart.md` and `Makefile` and record the exact validation commands that must pass after the rename.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Update the shared constructor infrastructure and bootstrap call sites that many downstream files depend on.

**⚠️ CRITICAL**: Complete this phase before the user-story phases so later rename tasks build on the new shared naming consistently.

- [ ] T003 Rename the application state constructor from `NewAppState` to `MakeAppState` in `backend/state/manager.go` and update its bootstrap call site in `backend/main.go`.
- [ ] T004 [P] Rename Proxmox client constructors from `NewClient`, `NewClientCookieAuth`, `NewRestyClient`, `NewRestyClientFromEnv`, and `NewLRUCache` to their `Make...` equivalents in `backend/proxmox/telmate_client.go`, `backend/proxmox/resty_client.go`, `backend/proxmox/helpers.go`, and `backend/proxmox/cache.go`.
- [ ] T005 Rename shared handler-base constructors from `NewBaseAdminHandler`, `NewBaseFormHandler`, and `NewBaseAPIHandler` to `Make...` in `backend/handlers/base_handlers.go` and update any internal chaining within that file.
- [ ] T006 [P] Rename reusable helper constructors from `NewErrorHelper`, `NewFormSession`, `NewTemplateDataWithOptions`, `NewMessageHelper`, `NewInputSanitizer`, `NewValidationHelper`, `NewCSSHandler`, and `NewHandlerContext` to `Make...` in `backend/handlers/errors.go`, `backend/handlers/form_session.go`, `backend/handlers/template_data.go`, `backend/handlers/sanitize.go`, `backend/handlers/validation.go`, `backend/handlers/css.go`, and `backend/handlers/helpers.go`.
- [ ] T007 [P] Rename route utility constructors from `NewRouteHelpers` and `NewAdminPageRoutes` to `Make...` in `backend/handlers/route_helpers.go` and update the shared admin-route call sites such as `backend/handlers/settings_limits.go`.
- [ ] T008 [P] Rename remaining shared utility constructors from `NewErrorWrapper`, `NewRateLimiter`, and `NewMiddlewareLogger` to `Make...` in `backend/utils/errors.go`, `backend/middleware/ratelimit.go`, and `backend/middleware/util.go`, then update any compile-relevant references under `backend/`.

**Checkpoint**: Shared constructors and their foundational bootstrap references now use `Make...`, enabling feature-specific rename passes.

---

## Phase 3: User Story 1 - Keep constructor usage consistent after the rename (Priority: P1) 🎯 MVP

**Goal**: Rename all targeted constructor definitions and compile-relevant call sites so the approved `Make...` naming is used consistently across the backend.

**Independent Test**: Search the targeted backend packages for stale approved `New...` constructor names and confirm the application builds successfully with only `Make...` references remaining for the approved scope.

### Implementation for User Story 1

- [ ] T009 [P] [US1] Rename API-layer constructors from `NewAuthHandler`, `NewVMHandler`, and `NewVMActionHandler` to `Make...` in `backend/api/v1/auth.go`, `backend/api/v1/vms.go`, and `backend/api/v1/vm_actions.go`, then update compile-relevant consumers.
- [ ] T010 [P] [US1] Rename admin and settings constructors from `NewAdminOptimizedHandler`, `NewAdminVMsHandler`, `NewSettingsHandler`, `NewStorageHandler`, `NewUserPoolHandler`, `NewCloudInitHandler`, and `NewVMBRHandler` to `Make...` in `backend/handlers/admin.go`, `backend/handlers/admin_vms.go`, `backend/handlers/settings.go`, `backend/handlers/storage.go`, `backend/handlers/user_pool.go`, `backend/handlers/admin_cloudinit.go`, and `backend/handlers/vmbr.go`.
- [ ] T011 [P] [US1] Rename user-facing handler constructors from `NewAuthHandler`, `NewProfileHandler`, `NewSearchOptimizedHandler`, `NewHealthHandler`, `NewLanguageHandler`, and `NewDocsHandler` to `Make...` in `backend/handlers/auth.go`, `backend/handlers/profile.go`, `backend/handlers/search.go`, `backend/handlers/health.go`, `backend/handlers/language.go`, and `backend/handlers/docs.go`.
- [ ] T012 [P] [US1] Rename VM workflow constructors from `NewVMCreateOptimizedHandler`, `NewDiskHandler`, `NewVMSnapshotsHandler`, `NewTagsHandler`, and the handler-scoped `NewVMHandler` to `Make...` in `backend/handlers/vm_create.go`, `backend/handlers/disks.go`, `backend/handlers/vm_snapshots.go`, `backend/handlers/tags.go`, and `backend/handlers/vm_details_base.go`.
- [ ] T013 [US1] Update application bootstrap and route wiring to use the renamed constructors in `backend/main.go`, `backend/app/app.go`, `backend/handlers/vm_details_base.go`, `backend/handlers/settings_limits.go`, and any other compile-relevant files surfaced by repository search.
- [ ] T014 [US1] Perform a repository-wide pass over compile-relevant Go files under `backend/` to replace remaining approved `New...` call sites with `Make...` references without touching out-of-scope symbols.

**Checkpoint**: User Story 1 is complete when the approved constructor definitions and compile-relevant consumers consistently use `Make...` names.

---

## Phase 4: User Story 2 - Preserve existing behavior and public intent (Priority: P2)

**Goal**: Keep the rename mechanical only, preserving signatures, package boundaries, duplicate-name disambiguation, and developer-facing intent.

**Independent Test**: Review renamed constructors and selected workflows to confirm only symbol names changed while function signatures, returned values, and package ownership stayed the same.

### Implementation for User Story 2

- [ ] T015 [P] [US2] Verify and preserve package-local duplicate constructor renames for `AuthHandler` and `VMHandler` in `backend/api/v1/auth.go`, `backend/api/v1/vms.go`, `backend/handlers/auth.go`, and `backend/handlers/vm_details_base.go` so both packages keep their existing responsibilities.
- [ ] T016 [P] [US2] Review helper and wrapper flows that instantiate renamed constructors in `backend/handlers/form_session.go`, `backend/handlers/helpers.go`, `backend/handlers/validation.go`, `backend/handlers/template_data.go`, `backend/handlers/route_helpers.go`, and `backend/utils/errors.go` and ensure signatures and return behavior stay unchanged.
- [ ] T017 [US2] Update compile-relevant developer workflow references that would become misleading after the rename, including the example usage in `backend/handlers/doc.go` and any equivalent references discovered in repository documentation or helper code.
- [ ] T018 [US2] Diff-check the renamed constructor definitions in `backend/state/manager.go`, `backend/proxmox/telmate_client.go`, `backend/proxmox/resty_client.go`, and `backend/handlers/*.go` to confirm no logic, parameter, or return-value changes were introduced beyond the symbol rename.

**Checkpoint**: User Story 2 is complete when the rename remains purely mechanical and package-local behavior is unchanged.

---

## Phase 5: User Story 3 - Verify the refactor safely before merge (Priority: P3)

**Goal**: Prove the renamed codebase is clean by running the established formatting, offline test, and lint workflows and fixing any rename-related fallout.

**Independent Test**: Run the agreed repository verification commands and confirm they complete without constructor-rename-related failures.

### Implementation for User Story 3

- [ ] T019 [US3] Run the formatting gate defined by `Makefile` with `make go-fmt` and commit any formatting-only updates in the affected Go files under `backend/`.
- [ ] T020 [US3] Run the offline verification gate defined by `Makefile` with `make test-offline` and resolve any missed constructor rename references in the reported files under `backend/` and `backend/tests/`.
- [ ] T021 [US3] Run the lint verification gate defined by `Makefile` with `make go-lint` and fix any rename-related issues surfaced in the affected Go files under `backend/`.
- [ ] T022 [US3] Re-run the verification workflow from `specs/001-constructor-rename/quickstart.md` and update the implementation notes in `specs/001-constructor-rename/tasks.md` if additional follow-up files or commands were required.

**Checkpoint**: User Story 3 is complete when formatting, offline tests, and lint all pass after the mechanical rename.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final cleanup, audit, and readiness checks across all user stories.

- [ ] T023 [P] Perform a final stale-symbol audit with repository search under `backend/` and `specs/001-constructor-rename/` to confirm no approved compile-relevant `New...` names remain.
- [ ] T024 [P] Review the no-op planning contract in `specs/001-constructor-rename/contracts/openapi.yaml` and the implementation summary files in `specs/001-constructor-rename/` to ensure they still accurately describe a pure mechanical rename.
- [ ] T025 Prepare the branch for review by summarizing the final rename scope, affected file groups, and verification results in `specs/001-constructor-rename/tasks.md` or the pull request description.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; start immediately.
- **Foundational (Phase 2)**: Depends on Setup completion; establishes shared constructors and bootstrap naming.
- **User Story 1 (Phase 3)**: Depends on Foundational completion because it updates the main definition and call-site clusters.
- **User Story 2 (Phase 4)**: Depends on User Story 1 completion so behavior review happens on the final renamed symbols.
- **User Story 3 (Phase 5)**: Depends on User Stories 1 and 2 completion so verification runs on the intended final state.
- **Polish (Phase 6)**: Depends on all prior phases.

### User Story Dependencies

- **US1 (P1)**: Starts after Phase 2 and delivers the MVP mechanical rename.
- **US2 (P2)**: Starts after US1 and confirms the rename stayed non-behavioral and package-local.
- **US3 (P3)**: Starts after US1 and US2 and validates the repository gates.

### Within Each User Story

- Rename definitions before broad repository call-site cleanup.
- Update bootstrap and route wiring before final repository-wide stale-symbol searches.
- Complete behavior-preservation review before the final verification runs.
- Resolve verification fallout before moving to polish.

### Parallel Opportunities

- T004, T006, T007, and T008 can run in parallel after T003.
- T009, T010, T011, and T012 can run in parallel once Phase 2 is complete.
- T015 and T016 can run in parallel after User Story 1 is complete.
- T023 and T024 can run in parallel during polish.

---

## Parallel Example: User Story 1

```bash
# Parallel constructor-definition rename passes across independent file groups:
Task: "T009 [US1] Rename API-layer constructors in backend/api/v1/auth.go, backend/api/v1/vms.go, and backend/api/v1/vm_actions.go"
Task: "T010 [US1] Rename admin/settings constructors in backend/handlers/admin.go, backend/handlers/admin_vms.go, backend/handlers/settings.go, backend/handlers/storage.go, backend/handlers/user_pool.go, backend/handlers/admin_cloudinit.go, and backend/handlers/vmbr.go"
Task: "T011 [US1] Rename user-facing handler constructors in backend/handlers/auth.go, backend/handlers/profile.go, backend/handlers/search.go, backend/handlers/health.go, backend/handlers/language.go, and backend/handlers/docs.go"
Task: "T012 [US1] Rename VM workflow constructors in backend/handlers/vm_create.go, backend/handlers/disks.go, backend/handlers/vm_snapshots.go, backend/handlers/tags.go, and backend/handlers/vm_details_base.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1 to confirm the approved rename map and quality gates.
2. Complete Phase 2 to align shared constructors and foundational call sites.
3. Complete Phase 3 to rename all targeted definitions and compile-relevant references.
4. Validate with a targeted stale-symbol search and successful build before expanding scope.

### Incremental Delivery

1. Finish Setup + Foundational to stabilize shared naming.
2. Deliver US1 as the mechanical rename increment.
3. Deliver US2 as the behavior-preservation audit.
4. Deliver US3 as the verification and cleanup increment.
5. Finish with the polish audit and review summary.

### Parallel Team Strategy

1. One contributor handles foundational shared constructors (`state`, `proxmox`, helper utilities).
2. A second contributor handles API and handler rename clusters for US1.
3. A third contributor handles behavior review and developer-facing references once US1 lands.
4. All contributors converge on the verification phase and fix any fallout surfaced by formatting, tests, or lint.

---

## Notes

- The prerequisite script currently rejects execution on `main`; this task list was generated from the existing feature directory `specs/001-constructor-rename/` and its design artifacts.
- Keep the scope limited to the approved constructor rename mapping from `spec.md`.
- Do not rename types, packages, or unrelated `New...` symbols outside the approved scope.
- Treat generated planning artifacts as in-scope only when stale names would mislead maintainers or disrupt the documented workflow.
- Stop after each phase checkpoint to confirm the rename remains mechanical and independently verifiable.
