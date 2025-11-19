# PVMSS administrator guide

This guide covers all administrative features and workflows available in PVMSS, including system configuration, user management, and application maintenance.

The PVMSS application administrator has complete access to all application features. There is no separate administrator role, no auditor or observer role. By navigating to the page <http://ip_or_domain-name/admin>, you will access the administration interface after validating the connection with the administrator password.

## Getting started guide

1. Access the administration panel on `/admin` (administrator password required)
2. Create tags to categorize virtual machines
3. Review and configure the following options that you want to make available when creating virtual machines:
    - available storage
    - ISO images
    - network bridges (vmbr)
    - resource limits (CPU, RAM, and disk size)
4. Create as many user accounts as needed
5. Communicate to your users the availability of the PVMSS application so they can start creating their VMs
6. Monitor PVMSS application logs to detect any issues

## Application configuration

In this interface, several sections will be accessible through a vertical navigation menu on the left.

### Application information (App info)

This section (accessible at `/admin/appinfo`) provides a read-only overview of the PVMSS instance:

- **Build information**: application version, Go version, operating system and architecture.
- **Environment**: current mode (`development`, `production`, or `offline`) based on the `PVMSS_ENV` and `PVMSS_OFFLINE` environment variables.
- **Proxmox cluster status**: whether PVMSS is connected to a single node or to a Proxmox cluster, cluster name, and number of nodes when available.
- **Environment variables (safe subset)**: non-sensitive configuration such as `PROXMOX_URL`, `PROXMOX_VERIFY_SSL`, `PVMSS_ENV`, `PVMSS_OFFLINE`, `PVMSS_SETTINGS_PATH`.

Use this page to quickly validate that the instance is running with the expected configuration and connected to the correct Proxmox environment.

### Node management

This section displays the list of all Proxmox VE hosts, with a display showing current CPU and memory consumption. Server status (Online, offline) is also displayed.

Node information is refreshed by a background worker every 30 seconds (value defined by `NodeCacheRefreshInterval`). The administration pages only read this local cache: the display is therefore instantaneous even on large Proxmox clusters, and a slow node no longer impacts navigation. The data remains available offline as long as the last update is not too old.

### Tag management

This section allows you to manage tags used to categorize virtual machines. All tags created in PVMSS are displayed and can be deleted. A tag is immutable. The `pvmss` tag is a default tag and cannot be deleted.

Additionally, a counter of virtual machines per tag is displayed.

Parameters are saved in a JSON format file (path: `{"tags": ["pvmss","tag"]}`).

### Storage management

This section allows you to manage Proxmox storage used to store virtual machines. All storage backends that can host VM disk images are displayed, grouped by node when PVMSS is connected to a cluster.

An "Enable" or "Disable" button allows you to select which `(node, storage)` pairs will be offered on the *Create VM* page. Enabling a storage here does not modify the underlying Proxmox configuration; it only controls what the self‑service UI exposes to users.

Parameters are saved in a JSON format file (path: `{"enabled_storages": ["node-name:storage_name"]}`).

At the bottom of the page, a **Disk configuration** section lets you define the **maximum number of disks per VM** (`MaxDiskPerVM`). This value controls how many disk slots are available on the *Create VM* page, in addition to the technical limits of each disk bus (VirtIO, SCSI, SATA, IDE).

### ISO management

This section allows you to manage ISOs used to create virtual machines. The interface does not allow adding or removing ISO files from storage, but selecting ISOs that will be available for virtual machine creation. All storage allowing ISO file storage is parsed and only ISO files are displayed (a filter is applied, implemented in the code).

An "Enable" or "Disable" button allows you to select ISOs that will be available for virtual machine creation. It is not possible to rename ISO files through the interface.

Parameters are saved in a JSON format file (path: `{"isos": ["storage_name:iso/image_name.iso"]}`).

### Network bridge management (VMBR)

This section allows you to manage network bridges used for virtual machines. All network bridges created on the Proxmox hosts are displayed. "OpenVSwitch" type network bridges are not displayed.

