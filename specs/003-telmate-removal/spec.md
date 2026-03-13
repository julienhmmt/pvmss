# Feature Specification: Remove Telmate Dependency

**Feature Branch**: `001-telmate-removal`  
**Created**: 2026-03-06  
**Status**: Draft  
**Input**: User description: "Fully eliminate github.com/Telmate/proxmox-api-go from the codebase by migrating the remaining Proxmox calls to Resty, removing Telmate-backed state manager access and dead code, and validating that the application still builds, lints, and passes offline tests after the dependency is removed."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run all Proxmox operations through one maintained client path (Priority: P1)

As a maintainer, I want all remaining Proxmox operations to use the maintained Resty-based path so that the application depends on one consistent integration approach instead of split client behavior.

**Why this priority**: This delivers the main user value of the feature by removing the operational and maintenance risk of keeping two Proxmox integration paths alive.

**Independent Test**: Can be tested by reviewing the affected operations and confirming that previously Telmate-backed flows now use the maintained integration path while preserving expected application behavior.

**Acceptance Scenarios**:

1. **Given** remaining Proxmox operations still depend on the Telmate-backed path, **When** the migration is completed, **Then** those operations use the maintained Resty-based path instead.
2. **Given** a maintainer exercises the migrated administrative, VM, node, authentication, and console-related flows, **When** the feature is complete, **Then** the flows continue to behave as expected without relying on Telmate-backed access.

---

### User Story 2 - Remove obsolete Telmate-specific code safely (Priority: P2)

As a maintainer, I want the obsolete Telmate-specific client code, state access, and dead helpers removed once they are no longer needed so that the codebase is simpler and easier to maintain.

**Why this priority**: Once all supported flows use the maintained path, removing obsolete code completes the cleanup and reduces future confusion.

**Independent Test**: Can be tested by verifying that Telmate-specific files, interfaces, dependency entries, and state manager access paths are absent while repository validation still succeeds.

**Acceptance Scenarios**:

1. **Given** all in-scope Telmate-backed usages have been migrated, **When** the cleanup is finalized, **Then** obsolete Telmate-specific code and dependency declarations are removed from the maintained codebase.
2. **Given** the obsolete code has been removed, **When** maintainers run the standard repository validation workflow, **Then** the application still builds, lints, and passes offline tests.

---

### User Story 3 - Keep the migration staged and verifiable (Priority: P3)

As a maintainer, I want the migration performed in ordered phases with validation after each phase so that breakage can be detected early and the change remains reviewable.

**Why this priority**: This feature touches many files and integration points. Phased delivery lowers risk and improves confidence.

**Independent Test**: Can be tested by confirming that the implementation plan groups the work into verifiable phases and that completion evidence exists for each phase before dead-code removal is finalized.

**Acceptance Scenarios**:

1. **Given** the migration spans multiple integration areas, **When** implementation proceeds, **Then** the work is executed in ordered phases with validation at each stage before moving to the next.

---

### Edge Cases

- What happens if one remaining Telmate-backed call site is discovered late in the cleanup? It must be migrated or explicitly documented as a blocker before dependency removal can be completed.
- What happens if one migrated flow requires a different authentication mechanism than the standard token-based Resty path? The feature must preserve functional behavior through a supported alternative without reintroducing the Telmate dependency.
- What happens if deleting Telmate-specific state manager access breaks tests or mocks? The mocks and test support code must be updated as part of the same feature before completion is declared.
- What happens if removing dead code causes a compile, lint, or offline test failure? The cleanup remains incomplete until validation passes again.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The feature MUST migrate all remaining in-scope Telmate-backed Proxmox operations to supported Resty-based equivalents or new Resty-based replacements.
- **FR-002**: The feature MUST preserve existing user-visible behavior for in-scope authentication, VM, node, console, pool, cluster, and administrative flows while their internal integration path changes.
- **FR-003**: The feature MUST provide Resty-based support for any remaining in-scope Proxmox operation that does not yet have a maintained equivalent.
- **FR-004**: The feature MUST remove Telmate-backed state manager access once all in-scope consumers have been migrated away from it.
- **FR-005**: The feature MUST update maintained tests, mocks, and support code that depend on Telmate-backed interfaces or state access.
- **FR-006**: The feature MUST remove obsolete Telmate-specific files, helpers, interfaces, and dependency declarations only after all in-scope usage has been eliminated.
- **FR-007**: The feature MUST execute the migration in ordered, verifiable phases so that maintainers can validate correctness before dead code and dependency removal.
- **FR-008**: The feature MUST include repository validation that demonstrates the migrated application still builds, lints, and passes offline tests after Telmate removal.
- **FR-009**: The feature MUST ensure that all maintained Proxmox interactions use one supported client strategy after completion.
- **FR-010**: Any newly discovered Telmate dependency encountered during implementation MUST be migrated or documented as a blocker before the feature can be considered complete.

### Key Entities *(include if feature involves data)*

- **Telmate-backed operation**: A maintained application behavior that still depends on the legacy Proxmox integration path and must be migrated.
- **Resty-backed operation**: A maintained application behavior that uses the supported Proxmox integration path after migration.
- **State manager integration surface**: The set of application access points that expose Proxmox client capabilities to handlers, services, and tests.
- **Dependency removal package**: The group of obsolete files, interfaces, helpers, and module references that become removable once no maintained Telmate-backed usage remains.

### Assumptions

- Existing Resty-backed operations already establish the supported direction for Proxmox access in this codebase.
- Standard repository validation is sufficient to prove the migration did not introduce regressions at this stage.
- The migration can be performed incrementally without requiring a breaking external API change.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All known in-scope Telmate-backed application operations are reduced to zero remaining maintained usages.
- **SC-002**: Telmate-specific state manager access and dependency declarations are absent from the maintained codebase after the feature is completed.
- **SC-003**: The standard repository validation workflow completes successfully after Telmate removal.
- **SC-004**: Review of the final changeset confirms that maintained Proxmox interactions use one supported client strategy across the application.
