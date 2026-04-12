# Feature Plan: Disk Resize, Add, and Remove from Detail Page

## Goal
Allow users and administrators to manage attached storage disks natively within the VM details view, without needing to use the Proxmox UI directly.

## Current State
Disks are displayed in the VM details view, but cannot be modified, added, or removed through the PVMSS interface.

## Backend Implementation

### 1. Add Disk Endpoint
- **Route**: `POST /api/vms/{vmid}/disks`
- **Action**: Maps to `PUT /nodes/{node}/qemu/{vmid}/config`.
- **Payload**: Requires storage ID, size (in GB), and bus type (virtio, scsi, sata, ide).
- **Logic**: Determine the next available disk slot for the chosen bus (e.g., if `scsi0` and `scsi1` exist, use `scsi2`). Send the parameter `scsi2=local-lvm:32` to Proxmox.

### 2. Resize Disk Endpoint
- **Route**: `PUT /api/vms/{vmid}/disks/{diskId}/resize`
- **Action**: Maps to `PUT /nodes/{node}/qemu/{vmid}/resize`.
- **Payload**: Requires `disk` (e.g., `scsi0`) and `size` (e.g., `+10G`).
- **Validation**: Ensure the requested size is an increase. Proxmox does not support shrinking disks via the API safely.

### 3. Remove/Detach Disk Endpoint
- **Route**: `DELETE /api/vms/{vmid}/disks/{diskId}`
- **Action**: Maps to `PUT /nodes/{node}/qemu/{vmid}/config`.
- **Payload**: Requires `delete={diskId}`.
- **Logic**: Note that deleting from config usually detaches the disk (makes it unused) rather than securely wiping the underlying storage volume immediately, depending on Proxmox settings. May need a flag to explicitly purge the underlying volume if desired.

## Frontend Implementation

1. **Disks UI Update**:
   - Update the Disks section/card in the VM Details page to include action buttons (Resize, Detach/Delete) per disk.
   - Add a global "Add Disk" button in the Disks section header.

2. **Add Disk Modal**:
   - Form fields: Storage selector (fetch available storages for the node), Bus Type selector, and Size input.
   - Validation to ensure valid storage and size.

3. **Resize Disk Modal**:
   - Input for additional size (e.g., "Add X GB").
   - Display current size and new calculated total size.
   - Strict validation to prevent negative values.

4. **Remove Disk Confirmation**:
   - A dangerous action modal confirming the removal of the disk.
   - Explain the difference between detaching vs destroying data (depending on implemented backend behavior).

## Challenges & Considerations
- **Disk Numbering**: The backend must accurately parse existing disks to find the next available ID for a new disk to prevent overwriting.
- **Running VMs**: Adding and resizing disks often works on running VMs, but removing might require the VM to be stopped depending on the bus type and OS. Handle errors gracefully.
