# Feature Specification: Frontend SPA Rewrite

**Feature Branch**: `001-frontend-spa-rewrite`  
**Created**: 2026-03-06  
**Status**: Draft  
**Input**: User description: "Frontend rewrite from Go templ/HTMX hybrid UI to a SvelteKit SPA with JWT-only auth, REST API coverage, noVNC integration, admin routes, and legacy frontend preservation during cutover"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Access and operate my virtual machines from one modern web app (Priority: P1)

As an authenticated user, I can sign in to a single web application and complete my core VM tasks from a consistent interface, including viewing my machines, opening a VM details page, performing lifecycle actions, creating a VM, searching for VMs, and managing my own profile.

**Why this priority**: The rewrite only delivers value if end users can complete the primary VM management workflows without relying on legacy server-rendered pages.

**Independent Test**: Can be fully tested by signing in as a standard user and completing the end-to-end flows for dashboard viewing, VM details access, VM creation, search, and profile/password management using only the new application.

**Acceptance Scenarios**:

1. **Given** a valid user account, **When** the user signs in, **Then** the application grants access without a full page reload and displays the user’s VM dashboard.
2. **Given** an authenticated user with assigned VMs, **When** the user opens a VM details page, **Then** the application shows the VM’s current details, resource metrics, actions, disks, network information, and snapshots in a single unified experience.
3. **Given** an authenticated user with permission to create VMs, **When** the user submits a valid VM creation request, **Then** the application confirms the request and the new VM becomes visible through the new interface.
4. **Given** an authenticated user, **When** the user searches by VM name, identifier, or tags, **Then** the application returns matching VMs and allows navigation to the selected result.

---

### User Story 2 - Use secure session continuity without re-entering credentials unnecessarily (Priority: P2)

As an authenticated user, I remain signed in during normal use, the application renews access when allowed, and I am returned to the login page only when my authenticated session can no longer be refreshed.

**Why this priority**: A single-page application must preserve a secure and smooth authentication experience, otherwise users will face broken navigation and repeated sign-ins during normal operation.

**Independent Test**: Can be tested independently by signing in, using protected pages over time, forcing an expired access token, verifying silent renewal, and verifying a clean sign-out or redirect when renewal is no longer valid.

**Acceptance Scenarios**:

1. **Given** an authenticated user with a valid renewable session, **When** the current access token expires during navigation or data loading, **Then** the application renews access transparently and retries the original request.
2. **Given** an authenticated user whose session can no longer be renewed, **When** the user attempts to access a protected route, **Then** the application clears the user’s authenticated state and redirects to the login page.
3. **Given** an authenticated user, **When** the user signs out, **Then** protected data is no longer accessible and the application returns to an unauthenticated state.

---

### User Story 3 - Administer the platform from the same application (Priority: P3)

As an administrator, I can use protected admin routes in the same application to review infrastructure information and manage administrative settings, while non-admin users are prevented from reaching admin capabilities.

**Why this priority**: Administrative parity is required for complete cutover, but user-facing VM workflows deliver the first essential value and therefore take precedence.

**Independent Test**: Can be tested independently by signing in as an administrator, accessing each admin section, validating data visibility and management actions, and confirming non-admin access is blocked.

**Acceptance Scenarios**:

1. **Given** an administrator, **When** the administrator opens the admin area, **Then** the application shows the available admin sections and loads their current data.
2. **Given** an administrator, **When** the administrator updates a supported setting or managed record, **Then** the application confirms the change and the updated state is reflected consistently.
3. **Given** a non-admin authenticated user, **When** the user attempts to open an admin route, **Then** the application denies access and routes the user away from protected administration features.

---

### User Story 4 - Open an interactive VM console from the new application (Priority: P4)

As an authenticated user, I can launch an interactive console for a VM from the new application and receive clear feedback when a console cannot be established.

**Why this priority**: Console access is a critical operational capability, but it can be delivered after core navigation and management workflows are working.

**Independent Test**: Can be tested independently by opening the console for a running VM, confirming an interactive session is established, and verifying clear error handling for unavailable or unauthorized console sessions.

**Acceptance Scenarios**:

1. **Given** an authenticated user and a VM with console access available, **When** the user opens the console page, **Then** an interactive console session is established from within the new application.
2. **Given** an authenticated user and a VM whose console cannot be opened, **When** the user requests the console, **Then** the application displays a clear failure state and allows the user to retry or return to VM management.

### Edge Cases

