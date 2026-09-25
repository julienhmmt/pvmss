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
over SSH, through a small helper (`pvmss-snippet`) installed on every node
for a dedicated `pvmss` user. PVMSS needs:

- the global private key, `PVMSS_SSH_KEY_FILE` (server setting);
- per cluster, in **Infrastructure > Clusters > Edit**: the snippet storage, the SSH
  user, the SSH port and the pinned host keys;
- network access from PVMSS to **every node's IP as listed in
  `/cluster/status`** on the SSH port (it can differ from the API URL).

The commands below are ready to use: set the variables first. The full
reference (Compose, Helm, Kubernetes, key rotation, uninstall) is
`docs/cloud-init-ssh.md` in the PVMSS repository.

**1. PVMSS's key** (workstation):

```sh
ssh-keygen -t ed25519 -N '' -C pvmss -f pvmss_ed25519
```

ed25519 is recommended; ECDSA and RSA work too (RSA: 2048 bits minimum,
3072+ recommended, e.g. `ssh-keygen -t rsa -b 4096 …`). No passphrase.

Give PVMSS the private key: mount it read-only, readable by uid 65532 (the
container user), and set `PVMSS_SSH_KEY_FILE=/etc/pvmss/ssh/id_ed25519`.

- Compose: `- ./pvmss_ed25519:/etc/pvmss/ssh/id_ed25519:ro`, then
  `sudo chown 65532:65532 pvmss_ed25519 && sudo chmod 0400 pvmss_ed25519`.
- Helm: `kubectl -n pvmss create secret generic pvmss-ssh
  --from-file=id_ed25519=./pvmss_ed25519` and
  `--set cloudInit.sshKeySecret=pvmss-ssh`.

After a restart, the public key appears in **Infrastructure > Clusters > Edit**.

**Which storage?** Any directory-backed storage with the Snippets content
type: `local` (recommended: a few KB per template, no network dependency at
VM start), NFS, CIFS, CephFS. **S3/object storage is not supported**: Proxmox
VE has no native S3 storage, and FUSE mounts or third-party S3 plugins do not
guarantee the file is written and listed.

**2. Snippets content type** (any one node, as root, once - storage
configuration is cluster-wide; or Datacenter > Storage > Edit > Content):

```sh
STORAGE=local
CUR=$(pvesh get /storage/$STORAGE --output-format json | perl -MJSON -0ne 'print decode_json($_)->{content}')
case ",$CUR," in *,snippets,*) echo already ;; *) pvesm set "$STORAGE" --content "$CUR,snippets" ;; esac
```

Node IPs PVMSS will connect to:

```sh
pvesh get /cluster/status --output-format json \
  | perl -MJSON -0ne 'print "$_->{name} $_->{ip}\n" for grep { $_->{type} eq "node" } @{decode_json($_)}'
```

**3. Every node.** PVMSS serves its setup script at
`/api/v1/pvmss-node-setup.sh` (embedded in the binary; the same file is
`tools/pvmss-node-setup.sh` in the repository). The exact command, with this
PVMSS's URL, the storage, the user and the key, is shown with a copy button in
**Infrastructure > Clusters > Edit**, next to a **View the script** link. As
root on each node:

```sh
PVMSS=https://pvmss.example.com       # PVMSS's URL as seen from the node
curl -fsSL "$PVMSS/api/v1/pvmss-node-setup.sh" \
  | sh -s -- --storage local --user pvmss --key 'ssh-ed25519 AAAA... pvmss'
```

Self-signed certificate on PVMSS: `curl -kfsSL`. For all nodes at once, from
a workstation with root SSH to them:

```sh
NODES="192.168.1.11 192.168.1.12 192.168.1.13"
STORAGE=local
PUBKEY=$(cat pvmss_ed25519.pub)
for n in $NODES; do
  ssh root@"$n" "curl -fsSL '$PVMSS/api/v1/pvmss-node-setup.sh' | sh -s -- --storage $STORAGE --user pvmss --key '$PUBKEY'"
done
```

Nodes that cannot reach PVMSS: copy `tools/pvmss-node-setup.sh` to them
(`scp`) and run `sh pvmss-node-setup.sh` with the same options. The script is
idempotent; a truncated download runs nothing.

The URL in the command is the page's address (HTTP or HTTPS, IP or FQDN, any
port): the nodes must be able to reach it. The form warns when it is
`localhost` or Vite's port 5173; behind a reverse proxy sub-path, add the
sub-path by hand. The script refuses a key that is not one valid line (RSA
under 2048 bits included), `root` as user, and a storage without Snippets; on
NFS/CIFS without ACL support it falls back to the directory's group, and it
stops with an explicit message when the user still cannot write there.

The script installs `/usr/local/bin/pvmss-snippet`, writes
`/etc/pvmss-snippet.conf`, creates the `pvmss` user with write access to the
storage's `snippets/` directory only, installs the key with a forced command
(no shell, no forwarding), and prints the node's host key.

**4. Check a node** (workstation):

```sh
NODE=192.168.1.11
SSH="ssh -i pvmss_ed25519 -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new pvmss@$NODE"
$SSH check                                               # pvmss-snippet ok <snippets dir>
printf '#cloud-config\n' | $SSH write pvmss-selftest.yml
ssh root@$NODE "pvesm list $STORAGE --content snippets | grep pvmss-selftest"
$SSH remove pvmss-selftest.yml
$SSH id                                                  # must be refused (usage: ...)
```

**5. Infrastructure > Clusters > Edit.** The SSH user needs pinned host keys, and
**Scan host keys** needs a saved cluster. Either:

- paste the host keys and save once:

  ```sh
  for n in $NODES; do ssh-keyscan -t ed25519 "$n" 2>/dev/null; done
  ```

  then set the snippet storage, SSH user `pvmss`, port, paste the lines in
  **Pinned host keys**, **Save**; or
- set the snippet storage and port, **Save**; reopen, **Scan host keys**,
  compare with the lines printed by the setup script, set SSH user `pvmss`,
  **Save**.

Host keys are always verified. The badge turns "cloud-init: on" and PVMSS
republishes the baseline and the templates in the background. When it stays
off, the badge names the missing piece: no SSH key (`PVMSS_SSH_KEY_FILE`), no
SSH user, no pinned host keys, or no snippet storage.

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
