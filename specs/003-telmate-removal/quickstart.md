# Quickstart: Remove Telmate Dependency

## Goal

Migrate the remaining legacy Proxmox integration behavior to the supported Resty-based path, remove Telmate-backed infrastructure, and verify the application remains safe to merge.

## Prerequisites

- You are on branch `001-telmate-removal`.
- You are working from repository root: `/Users/jh/git/gh/pvmss`.
- Standard Go toolchain and repository validation tools are available.

## Execution Sequence

1. Review the feature artifacts:
   - `specs/001-telmate-removal/spec.md`
   - `specs/001-telmate-removal/plan.md`
   - `specs/001-telmate-removal/research.md`
   - `plans/telmate-migration-removal.md`

2. Execute the migration in phases:
   - Phase 1: Replace remaining legacy call sites that already have Resty equivalents.
   - Phase 2: Add missing Resty helper coverage and migrate those call sites.
   - Phase 3: Remove Telmate-backed state manager access and update dependent tests and mocks.
   - Phase 4: Delete obsolete Telmate-specific code and remove the Telmate module dependency.

3. After each phase, run the repository validation expected for this migration.

4. Before removing obsolete files and module references, confirm that no maintained Telmate-backed usage remains in scope.

5. Review the final diff to confirm the codebase converges on one supported Proxmox client strategy.

## Expected Validation

- Formatting completes successfully.
- Linting completes successfully.
- Offline tests complete successfully.
- Final build verification succeeds.
- No maintained Telmate-backed usage remains.

## Expected Outcome

- Maintained Proxmox behavior uses one supported Resty-based client strategy.
- Telmate-backed state manager access is removed.
- Telmate-specific dead code and dependency declarations are absent.
- The repository remains safe to merge under the project's quality gates.
