# Admin guide

This page is visible to administrators only. It summarizes the admin surface;
the full walkthrough is the [Administrator guide](/docs/admin-guide).

## Infrastructure

**Clusters** connects one or more Proxmox environments (URL, API token,
connection test). Each cluster has an immutable id (chosen at creation; `default` for the first one) and an editable label; the id cannot be renamed. **Nodes**
approves the hosts users may create VMs on. **PVMSS users** creates self-service
users (Proxmox user + pool + ACL in one step).

### VM templates carry the sign-in

PVMSS adds no SSH key and no password to the VMs it creates (cloud-init is
paused). A clone keeps exactly what its Proxmox template carries, so build
every approved template with the sign-in users will need:

- in the template's **Cloud-Init** tab in Proxmox, set the user and either an
  SSH public key or a password, or bake an account into the image itself;
- install and enable `qemu-guest-agent`, so the VM reports its address and
  the **Connect** tab can show the SSH command.

Every VM cloned from one template shares that sign-in. Hand it to your users,
or keep one template per team. ISO installs are unaffected: users create
their own account during the installation, from the browser console.

## Resources and provisioning

The **Resources** group lets you approve or hide the storages, ISOs, VM
templates, and bridges that VM creation may reference. The
**Provisioning** group manages VM profiles and tags. Discovered resources appear
automatically; toggle the enabled switch to control what users see. When a
resource disappears from Proxmox, its stale approval is flagged so you can
remove it.

## Migrating a VM

On **Infrastructure > Nodes**, open a node: every PVMSS-managed VM has a
**Migrate...** button. The dialog checks where the VM can go, you pick a
target node, review the summary and confirm. The migration runs as a Proxmox
task, followed in the task tray (you can close the dialog meanwhile), and the
VM leaves the source node's list when it completes. A running VM migrates
live, a stopped one offline.

Limits:

- Administrators only, one VM at a time, within one cluster.
- Only PVMSS-managed VMs (tagged `pvmss`).
- Targets are approved, online nodes that Proxmox accepts for that VM.
- There is no target-storage picker: local disks are carried by Proxmox
  (`with-local-disks`) when the VM has any.
- The per-node capacity caps are warnings, not blocks: a target above its cap
  shows a badge and stays selectable.
- A locked VM, or a running VM with local resources (PCI or USB passthrough),
  is refused; stop the VM or wait for the lock to clear.
- HA-managed VMs and cross-cluster migration are left to Proxmox.

The migration is recorded in the audit log (`vm.migrate`, with the source and
target node). The Proxmox service token needs `VM.Migrate` (see the
[Proxmox permissions](/docs/proxmox-permissions)).

## Policy

**Limits** sets the per-cluster gabarit (max sockets, cores, memory, disk per
VM, network cards, snapshots, isolation VLAN) and the
per-user quota (max VMs). **Node capacity** caps how much of each node PVMSS
may allocate. Both are enforced server-side before any Proxmox call.

## Documentation

This page itself is managed under **Documentation** in the admin nav. Admins
can author, edit, toggle, and delete Markdown pages in English and French.
Built-in pages (like this one) are marked **system** and cannot be deleted,
but their content may be edited. Each page has an audience of `user` (public)
or `admin` (admin-only).

## System

The **App Info** page shows the running version, environment, and cluster
status. **Settings** holds the audit log (with retention and prune preview)
and the database export / two-phase import.
