# Cloud-init setup (administrator)

This guide explains how cloud-init works in PVMSS and how administrators
prepare it for their users. Cloud-init configures a VM on first boot without
logging in: packages, files, commands, and more.

## How it works

- Two sources of **cloud-init documents**: administrator **templates**
  (`/admin/cloudinit-templates`, per cluster) and users' **own files**
  (`/cloud-init`, up to 20 per user). Both are `#cloud-config` documents
  stored in the PVMSS database.
- At VM creation, the user picks one document from a grouped select. PVMSS
  writes a **per-VM copy** named `pvmss-<vmid>.yml` into the cluster's
  snippet storage, verifies it is visible, and attaches it to the VM as
  vendor data (`cicustom=vendor=…`). The copy is the unit of truth: editing
  the template or file later never changes existing VMs.
- Vendor data merges with the user data Proxmox generates from the VM form
  (user, password, SSH keys, network). `packages`, `package_update`,
  `runcmd`, `bootcmd`, `write_files`, `apt`, `timezone`, `ntp`… all apply. A
  `users:` key in the document is overridden by the generated account - tell
  users to put accounts and keys in the form.
- After creation, the VM's **Cloud-init** tab shows the document. When the
  policy's **Allow custom cloud-init YAML** is on, users can edit it: the
  save overwrites the VM's own file and applies on the next boot.

## Prerequisite: a snippet write target

Proxmox's REST API cannot write `snippets` files, so PVMSS writes them
through a directory mounted into its container. Follow **Enabling cloud-init
documents** in the [administrator guide](/docs/admin-guide): shared storage
with the Snippets content type, mount its `snippets/` directory into the
container, then set *Snippet directory* and *Snippet storage* on the cluster
in **Admin › Clusters**.

Without a write target, the document picker is hidden from the wizard and a
create request carrying a document is refused before any VMID is spent.

## Administrator tasks

1. Open **Admin > Cloud-init templates**.
2. Create a template with a label and the `#cloud-config` content.
3. The portal validates the `#cloud-config` header and YAML syntax; it does
   not validate cloud-init semantics.
4. Enable the template so it appears in the users' picker. Disable it to hide
   it without deleting.

Templates are static - there are no template variables. User-specific values
(user, password, SSH keys, network) come from the VM form.

## Example templates

Package installation:

```yaml
#cloud-config
package_update: true
package_upgrade: true
packages:
  - qemu-guest-agent
  - vim
  - htop
  - curl
runcmd:
  - systemctl enable --now qemu-guest-agent
```

Custom files and commands:

```yaml
#cloud-config
timezone: Europe/Paris
write_files:
  - path: /etc/motd
    content: |
      Provisioned by PVMSS.
runcmd:
  - systemctl enable --now docker
```

## Cloud-image VMs and the baseline snippet

VMs created from a **cloud image** additionally get a fixed baseline
snippet, `pvmss-baseline.yml`, when one exists in the same `snippets/`
directory - a convenient place to install `qemu-guest-agent` cluster-wide.
Its absence is silent, not an error.

## Troubleshooting

- **Snippet written locally but the VM cannot start** (or `cicustom` points
  at a missing file): the configured snippet directory is not the same
  physical directory Proxmox reads snippets from. PVMSS does not SSH to
  Proxmox and does not upload snippets via the API; it writes to a directory
  that must be shared with (or be) the Proxmox storage's snippets path. On
  the Proxmox host run `pvesm path <storage>` - the snippets path is that
  path plus `/snippets`. That exact path must be what PVMSS writes to.
- **Picker hidden in the wizard**: the cluster has no snippet write target
 - check **Admin › Clusters** (badge "cloud-init: on").
- **Create refused with `cloudinit_write_unavailable`**: the directory is not
  mounted, not writable by the container user (uid 65532), or the storage id
  does not match the Proxmox storage that owns it.
- **Document not applied**: on a node, `qm config <vmid> | grep cicustom`
  must show `vendor=<storage>:snippets/pvmss-<vmid>.yml`; check
  `/var/log/cloud-init.log` inside the guest.
- **Changes not taking effect**: most modules run once, on first boot. See
  the user-facing [cloud-init how-to](/docs/cloud-init-howto) for the
  `cloud-init clean` procedure.
- **Invalid YAML**: the portal rejects documents that are not valid YAML or
  that omit the `#cloud-config` header.

## Limitations

- Templates are static; no template variables.
- Only YAML syntax and the `#cloud-config` header are validated.
- Deleting a VM in PVMSS removes its file; VMs deleted directly in Proxmox
  leave orphans (compare `snippets/pvmss-*.yml` with `qm list`).
- Documents are stored in plain text; they are not a place for secrets.
