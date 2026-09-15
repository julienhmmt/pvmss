# Admin guide

This page is visible to administrators only. It summarizes the admin surface;
the full walkthrough is the [Administrator guide](/docs/admin-guide).

## Infrastructure

**Clusters** connects one or more Proxmox environments (URL, API token,
connection test) and sets the per-cluster cloud-init write target. **Nodes**
approves the hosts users may create VMs on. **Pools** creates self-service
users (Proxmox user + pool + ACL in one step).

### Cloud-init write target (snippet storage)

PVMSS writes the cloud-init vendor-data for each VM as a snippet file on the
cluster, then attaches it via `cicustom`. The Proxmox REST API cannot write
snippets, so PVMSS writes directly to a directory that is bind-mounted into
its container. To enable cloud-init documents and the generated baseline
(qemu-guest-agent) on a cluster:

> **No SSH, no API upload.** PVMSS does not SSH to Proxmox and the Proxmox
> REST API cannot write snippet files. PVMSS writes the file to a directory
> inside its own process, and that directory must be the *same physical
> directory* Proxmox reads snippets from - made visible by a bind mount, NFS,
> or running PVMSS on the Proxmox host. If the file appears under your
> configured snippet directory but the VM cannot start, the directory is not
> the one Proxmox serves snippets from. Verify on the Proxmox host with
> `pvesm path <storage>` - the snippets path is that path plus `/snippets`.

1. Pick a Proxmox storage that has the **snippets** content type enabled. To
   enable it: **Datacenter → Storage → <storage> → Content**, tick
   `snippets`. The storage must be visible to every node that will host
   image-born VMs - a shared storage (NFS, CephFS, `dir` on a shared mount)
   is the usual choice.
2. In **Admin → Clusters → Edit**, the **Snippet storage** field lists every
   snippet-capable storage PVMSS can see on the cluster. Select one. If the
   list is empty, no storage on the cluster advertises the snippets content
   type yet - enable it in Proxmox and reopen the form.
3. Mount that storage's `snippets/` directory into the PVMSS container at a
   known path and enter that path as **Snippet directory**. The path must be
   the same directory Proxmox reads from for that storage. For a `dir`
   storage with `path /var/lib/vz`, the snippets directory is
   `/var/lib/vz/snippets`; for NFS or CephFS, mount the share and use its
   `snippets/` subdirectory.
4. Save and run **Test**. The cluster row should now show `cloud-init: on`.

Leave both fields empty to disable cloud-init documents on the cluster. VMs
created from cloud images will then report `Baseline cloud-init not
delivered` and the qemu-guest-agent baseline will not be attached.

### Alternative: SSH snippet delivery

When PVMSS cannot share a filesystem with Proxmox (different hosts, no NFS,
no bind mount), it can deliver snippet files over SSH instead. PVMSS
resolves each Proxmox node's IP via `/cluster/status` and SSHes to the
specific node where the VM is created, so no per-cluster SSH config is
needed - just one global key.

Set these environment variables on the PVMSS server:

| Variable             | Notes                                                       |
| -------------------- | ----------------------------------------------------------- |
| `PVMSS_SSH_USER`     | SSH user on the Proxmox hosts (e.g. `root`)                 |
| `PVMSS_SSH_KEY_FILE` | Path to the private key file (required when user is set)    |
| `PVMSS_SSH_PORT`     | SSH port (default 22)                                       |

The key must be authorized on every Proxmox node that will host cloud-init
VMs (add it to `~/.ssh/authorized_keys` for the SSH user). The **Snippet
directory** configured per cluster is then the path on the Proxmox host, not
a local mount. Host-key verification is not enforced; use this on a trusted
management network.

**Multi-node clusters:** SSH writes the snippet to the specific node where
the VM is created. For a cluster, you should still use a shared snippet
storage (NFS, CephFS) so the snippet is visible to all nodes - otherwise a
migrated VM would point at a file the new node cannot see. SSH delivery is
primarily for single-node Proxmox or clusters that already have shared
storage but no shared filesystem mount into the PVMSS container.

When `PVMSS_SSH_USER` is empty (the default), SSH delivery is off and the
shared-filesystem approach above is used.

## Catalog

The **Catalog** section lets you approve or hide the storages, ISOs, cloud
images, VM templates, and bridges that VM creation may reference, and manage
hardware profiles, tags, and cloud-init templates. Discovered resources appear
automatically; toggle the enabled switch to control what users see. When a
resource disappears from Proxmox, its stale approval is flagged so you can
remove it.

## Policy

**Limits** sets the per-cluster gabarit (max sockets, cores, memory, disk per
VM, network cards, snapshots, custom cloud-init YAML, isolation VLAN) and the
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
