# Implementation Plan: Remove Deprecated Functions

**Branch**: `001-remove-deprecated-functions` | **Date**: 2026-03-06 | **Spec**: `/Users/jh/git/gh/pvmss/specs/001-remove-deprecated-functions/spec.md`
**Input**: Feature specification from `/Users/jh/git/gh/pvmss/specs/001-remove-deprecated-functions/spec.md`

## Summary

Replace all maintained usages of the deprecated `NewHandlerContext` entry point with `HandlerContextWith`, remove the deprecated wrapper once all references are migrated, and verify the cleanup with the repository's standard Go formatting, linting, and offline test workflow. Keep unrelated Telmate migration cleanup explicitly out of scope.

## Technical Context

**Language/Version**: Go 1.24.4  
**Primary Dependencies**: Go standard library, `github.com/julienschmidt/httprouter`, `github.com/rs/zerolog`, existing handler/context utilities in `backend/handlers`  
**Storage**: N/A for this feature; repository source files only  
**Testing**: `go test`, existing offline integration test flow, `golangci-lint`, `go fmt`, Makefile targets `test-offline`, `go-lint`, and `go-fmt`  
**Target Platform**: Linux-hosted Go web application with local development on macOS  
**Project Type**: web application with Go backend and static frontend assets  
**Performance Goals**: No measurable runtime regression in handler initialization behavior; build and test times remain within normal maintenance workflow expectations  
**Constraints**: Must remain non-breaking, must not change external routes or request behavior, must not expand into Telmate-to-Resty migration work, must satisfy repository quality gates  
**Scale/Scope**: One deprecated wrapper, approximately 25 known call sites across backend handlers and tests, plus any newly discovered references found during verification

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Backward Compatibility First**: Pass. The plan preserves existing request behavior and only changes the internal entry point used to create handler context.
- **II. Secure by Default**: Pass. No authentication, session, secret, or input-validation behavior is changed.
- **III. Testable Change Delivery**: Pass with required validation. The change is a maintenance refactor, so the plan requires repository validation through formatting, linting, and offline automated tests. Additional regression tests are not required unless behavior changes are discovered.
- **IV. Observability and Operability**: Pass. No new operational surface is introduced; existing logging and error behavior must remain unchanged.
- **V. Simplicity and Maintainability**: Pass. Replacing a deprecated wrapper with the maintained helper reduces duplication and clarifies the supported path.

**Post-Design Re-check**: Pass. The design artifacts keep the scope limited to source cleanup and verification, with no constitutional violations introduced.

## Project Structure

### Documentation (this feature)

```text
specs/001-remove-deprecated-functions/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── maintenance-verification.openapi.yaml
└── tasks.md
```

### Source Code (repository root)

```text
backend/
├── handlers/
│   ├── handler_context.go
│   ├── helpers.go
│   ├── auth.go
│   ├── profile.go
│   ├── search.go
│   ├── vm_actions_lifecycle.go
│   ├── vm_actions_misc.go
│   ├── vm_actions_resources.go
│   ├── vm_delete.go
│   └── vm_details_info.go
├── tests/
├── go.mod
└── go.sum

frontend/
├── css/
├── js/
└── scss/

specs/
└── 001-remove-deprecated-functions/
```

**Structure Decision**: Use the existing web-application repository structure. Implementation work is confined to backend handler source files and maintained tests; no frontend or deployment structure changes are required.

## Phase 0: Research and Decisions

- Confirm the maintained replacement for the deprecated wrapper and whether it is behaviorally equivalent for all known call sites.
- Confirm the repository validation workflow expected for a small backend maintenance cleanup.
- Confirm the correct scope boundary for Telmate migration TODOs so the feature remains non-invasive.

## Phase 1: Design Focus

- Document the small domain model for deprecated API cleanup and reference inventory.
- Record a maintenance contract showing that no external HTTP API changes are introduced.
- Provide a quickstart sequence for maintainers to execute the cleanup and validate completion.
- Update agent context using the generated implementation plan so future work remains aligned with the planned technology stack.

## Complexity Tracking

No constitutional violations or exceptional complexity are expected for this feature.

| Violation | Why Needed | Simpler Alternative Rejected Because |
| --------- | ---------- | ----------------------------------- |
