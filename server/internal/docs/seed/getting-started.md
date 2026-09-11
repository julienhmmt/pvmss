# Getting started

Welcome to PVMSS, the self-service portal for your Proxmox virtual machines.
This guide walks you through the essentials: logging in, finding your VMs, and
creating a new one.

## Logging in

Pick your cluster and use your Proxmox credentials on the [login page](/login).
If the selected cluster is unreachable, the form says so — try again later or
pick another cluster.

## Finding your VMs

Once authenticated, the home page shows your VM counts, your quota, and any
task still running. The **My VMs** page lists every virtual machine your pool
owns across all configured clusters. Use the search box to filter by name,
VMID, or tag, and the cluster selector to narrow the view to one cluster.
Filters are kept in the URL, so a filtered view can be bookmarked.

## Creating a VM

1. Open **Create a VM** from the home page or the VMs page.
2. Pick a source: an ISO, a Proxmox template, or a cloud image.
3. Pick a hardware profile or enter custom values; the node is chosen for
   you unless you switch to Detailed mode.
4. Optionally attach a cloud-init document (an admin template or one of
   [your files](/cloud-init)).
5. Submit — the portal provisions the VM and shows progress in the task tray.

For more, see the [VM creation guidelines](/docs/vm-creation-guidelines) and
the [user guide](/docs/user-guide).
