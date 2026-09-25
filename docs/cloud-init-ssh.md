# Cloud-init templates over SSH - setup and operations

PVMSS administrators write cloud-init templates; users pick one when they
create a VM. The Proxmox REST API cannot write `snippets` files, so PVMSS
**publishes** every template to every node of the cluster over SSH, through a
tiny helper that can do nothing else. This guide has everything needed to
set it up, verify it, and run it. Every command block is copy-paste ready:
set the variables at the top of the block and run it.

- [How it works](#how-it-works)
- [Requirements](#requirements)
- [Setup, step by step](#setup-step-by-step) and [the setup script](#the-setup-script)
- [Deployment variants](#deployment-variants) (Compose dev, Compose, docker run, Helm, plain manifest)
- [Day-2 operations](#day-2-operations) (new node, reinstall, key rotation, uninstall)
- [Troubleshooting](#troubleshooting)
- [Security model](#security-model)

## How it works

```
Admin > Cloud-init templates: save
  │  PVMSS merges the template on top of the generated baseline
  │  file name = pvmss-tpl-<template id>-<sha256[:12]>.yml   (content-addressed, immutable)
  ▼
for each node listed by GET /cluster/status (its IP there, the cluster's SSH port)
  │  ssh <ssh user>@<node ip>   - host key checked against the cluster's pinned host keys
  │  forced command: pvmss-snippet write <name>   (content on stdin, 64 KiB max)
  │  helper writes <snippets dir>/<name> atomically
  ▼
PVMSS lists <storage>:snippets on that node through the API → "published n/m nodes"

Create VM with a template
  │  no write: PVMSS checks the file is on the VM's node (API), else 409 cloudinit_not_published
  ▼
qm set <vmid> --cicustom vendor=<storage>:snippets/pvmss-tpl-…yml
```

- The template is **vendor data**: it merges with the user data Proxmox
  generates from the VM form (user, password, SSH keys, network). Accounts and
  keys belong in the form; a `users:` key in a template is overridden.
- Editing a template publishes a **new** file; existing VMs keep theirs.
- The baseline (qemu-guest-agent) is merged under every template and also
  published alone as `pvmss-baseline-<hash>.yml` for cloud-image VMs created
  without a template.
- PVMSS republishes everything in the background at startup and after a
  cluster's SSH settings are saved. **Admin > Cloud-init templates > Publish to
  all nodes** does it on demand.
- Old versions are never deleted from the nodes (a few KB each).

Configuration is split in two:

| Where | What |
| --- | --- |
| `PVMSS_SSH_KEY_FILE` (env, global) | path of PVMSS's private key inside the container |
| Infrastructure > Clusters > Edit (per cluster) | snippet storage, SSH user, SSH port, pinned host keys |

`PVMSS_SSH_USER` and `PVMSS_SSH_PORT` are **no longer read**; PVMSS logs a
warning at startup when they are still set.

Publishing is on for a cluster only when all four are present. Otherwise the
cluster badge shows the first missing one:

| Status | Missing |
| --- | --- |
| `no_ssh_key` | `PVMSS_SSH_KEY_FILE` not set (no public key shown in the cluster form) |
| `no_ssh_user` | SSH user empty in Infrastructure > Clusters |
| `no_host_keys` | no pinned host key saved |
| `no_snippet_storage` | snippet storage not selected |

When publishing is off, the template picker is hidden from the create wizard
and a create request carrying a template is refused (`cloudinit_write_unavailable`)
before any VMID is spent.

## Requirements

- Proxmox VE 8 or later, root SSH access to every node **for the setup only**.
- A storage available on every node that can hold snippets (`local` is fine:
  each node gets its own copy). See [Which storage?](#which-storage).
- Network: the PVMSS container must reach **every node's IP as listed in
  `/cluster/status`** (the corosync address) on the SSH port. That may differ
  from the API URL's host. List them on any node:

  ```sh
  pvesh get /cluster/status --output-format json \
    | perl -MJSON -0ne 'print "$_->{name} $_->{ip}\n" for grep { $_->{type} eq "node" } @{decode_json($_)}'
  ```

  A standalone node without a cluster uses the API URL's host.

### Which storage?

PVMSS writes each file on **every node** and then asks Proxmox, node by node,
to list it. Any storage that Proxmox exposes as a directory with the
**Snippets** content type works: `dir` (`local` included - the simplest and
most robust choice: a few KB per template, no network dependency when a VM
boots), `nfs`, `cifs`, `cephfs`. On a shared storage every node writes the
same content-addressed file, which is harmless.

**S3 / object storage: not supported.** Proxmox VE (9.2) has no native S3
storage type, and snippets must be a file Proxmox reads when it starts the VM.
Workarounds exist but are not supported by PVMSS:

- *S3 mounted with FUSE (s3fs, rclone mount) and declared as a `dir`
  storage*: technically writable, but ACLs are usually missing, and a slow or
  unreachable bucket blocks `pvestatd` and makes VM starts fail. Not
  recommended.
- *Third-party S3 storage plugins* (e.g. proxs3): they serve files from a
  local cache and sync with the bucket through their own daemon. Files the
  helper writes into that cache are not guaranteed to reach the bucket or be
  listed, so publishing fails with `written, but Proxmox does not list ...`
  (safe, but useless).

Nothing is gained anyway: snippets are tiny, PVMSS already copies them to
every node, and they are republished at startup and on demand. Use `local`.

## Setup, step by step

### 1. Generate PVMSS's key (workstation)

```sh
KEY=pvmss_snippets_ed25519          # file name; git-ignored in this repo
[ -f "$KEY" ] || ssh-keygen -t ed25519 -N '' -C pvmss -f "$KEY"
ssh-keygen -y -f "$KEY" > "$KEY.pub"
cat "$KEY.pub"
```

The key has no passphrase (PVMSS runs unattended). It can only run the
helper (forced command, no shell), see [Security model](#security-model).

Supported key types: **ed25519** (recommended), **ECDSA** (nistp256/384/521)
and **RSA** (2048 bits minimum, 3072+ recommended; PVMSS signs with
`rsa-sha2-256/512`, never SHA-1). PVMSS reads OpenSSH, PKCS#1 and PKCS#8
private keys. For RSA:

```sh
ssh-keygen -t rsa -b 4096 -N '' -C pvmss -f pvmss_snippets_rsa
```

### 2. Enable the Snippets content type on the storage (any one node, once)

Storage configuration is cluster-wide. Run on one node as root:

```sh
STORAGE=local
CUR=$(pvesh get /storage/$STORAGE --output-format json | perl -MJSON -0ne 'print decode_json($_)->{content}')
case ",$CUR," in
  *,snippets,*) echo "snippets already enabled on $STORAGE" ;;
  *) pvesm set "$STORAGE" --content "$CUR,snippets" && echo "snippets enabled on $STORAGE" ;;
esac
```

(Or in the GUI: Datacenter > Storage > `local` > Edit > Content: add Snippets.)

### 3. Prepare every node

The setup script is idempotent: rerun it any time. It installs
`/usr/local/bin/pvmss-snippet`, writes `/etc/pvmss-snippet.conf` (the
storage's `snippets/` directory), creates the dedicated system user with
write access to that directory only (ACL), installs PVMSS's public key with a
forced command, and prints the node's host key line. See
[The setup script](#the-setup-script) for where it lives.

**A. Download it from PVMSS** (the node reaches PVMSS over HTTP(S)). The exact
command, with PVMSS's URL, storage, user and key filled in, is shown in
**Infrastructure > Clusters > Edit** (copy button). As root on each node:

```sh
PVMSS=https://pvmss.example.com      # PVMSS's URL as seen from the node
curl -fsSL "$PVMSS/api/v1/pvmss-node-setup.sh" \
  | sh -s -- --storage local --user pvmss --key 'ssh-ed25519 AAAA... pvmss'
```

The URL in the command is the address of the page you are on. It works with
HTTP or HTTPS, an IP or an FQDN, any port - as long as **the nodes can reach
it**:

- Self-signed certificate on PVMSS: `curl -kfsSL …`.
- Page opened on `localhost`/`127.0.0.1` or on Vite's port 5173 (dev stack):
  the form warns you; use PVMSS's address as seen from the nodes (dev stack:
  the backend, `http://<workstation IP>:50000`).
- PVMSS published under a sub-path by a reverse proxy
  (`https://example.com/pvmss/`): the command drops the sub-path; add it by
  hand (`https://example.com/pvmss/api/v1/pvmss-node-setup.sh`).
- Non-default SSH port set on the cluster: the command adds `--port N` so the
  printed host key line reads `[ip]:N ...`, the form PVMSS pins.

From a workstation with root SSH to the nodes, for all of them at once:

```sh
PVMSS=https://pvmss.example.com
NODES="192.168.1.11 192.168.1.12 192.168.1.13"
STORAGE=local
SSH_USER=pvmss
PUBKEY=$(cat pvmss_snippets_ed25519.pub)

for n in $NODES; do
  echo "=== $n"
  ssh root@"$n" "curl -fsSL '$PVMSS/api/v1/pvmss-node-setup.sh' | sh -s -- --storage $STORAGE --user $SSH_USER --key '$PUBKEY'"
done
```

**B. Copy it from the repository** (the nodes cannot reach PVMSS):

```sh
for n in $NODES; do
  echo "=== $n"
  scp tools/pvmss-node-setup.sh root@"$n":/root/pvmss-node-setup.sh
  ssh root@"$n" "sh /root/pvmss-node-setup.sh --storage $STORAGE --user $SSH_USER --key '$PUBKEY'"
done
```

### 4. Check a node by hand (workstation)

```sh
NODE=192.168.1.11
SSH="ssh -i pvmss_snippets_ed25519 -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new pvmss@$NODE"

$SSH check                                              # -> pvmss-snippet ok /var/lib/vz/snippets
printf '#cloud-config\n' | $SSH write pvmss-selftest.yml   # silent on success
ssh root@$NODE "pvesm list local --content snippets | grep pvmss-selftest"   # Proxmox sees it
$SSH remove pvmss-selftest.yml
$SSH id                                                 # must be refused: "usage: write <name> | remove <name> | check"
```

Host key lines to compare with PVMSS's scan (same format as the pinned keys):

```sh
for n in $NODES; do ssh-keyscan -t ed25519 "$n" 2>/dev/null; done
```

(Non-default port: `ssh-keyscan -p 2222 …` prints `[ip]:2222 ssh-ed25519 …`,
which is also the form PVMSS expects.)

### 5. Give PVMSS the key

Mount the private key read-only and set `PVMSS_SSH_KEY_FILE`, see
[Deployment variants](#deployment-variants). At startup PVMSS logs
`SSH cloud-init publishing enabled` with its public key.

The image runs as the distroless `nonroot` user (uid/gid 65532): the key file
must be readable by it.

### 6. Configure the cluster (PVMSS UI)

The form refuses an SSH user without pinned host keys, and **Scan host keys**
only works on a saved cluster (it uses the saved port). Two ways:

**A. Paste the host keys (one save).** Get the lines from the workstation,
using the node IPs from `/cluster/status` (see [Requirements](#requirements)):

```sh
for n in $NODES; do ssh-keyscan -t ed25519 "$n" 2>/dev/null; done
# non-default port: ssh-keyscan -t ed25519 -p 2222 "$n"  ->  [ip]:2222 ssh-ed25519 ...
```

Infrastructure > Clusters > Edit: select the **snippet storage**, set **SSH user**
`pvmss`, **SSH port**, paste the lines in **Pinned host keys**, **Save**.

**B. Scan from PVMSS (two saves).**

1. Infrastructure > Clusters > Edit: select the **snippet storage**, set the **SSH
   port**, leave the SSH user empty, **Save**.
2. Reopen **Edit**, click **Scan host keys**, compare each line with the one
   printed by the setup script or `ssh-keyscan`, set **SSH user** `pvmss`,
   **Save**.

Either way, check that the public key shown in the form is the one installed
on the nodes. The badge turns **cloud-init: on** and PVMSS publishes the
baseline and every template in the background.

The setup script prints the node's first `hostname -I` address; if it is not
the `/cluster/status` IP, pin the line under the `/cluster/status` IP (the
scan does this automatically).

### 7. Publish a template and test

1. **Admin > Cloud-init templates > New template**, for example:

   ```yaml
   #cloud-config
   package_update: true
   packages:
     - htop
   runcmd:
     - echo "pvmss template applied" > /etc/motd
   ```

2. Save: the **Published** column must read `n/n nodes`.
3. On a node: `ls -l /var/lib/vz/snippets/pvmss-*` (path for `local`).
4. Create a VM from a cloud image with this template, then on its node:

   ```sh
   qm config <vmid> | grep cicustom      # vendor=local:snippets/pvmss-tpl-<id>-<hash>.yml
   cat /var/lib/vz/snippets/pvmss-tpl-<id>-<hash>.yml   # the merged document
   qm cloudinit dump <vmid> user         # the user data generated from the VM form
   ```

   In the guest: `cloud-init status --long`, `/var/log/cloud-init.log`.

### The setup script

One script, three places, always the same bytes:

| Where | What for |
| --- | --- |
| `tools/pvmss-node-setup.sh` | the copy to read, review and edit in the repository |
| `server/internal/nodesetup/pvmss-node-setup.sh` | the copy embedded in the PVMSS binary (`go:embed`) |
| `GET /api/v1/pvmss-node-setup.sh` | served by every PVMSS instance, public, `text/plain` (open it in a browser to read it; **Infrastructure > Clusters > Edit > View the script**) |

The route needs no session: a node has none, and the script holds no secret
(the key passed to it is PVMSS's *public* key).

What the script guarantees:

- **Arguments checked before anything changes**: `--key` must be one line of
  a supported type (`ssh-ed25519`, `ecdsa-sha2-nistp256/384/521`, `ssh-rsa`)
  that `ssh-keygen -l` accepts, RSA at least 2048 bits - a multi-line value
  would otherwise add unrestricted lines to `authorized_keys`; `--user` is a
  valid user name and never `root`; `--port` is 1-65535; `--storage` is a
  storage id with the Snippets content type on this node.
- **Write access**: an ACL on the storage's `snippets/` directory; on
  filesystems without ACL support (many NFS/CIFS mounts) the directory's
  group instead; then it checks the user can really write there, and stops
  with an explicit message otherwise (NFS `root_squash`: grant the access on
  the storage server, or use a local storage).
- **Host key line**: printed under the node's IP from `/cluster/status` (the
  address PVMSS connects to), ed25519 first (PVMSS's preferred algorithm),
  else ECDSA, else RSA.
- **Atomic run**: everything runs inside a `main` function called on the last
  line, so a truncated download is a syntax error and runs nothing.
- **Idempotent**: rerunning it updates the helper, the ACL and the key.

Options: `--storage ID` (default `local`), `--user NAME` (default `pvmss`),
`--port N` (default 22, only for the printed line), `--key 'KEY'` (required).

To change the script: edit `tools/pvmss-node-setup.sh`, then
`cp tools/pvmss-node-setup.sh server/internal/nodesetup/`. `go test
./internal/nodesetup/` fails while the two copies differ.

## Deployment variants

### Docker Compose - development (`docker-compose.dev.yml`)

Already wired: `PVMSS_SSH_KEY_FILE=/etc/pvmss/ssh/id_ed25519` and
`./pvmss_snippets_ed25519` mounted read-only. Generate the key **before** the
first `up`; otherwise Docker creates an empty directory at that path and the
server exits with `read SSH key file`.

```sh
[ -f pvmss_snippets_ed25519 ] || ssh-keygen -t ed25519 -N '' -C pvmss -f pvmss_snippets_ed25519
ssh-keygen -y -f pvmss_snippets_ed25519 > pvmss_snippets_ed25519.pub
chmod 0644 pvmss_snippets_ed25519      # dev only: the container runs as uid 65532
docker compose -f docker-compose.dev.yml up -d --build
docker compose -f docker-compose.dev.yml logs pvmss-dev | grep -i ssh
```

To run the dev stack without cloud-init templates, comment out the key volume
and `PVMSS_SSH_KEY_FILE` in the file.

### Docker Compose - production

```yaml
services:
  pvmss:
    environment:
      PVMSS_SSH_KEY_FILE: /etc/pvmss/ssh/id_ed25519
    volumes:
      - ./pvmss_ed25519:/etc/pvmss/ssh/id_ed25519:ro
```

Make the key readable by uid 65532 without opening it to everyone:

```sh
sudo chown 65532:65532 pvmss_ed25519 && sudo chmod 0400 pvmss_ed25519
```

### docker run / podman run

```sh
docker run -d --name pvmss --env-file .env -p 50000:50000 \
  -v ./pvmss.db:/data/pvmss.db \
  -v ./pvmss_ed25519:/etc/pvmss/ssh/id_ed25519:ro \
  -e PVMSS_SSH_KEY_FILE=/etc/pvmss/ssh/id_ed25519 \
  jhmmt/pvmss:latest
```

Rootless Podman maps uids: `podman unshare chown 65532:65532 pvmss_ed25519`.

### Helm

```sh
kubectl -n pvmss create secret generic pvmss-ssh --from-file=id_ed25519=./pvmss_ed25519
helm upgrade --install pvmss ./helm -n pvmss --set cloudInit.sshKeySecret=pvmss-ssh
```

The chart mounts the Secret at `/etc/pvmss/ssh/` (mode 0440, `fsGroup`
65532) and sets `PVMSS_SSH_KEY_FILE`. `cloudInit.sshKeyItem` changes the key
name inside the Secret (default `id_ed25519`).

### Plain Kubernetes manifest (`pvmss-deployment.yaml`)

Create the same Secret, then uncomment the `PVMSS_SSH_KEY_FILE` env entry, the
`ssh-key` volumeMount and the `ssh-key` volume.

Pods must reach the node IPs on the SSH port (NetworkPolicy / egress rules).

## Day-2 operations

**Node added to the cluster** - run step 3 on it, then in PVMSS:
Infrastructure > Clusters > Edit > Scan host keys > Save (PVMSS republishes
everything). Until then, creating a VM with a template on that node is
refused with `cloudinit_not_published`.

**Node reinstalled** (new host key) - publishing to it fails with
`host key mismatch`. Run step 3 on it, rescan, compare, save. A mismatch
without a reinstall is a red flag: do not accept it blindly.

**Node was offline during a save** - Admin > Cloud-init templates >
Publish to all nodes.

**Rotate PVMSS's key**

```sh
ssh-keygen -t ed25519 -N '' -C pvmss -f pvmss_ed25519.new
PUBKEY=$(ssh-keygen -y -f pvmss_ed25519.new)
for n in $NODES; do
  ssh root@"$n" "curl -fsSL '$PVMSS/api/v1/pvmss-node-setup.sh' | sh -s -- --storage $STORAGE --user pvmss --key '$PUBKEY'"
done
mv pvmss_ed25519.new pvmss_ed25519      # then restart PVMSS (Helm: update the Secret, restart the pod)
```

The setup script replaces `authorized_keys`, so the old key stops working on
each node as soon as it runs; publishing fails in between.

**Disable publishing on one cluster** - empty the SSH user in Infrastructure > Clusters.
Already created VMs keep working (their files stay on the nodes).

**Uninstall from a node** (as root; VMs pointing at `pvmss-*.yml` files will
no longer find them)

```sh
. /etc/pvmss-snippet.conf                  # sets SNIPPET_DIR
setfacl -x u:pvmss "$SNIPPET_DIR" 2>/dev/null || true
userdel -r pvmss
rm -f /usr/local/bin/pvmss-snippet /etc/pvmss-snippet.conf
# optional, breaks VMs that still reference them:
# rm -f "$SNIPPET_DIR"/pvmss-*.yml
```

## Troubleshooting

Per-node errors appear under the **Published** column of Admin > Cloud-init
templates.

| Symptom | Cause | Fix |
| --- | --- | --- |
| Server exits: `read SSH key file` / `parse SSH key` | path wrong, a directory (key missing before `docker compose up`), unreadable by uid 65532, or passphrase-protected key | generate the key, fix ownership/mode, restart |
| `ssh -i …`: `Load key …: error in libcrypto` | key file lost its final newline (copy/paste, editor) | `printf '\n' >> <key>` |
| `written, but Proxmox does not list ...` on an S3-backed storage | S3/object storage is not supported for snippets | use `local` (see [Which storage?](#which-storage)) |
| Node: `curl: (60) SSL certificate problem` | PVMSS uses a self-signed certificate | `curl -kfsSL …` |
| Node: `curl: (22) … 404` on `/api/v1/pvmss-node-setup.sh` | PVMSS older than the embedded script, or a proxy path prefix | upgrade PVMSS, or copy `tools/pvmss-node-setup.sh` (step 3 B) |
| Setup: `RSA key too short` | RSA key under 2048 bits | new key: `ssh-keygen -t ed25519` or `-t rsa -b 4096` |
| Setup: `--key must be a single line` / `not a valid SSH public key` / `unsupported key type` | key pasted with a line break, truncated, or with options in front | copy the key again from Infrastructure > Clusters |
| Setup: `cannot give '<user>' write access` | no ACL support and `chgrp` refused (NFS `root_squash`) | grant write access on the storage server, or use a local storage |
| Badge `no_ssh_key` | `PVMSS_SSH_KEY_FILE` unset | set it, restart |
| Badge `no_ssh_user` / `no_host_keys` / `no_snippet_storage` | cluster not fully configured | step 6 |
| Scan: `connect <ip>:22: i/o timeout` | PVMSS cannot reach the node IP from `/cluster/status` | firewall / routing / NetworkPolicy |
| `host ... is not in the cluster's pinned host keys` | node added, or pinned under another address | rescan, save |
| `host key mismatch` | node reinstalled - or an attack | step 3 + rescan, only after checking |
| `ssh handshake ... unable to authenticate` | PVMSS's key not in `~pvmss/.ssh/authorized_keys` | rerun step 3 with the key shown in Infrastructure > Clusters |
| `invalid snippet name` / `usage:` | helper out of date | rerun step 3 |
| `written, but Proxmox does not list ...` | `/etc/pvmss-snippet.conf` not the storage's `snippets/` dir, or Snippets content not enabled | step 2, rerun step 3 with the right `--storage` |
| `node is offline` | node down during publish | Publish to all nodes when it is back |
| Create refused `cloudinit_not_published` | file missing on the VM's node | Publish to all nodes |
| Create refused `cloudinit_write_unavailable` | publishing off on the cluster | see the badge |
| Template not applied in the guest | image without cloud-init, or already initialised | `qm config <vmid> | grep cicustom`, `cloud-init status --long`; most modules run on first boot only |

Useful commands on a node:

```sh
cat /etc/pvmss-snippet.conf
ls -l "$(. /etc/pvmss-snippet.conf; echo "$SNIPPET_DIR")"/pvmss-*
getfacl /var/lib/vz/snippets | grep pvmss
cat ~pvmss/.ssh/authorized_keys
journalctl -u ssh --since -1h | grep pvmss
```

## Security model

- PVMSS holds one private key; nodes accept it **only** for the `pvmss` user,
  with `restrict,command="/usr/local/bin/pvmss-snippet"`: no shell, no PTY, no
  forwarding, whatever command PVMSS sends.
- The helper accepts only `write <name>`, `remove <name>` and `check`. `<name>`
  must match `^pvmss-[A-Za-z0-9._-]+\.ya?ml$` (no path); content is capped at
  64 KiB and written atomically. PVMSS checks the name too before sending.
  `check` is a diagnostic probe; PVMSS itself never sends it.
- The `pvmss` user can write only the storage's `snippets/` directory (ACL);
  it is not in any Proxmox group and has no API rights.
- Host keys are always verified against the pinned list; there is no
  insecure mode. The scan is trust-on-first-use: compare fingerprints.
- Snippets are plain text on the nodes and readable by VM administrators:
  never put secrets in templates.
