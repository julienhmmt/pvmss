# Feature Specification: Remove Deprecated Functions

**Feature Branch**: `001-remove-deprecated-functions`  
**Created**: 2026-03-06  
**Status**: Draft  
**Input**: User description: "Remove deprecated NewHandlerContext by replacing all call sites with HandlerContextWith, deleting the deprecated wrapper, and explicitly keeping Telmate migration cleanup out of scope until Resty replacements exist."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Remove active usage of the deprecated entry point (Priority: P1)

As a maintainer, I want all active handler code paths to use the supported handler context entry point so that the codebase has a single, authoritative way to initialize handler context behavior.

**Why this priority**: This is the core value of the change. If active call sites still use the deprecated entry point, the feature does not achieve its cleanup goal.

**Independent Test**: Can be fully tested by reviewing all handler packages and automated checks to confirm that active code paths no longer reference the deprecated entry point while behavior remains unchanged.

**Acceptance Scenarios**:

1. **Given** a codebase that still contains active references to the deprecated handler context entry point, **When** the cleanup is completed, **Then** all active handler call sites use the supported entry point instead.
2. **Given** a maintainer reviewing the updated code, **When** they inspect the handler files listed in the cleanup scope, **Then** they find no remaining active references to the deprecated entry point.

---

### User Story 2 - Remove the deprecated wrapper safely (Priority: P2)

As a maintainer, I want the deprecated wrapper removed once no supported code depends on it so that the codebase no longer advertises obsolete APIs.

**Why this priority**: Removing the wrapper is the visible completion step for the deprecation cleanup, but it depends on the active call sites being updated first.

**Independent Test**: Can be tested by confirming the deprecated wrapper definition is absent and the project still passes formatting, build, and relevant verification checks.

**Acceptance Scenarios**:

1. **Given** all in-scope call sites have been migrated, **When** the cleanup is finalized, **Then** the deprecated wrapper is removed from the codebase.
2. **Given** the deprecated wrapper has been removed, **When** verification checks are run, **Then** the application still builds and tests without regressions attributable to this cleanup.

---

### User Story 3 - Preserve scope boundaries for larger migration work (Priority: P3)

As a maintainer, I want the cleanup to leave Telmate migration items untouched so that this small deprecation-removal change does not expand into a larger migration effort.

**Why this priority**: Clear scope control prevents accidental coupling with broader infrastructure migration work and keeps the feature shippable.

**Independent Test**: Can be tested by reviewing the resulting changeset and confirming that Telmate migration TODO areas remain unchanged unless they are directly required by the deprecated entry point removal.

**Acceptance Scenarios**:

1. **Given** separate migration work is still pending, **When** this feature is implemented, **Then** the deprecated wrapper cleanup is completed without removing or refactoring unrelated Telmate migration TODO items.

---

### Edge Cases

- What happens if one of the known call sites exists only in tests or helper code and not in production flow? It must still be migrated if it references the deprecated entry point and remains in supported source.
- What happens if a new reference to the deprecated entry point is introduced while the cleanup is in progress? Verification must fail the feature until that reference is removed.
- What happens if removal of the wrapper reveals hidden dependencies outside the originally identified files? Those dependencies become part of the in-scope cleanup, but broader Telmate migration work remains out of scope.
- What happens if verification shows behavior changes in request handling? The cleanup must be considered incomplete until behavior parity is restored.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The codebase MUST replace every in-scope use of the deprecated handler context entry point with the supported handler context entry point.
- **FR-002**: The cleanup MUST include all currently identified call sites in active source files and test files that are part of normal repository maintenance.
- **FR-003**: The deprecated wrapper MUST be removed only after all remaining in-scope references have been eliminated.
- **FR-004**: The feature MUST preserve existing handler behavior, request flow expectations, and observable outcomes for consumers of the updated code paths.
- **FR-005**: The feature MUST include a verification step that confirms no remaining references to the deprecated wrapper exist in maintained application source.
- **FR-006**: The feature MUST complete the standard validation expected for a small maintenance cleanup so that maintainers can confirm the change is safe to merge.
- **FR-007**: The feature MUST explicitly exclude unrelated Telmate migration TODO removal and any larger migration refactor that is not required to remove the deprecated wrapper.
- **FR-008**: Any newly discovered dependency on the deprecated wrapper during implementation MUST be migrated or documented as a blocker before the feature can be considered complete.

### Key Entities *(include if feature involves data)*

- **Deprecated handler context entry point**: The obsolete public entry point currently retained for backward compatibility and targeted for removal by this feature.
- **Supported handler context entry point**: The single maintained entry point that all in-scope handler code should use after cleanup.
- **In-scope call site inventory**: The set of source locations identified for migration as part of this feature, including active handler files and any maintained tests that still reference the deprecated entry point.

### Assumptions

- The supported handler context entry point is behaviorally equivalent for all identified usages of the deprecated wrapper.
- Telmate migration TODO items remain tracked elsewhere and do not need to be resolved to deliver this cleanup.
- Standard repository validation commands are sufficient to demonstrate that the cleanup did not introduce regressions.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All known in-scope references to the deprecated handler context entry point are reduced from the current inventory to zero.
- **SC-002**: The deprecated wrapper definition is absent from the maintained codebase after the feature is completed.
- **SC-003**: Verification steps complete successfully without failures caused by the cleanup.
- **SC-004**: Review of the final changeset confirms no unrelated Telmate migration TODO cleanup was included in this feature.
