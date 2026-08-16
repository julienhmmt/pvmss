# User guide

PVMSS (Proxmox Virtual Machine Self-Service) is a self-service portal that lets
you create, manage, and access the consoles of virtual machines hosted on a
Proxmox Virtual Environment server, without using the Proxmox interface
directly.

## Quick start

1. **Log in** on the [login page](/login) with the credentials your administrator gave you.
2. **Find your VMs** from the My VMs page; search by name or VMID.
3. **Create a VM** with the "Create a VM" button, then fill in the required parameters.
4. **Open the console** once the VM is created and started, through the integrated noVNC client.
5. **Manage your profile** to see your VMs and change your password.

## Creating a virtual machine

Open the configuration form via "Create a VM" after signing in. Configure:

- **Node**: the Proxmox node where the VM will be created, among the nodes your administrator approved. A node may be disabled if it has reached the configured limits.
- **Name and description**: a lowercase, hyphenated, unique name within your pool. A clear name (like `web-prod-01`) makes the list searchable and the audit log readable.
- **Operating system**: an ISO image from the administrator-approved list.
- **Profile (optional)**: if your administrator published hardware profiles, pick one to fill CPU, memory, disk, and bus automatically.
- **Resources**: CPU (sockets and cores), memory (MB or GB), and disks. Values are clamped by the cluster policy and your per-user quota.
- **Storage**: a storage approved by your administrator.
- **Network**: one or more network cards. For each card you can choose the bridge (VMBR), the card model (VirtIO, E1000, E1000E, RTL8139, VMXNet3), an optional MAC address, an optional VLAN tag (1-4096), and an optional network speed (MB/s).
- **Firmware & security**: EFI boot (UEFI) and optional TPM v2.0 for guests that require it (for example Windows 11).
- **Cloud-init**: pick an administrator-curated template, or leave it for later.
- **Startup**: choose whether the VM starts automatically after creation.
- **Tags**: add predefined tags to organize your VMs.

You can create one VM at a time. When you reach your quota (max VMs, CPU, memory, or disk) the request is rejected before any Proxmox call is made.

## Finding a virtual machine

Use the search to locate a VM by name, VMID, or tag. Results show the VMID, name, host node, tags (except the internal `pvmss` tag), status, and a button to open the details.

When PVMSS is connected to more than one Proxmox environment, use the **cluster selector** at the top of the My VMs page to scope the list to one cluster or to all of them. A VM is always identified by its `cluster` and its `VMID`, so the same VMID can exist on different clusters without conflict. The detail and console URLs include the cluster, so bookmarks stay valid per cluster.

## Managing a virtual machine

The VM details page gives you full control:

- **Start** — power on the VM.
- **Console** — open the integrated noVNC console in a new window.
- **Restart** — reboot the VM.
- **Shutdown** — graceful ACPI shutdown.
- **Stop** — force stop (immediate power off).
- **Reset** — force a reset.
- **Refresh** — refresh the VM information (invalidate the cache).
- **Delete** — permanently delete the VM (requires confirmation).

Prefer **Shutdown** (graceful) over **Stop** (forced). If you see repeated messages about the QEMU guest agent being unavailable, install or enable the agent inside the VM, or use **Stop**.

### Editing resources

While a VM is **stopped**, you can edit some of its resources from the details page:

- CPU (sockets and cores), within policy limits.
- Memory (MB/GB), within policy limits.
- Network cards (bridge, model, optional MAC).
- Cloud-init snippet (custom `#cloud-config`).
- CD-ROM / ISO (load or eject an ISO).

Disk size growth and other structural changes beyond what the policy allows must be done in Proxmox.

## Cloud-init

Cloud-init configures a VM on first boot without logging in: users, SSH keys, packages, and more.

- On **Create a VM**, pick an administrator-curated template from the cloud-init dropdown; its content is applied verbatim.
- After creation, open the VM's **Cloud-init** tab to view or edit the snippet. The editor accepts any valid `#cloud-config` document. Changes are pushed to the cluster and take effect on the next boot.

Supported fields include `packages`, `users`, `write_files`, and `runcmd`. See the upstream [cloud-init docs](https://cloudinit.readthedocs.io/) for the full schema.

## Snapshots

Snapshots save the complete state of a VM at a moment in time and restore it later.

- **Create**: open the VM details page, go to the snapshots section, enter a name (alphanumeric, hyphens, underscores, max 40 characters) and an optional description, optionally include RAM state, then click Create.
- **View**: the list shows name, description, creation date, and state (with RAM or disk only). The current state is marked with a star.
- **Edit description**: use the pencil button on a snapshot row.
- **Rollback**: restores the VM to the snapshot state. This is destructive — changes made after the snapshot are lost.
- **Delete**: permanently removes a snapshot and frees its storage.

Your administrator may set a maximum number of snapshots per VM. Snapshots consume storage, so delete old ones when no longer needed.

## Profile and password

Your **Profile** page summarizes your VMs (total, running, stopped) and provides a secure form to change your password. The **API tokens** page lets you create personal access tokens for scripting, if your administrator enabled them.

## Best practices

- Use descriptive, hyphenated VM names.
- Prefer a cloud-init template over manual post-install setup.
- Start from a profile when one fits your workload.
- Keep snapshots for meaningful checkpoints only.

## Known limitations

- Resource reconfiguration is limited to CPU, memory, network cards, and ISO for a stopped VM. Growing disks or changing disk count beyond policy limits requires Proxmox.
- Only KVM/QEMU VMs are supported; LXC containers are not.
- Backups and live migration are handled in Proxmox, not in PVMSS.
- Advanced networking (firewall rules, SDN) is configured in Proxmox.

## Security and privacy

- Console sessions are authenticated and session-based.
- Each user can view and manage only the VMs in their own pool.
- Administrator access is separate and requires additional authentication.

## Tips and tricks

- Use the search page for fast start/stop actions without opening details.
- Open multiple console windows to manage several VMs at once.
- Bookmark the portal URL and specific VM detail pages.
- The application follows your browser language preference (English or French).
