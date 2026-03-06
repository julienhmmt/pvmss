# Implementation Plan: Frontend SPA Rewrite

**Branch**: `001-frontend-spa-rewrite` | **Date**: 2026-03-06 | **Spec**: [`/Users/jh/git/gh/pvmss/specs/001-frontend-spa-rewrite/spec.md`]  
**Input**: Feature specification from `/Users/jh/git/gh/pvmss/specs/001-frontend-spa-rewrite/spec.md`

## Summary

Replace the current hybrid frontend with a single-page application served by the existing Go backend, while expanding backend JSON capabilities to cover all user and admin workflows. The plan uses a big-bang cutover, keeps legacy frontend files available for engineering reference, preserves public health endpoints, and formalizes the new authentication/session continuity model for protected routes and interactive console usage.

## Technical Context

**Language/Version**: Go (existing backend), TypeScript (new frontend SPA)  
**Primary Dependencies**: SvelteKit SPA runtime, typed browser API client, JWT-based auth flow, noVNC client integration  
**Storage**: Existing application configuration files, existing backend persistence and Proxmox-backed data sources, browser memory for short-lived access state  
**Testing**: Go automated tests, frontend automated unit/component tests, contract validation, end-to-end regression coverage for primary user/admin workflows  
**Target Platform**: Internal web application served from the existing backend for desktop and tablet browser use  
**Project Type**: Web application  
**Performance Goals**: Protected route navigation and primary page loads should meet the feature success target of usable page state within 2 seconds under normal internal conditions  
**Constraints**: Big-bang migration, secure-by-default auth handling, preserve public health endpoints, preserve legacy frontend as engineering reference, avoid exposing sensitive credentials or tokens in browser-persistent storage  
**Scale/Scope**: Full replacement of in-scope user and admin frontend workflows, new backend API surface for all required operations, cutover-ready documentation artifacts for one major product migration

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Backward Compatibility First**: **Pass with managed exception scope**. This feature intentionally replaces the existing frontend experience and authentication interaction model as part of an approved cutover. Public health endpoints and operational backend compatibility outside the approved UI migration remain preserved. Any additional breaking changes outside this approved scope are out of bounds.
- **II. Secure by Default**: **Pass**. The planned auth model keeps short-lived access state out of browser-persistent storage, keeps renewal state in protected cookie transport, and requires explicit admin authorization for admin routes and APIs.
- **III. Testable Change Delivery**: **Pass**. The plan requires automated backend, frontend, contract, and end-to-end coverage for primary flows before cutover readiness.
- **IV. Observability and Operability**: **Pass**. The design requires actionable error states for user workflows, structured backend observability for auth/API/console flows, and timeout-aware handling for external Proxmox interactions.
- **V. Simplicity and Maintainability**: **Pass**. The plan consolidates the frontend into one application, replaces mixed frontend paradigms with a single UI architecture, and organizes backend API behavior by clear responsibility.

**Post-Design Re-check**: Pass. Phase 1 artifacts preserve the same exception scope, document security boundaries, and keep implementation organization aligned with the constitution.

## Project Structure

### Documentation (this feature)

```text
specs/001-frontend-spa-rewrite/
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
├── handlers/
├── middleware/
├── proxmox/
├── state/
└── tests/

frontend-svelte/
├── src/
│   ├── lib/
│   │   ├── api/
│   │   ├── components/
│   │   ├── stores/
│   │   ├── types/
│   │   └── utils/
│   └── routes/
├── static/
└── tests/

frontend-legacy/

specs/
└── 001-frontend-spa-rewrite/
```

**Structure Decision**: Use a web application structure with the existing Go backend as the API and static host, a new `frontend-svelte/` application for the SPA, and `frontend-legacy/` as preserved reference material during migration. This keeps responsibilities explicit and supports the planned big-bang cutover.

## Phase 0 Research Focus

- Define the session continuity model for memory-held access state and renewable protected session state.
- Define the browser refresh/direct-navigation recovery model for protected routes.
- Define the VM console interaction pattern and failure handling in the new application.
- Define API contract organization that maps current backend responsibilities into a cohesive versioned surface.
- Define migration boundaries for what is preserved, removed, and deferred in the cutover.

## Phase 1 Design Focus

- Formalize frontend domain entities and request/response models.
- Produce a contract-first API surface for authentication, VMs, snapshots, console, search, health, and admin operations.
- Document a developer quickstart that supports both backend and new frontend workflows during migration.
- Update Windsurf agent context so subsequent implementation steps reference the new stack and structure.

## Complexity Tracking

- **Violation**: Approved frontend breaking-change scope
- **Why needed**: The feature intentionally replaces the legacy UI and auth interaction model as part of a planned cutover.
- **Simpler alternative rejected because**: Incremental coexistence would preserve compatibility longer but adds duplicated auth, routing, and page ownership complexity that the design explicitly rejects.
