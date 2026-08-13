# Getting started

Welcome to PVMSS, the self-service portal for your Proxmox virtual machines.
This guide walks you through the essentials: logging in, finding your VMs, and
creating a new one.

## Logging in

Use your Proxmox credentials on the [login page](/login). If your administrator
has enabled single sign-on, you may also pick your cluster's OIDC provider from
the login screen.

## Finding your VMs

Once authenticated, the **My VMs** page lists every virtual machine your pool
owns across all configured clusters. Use the search box to filter by name or
VMID, and the cluster selector to narrow the view to one cluster.

## Creating a VM

1. Open **Create a VM** from the home page or the VMs page.
2. Pick a hardware profile or enter custom values.
3. Choose a node, storage, and (optionally) an ISO or cloud-init template.
4. Submit — the portal provisions the VM and streams task progress to the
   navbar tray.

For more, see the [VM creation guidelines](/docs/vm-creation-guidelines).
