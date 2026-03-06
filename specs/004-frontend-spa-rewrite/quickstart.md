# Quickstart: Frontend SPA Rewrite

## Goal

Provide a repeatable development workflow for building and validating the new frontend SPA and its supporting backend API without relying on the legacy frontend as a runtime dependency.

## Prerequisites

- Go toolchain available for the existing backend.
- Node.js and npm available for the new frontend application.
- Existing application configuration available locally.
- Access to a reachable Proxmox environment or offline-compatible development mode where applicable.

## Repository Preparation

1. Check out branch `001-frontend-spa-rewrite`.
2. Confirm the feature artifacts exist under `specs/001-frontend-spa-rewrite/`.
3. Keep the legacy frontend available for engineering reference only.

## Development Workflow

### 1. Backend API development

1. Start from the existing backend application.
2. Implement or update versioned API handlers required by the contract.
3. Preserve public health endpoints.
4. Add or update automated backend tests for changed behavior.

### 2. Frontend SPA development

1. Create the new `frontend-svelte/` application structure.
2. Implement application layout, guarded routing, and authentication bootstrap.
3. Implement typed API integration by domain.
4. Build user workflows first, then admin workflows, then cutover cleanup.

### 3. Local validation

1. Run backend tests.
2. Run frontend linting and automated tests.
3. Validate contract alignment between frontend usage and backend responses.
4. Execute end-to-end checks for login, dashboard, VM details, VM creation, search, profile, console, and admin access control.

## Recommended Milestone Order

1. Authentication bootstrap and protected route handling.
2. Dashboard and VM details.
3. VM creation, search, and profile management.
4. Snapshot and console flows.
5. Admin sections.
6. Static hosting, catch-all routing, and legacy cutover cleanup.

## Verification Checklist

- Protected routes restore or redirect correctly after browser refresh.
- Public health endpoints remain available without authentication.
- User workflows no longer require legacy rendered pages.
- Admin routes are denied for non-admin users.
- Console flow surfaces actionable connection states.
- Legacy frontend remains in the repository only as reference material during migration.

## Handoff to Task Planning

Once implementation decomposition is needed, run `/speckit.tasks` using the completed planning artifacts in `specs/001-frontend-spa-rewrite/`.
