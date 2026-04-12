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
