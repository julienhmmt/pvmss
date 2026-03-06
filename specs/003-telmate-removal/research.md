# Research: Remove Telmate Dependency

## Decision 1: Migrate remaining call sites in phases before deleting shared Telmate infrastructure

- **Decision**: Complete the migration in ordered phases: direct Resty swaps first, then new Resty helper creation, then state manager cleanup, and finally dead-code and dependency removal.
- **Rationale**: This ordering keeps behavior stable, limits the size of each reviewable change group, and makes it possible to validate regressions before irreversible cleanup occurs.
- **Alternatives considered**:
  - Remove Telmate infrastructure immediately and fix breakages afterward: rejected because it would create a large, hard-to-diagnose failure surface.
  - Migrate all files in one large change: rejected because it weakens traceability and increases rollback difficulty.

## Decision 2: Use Resty for all supported Proxmox interactions, including newly added helper coverage

- **Decision**: Standardize all maintained Proxmox interactions on the Resty-based client path and add new Resty helpers where feature-complete replacements do not yet exist.
- **Rationale**: One supported integration path reduces duplication, lowers onboarding cost, and removes inconsistent behavior between legacy and maintained code paths.
- **Alternatives considered**:
  - Keep a mixed model where Telmate remains for a few special cases: rejected because it preserves the dual-client burden and blocks dependency removal.
  - Wrap Telmate behind a new abstraction instead of removing it: rejected because the codebase already treats Resty as the supported direction.

## Decision 3: Preserve special authentication behavior without reintroducing Telmate dependency

- **Decision**: For flows that need behavior beyond the standard token-authenticated Resty path, preserve the required semantics through dedicated Resty-based handling rather than retaining Telmate.
- **Rationale**: Some operations require different authentication context, but that does not justify keeping the deprecated dependency if the behavior can be preserved through supported request handling.
- **Alternatives considered**:
  - Keep Telmate only for authentication-sensitive flows: rejected because it would leave the migration incomplete.
  - Change the user-visible behavior of those flows to fit the standard path: rejected because backward compatibility is mandatory.

## Decision 4: Remove Telmate-backed state manager access only after all consumers are migrated

- **Decision**: Delete Telmate-oriented state manager methods and fields only after handler, helper, and test consumers have moved to the supported client strategy.
- **Rationale**: The state manager is a central dependency surface; removing it too early would create broad compile and test breakage.
- **Alternatives considered**:
  - Delete state manager access first and migrate callers afterward: rejected because it would destabilize too many files at once.
  - Leave deprecated state manager access in place after migration: rejected because it would preserve unused abstraction surface.

## Decision 5: Use existing repository quality gates as migration proof

- **Decision**: Require formatting, linting, offline tests, and final build verification as the evidence that the migration is safe to merge.
- **Rationale**: The repository already defines these commands as the expected quality gates for backend maintenance work.
- **Alternatives considered**:
  - Use only compilation as proof: rejected because the constitution requires testable delivery.
  - Add bespoke test infrastructure specific to this migration: rejected because the existing project workflow already provides the right validation depth.
