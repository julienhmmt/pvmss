# Data Model: Constructor Rename to Make

## Overview

This feature does not introduce persisted runtime data. Its planning model centers on the artifacts that define and verify the rename scope.

## Entities

### ConstructorRename

- **Purpose**: Represents one approved constructor rename in scope for the feature.
- **Fields**:
  - `oldName`: Existing constructor name using the `New...` prefix.
  - `newName`: Replacement constructor name using the `Make...` prefix.
  - `packagePath`: Owning package of the constructor.
  - `filePath`: Source file that contains the constructor definition.
  - `status`: Planning state for the rename, such as identified, updated, or verified.
- **Validation Rules**:
  - `oldName` and `newName` must be non-empty.
  - `newName` must map directly to the corresponding `Make...` form of `oldName`.
  - Each rename entry must point to exactly one owning package and definition file.
- **Relationships**:
  - One `ConstructorRename` can have many `CallSiteReference` entries.
  - One `ConstructorRename` contributes to one `VerificationOutcome` summary.

### CallSiteReference

- **Purpose**: Represents a compile-relevant usage of a targeted constructor.
- **Fields**:
  - `constructorName`: The constructor symbol used at the call site.
  - `packagePath`: Package where the call site exists.
  - `filePath`: File containing the reference.
  - `referenceType`: Definition consumer, test helper, wrapper helper, or other compile-relevant usage.
  - `status`: Planning state for the reference, such as pending update or verified.
- **Validation Rules**:
  - Every call site must correspond to a `ConstructorRename` entry in scope.
  - References outside the approved rename scope must not be changed.
  - Each updated reference must use the exact renamed constructor symbol.
- **Relationships**:
  - Many `CallSiteReference` entries belong to one `ConstructorRename`.

### VerificationOutcome

- **Purpose**: Captures the result of the required verification workflow for the refactor.
- **Fields**:
  - `formatStatus`: Outcome of repository formatting verification.
  - `testStatus`: Outcome of offline automated tests.
  - `lintStatus`: Outcome of static analysis and linting.
  - `issues`: Any rename-related failures or omissions found during verification.
- **Validation Rules**:
  - A completed feature requires passing status for formatting, tests, and linting.
  - Any verification issue must map back to one or more rename entries or call sites for remediation.
- **Relationships**:
  - One `VerificationOutcome` summarizes the state of many `ConstructorRename` and `CallSiteReference` entries.

## State Transitions

### ConstructorRename Lifecycle

1. `identified` → rename is listed in the approved scope.
2. `updated` → definition and known references are renamed.
3. `verified` → formatting, tests, and linting no longer report issues related to this rename.

### CallSiteReference Lifecycle

1. `discovered` → compile-relevant reference is identified.
2. `updated` → reference uses the renamed symbol.
3. `verified` → repository verification confirms no stale symbol remains in the validated workflow.
