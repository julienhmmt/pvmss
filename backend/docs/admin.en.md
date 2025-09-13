# PVMSS Administrator Guide

This guide covers all administrative features and workflows available in PVMSS, including system configuration, user management, and application maintenance.

The PVMSS application administrator has complete access to all application features. There is no separate administrator role, no auditor or observer role. By navigating to the page <http://ip_or_domain-name/admin>, you will access the administration interface after validating the connection with the administrator password.

## Getting Started Guide

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

## Application Configuration

In this interface, several sections will be accessible through a vertical navigation menu on the left.

### Node Management

This section displays the list of all Proxmox VE hosts, with a display showing current CPU and memory consumption. Server status (Online, offline) is also displayed.

### Tag Management

This section allows you to manage tags used to categorize virtual machines. All tags created in PVMSS are displayed and can be deleted. A tag is immutable. The `pvmss` tag is a default tag and cannot be deleted.

Additionally, a counter of virtual machines per tag is displayed.

Parameters are saved in a JSON format file (path: `{"tags": ["pvmss","tag"]}`).

### Storage Management

This section allows you to manage storage used to store virtual machines. All storage supporting virtual disk files is displayed.

An "Enable" or "Disable" button allows you to select storage that will be used to store virtual machines.

The `*local*` storage is the default storage and cannot be used. Parameters are saved in a JSON format file (path: `{"enabled_storages": ["storage_name"]}`).

### ISO Management

This section allows you to manage ISOs used to create virtual machines. The interface does not allow adding or removing ISO files from storage, but selecting ISOs that will be available for virtual machine creation. All storage allowing ISO file storage is parsed and only ISO files are displayed (a filter is applied, implemented in the code).

An "Enable" or "Disable" button allows you to select ISOs that will be available for virtual machine creation. It is not possible to rename ISO files through the interface.

Parameters are saved in a JSON format file (path: `{"isos": ["storage_name:iso/image_name.iso"]}`).

### Network Bridge Management (VMBR)

This section allows you to manage network bridges used for virtual machines. All network bridges created in the Proxmox host are displayed. "OpenVSwitch" type network bridges are not displayed.

An "Enable" or "Disable" button allows you to select network bridges that will be used for virtual machines.

Parameters are saved in a JSON format file (path: `{"vmbrs": ["network_bridge_name"]}`).

### Resource Limits Management

This section allows you to manage limits for virtual machines as well as for nodes.

The form for virtual machine limits allows you to define the minimum and maximum CPU cores, memory amount, and virtual storage size that a new virtual machine can have.

A second form, dedicated to node limits, allows you to define the minimum and maximum CPU cores and memory amount that a node can support. This form allows you to define global limits for nodes.

Parameters for virtual machine limits are saved in a JSON format file (path: `{"limits": {"vm": {"cores": {"max": 2,"min": 1},"disk": {"max": 10,"min": 1},"ram": {"max": 4,"min": 1},"sockets": {"max": 1,"min": 1}}}}`).

Parameters for node limits are saved in a JSON format file (path: `{"limits": {"nodes": {"node-name": {"cores": {"max": 8,"min": 2},"ram": {"max": 32,"min": 2},"sockets": {"max": 1,"min": 1}}}}}`).

### User Management

This section allows you to manage PVMSS application users. Rather than storing users in a database, users are directly created in the Proxmox VE node, using the provided API.

A user account consists of a username, a realm, a password, and a role. The realm is `@pve` and is not modifiable. The role for all users is `PVEVMUser`.

So that each user can have their VMs in a single unique folder, a Proxmox pool is created for each user, whose name consists of `pvmss_` and the username.

For example, for the user `essai`, the pool will be `pvmss_essai` and their account will be `essai@pve`. It is not possible to modify the user account, but it is possible to delete it. This deletion will also delete the Proxmox pool and all associated VMs.

## Known Limitations

- The PVMSS application is designed to work on Proxmox VE 8.0 servers and higher
- It is not possible to connect an external authentication system to the PVMSS application (OIDC, SAML, etc.)
- Only one Proxmox node is supported. If you want to manage multiple Proxmox nodes, you will need to create a PVMSS application instance for each Proxmox node.
