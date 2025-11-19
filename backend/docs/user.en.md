# PVMSS user guide

PVMSS (Proxmox Virtual Machine Self-Service) is an intuitive web application that simplifies creating, managing, and accessing the consoles of virtual machines hosted on a Proxmox Virtual Environment server.

## Quick start guide

1. **Log in to the application**: Sign in to PVMSS with your credentials to access the virtual machine creation and management features.
2. **Search for virtual machines**: Use the search function to locate a specific VM by its name or VMID and view its details.
3. **Create a virtual machine**: Click the "Create VM" button to open the configuration form, then fill in the required parameters.
4. **Access the console**: After the VM is created and started, click the "Console" button to connect to its graphical interface through the integrated noVNC web client.
5. **Manage your profile**: Open your profile to review and update your description and manage the tags assigned to you.

## Main features

### Creating a virtual machine

To create a VM, open the configuration form via the "Create VM" button after signing in to PVMSS. Configure the following parameters:

- **Node**: Select the Proxmox node where the VM will be created (among the administrator-configured nodes). Some nodes may be disabled if they have reached the limits defined by administrators.
- **Name and description**: Enter a unique name (alphanumeric characters, hyphens, and underscores only) and a description to identify your VM.
- **Operating system**: Choose an ISO image from a list defined by administrators to install the OS.
- **Resources**: Configure the required resources:
  - **CPU**: Number of CPU sockets and cores (within administrator-defined limits)
  - **Memory (RAM)**: Amount of memory in MB or GB (within administrator-defined limits)
  - **Disks**: Size of the main disk in GB and, if allowed by administrators, one or more additional data disks
  - **Disk bus type**: Storage bus used for disks (VirtIO, SCSI, SATA, IDE), which may impact performance and maximum number of disks
- **Storage**: Select the storage where the VM disks will be created, among the storages enabled by administrators.
- **Network**: Configure one or more network cards (depending on administrator settings):
  - **Network bridge**: Select the network bridge (VMBR) for connectivity
  - **Network card model**: Choose the adapter model (VirtIO, E1000, E1000E, RTL8139, VMXNet3)
  - **MAC address**: Optionally specify a MAC address or let PVMSS generate one automatically
- **Firmware & security**:
  - **EFI boot**: Enable UEFI firmware (typically enabled by default for modern operating systems)
  - **TPM (Trusted Platform Module)**: Optionally enable TPM v2.0 for operating systems that require it (for example Windows 11)
- **Startup**: Choose whether the VM should be started automatically after creation.
- **Tags**: Add predefined tags to organize your VMs and make them easier to find.

**Important notes:**

- You can create only one VM at a time.
- Resource limits per VM (CPU, RAM, disk) are imposed by administrators.
- Additional quotas may apply, such as the maximum number of VMs per user or per node. If you reach these limits, you may not be able to create new VMs until an administrator adjusts the configuration.

### Searching for a virtual machine

Use the search function to locate a VM by its **name** or **VMID** (unique identifier assigned to each VM by Proxmox). The search is case-insensitive and supports partial matches.

The results list shows:

- The VMID
- The VM name
- The node hosting the VM
- The current status (running, stopped, etc.)
- The "VM Details" button (to view complete VM information and access advanced management features)

### Managing a virtual machine

The VM details page provides comprehensive management and monitoring capabilities:

#### Control actions

- **Start**: Power on the virtual machine
- **Console**: Open the integrated noVNC console in a new window for graphical access
- **Restart**: Reboot the virtual machine
- **Shutdown**: Shut down gracefully (sends an ACPI shutdown signal)
- **Stop**: Force-stop the virtual machine (immediate shutdown)
- **Reset**: Forcefully reset the virtual machine
- **Refresh**: Refresh the VM information (invalidate the cache)
- **Delete**: Permanently delete the virtual machine (requires confirmation)

### Configuration details

View real-time information about your VM:

- **Status**: Current state (running, stopped, etc.)
- **Uptime**: Duration the VM has been running
- **CPU usage**: Current processor utilization percentage
- **Memory usage**: Current RAM usage (used/total)
- **Disk usage**: Storage space used by the VM
- **Network**: Display of the VM's network parameters

Review detailed configuration information:

- VM name and description
- Node location
- CPU core and memory allocation
- Disk configuration
- Network settings
- Assigned tags

### Profile management

You can update certain VM properties from the VM details page and from your profile:

- **Description**: Update the VM description (plain text or simple Markdown, depending on administrator configuration)
- **Tags**: Add or remove tags for better organization

### Editing virtual machine resources

In addition to description and tags, you can modify some resources of an existing VM from the *VM details* page, as long as the VM is **stopped**:

