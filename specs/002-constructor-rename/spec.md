# Feature Specification: Constructor Rename to Make

**Feature Branch**: `001-constructor-rename`  
**Created**: 2026-03-06  
**Status**: Draft  
**Input**: User description: "Refactor: rename Go constructor functions from `New...` to `Make...` across the codebase as a pure mechanical rename with no logic changes, updating definitions and call sites and validating with formatting, tests, and linting."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Keep constructor usage consistent after the rename (Priority: P1)

As a maintainer, I want every targeted constructor function and its call sites renamed from the `New...` prefix to the `Make...` prefix so that the codebase follows one consistent naming convention without introducing behavior changes.

**Why this priority**: This is the core purpose of the refactor. Without consistent renaming across definitions and consumers, the codebase will not compile and the refactor will not be usable.

**Independent Test**: This can be fully tested by checking that every targeted constructor definition and every consumer reference use the new `Make...` name and that the application builds successfully.

**Acceptance Scenarios**:

1. **Given** a targeted constructor function currently named with the `New...` prefix, **When** the refactor is applied, **Then** the function definition uses the corresponding `Make...` name.
2. **Given** a file that calls a targeted constructor, **When** the refactor is applied, **Then** every matching call site uses the new `Make...` name and no stale `New...` reference remains for that targeted constructor.

---

### User Story 2 - Preserve existing behavior and public intent (Priority: P2)

As a maintainer, I want the rename to be mechanical only so that no runtime behavior, ownership boundaries, or data contracts change while adopting the new constructor naming convention.

**Why this priority**: A naming refactor is only low risk if it does not alter behavior. Preserving semantics protects existing users and reduces regression risk.

**Independent Test**: This can be tested by verifying that only constructor names and their references change, while function signatures, returned values, and user-facing behavior remain unchanged.

**Acceptance Scenarios**:

1. **Given** a targeted constructor before the rename, **When** the refactor is reviewed, **Then** its parameters, return values, and observable behavior remain unchanged apart from its name.
2. **Given** application workflows that depend on renamed constructors, **When** the system is exercised after the refactor, **Then** those workflows behave the same as before the rename.

---

### User Story 3 - Verify the refactor safely before merge (Priority: P3)

As a maintainer, I want the renamed codebase to pass the established verification steps so that I can merge the refactor with confidence.

**Why this priority**: Verification is a necessary safety net, but it only becomes relevant after the rename itself is in place.

**Independent Test**: This can be tested by running the agreed formatting, test, and lint steps and confirming they complete without constructor-rename-related failures.

**Acceptance Scenarios**:

1. **Given** the constructor rename is complete, **When** the standard formatting step is run, **Then** the codebase remains properly formatted.
2. **Given** the constructor rename is complete, **When** the standard test and lint steps are run, **Then** they report no failures caused by missed or inconsistent constructor renames.

---

### Edge Cases

- Constructors with the same function name in different packages must both be renamed without introducing package-level ambiguity.
- References in tests, helper utilities, and less frequently used code paths must be updated alongside production call sites.
- Files that mention the old names in generated or planning artifacts must not be treated as executable call sites unless they affect compilation or required developer workflows.
- If a targeted constructor is referenced through interfaces, function variables, or wrapper helpers, the rename must still leave those flows compilable and behaviorally unchanged.
- If any targeted constructor is missing from the rename set, the verification steps must surface the inconsistency before merge.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The refactor MUST rename each constructor function listed in the feature description from its `New...` name to the corresponding `Make...` name.
- **FR-002**: The refactor MUST update every compile-relevant reference to each targeted constructor so that consumers use the renamed `Make...` function.
- **FR-003**: The refactor MUST preserve the existing function signatures, return values, and runtime behavior of every targeted constructor.
- **FR-004**: The refactor MUST avoid renaming types, packages, or non-targeted functions unless such a change is strictly required to complete the constructor rename safely.
- **FR-005**: The refactor MUST handle duplicate constructor names that exist in different packages, such as `NewVMHandler` and `NewAuthHandler`, by renaming each within its own package scope without altering package boundaries.
- **FR-006**: The refactor MUST update verification-related artifacts that are required for normal development flow if they reference targeted constructor names and would otherwise become misleading or unusable.
- **FR-007**: The refactor MUST leave the codebase in a state where the agreed formatting, test, and lint verification steps can be executed successfully after the rename.
- **FR-008**: The refactor MUST provide a complete mapping of old constructor names to new constructor names so maintainers can verify the rename scope.

### Key Entities *(include if feature involves data)*

- **Constructor Rename Mapping**: The authoritative list of constructor functions included in the refactor, including each original `New...` name, its replacement `Make...` name, and its owning file or package.
- **Call Site Reference**: Any compile-relevant usage of a targeted constructor that must be updated to the renamed function while preserving behavior.
- **Verification Outcome**: The result of the required formatting, test, and lint checks used to confirm the rename is complete and safe to merge.

### Assumptions

- The feature description’s list of 45 constructor functions is the complete intended rename scope for this refactor.
- The rename is mechanical and does not require changes to business rules, user-visible content, or API payloads.
- Developer documentation or planning notes may be updated when useful, but executable code correctness and verification readiness take precedence.
- Existing verification steps named in the feature description remain the accepted definition of completion for this refactor.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of constructor functions explicitly included in the approved rename list use the `Make...` prefix after the refactor.
- **SC-002**: 100% of compile-relevant references to targeted constructors are updated so the codebase has no rename-related compilation failures.
- **SC-003**: Review of renamed constructors finds no intentional behavior changes beyond the function-name replacement.
- **SC-004**: The project’s agreed formatting, test, and lint verification steps complete without failures attributable to the constructor rename.