An "Enable" or "Disable" button allows you to select which `(node, bridge)` pairs will be available when creating or editing virtual machines.

Parameters are saved in a JSON format file (path: `{"vmbrs": ["node-name:network_bridge_name"]}`).

At the top of the page, a **Network cards configuration** section lets you define the **maximum number of network cards per VM** (`MaxNetworkCards`). This value controls how many network card sections are displayed on the *Create VM* page. It is currently clamped between 1 and 10.

### Resource limits management

This section allows you to manage limits for virtual machines, nodes, and users.

The **virtual machine limits** form allows you to define the minimum and maximum CPU sockets, CPU cores, memory amount, and virtual storage size that a new virtual machine can have. These limits are enforced both on the *Create VM* page and on the *Edit resources* form of the VM details page.

A second form, dedicated to **node limits**, allows you to define aggregate limits for each node (maximum total cores and memory that all VMs managed by PVMSS should consume on that node). The admin page also displays current aggregate usage per node with progress bars so you can quickly see when a node is close to its configured capacity.

Parameters for virtual machine limits are saved in a JSON format file (path: `{"limits": {"vm": {"cores": {"max": 2, "min": 1}, "disk": {"max": 10, "min": 1}, "ram": {"max": 4, "min": 1}, "sockets": {"max": 1, "min": 1}}}}`).

Parameters for node limits are saved in a JSON format file (path: `{"limits": {"nodes": {"node-name": {"cores": {"max": 8, "min": 2}, "ram": {"max": 32, "min": 2}, "sockets": {"max": 1, "min": 1}}}}}`).

Finally, a **User limits** section lets you define the **maximum number of VMs per user** (`max_vm_per_user`). This global limit is stored alongside the other settings (for example: `{"max_vm_per_user": 5}`) and is enforced when users try to create new VMs.

### User management

This section allows you to manage PVMSS application users. Rather than storing users in a database, users are directly created in the Proxmox VE node, using the provided API.

A user account consists of a username, a realm, a password, and a role. The realm is `@pve` and is not modifiable. The role for all users is `PVEVMUser`.

So that each user can have their VMs in a single unique folder, a Proxmox pool is created for each user, whose name consists of `pvmss_` and the username.

For example, for the user `essai`, the pool will be `pvmss_essai` and their account will be `essai@pve`. It is not possible to modify the user account, but it is possible to delete it. This deletion will also delete the Proxmox pool and all associated VMs.

### Administrator accounts

In addition to the built-in administrator account (configured with `ADMIN_PASSWORD_HASH`), PVMSS supports administrator accounts created directly in Proxmox VE. This allows multiple administrators to access the PVMSS admin interface using their Proxmox credentials.

#### Creating an administrator account

To create an administrator account, use the Proxmox command line:

```bash
# Create the user in the 'pve' realm (required)
pveum user add admin-user@pve

# Set the password
pveum passwd admin-user@pve

# Grant PVEAdmin role at the root level (required for full admin access)
pveum aclmod / -user admin-user@pve -role PVEAdmin
```

**Important notes:**

- **Realm**: Must be `@pve`, not `@pam`. The `pve` realm is required for proper integration with PVMSS authentication.
- **Role**: Must be `PVEAdmin` only. This role provides full administrative access to Proxmox and grants admin access in PVMSS.
- **No pool creation**: Unlike regular users, administrators do not get a dedicated pool.

After creation, the user can log in to PVMSS using `admin-user@pve` credentials and will automatically have access to the admin interface.

## Known limitations

- The PVMSS application is designed to work on Proxmox VE 9.0 servers and higher
- It is not possible to connect an external authentication system to the PVMSS application (OIDC, SAML, etc.)
- PVMSS supports both standalone Proxmox servers and Proxmox clusters, but advanced cluster operations (such as live migration, high availability configuration, or backup orchestration) must still be performed directly from the Proxmox interface.
