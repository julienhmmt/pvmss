# Cloud-init setup (administrator)

This guide explains how cloud-init works in PVMSS and how administrators
prepare cloud-init for their users. Cloud-init configures a VM on first boot
without logging in: users, SSH keys, packages, and more.

## How it works

- Administrators curate **cloud-init templates** in the admin panel
  (`/admin/cloudinit-templates`). A template is a `#cloud-config` document the
  administrator writes and validates. Templates are stored in the PVMSS
  database, not on Proxmox storage.
- At VM creation, a user picks a template from the cloud-init dropdown. Its
  content is applied verbatim to the new VM's cloud-init drive.
- After creation, a user can open the VM's **Cloud-init** tab and view or edit
  the snippet. The editor accepts any valid `#cloud-config` document. Changes
  are pushed to the cluster's cloud-init storage and take effect on the next
  boot.

## Administrator tasks

1. Open **Admin > Cloud-init templates**.
2. Create a template with a label and the `#cloud-config` content.
3. Validate that the content starts with `#cloud-config` and contains valid
   YAML. The portal validates the header and YAML syntax; it does not validate
   cloud-init semantics.
4. Enable the template so it appears in the user's creation dropdown. Disable
   it to hide it without deleting.

Templates are static. User-specific values (user, password, SSH keys, network)
are set through the VM's cloud-init fields or by editing the snippet after
creation.

## Supported fields

Common fields you can use in a template:

- `packages` — a list of packages to install.
- `users` — user accounts with SSH authorized keys.
- `write_files` — arbitrary file content written before first boot.
- `runcmd` — a list of commands run on first boot.

See the upstream [cloud-init docs](https://cloudinit.readthedocs.io/) for the
full schema.

## Example templates

Basic user setup:

```yaml
#cloud-config
users:
  - name: admin
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - ssh-ed25519 AAAAC3... user@example.com
```

Package installation:

```yaml
#cloud-config
package_update: true
package_upgrade: true
packages:
  - vim
  - htop
  - curl
  - git
```

Custom script execution:

```yaml
#cloud-config
runcmd:
  - echo "Hello from cloud-init" > /tmp/hello.txt
  - systemctl enable --now docker
```

## Troubleshooting

- **Template not applied**: confirm the content starts with `#cloud-config` and
  that the VM's cloud-init drive is present. Check the VM console log
  `/var/log/cloud-init.log` inside the guest.
- **Changes not taking effect**: cloud-init runs on boot. Restart the VM after
  editing the snippet for the new configuration to apply.
- **Invalid YAML**: the portal rejects templates that are not valid YAML or
  that omit the `#cloud-config` header.

## Limitations

- Templates are static; there are no template variables. User-specific values
  must be set through the basic cloud-init fields or by editing the per-VM
  snippet.
- Only YAML syntax and the `#cloud-config` header are validated, not
  cloud-init semantic correctness.
