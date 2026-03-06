# Implementation Plan: Remove Telmate Dependency

**Branch**: `001-telmate-removal` | **Date**: 2026-03-06 | **Spec**: `/Users/jh/git/gh/pvmss/specs/001-telmate-removal/spec.md`
**Input**: Feature specification from `/Users/jh/git/gh/pvmss/specs/001-telmate-removal/spec.md`

## Summary

Complete the migration from the legacy Telmate Proxmox integration path to the maintained Resty-based path, then remove Telmate-backed state manager access, obsolete helper code, and the Telmate module dependency. Deliver the change in phased increments with validation after each phase so operational behavior remains stable while the codebase converges on one supported Proxmox client strategy.

## Technical Context

**Language/Version**: Go 1.24.4  
**Primary Dependencies**: Go standard library, `github.com/go-resty/resty/v2`, `github.com/julienschmidt/httprouter`, `github.com/rs/zerolog`, existing backend handler and state packages  
**Storage**: N/A for feature behavior; repository source files, module metadata, and tests are the primary change surface  
**Testing**: `go test`, offline integration test flow, `golangci-lint`, `go fmt`, and Makefile targets `test-offline`, `go-lint`, `go-fmt`, plus final build verification  
**Target Platform**: Go web application deployed on Linux, with development and maintenance on macOS  
**Project Type**: web application with Go backend and static frontend assets  
**Performance Goals**: No measurable regression in request handling, Proxmox operation availability, or maintenance workflow reliability after migration  
**Constraints**: Must remain non-breaking, must preserve security-sensitive authentication flows, must remove the Telmate module only after all maintained usages are gone, must keep phased validation evidence, must satisfy repository quality gates  
**Scale/Scope**: Multi-phase backend migration covering remaining Proxmox call sites, newly required Resty helpers, state manager interface cleanup, test/mock cleanup, and dead-code removal across handlers, proxmox helpers, state management, and module files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Backward Compatibility First**: Pass with strict guardrails. The migration is internal and must preserve all production-facing routes, configuration behavior, and Proxmox workflow outcomes while changing only the underlying client strategy.
- **II. Secure by Default**: Pass with explicit attention required. Authentication and password-update flows are in scope, so the plan requires preserving secure handling of tickets, CSRF tokens, cookies, and token-authenticated requests without weakening current controls.
- **III. Testable Change Delivery**: Pass with mandatory validation. Because the feature changes integration plumbing across multiple operational paths, each phase requires automated validation and the final feature requires build, lint, and offline test confirmation.
- **IV. Observability and Operability**: Pass. Existing structured logging and actionable error behavior must remain intact, and any newly added Resty helpers must preserve operational diagnosability.
- **V. Simplicity and Maintainability**: Pass. Converging on one supported client path and removing obsolete integration layers directly supports the constitution's maintainability principle.

**Post-Design Re-check**: Pass. The design artifacts keep the migration phased, non-breaking, and validation-driven, with no unjustified constitutional violations.

## Project Structure

### Documentation (this feature)

```text
specs/001-telmate-removal/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── migration-verification.openapi.yaml
└── tasks.md
```

### Source Code (repository root)

```text
backend/
├── handlers/
│   ├── auth.go
│   ├── admin.go
│   ├── profile.go
│   ├── search.go
│   ├── storage.go
│   ├── user_pool.go
│   ├── vm_actions_misc.go
│   ├── vm_console_helpers.go
│   ├── vm_details_info.go
│   └── vmbr.go
├── proxmox/
│   ├── resty_client.go
│   ├── vms.go
│   ├── nodes.go
│   ├── cluster.go
│   ├── vnc.go
│   ├── access.go
│   ├── telmate_client.go
│   ├── cache.go
│   └── interfaces.go
├── state/
│   ├── interface.go
│   ├── manager.go
│   └── manager_proxmox.go
├── tests/
├── go.mod
└── go.sum

frontend/
├── css/
├── js/
└── scss/

specs/
└── 001-telmate-removal/
```

**Structure Decision**: Use the existing web application structure. Changes are concentrated in backend handler, Proxmox integration, state manager, and test/mock files, with documentation artifacts stored in the feature spec directory.

## Phase 0: Research and Decisions

- Confirm the migration strategy for remaining call sites that already have Resty equivalents.
- Confirm how to implement new Resty helpers for operations that still rely on Telmate-specific behavior, especially authentication and VNC-related flows.
- Confirm the safest order for removing state manager Telmate access, test mocks, dead code, and the module dependency.

## Phase 1: Design Focus

- Model remaining Telmate-backed behaviors, their Resty-backed replacements, and the dependency-removal lifecycle.
- Record a maintenance contract that captures phased verification rather than user-facing API expansion.
- Provide a quickstart for maintainers executing the migration in ordered stages with validation checkpoints.
- Update agent context from the completed plan so subsequent implementation work reflects the active stack and migration direction.

## Complexity Tracking

No constitutional violations require exception handling for this plan.

| Violation | Why Needed | Simpler Alternative Rejected Because |
| --------- | ---------- | ----------------------------------- |