- **CPU**: Number of sockets and cores (within the limits defined by administrators)
- **Memory (RAM)**: Allocated memory in MB/GB (within the limits defined by administrators)
- **Network cards**:
  - Bridge used by each network card
  - Network card model (VirtIO, E1000, E1000E, RTL8139, VMXNet3)
  - MAC address of each card (optional)
- **CD-ROM / ISO**: Loaded ISO image for the virtual CD-ROM drive, or ejection of the current ISO

Some operations remain restricted and may still require a new VM to be created by copying data manually, for example:

- Changing disk size or the number of disks beyond what administrators allow
- Migrating to a different storage backend when not supported by Proxmox
- Structural changes that are not exposed in the PVMSS interface

### Console access

PVMSS provides integrated console access to your VMs through noVNC, a web-based VNC client.

1. Sign in to the PVMSS application.
2. Open the VM details page (via the search function or from your profile).
3. Ensure the VM is running (start it if necessary).
4. Click the "Console" button.

#### Console features

- **Full keyboard and mouse support**: Interact with your VM as if you were using a physical monitor.
- **Connection indicators**: Visual feedback showing the connection status.
- **Automatic reconnection**: The console tries to reconnect if the connection is lost.

#### Console troubleshooting

If you encounter console connection issues:

- Make sure the VM is running (the console only works for running VMs).
- Sign out of PVMSS and sign back in.
- Refresh the console window if the connection drops.
- Contact your administrator if the issues persist.

**Note**: The console session uses your PVMSS credentials and provides secure access to your VM's graphical interface.

## Best practices

- **Proper shutdown**: Always use the "Shutdown" (graceful) button instead of "Stop" whenever possible to avoid data loss and ensure the OS shuts down cleanly.
- **Naming convention**: Use clear, descriptive names that follow your organization's standards for your virtual machines. Use only alphanumeric characters, hyphens, and underscores.
- **Resource planning**: Plan your resource needs before creating a VM. Contact your administrator if you need resources beyond the configured limits.
- **Tag organization**: Use tags consistently to organize your VMs and make them easier to locate.
- **Console security**: Close the console window when not in use to free resources.
- **Credential security**: Never share your login credentials to keep your account and VMs secure.
- **Regular monitoring**: Check your VM's resource usage regularly to ensure it operates efficiently.

## Support

The PVMSS application is maintained by your organization's IT team. Contact your administrator for assistance in the following cases:

- **Password loss**: You can update your password in the profile page. Your administrator can reset your password from the Proxmox node if you lost it.
- **Resource limit increases**: Contact your administrator if you need more CPU, RAM, or disk than the configured limits allow.
- **Difficulties creating a virtual machine**: Reach out for issues with VM creation, configuration, or deployment.
- **Console access problems**: Contact your administrator for console connection or usage issues.
- **Permission issues**: Let your administrator know if you cannot access certain features or VMs.
- **Technical problems**: Report any errors, bugs, or unexpected behavior in the application.
- **Feature requests**: Suggest new ISOs, network bridges, or other resources through your administrator.

## Known limitations

The PVMSS application currently does not support:

- **Full resource reconfiguration**: While you can change CPU, memory, network cards and ISO for a stopped VM, some operations remain unavailable (for example, growing disks, changing the number of disks beyond administrator limits, or editing certain low-level Proxmox options).
- **LXC containers**: Only KVM/QEMU VMs are supported. LXC container creation is unavailable.
- **Snapshots**: VM snapshot creation and management are not available through PVMSS.
- **Backups**: VM backup and restore operations must be handled directly by administrators in Proxmox.
- **Live migration**: Moving VMs between nodes is not available via PVMSS.
- **Advanced networking**: Advanced networking features (VLANs, firewall rules, etc.) must be configured by administrators, even though multiple network cards and network models are supported in PVMSS.
- **Direct Proxmox access**: PVMSS is designed as a simplified interface and does not provide access to all Proxmox features.

## Security and privacy

- Console sessions are authenticated and session-based.
- Each user can view and manage only their own virtual machines.
- Administrator access is separate from user access and requires additional authentication.

## Tips and tricks

- **Quick VM startup**: Use the search page for fast start/stop actions without opening the VM details page.
- **Browser bookmarks**: Bookmark the PVMSS URL and specific VM detail pages for quick access.
- **Multiple windows**: You can open multiple VM console windows simultaneously to manage several VMs.
- **Language switching**: The application automatically detects your browser's language preference. Adjust your browser language settings to switch between French and English.
- **Keyboard shortcuts**: Most modern browsers support keyboard shortcuts in the console window (Ctrl+C, Ctrl+V for clipboard operations).
