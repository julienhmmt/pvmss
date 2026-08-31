# Cloud-init how-to

Cloud-init lets you configure a VM on first boot without logging in: users,
SSH keys, packages, and more. PVMSS ships admin-curated cloud-init templates
you can pick at creation time, and lets you attach a custom snippet per VM.

## Using a template

On the **Create a VM** form, pick a template from the cloud-init dropdown. The
template's content is applied verbatim; you do not need to edit it.

## Editing a VM's snippet

After creation, open the VM's **Cloud-init** tab to view or edit the snippet.
The editor accepts any valid `#cloud-config` document. Changes are pushed to
the cluster's cloud-init storage and take effect on the next boot.

## What applies when

Cloud-init modules do not all replay the same way:

- **Network settings** (IP, gateway, DNS, search domain) are reapplied at
  every boot — a reboot is enough.
- **The password** is delivered immediately to the running guest through the
  QEMU guest agent; no reboot is involved.
- **The user and the SSH-key list** (and most of a custom snippet) are
  consumed by per-instance modules that run **once, on a new VM's first
  boot**. Changing them on an already-provisioned VM updates the config but
  does not replay inside the guest.

To reapply user/SSH-key/snippet changes on an already-provisioned VM, run
inside the guest, then reboot:

```sh
sudo cloud-init clean --logs --seed && sudo reboot
```

This resets cloud-init's per-instance state, so the next boot replays the
modules with the new configuration. To add an SSH key to a running VM
without any of this, use the **Add key now** section of the Cloud-init tab:
it injects the key immediately through the guest agent and also saves it to
the config for future boots.

## Supported fields

The portal validates that your snippet starts with `#cloud-config`. Common
fields include:

- `packages` — a list of packages to install.
- `users` — user accounts with SSH keys.
- `write_files` — arbitrary file content written before first boot.
- `runcmd` — a list of commands run on first boot.

See the upstream [cloud-init docs](https://cloudinit.readthedocs.io/) for the
full schema.

## Snippets are not a vault

Snippet content is stored in plain text — both in the portal's database and on
the cluster's cloud-init storage, where cloud-init must be able to read it —
and any administrator can view it. Never put passwords, API tokens, or private
keys in a snippet; use the cloud-init **password** field (delivered through
the guest agent and never stored) and SSH keys instead.
