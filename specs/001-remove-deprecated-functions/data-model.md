# Data Model: Remove Deprecated Functions

## Overview

This feature does not introduce persisted application data. The design model describes the maintained source artifacts that participate in the cleanup and validation workflow.

## Entities

### DeprecatedFunctionReference

- **Description**: A maintained source location that calls the deprecated handler context entry point.
- **Fields**:
  - `filePath`: Repository-relative location of the source file.
  - `symbolName`: Name of the deprecated symbol currently referenced.
  - `referenceType`: Source classification such as production code or maintained test code.
  - `migrationStatus`: One of `identified`, `updated`, or `verified`.
- **Validation Rules**:
  - `symbolName` must match the deprecated entry point targeted by the feature.
  - `filePath` must reference maintained application source or maintained tests.
  - `migrationStatus` cannot move to `verified` until repository validation is complete.

### SupportedFunctionReference

- **Description**: A maintained source location that uses the supported handler context entry point after cleanup.
- **Fields**:
  - `filePath`: Repository-relative location of the source file.
  - `symbolName`: Name of the supported entry point.
  - `replacementReason`: Short explanation of why the maintained entry point is used.
- **Validation Rules**:
  - `symbolName` must match the maintained handler context entry point.
  - `filePath` must correspond to a migrated deprecated reference or a newly discovered in-scope reference.

### CleanupVerificationRun

- **Description**: A validation record for proving the deprecated wrapper can be safely removed.
- **Fields**:
  - `formattingStatus`: Pass or fail result for source formatting.
  - `lintStatus`: Pass or fail result for linting.
  - `testStatus`: Pass or fail result for offline automated tests.
  - `referenceScanStatus`: Pass or fail result confirming no remaining maintained references.
  - `notes`: Optional maintainership notes for blockers or anomalies.
- **Validation Rules**:
  - All statuses must be pass before the cleanup can be considered complete.
  - `notes` is required when any status is fail.

## Relationships

- A `DeprecatedFunctionReference` is transformed into a `SupportedFunctionReference` during migration.
- A `CleanupVerificationRun` validates the entire migrated reference inventory rather than a single file.

## State Transitions

### DeprecatedFunctionReference lifecycle

1. `identified` → reference is known to use the deprecated entry point.
2. `updated` → source has been changed to the supported entry point.
3. `verified` → repository validation and reference scanning confirm the migration is complete.

### Cleanup completion criteria

- All identified deprecated references reach `verified`.
- The deprecated wrapper definition is removed.
- The verification run records pass status for formatting, linting, tests, and reference scan.
