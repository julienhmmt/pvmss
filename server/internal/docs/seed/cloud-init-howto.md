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

## Supported fields

The portal validates that your snippet starts with `#cloud-config`. Common
fields include:

- `packages` — a list of packages to install.
- `users` — user accounts with SSH keys.
- `write_files` — arbitrary file content written before first boot.
- `runcmd` — a list of commands run on first boot.

See the upstream [cloud-init docs](https://cloudinit.readthedocs.io/) for the
full schema.
