# Feature Plan: Snapshot `vmstate` Checkbox in UI

## Goal
Allow users to capture the RAM state of a VM when taking a snapshot, providing true point-in-time restoration capabilities including running applications.

## Current State
The application allows taking snapshots, but does not currently expose the option to include the VM's RAM state (`vmstate`).

## Backend Implementation

1. **Update Snapshot Creation Handler**:
   - In `backend/handlers/vm_snapshots.go`, modify the create snapshot API endpoint to accept an optional `vmstate` boolean parameter in its payload.

2. **Proxmox API Integration**:
   - When calling `POST /nodes/{node}/qemu/{vmid}/snapshot`, include the `vmstate=1` parameter if the user requested it.
   - Example parameter map: `params["vmstate"] = "1"`

3. **Validation & Pre-checks**:
   - Ensure the VM is currently running. Taking a RAM state snapshot of a stopped VM is invalid.
   - *Optional but recommended*: Check if the target storage supports `vmstate`. Typically, this requires `qcow2` files or specific block storage configurations (like Ceph/RBD or ZFS). If the storage does not support it, the Proxmox API will return an error, which the backend must handle gracefully and pass to the frontend.

## Frontend Implementation

1. **Snapshot Creation Modal Update**:
   - Add a checkbox to the snapshot creation form: `[ ] Include RAM state`.

2. **UI Logic & Feedback**:
   - Disable the checkbox if the VM is not in a `running` state, with a tooltip explaining why.
   - Add a warning tooltip or helper text near the checkbox: *"Saving RAM state will momentarily pause the VM and consume additional disk space equal to the VM's allocated memory."*

3. **Error Handling**:
   - If the backend returns a storage compatibility error, display a clear message to the user (e.g., "The underlying storage does not support saving RAM state. Try again without this option.").

## Challenges & Considerations

- **Performance Impact**: Taking a `vmstate` snapshot takes significantly longer than a standard disk snapshot, especially for VMs with large amounts of RAM. The UI must handle longer response times gracefully (refer to the Async Task Tracking plan).
- **Disk Space**: A `vmstate` snapshot requires space equivalent to the VM's RAM size. Ensure users are aware of this to prevent storage exhaustion.

## Implementation Status

**Completed:**

- ✅ Backend API accepts `vmstate` boolean parameter in `CreateSnapshot` handler (backend/api/v1/vm_details.go:536-540)
- ✅ Proxmox API integration includes `vmstate=1` when requested (backend/proxmox/resty_snapshots.go:37-39)
- ✅ Frontend API function accepts `vmstate` parameter (frontend/src/lib/api/snapshots.ts:59)

**Missing:**

- ❌ Backend validation: No check if VM is running before allowing vmstate
- ❌ Backend validation: No storage compatibility check
- ❌ Frontend UI: No checkbox for "Include RAM state" in snapshot creation form
- ❌ Frontend UI: No disable logic when VM not running
- ❌ Frontend UI: No warning tooltip about performance/disk space impact
- ❌ Frontend UI: No error handling for storage compatibility errors

## Remaining Tasks

### Backend Tasks

1. **Add VM Running Validation** (backend/api/v1/vm_details.go)

   - In `CreateSnapshot` handler, check VM status before allowing vmstate
   - If `req.Vmstate` is true, fetch current VM status using `GetVMCurrentResty`
   - Return error if VM is not running: "RAM state snapshots can only be created while the VM is running"

2. **Add Storage Compatibility Validation** (backend/api/v1/vm_details.go)

   - Get VM storage configuration from `GetVMConfigResty`
   - Check storage type (qcow2, ZFS, Ceph/RBD support vmstate; dir, LVM may not)
   - Return clear error if storage incompatible: "The underlying storage does not support saving RAM state. Try again without this option."

### Frontend Tasks

3. **Add vmstate Checkbox to Form** (frontend/src/routes/(app)/vm/[id]/_tabs/TabSnapshots.svelte)

   - Add checkbox input below description field: `[ ] Include RAM state`
   - Bind to new state variable `snapVmstate`
   - Pass `snapVmstate` to parent component via callback
   - Update Props interface to include `snapVmstate` and `onSnapVmstateChange`

4. **Add VM Status-Based Disable Logic** (frontend/src/routes/(app)/vm/[id]/_tabs/TabSnapshots.svelte)

   - Add `vmStatus` to Props interface
   - Disable vmstate checkbox if `vmStatus !== 'running'`
   - Add tooltip: "RAM state can only be saved while the VM is running"

5. **Add Warning Tooltip** (frontend/src/routes/(app)/vm/[id]/_tabs/TabSnapshots.svelte)

   - Add helper text or info icon next to checkbox
   - Display warning: "Saving RAM state will momentarily pause the VM and consume additional disk space equal to the VM's allocated memory."

6. **Update Parent Component** (frontend/src/routes/(app)/vm/[id]/+page.svelte)

   - Add `snapVmstate` state variable (default: false)
   - Add `handleSnapVmstateChange` callback
   - Pass `snapVmstate` and `handleSnapVmstateChange` to TabSnapshots
   - Pass `vmStatus` to TabSnapshots
   - Update `createSnapshot` call to include `vmstate` parameter

7. **Add Error Handling** (frontend/src/routes/(app)/vm/[id]/_tabs/TabSnapshots.svelte)

   - Catch storage compatibility errors from backend
   - Display clear error message to user
   - Consider adding i18n translations for error messages

### i18n Tasks

8. **Add Translation Keys** (backend/i18n/active.en.toml & active.fr.toml)

   - `VM.Snapshot.IncludeRamState` - "Include RAM state"
   - `VM.Snapshot.RamStateHelp` - "Saving RAM state will momentarily pause the VM and consume additional disk space equal to the VM's allocated memory."
   - `VM.Snapshot.RamStateDisabled` - "RAM state can only be saved while the VM is running"
   - `VM.Snapshot.StorageNotSupported` - "The underlying storage does not support saving RAM state. Try again without this option."
   - `VM.Snapshot.VMNotRunning` - "RAM state snapshots can only be created while the VM is running"
