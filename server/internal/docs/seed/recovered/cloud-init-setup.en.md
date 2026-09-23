# Cloud-init setup (administrator)

This guide explains how cloud-init works in PVMSS and how administrators
prepare it for their users. Cloud-init configures a VM on first boot without
logging in: packages, files, commands, and more.

## How it works

- **Only administrators write cloud-init documents**: the **templates** of
  **Admin > Cloud-init templates** (per cluster, `#cloud-config`). Users pick
  one when they create a VM, or switch the VM to another one later on its
  **Cloud-init** tab. Users never write YAML.
- Saving a template **publishes** it: PVMSS merges it on top of the PVMSS
  baseline (qemu-guest-agent) and writes the result, over SSH, as an
  immutable file `pvmss-tpl-<id>-<hash>.yml` into the snippet storage of
  **every node**, then checks through the Proxmox API that each node lists
  it. The page shows the result per node ("3/3 nodes").
- Creating a VM never writes a file: PVMSS checks that the template's file
  is on the VM's node, then points the VM at it (`cicustom=vendor=…`). A
  template that is not on the node is refused before the VM is created.
- Editing a template publishes a **new** file: VMs keep the version they
  were created with.
- Vendor data merges with the user data Proxmox generates from the VM form
  (user, password, SSH keys, network). `packages`, `package_update`,
  `runcmd`, `bootcmd`, `write_files`, `apt`, `timezone`, `ntp`… all apply. A
  `users:` key in the document is overridden by the generated account - tell
  users to put accounts and keys in the form.

## Prerequisites: SSH publishing

Proxmox's REST API cannot write `snippets` files, so PVMSS publishes them
over SSH, through a small helper installed on every node.

1. **Storage**: in Proxmox, add **Snippets** to the content types of a
   storage available on every node (Datacenter > Storage > Edit). `local`
   works: the file is written on each node.
2. **PVMSS key**: generate a key pair (`ssh-keygen -t ed25519 -N '' -f
   pvmss_ed25519`) and give PVMSS the private key with `PVMSS_SSH_KEY_FILE`
   (read-only file; with Helm, a Secret named in `cloudInit.sshKeySecret`).
   The public key is shown in **Admin > Clusters > Edit**.
3. **Every node**, as root: `sh pvmss-node-setup.sh --storage <storage>
   --key '<PVMSS public key>'` (the exact command is shown in the cluster
   form). The script installs `/usr/local/bin/pvmss-snippet`, creates the
   dedicated user `pvmss` with write access to the storage's `snippets/`
   directory only, installs the key with a forced command (no shell, no
   forwarding), and prints the node's host key.
4. **Admin > Clusters > Edit**: set the snippet storage, the SSH user
   (`pvmss`) and the port, click **Scan host keys**, compare the
   fingerprints with the ones the script printed, and save. Host keys are
   always verified. PVMSS republishes the baseline and the templates in the
   background; the cluster badge turns "cloud-init: on".

PVMSS only ever sends `pvmss-snippet write <name>` (content on stdin) and
`pvmss-snippet remove <name>`; the helper checks the name
(`pvmss-*.yml`) and owns the directory. PVMSS never sends a path or a shell
command, and the key cannot open a shell.

Without SSH publishing, the template picker is hidden from the wizard and a
create request carrying a template is refused before any VMID is spent.

## Administrator tasks

1. Open **Admin > Cloud-init templates**.
2. Create a template with a label and the `#cloud-config` content. Saving
   publishes it; check the "Published" column.
3. Disable a template to hide it from users without deleting it. Deleting a
   template never breaks the VMs that use it: their file stays on the nodes.
4. After adding or reinstalling a node (or when a node was offline during a
   save), click **Publish to all nodes**.

Templates are static - there are no template variables. User-specific values
(user, password, SSH keys, network) come from the VM form.

## Example templates

Package installation:

```yaml
#cloud-config
package_update: true
packages:
  - vim
  - htop
  - curl
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

## The baseline

The generated baseline (see **Admin > Cloud-init baseline**) installs and
enables `qemu-guest-agent`. It is merged under every template, and published
on its own (`pvmss-baseline-<hash>.yml`) for cloud-image VMs created without
a template. When it is not on the VM's node, the VM still boots on its native
keys and its detail page reports the baseline as not delivered.

## Troubleshooting

- **"n/m nodes" in the Published column**: the failing node's error is
  shown under it.
  - `host ... is not in the cluster's pinned host keys` / `host key
    mismatch`: scan the host keys again and compare the fingerprints (a
    mismatch without a reinstall is a red flag).
  - `ssh handshake ... unable to authenticate`: the PVMSS key is not in
    `~pvmss/.ssh/authorized_keys` on that node - rerun the setup script.
  - `written, but Proxmox does not list ...`: the helper's directory
    (`/etc/pvmss-snippet.conf`) is not the storage's `snippets/` directory,
    or the storage does not have the Snippets content type on that node.
  - `node is offline`: publish again once it is back.
- **Create refused with `cloudinit_not_published`**: the template is not on
  the VM's node - publish again.
- **Document not applied**: on a node, `qm config <vmid> | grep cicustom`
  must show `vendor=<storage>:snippets/pvmss-...yml`; check
  `/var/log/cloud-init.log` inside the guest.
- **Changes not taking effect**: most modules run once, on first boot. See
  the user-facing [cloud-init how-to](/docs/cloud-init-howto) for the
  `cloud-init clean` procedure.
- **Invalid YAML**: the portal rejects documents that are not valid YAML or
  that omit the `#cloud-config` header.

## Limitations

- Templates are static; no template variables.
- Only YAML syntax and the `#cloud-config` header are validated.
- Old published versions are not deleted from the nodes (a few KB each).
- Documents are stored in plain text on the nodes; they are not a place for
  secrets.
