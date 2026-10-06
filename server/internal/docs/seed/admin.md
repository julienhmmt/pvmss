# Admin guide

This page is visible to administrators only. It summarizes the admin surface;
the full walkthrough is the [Administrator guide](/docs/admin-guide).

## Infrastructure

**Clusters** connects one or more Proxmox environments (URL, API token,
connection test) and sets the per-cluster cloud-init write target. **Nodes**
approves the hosts users may create VMs on. **Pools** creates self-service
users (Proxmox user + pool + ACL in one step).

### Cloud-init templates

Cloud-init documents are written only by administrators (**Admin >
Cloud-init**). The Proxmox REST API cannot write snippets, so PVMSS never
writes on the nodes: for each template it shows a command that the
administrator pastes, as root, on the nodes that must offer it. PVMSS reads
through the API which nodes have the file, and offers the template only on
those nodes.

1. In Proxmox, enable the **Snippets** content type on a storage (`local`
   works).
2. In **Infrastructure > Clusters > Edit**, select that snippet storage.
3. Write the template, copy its command, paste it on the chosen nodes, click
   **Verify**.

Leave the storage empty to disable cloud-init templates on the cluster. See
the [cloud-init setup guide](/docs/cloud-init-setup) for details and
troubleshooting.

## Catalog

The **Catalog** section lets you approve or hide the storages, ISOs, cloud
images, VM templates, and bridges that VM creation may reference, and manage
hardware profiles, tags, and cloud-init templates. Discovered resources appear
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
