# VM creation guidelines

This page collects the rules and recommendations that govern VM creation in
PVMSS. Your administrator may enforce stricter limits via policy; the values
shown here are the portal defaults.

## Naming

VM names are lowercase, hyphenated, and unique within your pool. Avoid generic
names like `vm1` — a descriptive name (`web-prod-01`) makes the VMs list
searchable and the audit log readable.

## Resources

- Start from a **profile** when one fits your workload; profiles encode the
  approved CPU, memory, and disk combinations and keep the catalog consistent.
- Custom values are clamped by the cluster policy: requests above the per-user
  quota or the node capacity are rejected before any Proxmox call is made.
- Disks use the storage you select; pick a storage that matches the disk's
  expected I/O profile.

## Cloud-init

Prefer a **cloud-init template** over a manual post-install setup. Templates
are admin-curated and validated server-side; selecting one guarantees the
snippet is well-formed. See the [cloud-init how-to](/docs/cloud-init-howto) for
the supported fields.

## After creation

New VMs appear in **My VMs** immediately. The first boot may take a minute
while cloud-init runs; the console tab shows live boot output.
