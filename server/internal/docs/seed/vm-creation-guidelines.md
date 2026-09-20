# VM creation guidelines

This page collects the rules and recommendations that govern VM creation in
PVMSS. Your administrator may enforce stricter limits via policy; the values
shown here are the portal defaults.

## Naming

VM names are hostnames: lowercase, hyphenated, at most 63 characters, and
unique within your pool (the name becomes the guest's DNS label through
cloud-init). Avoid generic names like `vm1` - a descriptive name
(`web-prod-01`) makes the VMs list searchable and the activity log readable.

## Sources

- **ISO** - installs from an administrator-approved image; the VM boots from
  the CD-ROM first.
- **Template** - clones an approved Proxmox template. The VM stays on the
  template's node, the disk cannot be smaller than the template's, and the
  wizard tells you when the target storage forces a full copy instead of a
  linked clone.
- **Cloud image** - imports an approved cloud image as the primary disk and
  requires the cloud-init fields (user, SSH keys, network). The VM starts
  only after the import finishes and cloud-init is applied.

## Resources

- Start from a **profile** when one fits your workload; profiles encode the
  approved CPU, memory, and disk combinations and keep the catalog consistent.
- Custom values are clamped by the cluster policy: requests above the per-user
  quota, the gabarit limits, or the node capacity are rejected before any
  Proxmox call is made.
- Leave the node unset and PVMSS picks the least loaded approved node with
  enough free storage.
- Disks use the storage you select; pick a storage that matches the disk's
  expected I/O profile.

## Firmware

New VMs boot in **UEFI** by default, with an EFI disk holding an **empty key
store**. That means UEFI itself (GPT, EFI variables, the q35 machine type) but
no **Secure Boot** — PVMSS deliberately never enables it. Secure Boot only
runs bootloaders signed by the keys enrolled in the EFI variables, and PVMSS
creates VMs from whatever ISO an administrator approved: most Linux install
media is unsigned (Arch's official image states outright that it does not
support Secure Boot), so with Secure Boot on the installer never starts and
the VM stops at the UEFI shell with no way back. If you need Secure Boot — for
a Windows 11 guest, say — enable it in Proxmox itself, where you can verify
that the specific ISO is signed.

Enable **TPM 2.0** for guests that require it (Windows 11).

## Cloud-init

Prefer a **cloud-init document** over a manual post-install setup. Pick an
admin template or one of your own files; the VM gets its own copy at
creation. See the [cloud-init how-to](/docs/cloud-init-howto).

## After creation

New VMs appear in **My VMs** as soon as the create task finishes. The first
boot may take a minute while cloud-init runs; the console shows live boot
output.
