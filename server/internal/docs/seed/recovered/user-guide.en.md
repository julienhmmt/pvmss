# User guide

PVMSS (Proxmox Virtual Machine Self-Service) is a self-service portal that lets
you create, manage, and access the consoles of virtual machines hosted on
Proxmox VE, without using the Proxmox interface directly.

## Quick start

1. **Log in** on the [login page](/login): pick your cluster, then enter the credentials your administrator gave you.
2. **Find your VMs** on the My VMs page; search by name, VMID, or tag.
3. **Create a VM** with the "Create a VM" button, then fill in the required parameters.
4. **Open the console** once the VM is created and started, through the integrated noVNC or serial client.
5. **Manage your cloud-init files** from their own page.

## Home

The home page shows your VM counts (total, running, stopped), your quota
usage, and the tasks still running. Every long operation (create, delete,
snapshot…) appears in the task tray at the top so you can navigate away while
it runs.

## Creating a virtual machine

Open the wizard via "Create a VM" after signing in. **Simple** mode asks only
what is needed; **Detailed** mode exposes every option. Configure:

- **Source**: an **ISO** image, a Proxmox **template** to clone, or a **cloud image** to import — all from the administrator-approved lists. A template clone stays on the template's node; a cloud image requires the cloud-init fields.
- **Name and description**: a lowercase, hyphenated, unique name within your pool. A clear name (like `web-prod-01`) makes the list searchable and the activity log readable.
- **Cluster and node**: the node is chosen automatically (least loaded approved node with enough storage) unless you pick one in Detailed mode.
- **Profile (optional)**: if your administrator published hardware profiles, pick one to fill CPU, memory, and disk automatically.
- **Resources**: sockets, cores, memory, and disk size. Values are clamped by the cluster policy and your per-user quota.
- **Storage**: a storage approved by your administrator; the wizard checks free space live.
- **Network**: one or more network cards, each with a bridge and a card model (VirtIO, E1000, E1000E, RTL8139, VMXNet3). The Proxmox firewall is always enabled; your administrator may impose an isolation VLAN.
- **Firmware**: UEFI (default on), Secure Boot (default off — needed for Windows, breaks most Linux ISOs), TPM 2.0 for guests that require it.
- **Cloud-init document**: an administrator template or one of [your own files](/cloud-init). See the [cloud-init how-to](/docs/cloud-init-howto).
- **Startup**: choose whether the VM starts automatically after creation.
- **Tags**: pick from the administrator-curated list.

The **Review** step summarizes everything before you submit. Your draft is
saved in the browser, so an interrupted creation can be resumed. When you
reach your quota (max VMs) or a gabarit limit, the request is rejected before
any Proxmox call is made.

## Finding a virtual machine

**My VMs** lists your machines with VMID, name, cluster, node, tags, status,
and quick actions (console, details). Search, filters, and sort order are
kept in the URL so a view can be bookmarked or shared. Select several rows to
run a **bulk power action**; each VM reports its own success or error.

When PVMSS is connected to more than one Proxmox environment, use the
**cluster selector** to scope the list to one cluster or to all of them. A VM
is always identified by its `cluster` and its `VMID`, so the same VMID can
exist on different clusters without conflict.

## Managing a virtual machine

The VM details page is organized in tabs.

### Overview

- **Start**, **Shutdown** (graceful, guest agent / ACPI), **Reboot**, **Stop** (hard power off), **Reset**, **Pause**, **Resume**.
- **Console** — open the graphical console.
- **Boot from CD-ROM** once — restart on the mounted ISO for a single boot.
- **Rename** and edit the **description** (Markdown is rendered).
- **Delete** — permanently delete the VM (confirmation dialog).
- **Metrics** — CPU, memory, disk, and network history over the last hour, day, or week.

Prefer **Shutdown** over **Stop**. If shutdown does nothing, the QEMU guest
agent is probably missing inside the VM: install it, or use **Stop**.

### Disks

Add a disk on an approved storage, grow an existing disk, or detach one. Bus
slots are limited per VM.

### Network

Edit each network card: bridge, model, VLAN tag, and rate limit (Mbps).

### Hardware

Change sockets, cores, and memory within policy limits, set tags from the
curated picker, and load or eject an ISO in the CD-ROM drive.

### Cloud-init

Set the user, password (delivered through the guest agent, never stored), SSH
keys, IP address, gateway, and DNS. **Add key now** injects a key into a
running VM immediately. The VM's cloud-init document is shown here and can be
edited when your administrator allows it. See the
[cloud-init how-to](/docs/cloud-init-howto) for what applies when.

### Snapshots

- **Create**: enter a name (starts with a letter, then letters, digits, hyphens or underscores — 2 to 40 characters), an optional description, and choose whether to include RAM state.
- **View**: name, description, creation date, and whether RAM was included; the current state is marked.
- **Rollback**: restores the VM to the snapshot state. This is destructive — changes made after the snapshot are lost.
- **Delete**: permanently removes a snapshot and frees its storage.

Your administrator may set a maximum number of snapshots per VM. Snapshots
consume storage, so delete old ones when no longer needed.

### Activity

Every action performed on the VM through PVMSS — who, what, when.

## Consoles

The console page offers two clients:

- **noVNC** — the graphical display, with the same power actions as the details page.
- **Serial** — a text terminal (xterm.js) for guests with a serial port; you can enable a serial port on a VM that has none.

Both are relayed by PVMSS with a single-use ticket; no direct access to
Proxmox is needed.

## Cloud-init files

The [Cloud-init files](/cloud-init) page holds your own `#cloud-config`
documents (up to 20). They appear under "My files" in the Create a VM picker.
Each VM gets its own copy at creation, so editing a file later never changes
existing VMs.

## Limits

Your administrator controls most limits per cluster; PVMSS enforces them
server-side before any Proxmox call is made.

- **Quota** — maximum number of VMs per user.
- **Gabarit** — per-VM ceilings on sockets, cores, memory, disk size, network cards, and snapshots.
- **VM name** — a lowercase hostname, at most 63 characters, unique in your pool.
- **Description** — at most 512 characters.
- **Bulk power actions** — up to 100 VMs per request.
- **Cloud-init files** — up to 20 stored documents per user.
- **Snapshot name** — a leading letter, then letters, digits, hyphens or underscores, 2 to 40 characters; `current` is reserved.

## Best practices

- Use descriptive, hyphenated VM names.
- Prefer a cloud-init document over manual post-install setup.
- Start from a profile when one fits your workload.
- Keep snapshots for meaningful checkpoints only.

## Known limitations

- Only KVM/QEMU VMs are supported; LXC containers are not.
- Backups and live migration are handled in Proxmox, not in PVMSS.
- Advanced networking (firewall rules, SDN) is configured in Proxmox.
- Password change is available through the API only for now.
- Personal API tokens are deactivated in this version; the API tokens page has no backend.

## Security and privacy

- Console sessions are authenticated and session-based.
- Each user can view and manage only the VMs in their own pool; this is enforced server-side on every request.
- Administrator access is separate and requires additional authentication.

## Tips and tricks

- Use the search page for fast start/stop actions without opening details.
- Bookmark filtered VM lists and specific VM detail pages.
- The application follows your browser language preference (English or French).