- What happens when a user refreshes the browser while viewing a protected route and the access token exists only in memory?
- How does the system handle a failed silent session renewal while multiple requests are in flight?
- What happens when a user opens a bookmarked route that is valid in the new application but data loading fails because the backing service is temporarily unavailable?
- How does the system handle a VM action request that succeeds asynchronously but does not complete immediately?
- What happens when the console session can be requested but the interactive connection cannot be established afterward?
- How does the system handle users with no assigned VMs, no pool, or no eligible creation targets?
- What happens when an administrator loses admin privileges during an active session?
- How does the system handle unsupported or legacy routes during the cutover so users do not land on broken pages?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a single authenticated web application that replaces legacy user-facing pages for login, dashboard, VM creation, VM details, VM search, VM profile management, VM console access, and administrator pages.
- **FR-002**: The system MUST require users to authenticate before accessing protected application routes or protected backend capabilities.
- **FR-003**: The system MUST allow authenticated users to sign in, sign out, retrieve their current identity, and change their password from the new application.
- **FR-004**: The system MUST maintain session continuity for authenticated users by renewing access when a valid renewal mechanism is present and by returning users to an unauthenticated state when renewal is no longer valid.
- **FR-005**: The system MUST ensure that protected backend requests are authorized per user identity and that administrative capabilities require administrator authorization.
- **FR-006**: The system MUST expose complete machine-readable backend capabilities needed by the new application for user authentication, VM listing, VM details, VM creation, VM actions, description updates, tag updates, resource updates, network toggles, snapshot operations, console access, search, profile management, health status, and administrative operations.
- **FR-007**: The system MUST allow authenticated users to view a list of their accessible VMs with current operational state and summary resource information.
- **FR-008**: The system MUST allow authenticated users to open a detailed view of an individual VM that includes its current state, configuration details, resource usage, disks, network interfaces, and snapshots.
- **FR-009**: The system MUST allow authenticated users to perform supported VM lifecycle actions from the new application and receive clear feedback about request acceptance, success, pending completion, or failure.
- **FR-010**: The system MUST allow eligible users to submit a VM creation request using all required creation inputs and must preserve user-entered values when validation fails.
- **FR-011**: The system MUST allow authenticated users to search VMs by supported search criteria and return only results the user is allowed to see.
- **FR-012**: The system MUST allow authenticated users to manage their own profile information relevant to the current product experience, including password changes.
- **FR-013**: The system MUST allow authenticated users to list, create, delete, and roll back VM snapshots where permitted by policy.
- **FR-014**: The system MUST allow authenticated users to request interactive console access for a VM and must present clear connection status and failure feedback throughout the console flow.
- **FR-015**: The system MUST provide administrator-only sections for infrastructure overview, storage, nodes, VMs, pools, tags, limits, bridges, cloud-init templates, ISO assets, settings, and application diagnostics.
- **FR-016**: The system MUST prevent non-administrator users from accessing administrator-only pages and administrator-only backend capabilities, even if routes or requests are attempted directly.
- **FR-017**: The system MUST preserve legacy frontend assets in a separate location for reference during migration until the cutover is complete.
- **FR-018**: The system MUST route non-API application URLs for the new frontend experience to the new application entry point so that direct navigation, refreshes, and bookmarks continue to work.
- **FR-019**: The system MUST continue to provide public health status endpoints without requiring authentication.
- **FR-020**: The system MUST present loading, empty, success, and error states consistently across primary user and administrator workflows.
- **FR-021**: The system MUST support responsive use across desktop and tablet-sized viewports for all primary user and administrator workflows.
- **FR-022**: The system MUST provide a user-controlled visual theme switch between light and dark presentation modes.

### Assumptions

- English is the only required application language for the initial release of this feature.
- Existing business rules, permissions, limits, and Proxmox-backed behaviors remain unchanged unless explicitly required to support the new application experience.
- The migration is a single cutover to the new frontend experience rather than a page-by-page coexistence model.
- Legacy frontend assets remain available for engineering reference during migration but are not part of the primary end-user experience after cutover.

### Key Entities *(include if feature involves data)*

- **Authenticated User Session**: Represents a user’s current authenticated state, including identity, authorization level, renewal eligibility, and sign-out state.
- **User Profile**: Represents end-user identity and account context visible in the application, including username, administrator status, assigned pool, and password-management capability.
- **Virtual Machine**: Represents a managed VM visible in the application, including identity, ownership scope, state, resource usage, configuration summary, tags, disks, network interfaces, and related console/snapshot actions.
- **VM Creation Request**: Represents the data a user submits to request a new VM, including compute, storage, network, image, boot, and optional initialization choices.
- **Snapshot**: Represents a point-in-time recoverable state for a VM, including name, description, creation context, and rollback/delete actions.
- **Console Session**: Represents a temporary authorization to connect an authenticated user to a VM’s interactive console, including connection state and failure conditions.
- **Admin Resource**: Represents administrator-managed configuration or inventory data such as nodes, storage, pools, tags, limits, bridges, ISO assets, cloud-init templates, settings, and application diagnostics.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In user acceptance testing, 100% of the currently supported user-facing workflows can be completed from the new application without requiring any legacy server-rendered page.
- **SC-002**: Authenticated navigation to supported protected routes results in a usable page state within a perceptibly responsive time under normal internal network conditions (excluding upstream virtualization platform outages). No hard numeric threshold is enforced at this stage; the criterion is that navigation does not feel sluggish during manual validation on the target network.
- **SC-003**: At least 90% of test participants can complete the primary workflows of signing in, locating a VM, opening its details, and triggering a VM action on their first attempt without facilitator intervention.
- **SC-004**: 100% of administrator workflows identified in scope are accessible from administrator-only routes in the new application and blocked for non-admin users during validation.
- **SC-005**: Direct navigation or browser refresh on any supported application route succeeds without showing a server-generated not-found page in 100% of validation scenarios.
- **SC-006**: The new application reaches cutover readiness with no unresolved dependency on legacy frontend runtime libraries for in-scope user and administrator workflows.
