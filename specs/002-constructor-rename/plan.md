# Implementation Plan: Constructor Rename to Make

**Branch**: `001-constructor-rename` | **Date**: 2026-03-06 | **Spec**: [/Users/jh/git/gh/pvmss/specs/001-constructor-rename/spec.md](/Users/jh/git/gh/pvmss/specs/001-constructor-rename/spec.md)
**Input**: Feature specification from `/specs/001-constructor-rename/spec.md`

## Summary

Perform a mechanical refactor that renames the approved set of Go constructor functions from the `New...` prefix to the `Make...` prefix across definitions and compile-relevant call sites, while preserving behavior, maintaining package boundaries, and validating the result with the project’s formatting, offline test, and lint workflows.

## Technical Context

**Language/Version**: Go 1.24.4, Markdown for planning artifacts  
**Primary Dependencies**: Go standard tooling, project-local Make targets, `golangci-lint`, Go test tooling  
**Storage**: File-based repository artifacts only; no schema or persisted data changes  
**Testing**: `go test`, integration-tagged Go tests, project Make targets such as `make test-offline`  
**Target Platform**: macOS developer environment and the project’s standard Go build/CI environment  
**Project Type**: Monorepo-style web application with Go backend and supporting frontend assets  
**Performance Goals**: No measurable runtime change; post-refactor builds and verification should complete within the project’s normal developer workflow expectations  
**Constraints**: No logic changes, no route or API contract changes, no type renames, no configuration format changes, no behavior regressions  
**Scale/Scope**: 45 approved constructor renames spanning definitions and call sites across backend packages, tests, and supporting compile-relevant files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Backward Compatibility First**: Pass. The feature is explicitly scoped as a non-breaking mechanical rename with no production-facing contract changes.
- **II. Secure by Default**: Pass. No secrets, auth flows, or security defaults are changed.
- **III. Testable Change Delivery**: Pass with planned verification. This refactor does not introduce new behavior, so verification will rely on formatting, build/test coverage through existing automated suites, and linting to catch missed call sites or regressions.
- **IV. Observability and Operability**: Pass. No operational workflows, background jobs, or logging semantics are modified.
- **V. Simplicity and Maintainability**: Pass. The feature reduces naming inconsistency through a scoped mechanical rename and avoids broader restructuring.

**Post-Design Check**: Pass. The generated research and design artifacts keep the feature non-breaking, avoid implementation drift, and preserve the existing verification gates.

## Project Structure

### Documentation (this feature)

```text
specs/001-constructor-rename/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── openapi.yaml
└── tasks.md
```

### Source Code (repository root)

```text
backend/
├── api/
│   └── v1/
├── app/
├── handlers/
├── middleware/
├── proxmox/
├── state/
├── tests/
└── utils/

frontend/
├── css/
├── js/
└── scss/

specs/
└── 001-constructor-rename/
```

**Structure Decision**: The implementation work is concentrated in the existing Go backend source tree and test helpers where constructor definitions and compile-relevant references live. Planning artifacts remain under `specs/001-constructor-rename/`. No new runtime modules or services are required.

## Phase 0: Research Summary

- Use a single authoritative rename mapping derived from the approved spec scope.
- Treat the change as a mechanical symbol rename across definitions and compile-relevant references only.
- Use existing project verification commands as the release gate for completeness.
- Avoid expanding scope into broader naming, type, or package refactors.

## Phase 1: Design Summary

Phase 1 models the feature around rename mappings, compile-relevant call sites, and verification outcomes.
The design includes a no-op public API contract artifact documenting that this feature does not alter runtime HTTP behavior.
The quickstart focuses on the maintainer workflow for applying and verifying the rename safely.

## Complexity Tracking

No constitutional violations or complexity exceptions are required for this feature.
