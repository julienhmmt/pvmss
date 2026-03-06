# Data Model: Frontend SPA Rewrite

## Authenticated User Session

**Purpose**: Represents the current authenticated application state for a user interacting with protected routes.

**Fields**:

- `username`: Visible account identifier.
- `isAdmin`: Authorization flag for administrator-only routes and capabilities.
- `pool`: Optional pool scope associated with the user.
- `accessState`: Current access state for protected API calls.
- `renewalState`: Whether the session can attempt renewal.
- `sessionStatus`: One of `anonymous`, `restoring`, `authenticated`, `expired`, `signed_out`.

**Validation rules**:

- `username` is required when `sessionStatus` is `authenticated`.
- `isAdmin` is required when `sessionStatus` is `authenticated`.
- `renewalState` must be `unavailable` after sign-out.

**State transitions**:

- `anonymous` -> `restoring` when the application boots on a protected route.
- `restoring` -> `authenticated` when identity restoration succeeds.
- `restoring` -> `anonymous` when restoration is not possible.
- `authenticated` -> `expired` when protected access can no longer be renewed.
- `authenticated` -> `signed_out` when the user explicitly signs out.

## User Profile

**Purpose**: Represents the account information and self-service capabilities available to the authenticated user.

**Fields**:

- `username`: Account identifier.
- `isAdmin`: Administrator status.
- `pool`: Optional assigned pool.
- `vmCount`: Optional count of visible VMs.
- `passwordChangeAllowed`: Whether password update is permitted for the current user.

**Validation rules**:

- `username` is required.
- `vmCount`, when present, cannot be negative.

**Relationships**:

- One user profile can be associated with zero or many visible virtual machines.

## Virtual Machine

**Purpose**: Represents a VM available in the application for summary and detailed views.

**Fields**:

- `vmid`: Stable VM identifier.
- `name`: Human-readable machine name.
- `node`: Hosting node identifier.
- `status`: Current lifecycle state.
- `cpuUsage`: Current CPU usage summary.
- `cpuCapacity`: Allocated CPU summary.
- `memoryUsage`: Current memory usage summary.
- `memoryCapacity`: Allocated memory summary.
- `diskUsage`: Current disk usage summary.
- `diskCapacity`: Allocated disk summary.
- `uptime`: Current runtime duration summary.
- `tags`: User-visible tag collection.
- `description`: User-visible description.
- `pool`: Ownership or grouping scope.
- `disks`: Attached disk collection.
- `networkCards`: Attached network interface collection.
- `snapshots`: Related snapshot collection when included in detail views.
- `availableActions`: Supported lifecycle and management actions for the current user and state.

**Validation rules**:

- `vmid`, `name`, `node`, and `status` are required.
- `tags` defaults to an empty collection.
- `availableActions` must be filtered by authorization and VM state.

**State transitions**:

- Lifecycle actions move the VM through `running`, `stopped`, `paused`, and transitional pending states.
- Detail refreshes may update metric fields independently of lifecycle state.

## VM Disk

**Purpose**: Represents a storage device attached to a VM.

**Fields**:

- `deviceName`: User-visible device identifier.
- `label`: Display label.
- `capacity`: Allocated size summary.
- `usage`: Current usage summary when available.
- `storageTarget`: Backing storage location.
- `isBootable`: Whether the disk participates in boot behavior.

**Validation rules**:

- `deviceName` and `capacity` are required.

**Relationships**:

- Many VM disks belong to one virtual machine.

## VM Network Card

**Purpose**: Represents a VM network interface and its operational attributes.

**Fields**:

- `interfaceName`: Interface identifier.
- `bridge`: Connected bridge or equivalent network target.
- `model`: Network adapter model label.
- `macAddress`: Optional hardware address display.
- `enabled`: Whether the interface is active.
- `ipAddresses`: Optional discovered IP addresses.

**Validation rules**:

- `interfaceName` and `bridge` are required for configured interfaces.
- `enabled` defaults to `true` for active interfaces.

**Relationships**:

- Many network cards belong to one virtual machine.

## VM Creation Request

**Purpose**: Represents all user-provided inputs needed to request a new VM.

**Fields**:

- `name`: VM name.
- `vmid`: Optional explicit identifier.
- `node`: Target node.
- `pool`: Target pool.
- `cores`: Requested CPU cores.
- `sockets`: Requested CPU sockets.
- `memory`: Requested memory allocation.
- `disks`: Requested disk definitions.
- `networks`: Requested network definitions.
- `iso`: Optional installation image selection.
- `cloudInit`: Optional initialization settings.
- `startOnCreate`: Whether the VM should start after creation.
- `efiEnabled`: Whether EFI boot is requested.
- `tpmEnabled`: Whether TPM is requested.

**Validation rules**:

- `name`, `node`, `pool`, `cores`, `sockets`, and `memory` are required for submission.
- `disks` must contain at least one valid disk definition.
- `networks` must contain at least one configured network definition when networking is required by policy.
- Optional values must preserve user input when validation fails.

## Snapshot

**Purpose**: Represents a recoverable VM checkpoint.

**Fields**:

- `name`: Snapshot identifier.
- `description`: Optional user-visible description.
- `createdAt`: Creation timestamp.
- `sourceState`: VM state at snapshot time when known.
- `canRollback`: Whether rollback is currently permitted.
- `canDelete`: Whether deletion is currently permitted.

**Validation rules**:

- `name` is required.
- Snapshot actions must be filtered by authorization and VM policy.

**Relationships**:

- Many snapshots belong to one virtual machine.

## Console Session

**Purpose**: Represents a temporary request-to-connect flow for an interactive VM console.

**Fields**:

- `vmid`: Related VM identifier.
- `connectionStatus`: One of `idle`, `requesting`, `connecting`, `connected`, `retryable_failure`, `terminal_failure`.
- `requestIssuedAt`: Time the console request was created.
- `expiresAt`: Expiry point for the temporary console authorization.
- `failureReason`: Optional user-safe failure explanation.

**Validation rules**:

- `vmid` is required.
- `expiresAt` must be later than `requestIssuedAt` when present.
- `failureReason` must not expose sensitive internal details.

## Admin Resource

**Purpose**: Represents a managed administrative domain object shown in the admin area.

**Subtypes**:

- `Node`
- `Storage`
- `Pool`
- `Tag`
- `Limits`
- `VMBR`
- `CloudInitTemplate`
- `ISO`
- `Settings`
- `AppInfo`

**Shared fields**:

- `resourceType`: Administrative domain classification.
- `displayName`: Primary label.
- `status`: Optional operational or availability state.
- `lastUpdatedAt`: Optional last refresh or mutation timestamp.

**Validation rules**:

- `resourceType` and `displayName` are required.
- Mutating actions require administrator authorization.
