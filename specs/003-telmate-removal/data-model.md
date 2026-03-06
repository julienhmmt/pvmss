# Data Model: Remove Telmate Dependency

## Overview

This feature does not add persisted business data. The design model describes the maintained integration artifacts that must move from the legacy Telmate-backed path to the supported Resty-based path.

## Entities

### LegacyIntegrationUsage

- **Description**: A maintained source location or behavior that still depends on the Telmate-backed Proxmox path.
- **Fields**:
  - `filePath`: Repository-relative location where the dependency exists.
  - `usageCategory`: Type of behavior such as VM configuration, node lookup, authentication, console access, state access, or test support.
  - `migrationStatus`: One of `identified`, `replaced`, `validated`, or `blocked`.
  - `removalDependency`: The obsolete Telmate-specific asset that cannot be removed until this usage is migrated.
- **Validation Rules**:
  - `filePath` must point to maintained application or test code.
  - `migrationStatus` may only become `validated` after phased verification succeeds.
  - `removalDependency` must map to a Telmate-specific method, file, interface, or module reference.

### RestyIntegrationCoverage

- **Description**: A supported Resty-based behavior that replaces a legacy integration usage.
- **Fields**:
  - `capabilityName`: Name of the maintained Proxmox behavior now served by Resty.
  - `coverageType`: Either `existing-equivalent` or `new-helper`.
  - `behaviorConstraint`: Summary of the compatibility or security behavior that must be preserved.
  - `validationScope`: The verification evidence required before the migration is accepted.
- **Validation Rules**:
  - `coverageType` must identify whether the capability already existed or had to be newly created.
  - `behaviorConstraint` must preserve user-visible behavior for the replaced legacy usage.

### StateManagerSurface

- **Description**: The maintained application interface through which handlers, helpers, and tests obtain Proxmox access.
- **Fields**:
  - `accessPattern`: The supported client access strategy after migration.
  - `consumerGroup`: Production handler, helper, startup path, or test/mock.
  - `cleanupStatus`: One of `legacy-present`, `consumer-migrated`, or `legacy-removed`.
- **Validation Rules**:
  - `cleanupStatus` cannot reach `legacy-removed` until all consumers have been migrated.
  - `accessPattern` must resolve to one supported client strategy after completion.

### DependencyRemovalCheckpoint

- **Description**: A verification checkpoint that determines whether Telmate-specific code and dependency declarations can be removed.
- **Fields**:
  - `phaseName`: Migration stage associated with the checkpoint.
  - `buildStatus`: Pass or fail result for compilation or build validation.
  - `lintStatus`: Pass or fail result for lint validation.
  - `testStatus`: Pass or fail result for offline tests.
  - `usageScanStatus`: Pass or fail result confirming no remaining in-scope legacy usage for the relevant phase.
- **Validation Rules**:
  - All statuses must be pass before the next destructive cleanup step can proceed.
  - A failed checkpoint blocks dependency-removal progression.

## Relationships

- A `LegacyIntegrationUsage` is replaced by one `RestyIntegrationCoverage` outcome.
- A `StateManagerSurface` is simplified as related legacy usages are migrated.
- A `DependencyRemovalCheckpoint` validates whether groups of legacy usages and obsolete code can be safely removed.

## State Transitions

### LegacyIntegrationUsage lifecycle

1. `identified` → legacy usage is known and in scope.
2. `replaced` → a Resty-backed replacement exists in maintained code.
3. `validated` → verification confirms the replacement behaves acceptably.
4. `blocked` → migration cannot proceed until a dependency or behavior issue is resolved.

### StateManagerSurface lifecycle

1. `legacy-present` → legacy Telmate-oriented access still exists.
2. `consumer-migrated` → callers have moved to the supported path.
3. `legacy-removed` → the Telmate-oriented surface is deleted.

### Dependency removal completion criteria

- All in-scope legacy usages are `validated`.
- The state manager surface is `legacy-removed`.
- Telmate-specific files, interfaces, and module declarations are absent.
- The final checkpoint records pass status for build, lint, tests, and usage scans.
