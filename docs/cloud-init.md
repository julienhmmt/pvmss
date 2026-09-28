# Cloud-init templates - setup and operations

PVMSS administrators write cloud-init templates; users pick one when they
create a VM. The Proxmox REST API cannot write `snippets` files (its
`upload` and `download-url` endpoints accept `iso`, `vztmpl` and `import`
only), so **PVMSS never writes on the nodes**. For each template it shows a
command; the administrator pastes it, as root, on the nodes that must offer
the template. PVMSS then reads, through the API, which nodes have the file.

No SSH key, no dedicated user, no helper, no setup script.

- [How it works](#how-it-works)
- [Setup, step by step](#setup-step-by-step)
- [Day-2 operations](#day-2-operations)
- [Removing the old SSH publishing setup](#removing-the-old-ssh-publishing-setup)
- [Troubleshooting](#troubleshooting)

## How it works

```
Admin > Cloud-init: save a template
  │  PVMSS merges it on top of the generated baseline (qemu-guest-agent)
  │  file name = pvmss-tpl-<template id>-<sha256[:12]>.yml   (content-addressed)
  ▼
The page shows, per template: the file, "On n/m nodes", and "Command to paste"
  │  admin pastes the command, as root, on each node that must offer it
  │  (the command writes <storage>:snippets/<file> through pvesm path)
  ▼
Verify (or any reload): PVMSS lists <storage>:snippets on every node (API)

Create VM
  │  the template picker offers only templates present on the VM's node
  │  PVMSS re-checks the file is on that node (API), else 409 cloudinit_not_published
  ▼
qm set <vmid> --cicustom vendor=<storage>:snippets/pvmss-tpl-…yml
```

- The template is **vendor data**: it merges with the user data Proxmox
  generates from the VM form (user, password, SSH keys, network). Accounts and
  keys belong in the form; a `users:` key in a template is overridden.
- **You choose the nodes.** A template is offered only on the nodes that have
  its file. Paste it on one node today, on the others later.
- Editing a template yields a **new** file name, so a new command to paste.
  Existing VMs keep the file they were created with: do not delete old files
  a VM still uses.
- The baseline (qemu-guest-agent) is merged under every template and also
  exists alone as `pvmss-baseline-<hash>.yml` (Admin > Cloud-init baseline),
  for cloud-image VMs created without a template. Without that file on the
  node, such VMs boot on the native keys only (no guest agent install).

## Setup, step by step

### 1. Enable the Snippets content type on the storage (once, any node)

Storage configuration is cluster-wide. As root on one node:

```sh
STORAGE=local
CUR=$(pvesh get /storage/$STORAGE --output-format json | perl -MJSON -0ne 'print decode_json($_)->{content}')
case ",$CUR," in
  *,snippets,*) echo "snippets already enabled on $STORAGE" ;;
  *) pvesm set "$STORAGE" --content "$CUR,snippets" && echo "snippets enabled on $STORAGE" ;;
esac
```

(Or in the GUI: Datacenter > Storage > `local` > Edit > Content: add
Snippets.) `local` is the simplest choice: a few KB per template, a copy on
each node that needs it. A shared storage (`nfs`, `cifs`, `cephfs`) works
too: one paste then serves every node. S3/object storage is not supported.

### 2. Select the storage in PVMSS

**Infrastructure > Clusters > Edit > Cloud-init templates**: pick the
snippet storage, save. The cluster reads "cloud-init: on". An empty storage
turns cloud-init templates off on that cluster.

### 3. Write a template

**Admin > Cloud-init > New template**, `#cloud-config` content, save.

### 4. Paste the command on the chosen nodes

In the template's row, open **Command to paste**, click **Copy**, and paste
it in a root shell on each node that must offer the template (the Proxmox
web shell works). It looks like:

```sh
F=$(pvesm path local:snippets/pvmss-tpl-web-3f2a9c81d0e4.yml) && mkdir -p "${F%/*}" && cat > "$F" <<'PVMSS_EOF'
#cloud-config
...
PVMSS_EOF
```

The quoted here-document writes the content verbatim (no `$` expansion).
Do the same for the baseline if you create cloud-image VMs without a
template (**Admin > Cloud-init baseline**).

### 5. Verify

Click **Verify** on the templates page. The row reads "On n/m nodes" and
lists each node as present or absent. Then create a VM on one of those
nodes with the template, and on the node:

```sh
qm config <vmid> | grep cicustom
```

## Day-2 operations

| Situation | Action |
| --- | --- |
| Offer a template on another node | Paste its command on that node, Verify |
| New or reinstalled node | Paste the commands of the templates it must offer (and the baseline) |
| Template edited | New file name: paste the new command on the nodes, Verify |
| PVMSS upgrade changed the baseline | Every file name changes: paste the new commands |
| Remove an old version | `rm` it on the node, only if no VM uses it (`grep -l <file> /etc/pve/qemu-server/*.conf`) |
| VM deleted | Nothing to do. A legacy per-VM file (`pvmss-<vmid>.yml`, older PVMSS) is logged and left on the node: `rm` it by hand |

## Removing the old SSH publishing setup

Older PVMSS versions published over SSH. After upgrading:

- Remove `PVMSS_SSH_KEY_FILE` and the key mount from Compose / Helm values
  (PVMSS logs a warning while it is still set) and delete the key file.
- On each node, as root:

  ```sh
  userdel -r pvmss 2>/dev/null; userdel -r pvmss-snippets 2>/dev/null
  rm -f /usr/local/bin/pvmss-snippet /etc/pvmss-snippet.conf
  ```

  Then remove any `Match User pvmss…` block you added to
  `/etc/ssh/sshd_config`, check with `sshd -t`, and `systemctl reload ssh`.
- Files already on the nodes keep working: VMs point at them by name.

## Troubleshooting

| Symptom | Cause / fix |
| --- | --- |
| "cloud-init: off" on the cluster | No snippet storage selected (step 2) |
| Storage list empty in the cluster form | Snippets content type not enabled (step 1) |
| Pasted, still "absent" | Pasted on another node, or into another storage than the one selected in step 2: `pvesm list <storage> --content snippets` on the node |
| `pvesm: storage '…' does not exist` | Storage id typo, or the storage is not available on that node |
| Node shows "node is offline" | The node is down: nothing can be read from it |
| Template missing from the create wizard | Its file is not on the VM's node: paste it there |
| Create refused with `cloudinit_not_published` | Same: the file is not on the node PVMSS picked |
| VM boots without the template's effect | `qm config <vmid> \| grep cicustom`, then `cloud-init status --long` in the guest |
