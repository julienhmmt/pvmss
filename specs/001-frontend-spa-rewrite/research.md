# Research: Frontend SPA Rewrite

## Decision 1: Keep short-lived access state in browser memory and use renewable protected cookie transport for session continuity

**Decision**: The application will keep the access token only in browser memory and use a protected renewal mechanism carried by browser cookie transport for silent session continuation.

**Rationale**:

- Reduces exposure of active credentials compared with persistent browser storage.
- Fits the requirement for secure-by-default session continuity in a single-page application.
- Supports automatic recovery from access-token expiration during normal use.

**Alternatives considered**:

- Persistent browser storage for access credentials: rejected because it increases credential exposure in the browser.
- Re-authentication on every expiry: rejected because it breaks normal workflow continuity and degrades usability.

## Decision 2: Recover protected routes after browser refresh by rehydrating identity through a guarded bootstrap check

**Decision**: On initial application load or browser refresh, the application will perform a guarded bootstrap flow that attempts to restore authenticated identity before rendering protected route content.

**Rationale**:

- Access state stored in memory is lost on refresh, so the application needs a deterministic recovery path.
- Prevents protected routes from flashing unauthorized content or failing unpredictably after direct navigation.
- Aligns with the requirement that bookmarks and browser refreshes continue to work for supported routes.

**Alternatives considered**:

- Persist access credentials across refreshes: rejected for security reasons.
- Force redirect to login on every refresh: rejected because it breaks expected navigation continuity.

## Decision 3: Treat VM console access as a dedicated application flow with explicit connection-state handling

**Decision**: Console access will be modeled as a dedicated frontend flow that first requests a temporary console authorization and then establishes the interactive session with explicit states for connecting, connected, retryable failure, and terminal failure.

**Rationale**:

- Console sessions are operationally sensitive and fail differently from normal data requests.
- Users need clear feedback and recovery options when the console path is unavailable.
- Separating console state from general VM detail state keeps the interaction easier to reason about and test.

**Alternatives considered**:

- Embedding console behavior directly into generic VM actions: rejected because it mixes interaction models and complicates recovery handling.
- Reusing legacy console pages during migration: rejected because the feature requires full cutover to the new application.

## Decision 4: Organize backend contracts by versioned capability domains under one API surface

**Decision**: The backend contract will be organized under a single versioned API surface with domain groupings for authentication, VMs, snapshots, console, search, health, and administration.

**Rationale**:

- Gives the frontend a single consistent integration surface.
- Supports future changes without rewriting the frontend routing model.
- Maps cleanly to the functional requirements and to the ownership boundaries already present in the backend.

**Alternatives considered**:

- Reusing mixed HTML and JSON endpoints: rejected because it preserves the hybrid architecture the rewrite is replacing.
- Separate admin and user API versions: rejected because it increases duplication without adding product value for this internal tool.

## Decision 5: Use a big-bang cutover while preserving the legacy frontend only as engineering reference

**Decision**: The migration will use a single cutover to the new application, while the legacy frontend remains available in the repository only for implementation reference until cleanup is complete.

**Rationale**:

- The approved design explicitly rejects long-lived coexistence because of routing and authentication complexity.
- Keeping the legacy frontend for engineering reference lowers migration risk without preserving dual runtime behavior.
- Simplifies ownership and reduces the chance of feature drift between old and new frontends.

**Alternatives considered**:

- Incremental page-by-page migration: rejected because mixed auth and routing models would increase complexity and operational risk.
- Immediate deletion of the legacy frontend: rejected because reference material is useful during implementation and verification.

## Decision 6: Preserve public operational health endpoints while limiting compatibility guarantees for replaced UI routes

**Decision**: Compatibility guarantees during cutover will explicitly preserve public health checks and backend operational expectations, while replaced UI routes are allowed to transition to the new application shell.

**Rationale**:

- Satisfies the constitution’s backward-compatibility principle within the approved migration scope.
- Protects existing operational probes and deployment health checks.
- Allows the frontend routing model to change without requiring long-term dual page support.

**Alternatives considered**:

- Preserve all legacy UI behavior indefinitely: rejected because it conflicts with the approved migration strategy.
- Remove legacy-compatible health behavior: rejected because it would create unnecessary operational risk.
