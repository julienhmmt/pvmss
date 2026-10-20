# Getting started

Welcome to PVMSS, the self-service portal for your Proxmox virtual machines.
This guide walks you through the essentials: logging in, finding your VMs, and
creating a new one.

## Logging in

Pick your cluster and use your Proxmox credentials on the [login page](/login).
If the selected cluster is unreachable, the form says so - try again later or
pick another cluster.

## Finding your VMs

Once signed in, you land on the **My VMs** page (the sidebar shows your quota
and any task still running). It lists every virtual machine your pool owns
across all configured clusters. Use the search box to filter by name,
VMID, or tag, and the cluster selector to narrow the view to one cluster.
Filters are kept in the URL, so a filtered view can be bookmarked.

## Creating a VM

1. Open **Create a VM** from the **My VMs** page.
2. Pick a source: an ISO or a Proxmox template.
3. Pick a hardware profile or enter custom values; the node is chosen for
   you unless you switch to Detailed mode.
4. Give it a name and submit - the portal provisions the VM and shows
   progress in the task tray.

## Signing in to your VM

PVMSS does not add an SSH key or a password to your VM:

- **Template** - the system comes with the sign-in your team built into the
  template (an SSH key or a password). Ask your administrator if you do not
  have it. The VM's **Connect** tab shows the SSH command once the machine
  reports its address.
- **ISO** - you install the system yourself from the browser console and
  choose your own account during the installation.

## Moving a VM

Users cannot move a VM between nodes. If a VM needs to be moved, ask your PVMSS
administrator.

For more, see the [VM creation guidelines](/docs/vm-creation-guidelines) and
the [user guide](/docs/user-guide).
