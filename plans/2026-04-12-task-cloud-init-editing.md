# Feature Plan: Cloud-Init Editing from Detail Page

> **Status (implemented 2026-06-28): DONE.**
> `PUT /api/v1/vms/:id/cloudinit` is registered in `backend/api/v1/router.go` and handled by `VMDetailsHandler.UpdateVMCloudInit` in `backend/api/v1/vm_details.go`. All fields are validated server-side in `backend/api/v1/validation.go` (user regex, password length, OpenSSH key prefix, ipconfigN token/IP/CIDR, nameserver IPs, searchdomain labels). Pool-membership AuthZ is enforced via `ownsVM()` (same pattern as the other VM detail mutators). The Proxmox call uses the new `proxmox.SetVMCloudInitConfigResty` in `backend/proxmox/cloudinit.go`, which distinguishes "skip" (nil) from "clear" (non-nil empty → Proxmox `delete=` param) and reuses the SSH-key URL encoding rules. The frontend `TabCloudInit.svelte` is now an editable form (user, password, ssh keys, DHCP/static IP, gateway, nameserver, searchdomain) with a reboot notice; `+page.svelte` passes `onSaved={load}` so config refreshes after a save. EN+FR i18n keys live under `vm.cloudInit.*`. Backend tests in `backend/api/v1/vm_cloudinit_test.go` cover the validators (table-driven) and handler validation rejection (bad user/ssh/ip/nameserver/searchdomain/password, empty update, invalid JSON, valid input reaching the offline gate, clearing fields). The SFTP filename boundary check (S6) is a separate, still-open item in `2026-05-22-backend-security-hardening.md` and is not exercised by this edit endpoint (it only touches `/nodes/{node}/qemu/{vmid}/config`, not SFTP).

> **Custom cloud-config YAML / per-VM template copy (2026-06-28): DONE.**
> Templates created by admins are never edited by users: at VM creation the template YAML is **copied** to a per-VM snippet `pvmss-<vmid>.yml` (`backend/api/v1/vm_create_cloudinit.go`) and the VM's `cicustom` points at that copy. The custom-YAML editor lives in `TabCloudInit.svelte` and is backed by `GET|PUT /api/v1/vms/:id/cloudinit/snippet` (`backend/api/v1/vm_details_cloudinit_snippet.go`). Editing writes only the VM's own snippet file, leaving the admin template untouched.
> Saving snippets requires SFTP (the Proxmox HTTP API cannot reliably read/write snippet files). When SFTP is disabled the editor falls back to a **read-only** view of the effective cloud-config via the Proxmox `cloudinit/dump?type=user` endpoint (`proxmox.GetVMCloudInitDumpResty`); the response carries an `editable` flag and the UI shows a read-only textarea with the `vm.cloudInit.customYamlReadOnly` notice. The snippet load runs in an `$effect` that tracks only `vmid` + SFTP availability and calls the loader inside `untrack()` to avoid a self-retriggering request loop.

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
