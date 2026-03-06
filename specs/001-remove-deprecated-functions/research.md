# Research: Remove Deprecated Functions

## Decision 1: Use `HandlerContextWith` as the sole maintained handler context entry point

- **Decision**: Replace all maintained uses of `NewHandlerContext` with `HandlerContextWith` and remove `NewHandlerContext` after migration is complete.
- **Rationale**: The deprecated function is a thin wrapper with no distinct behavior. Consolidating on the maintained entry point reduces duplicate API surface and makes future handler changes easier to reason about.
- **Alternatives considered**:
  - Keep both functions indefinitely: rejected because it preserves unnecessary duplication and keeps the deprecated API visible.
  - Replace only production call sites: rejected because maintained tests and helper code would still depend on the deprecated API.

## Decision 2: Treat this work as an internal maintenance cleanup with no external contract changes

- **Decision**: Limit the feature to internal source updates and validation, with no route, payload, configuration, or user-facing behavior changes.
- **Rationale**: The feature goal is deprecation cleanup, not behavior redesign. Keeping scope narrow satisfies the non-breaking maintenance requirement.
- **Alternatives considered**:
  - Combine cleanup with broader handler refactors: rejected because it increases risk and weakens traceability of regressions.
  - Fold the work into Telmate migration tasks: rejected because that migration is already tracked separately and has a much larger scope.

## Decision 3: Use the repository's standard maintenance validation workflow

- **Decision**: Validate the cleanup with the standard repository workflow: formatting, linting, offline tests, and explicit verification that no maintained source references `NewHandlerContext` remain.
- **Rationale**: This feature should prove safety through the same checks already expected for backend maintenance work.
- **Alternatives considered**:
  - Skip tests because the change is mechanical: rejected because the constitution requires testable delivery.
  - Add new bespoke infrastructure for validation: rejected because existing project commands already cover the required confidence level.

## Decision 4: Keep Telmate migration TODO items explicitly out of scope

- **Decision**: Do not remove or refactor Telmate migration TODO areas unless a direct dependency on `NewHandlerContext` removal is discovered.
- **Rationale**: The feature specification explicitly bounds scope to deprecated handler context cleanup.
- **Alternatives considered**:
  - Opportunistically clean nearby migration code: rejected because it introduces unrelated change risk.
  - Remove TODO comments without completing the underlying migration: rejected because it would create inaccurate project state.
