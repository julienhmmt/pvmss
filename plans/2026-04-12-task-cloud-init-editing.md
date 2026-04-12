# Feature Plan: Cloud-Init Editing from Detail Page

## Goal
Allow post-creation editing of Cloud-Init configurations (Username, Password, Network, SSH keys) directly from the VM details page.

## Current State
Cloud-Init can be configured during VM creation, but once the VM is created, changes to the Cloud-Init configuration must be done via the Proxmox UI.

## Backend Implementation

1. **Read Configuration**:
   - Ensure the `GET /api/vms/{vmid}/config` endpoint exposes Cloud-Init specific fields:
     - `ciuser` (Username)
     - `cipassword` (usually hidden/hashed, might not be readable)
     - `sshkeys` (Public SSH keys)
     - `ipconfig0`, `ipconfig1`, etc. (Network settings)
     - `nameserver`, `searchdomain` (DNS settings)

2. **Update Endpoint**:
   - **Route**: `PUT /api/vms/{vmid}/cloudinit`
   - **Action**: Validates the payload and sends a `PUT /nodes/{node}/qemu/{vmid}/config` request with the updated Cloud-Init fields.

3. **Regenerate Cloud-Init Drive**:
   - In Proxmox, changing the config doesn't apply the changes until the Cloud-Init drive is regenerated.
   - The backend should ideally trigger this regeneration automatically after a successful update, or provide an endpoint to do so. (Usually, a clean shutdown and start triggers it, or specific Proxmox API commands).

## Frontend Implementation

1. **Cloud-Init Tab/Section**:
   - Add a dedicated UI panel or tab in the VM Details page for "Cloud-Init".

2. **Form Interface**:
   - **User Settings**: Inputs for Username and a new Password (leave blank to keep current).
   - **SSH Keys**: A large textarea for pasting public SSH keys.
   - **Network Settings**:
     - Toggle between DHCP and Static IP.
     - If Static: Inputs for IP/CIDR (e.g., `192.168.1.50/24`) and Gateway.
   - **DNS Settings**: Inputs for DNS servers and search domains.

3. **Apply Actions**:
   - A "Save Changes" button.
   - Provide clear UI feedback that changes require a VM reboot to take effect within the guest OS.

## Challenges & Considerations
- **Password Readability**: Proxmox does not return the plaintext password in the config. The UI should reflect that the password is "Set" but cannot be viewed. Updating it overwrites the old one.
- **Multiple Network Interfaces**: A VM might have multiple network interfaces (`ipconfig0`, `ipconfig1`). The UI and backend need to handle parsing and updating specific interfaces.
- **OS Support**: Ensure users know Cloud-Init only works on VMs provisioned from Cloud-Init compatible templates.
