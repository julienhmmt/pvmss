# Cloud-init how-to

Cloud-init configures a VM on first boot without logging in: packages, files,
commands, time zone, and more. In PVMSS a **cloud-init document** is a
`#cloud-config` file that PVMSS copies onto the cluster and attaches to your
VM when it is created.

## Two kinds of documents

- **Admin templates** - written and curated by your administrator, listed
  first in the picker.
- **My files** - your own documents, managed on the [Cloud-init files](/cloud-init)
  page (up to 20). Only you can see and use them.

Both appear in one grouped select on the **Create a VM** form. The picker is
hidden when the target cluster has no cloud-init write target - ask your
administrator if you expected to see it.

## Your VM keeps its own copy

At creation, PVMSS writes the chosen document to the cluster as
`pvmss-<vmid>.yml` and attaches it to the VM. That copy belongs to the VM:
editing the template or your file afterwards does **not** change VMs that
already exist.

## What a document can do

The document is delivered as cloud-init *vendor data*, merged with the
settings from the VM form (user, password, SSH keys, network). Everything
in the vendor slot applies: `packages`, `package_update`, `package_upgrade`,
`runcmd`, `bootcmd`, `write_files`, `apt`, `timezone`, `ntp`, and the rest of
the [cloud-init schema](https://cloudinit.readthedocs.io/).

One known limit: a `users:` key in the document is overridden by the account
generated from the VM form. Put accounts and SSH keys in the form's Cloud-init
fields, not in the document.

## Editing a VM's document

After creation, the VM's **Cloud-init** tab shows the document. If your
administrator allows custom YAML in the policy, you can edit and save it: the
save overwrites the VM's own file and takes effect on the next boot. Saving an
empty document detaches it.

## What applies when

Cloud-init modules do not all replay the same way:

- **Network settings** (IP, gateway, DNS, search domain) are reapplied at
  every boot - a reboot is enough.
- **The password** is delivered immediately to the running guest through the
  QEMU guest agent; no reboot is involved.
- **The user, the SSH-key list, and most of a document** are consumed by
  per-instance modules that run **once, on a new VM's first boot**. Changing
  them on an already-provisioned VM updates the config but does not replay
  inside the guest.

To reapply user/SSH-key/document changes on an already-provisioned VM, run
inside the guest, then reboot:

```sh
sudo cloud-init clean --logs --seed && sudo reboot
```

To add an SSH key to a running VM without any of this, use the **Add key
now** section of the Cloud-init tab: it injects the key immediately through
the guest agent and also saves it to the config for future boots.

## Documents are not a vault

Document content is stored in plain text - in the portal's database and on
the cluster's snippet storage, where cloud-init must be able to read it - and
any administrator can view it. Never put passwords, API tokens, or private
keys in a document; use the cloud-init **password** field (delivered through
the guest agent and never stored) and SSH keys instead.
