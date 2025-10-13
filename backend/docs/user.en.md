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

- **Node**: Select the Proxmox node where the VM will be created (among the administrator-configured nodes).
- **Name and description**: Enter a unique name (alphanumeric characters, hyphens, and underscores only) and a description to identify your VM.
- **Operating system**: Choose an ISO image from a list defined by administrators to install the OS.
- **Resources**: Configure the required resources:
  - **CPU cores**: Number of processor cores (within administrator-defined limits)
  - **Memory (RAM)**: Amount of memory in MB (within administrator-defined limits)
  - **Disk size**: Storage capacity in GB (within administrator-defined limits)
  - **Network bridge**: Select the network bridge (VMBR) for network connectivity
- **Tags**: Add predefined tags to organize your VMs and make them easier to find.

**Important notes:**

- You can create only one VM at a time.
- Resource limits per VM (CPU, RAM, disk) are imposed by administrators.
- You cannot modify the resources of an existing VM after it has been created.

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

You can update certain VM properties:

- **Description**: Update the VM description
- **Tags**: Add or remove tags for better organization

**Note**: Hardware resources (CPU, RAM, disk) and the network bridge selection cannot be changed after the VM is created.

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

- **Resource modification**: You cannot change VM resources (CPU, memory, storage, network bridge) after creation. To adjust resources, create a new VM and migrate your data.
- **LXC containers**: Only KVM/QEMU VMs are supported. LXC container creation is unavailable.
- **Snapshots**: VM snapshot creation and management are not available through PVMSS.
- **Backups**: VM backup and restore operations must be handled directly by administrators in Proxmox.
- **Live migration**: Moving VMs between nodes is not available via PVMSS.
- **Advanced networking**: Only basic network bridge assignment is supported. Advanced networking features (VLANs, firewall rules, etc.) must be configured by administrators.
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
