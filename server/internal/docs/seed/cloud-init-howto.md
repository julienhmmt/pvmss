# Cloud-init how-to

Cloud-init configures a VM on first boot without logging in: packages, files,
commands, time zone, and more. In PVMSS a **cloud-init document** is a
`#cloud-config` template written by your administrator and published on the
cluster. You pick one when you create a VM; you never write YAML yourself.

## Choosing a document

The **Create a VM** form lists the templates your administrator made
available for the cluster. The picker is hidden when the cluster has no
cloud-init publishing - ask your administrator if you expected to see it.

A template is versioned: when your administrator edits it, VMs that already
exist keep the version they were created with. New VMs get the new version.

## What a document can do

The document is delivered as cloud-init *vendor data*, merged with the
settings from the VM form (user, password, SSH keys, network). Everything
in the vendor slot applies: `packages`, `package_update`, `package_upgrade`,
`runcmd`, `bootcmd`, `write_files`, `apt`, `timezone`, `ntp`, and the rest of
the [cloud-init schema](https://cloudinit.readthedocs.io/).

One known limit: a `users:` key in the document is overridden by the account
generated from the VM form. Put accounts and SSH keys in the form's Cloud-init
fields, not in the document.

## Changing a VM's document

After creation, the VM's **Cloud-init** tab shows the document the VM uses.
You can switch it to another template, or detach it. The change is picked up
at the next boot, but most of a document only runs on a new VM's first boot
(see below).

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

Templates are stored in plain text - in the portal's database and on the
cluster's snippet storage, where cloud-init must be able to read it. They
never carry your passwords or keys: use the cloud-init **password** field
(delivered through the guest agent and never stored) and SSH keys of the VM
form instead.
