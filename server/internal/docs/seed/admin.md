# Admin guide

This page is visible to administrators only. It summarizes the admin surface;
the full walkthrough is the [Administrator guide](/docs/admin-guide).

## Infrastructure

**Clusters** connects one or more Proxmox environments (URL, API token,
connection test) and sets the per-cluster cloud-init write target. **Nodes**
approves the hosts users may create VMs on. **Pools** creates self-service
users (Proxmox user + pool + ACL in one step).

### Cloud-init publishing (SSH)

Cloud-init documents are written only by administrators (**Admin > Cloud-init
templates**). PVMSS publishes each one, merged with the qemu-guest-agent
baseline, as an immutable `pvmss-tpl-<id>-<hash>.yml` file into the snippet
storage of every node, over SSH (the Proxmox REST API cannot write snippets).
Creating a VM never writes a file: the VM points at the published file.

1. In Proxmox, enable the **Snippets** content type on a storage available
   on every node (`local` works).
2. Generate a key pair and give PVMSS the private key:

   | Variable             | Notes                                                    |
   | -------------------- | -------------------------------------------------------- |
   | `PVMSS_SSH_KEY_FILE` | Path to the private key (read-only mount or Secret)      |

3. On every node, as root, run `tools/pvmss-node-setup.sh --storage <storage>
   --key '<PVMSS public key>'`. It installs the `pvmss-snippet` helper, a
   dedicated `pvmss` user limited to the storage's `snippets/` directory, and
   the key with a forced command (no shell).
4. In **Admin > Clusters > Edit**, set the snippet storage, the SSH user and
   port, and the pinned host keys: paste `ssh-keyscan -t ed25519 <node ip>`
   lines, or save without the SSH user first, reopen and click **Scan host
   keys**. Check the fingerprints and save. Host keys are always verified.
   The cluster row shows `cloud-init: on`, or the missing piece.

Leave the SSH user empty to disable cloud-init documents on the cluster. See
the [cloud-init setup guide](/docs/cloud-init-setup) for details and
troubleshooting.

## Catalog

The **Catalog** section lets you approve or hide the storages, ISOs, cloud
images, VM templates, and bridges that VM creation may reference, and manage
hardware profiles, tags, and cloud-init templates. Discovered resources appear
automatically; toggle the enabled switch to control what users see. When a
resource disappears from Proxmox, its stale approval is flagged so you can
remove it.

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
