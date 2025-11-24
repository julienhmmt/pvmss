# PVMSS administrator guide

This guide covers all administrative features and workflows available in PVMSS, including system configuration, user management, and application maintenance.

The PVMSS application administrator has complete access to all application features. There is no separate administrator role, no auditor or observer role. By navigating to the page <http://ip_or_domain-name/admin>, you will access the administration interface after validating the connection with the administrator password.

## Table of contents

- [Getting started guide](#getting-started-guide)
- [Application configuration](#application-configuration)
- [PVMSS vs Proxmox](#feature-comparison-pvmss-vs-proxmox-ve)
- [Logging and diagnostics](#logging-and-diagnostics)
- [Operational runbooks](#operational-runbooks)
- [Deployment checklist](#deployment-and-upgrade-checklist)
- [Security & access control](#security-and-access-control-recommendations)
- [Glossary](#glossary)
- [Known limitations](#known-limitations)

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

#### Example workflow: diagnose Proxmox connection and offline mode

1. Open `/admin/appinfo` and check the **Environment** badge:
   - `production` means normal online mode,
   - `development` means you are running a dev build,
   - `offline` means `PVMSS_OFFLINE=true` and Proxmox API calls are disabled by design.
2. Verify the **environment variables** table for `PROXMOX_URL`, `PROXMOX_VERIFY_SSL`, `PVMSS_ENV` and `PVMSS_OFFLINE` to ensure they match your expected deployment configuration.
3. In the **Proxmox information** section, check whether PVMSS detects a cluster or standalone mode and how many nodes are visible.
4. If the application reports offline mode or cannot reach Proxmox, review the backend logs on the PVMSS host (see "Logging and diagnostics" below) to identify connection errors or misconfiguration.
5. Once the configuration is fixed (environment variables, network connectivity, Proxmox permissions), restart PVMSS and refresh `/admin/appinfo` to confirm the expected environment and cluster status.

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

#### Network card speed (Network Speed)

End-users can optionally specify a **Network Speed** for each virtual network card on the *Create VM* and *Edit resources* pages. This setting is implemented using the Proxmox `rate` parameter on the NIC (for example: `net0=virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,rate=1000`).

- If the field is **left empty**, PVMSS does not set `rate` and the network card runs at **unlimited speed** (this is the Proxmox default).
- If a value is provided, it is interpreted as **Megabytes per second (MB/s)**.
- The allowed range is **1-10240 MB/s**.
- **Important**: `10240 MB/s` is a **hard limit imposed by Proxmox** for the `rate` parameter. PVMSS enforces the same limit and it is not possible to configure a higher value from the UI.

You can use this setting to gently cap the bandwidth of specific VMs, but it should not be considered a strict QoS or multi-tenant rate limiting mechanism.

#### VLAN configuration considerations

**Important**: Users can now specify VLAN tags (1-4096) when creating VMs. As an administrator, be aware of the following:

- **VLAN infrastructure**: Ensure your physical network switches and Proxmox bridges are properly configured for the VLAN IDs you allow users to use
- **Network isolation**: Incorrect VLAN configuration can cause VMs to become completely isolated from the network
- **VLAN range**: Only VLAN IDs 1-4096 are supported, with validation enforced at the application level
- **User guidance**: Provide clear documentation to users about which VLAN IDs they should use for their specific needs
- **Bridge configuration**: VLAN tagging works by appending `,tag=X` to the Proxmox network interface configuration (e.g., `net0=virtio,bridge=vmbr0,tag=100`)

**Potential issues to monitor**:

- Users specifying incorrect VLAN IDs that don't exist on your network infrastructure
- VMs losing network connectivity due to VLAN misconfiguration
- Multiple VMs using the same VLAN when they should be isolated

**Recommendations**:

- Document the available VLAN IDs and their purposes for your users
- Consider restricting bridge access if you need to control VLAN usage more strictly
- Monitor VM creation logs for VLAN-related issues
- Test VLAN configurations with a non-critical VM before allowing broader usage

#### MTU configuration considerations

End-users can optionally specify an **MTU (Maximum Transmission Unit)** per virtual network card on the *Create VM* and *Edit resources* pages.

- If the **MTU field is left empty**, PVMSS does **not** send any explicit `mtu=` parameter to Proxmox and the interface uses the **default MTU of 1500**.
- If a value is provided, it is interpreted as the MTU in **bytes** and must be in the range **576–9000**.
- Values outside this range are rejected at validation time with a clear error message.

From an administrator perspective:

- A wrong MTU can cause subtle issues (packet loss, fragmentation, timeouts), which are often hard to diagnose.
- Only environments with **carefully controlled networking** (for example jumbo frames and dedicated VLANs) should use non‑default MTUs.

**Strong recommendation**:

- Communicate clearly to users that **leaving MTU at 1500 is the safest and preferred choice**.
- Reserve custom MTU values for advanced scenarios explicitly validated by the network team.

### Resource limits management

This section allows you to manage limits for virtual machines, nodes, and users.

The **virtual machine limits** form allows you to define the minimum and maximum CPU sockets, CPU cores, memory amount, and virtual storage size that a new virtual machine can have. These limits are enforced both on the *Create VM* page and on the *Edit resources* form of the VM details page.

A second form, dedicated to **node limits**, allows you to define aggregate limits for each node (maximum total cores and memory that all VMs managed by PVMSS should consume on that node). The admin page also displays current aggregate usage per node with progress bars so you can quickly see when a node is close to its configured capacity.

Parameters for virtual machine limits are saved in a JSON format file (path: `{"limits": {"vm": {"cores": {"max": 2, "min": 1}, "disk": {"max": 10, "min": 1}, "ram": {"max": 4, "min": 1}, "sockets": {"max": 1, "min": 1}}}}`).

Parameters for node limits are saved in a JSON format file (path: `{"limits": {"nodes": {"node-name": {"cores": {"max": 8, "min": 2}, "ram": {"max": 32, "min": 2}, "sockets": {"max": 1, "min": 1}}}}}`).

Finally, a **User limits** section lets you define the **maximum number of VMs per user** (`max_vm_per_user`). This global limit is stored alongside the other settings (for example: `{"max_vm_per_user": 5}`) and is enforced when users try to create new VMs.

#### Example workflow: adjust limits when a user or node is saturated

1. From `/admin/vms` and the **Nodes** page, identify the node or user that is reaching CPU, memory, or VM count limits (for example, a node with very high usage or a user with many VMs).
2. Open `/admin/limits` and review the current **VM limits**, **node limits**, and **user limits**.
3. Decide whether you want to:
   - Increase or decrease the global per‑VM limits (CPU, RAM, disk),
   - Tighten or relax the aggregate limits for the affected node(s),
   - Adjust `max_vm_per_user` for all users.
4. Apply the changes and save the configuration. PVMSS will immediately enforce the new limits for subsequent VM creations and resource modifications.
5. Inform the impacted users of the new limits and, if necessary, ask them to shut down or delete VMs to comply with the updated policy.

### Global VM overview (Admin VMs)

This section (accessible at `/admin/vms`) provides a cluster-wide overview of all VMs known to PVMSS:

- A **summary badge** shows the total number of VMs across all nodes.
- A **table** lists each VM with its VMID, name, node, status, and tags.
- An **action button** on each row opens the VM details page, where lifecycle actions (start, stop, shutdown, reset, console) and resource edits (when the VM is stopped) are available.
- A **link to the search page** allows administrators to switch to more advanced filters.

Administrators cannot create new VMs from this page; it is strictly a monitoring and navigation view over existing VMs.

#### Example workflow: audit the VMs of a specific user

1. Open `/admin/userpool` and locate the user you want to audit. Note the Proxmox pool name (for example `pvmss_training`).
2. Ask the user to provide the names or tags of the VMs they have created, if needed.
3. Open `/admin/vms` and use the table, combined with the **Search** page, to locate these VMs by VMID, name, tags, or node.
4. For each VM, open the **VM details** page to review its configuration (CPU, memory, disks, network, EFI/TPM) and recent actions.
5. If you detect issues (for example, too many VMs on the same node or misconfigured resources), coordinate with the user and adjust limits or VM settings as appropriate.

### User management

This section allows you to manage PVMSS application users. Rather than storing users in a database, users are directly created in the Proxmox VE node, using the provided API.

A user account consists of a username, a realm, a password, and a role. The realm is `@pve` and is not modifiable. The role for all users is `PVEVMUser`.

So that each user can have their VMs in a single unique folder, a Proxmox pool is created for each user, whose name consists of `pvmss_` and the username.

For example, for the user `essai`, the pool will be `pvmss_essai` and their account will be `essai@pve`. It is not possible to modify the user account, but it is possible to delete it. This deletion will also delete the Proxmox pool and all associated VMs.

### User pool management

This section (accessible at `/admin/userpool`) provides a convenient interface to manage the **`pvmss_*` pools** and their corresponding users:

- A **creation form** lets you create a new user by specifying a username, password, and optional comment.
- Each created user is stored directly in Proxmox with the `PVEVMUser` role and associated with a dedicated pool named `pvmss_<username>`.
- A **table of existing pools** shows, for each user, the Proxmox pool name, optional comment, and the number of VMs in that pool.
- For each pool, you can **refresh** the VM count and **delete** the pool and its user account (which also deletes all associated VMs).

This page is the main entry point for administrators to manage self‑service users of PVMSS.

#### Example workflow: create a self-service user and review their VMs

1. Open `/admin/userpool` and create a new user by filling in the username, password, and optional comment, then submit the form.
2. Communicate the PVMSS URL and this new username/password to the user.
3. Ask the user to sign in to PVMSS, then use the **Create VM** page to create one or more VMs in their dedicated `pvmss_<username>` pool.
4. As an administrator, open `/admin/vms` to see the newly created VMs in the global list, or use the **Search** page to filter by VMID, name, tags, or node.
5. If needed, adjust **VM limits**, **node limits** or **user limits** on the **Limits** page to control how many resources this user and their VMs can consume.

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

#### Administrator capabilities on user virtual machines

- Administrator accounts are **not** associated with a dedicated PVMSS pool, and therefore do not have a "Create VM" workflow in the admin interface.
- To create self‑service VMs, administrators must either:
  - Log in as a regular PVMSS user (with its own `pvmss_*` pool), or
  - Create and manage VMs directly from the Proxmox interface.
- From the PVMSS admin interface, administrators can **view and manage user VMs** via the *Search* page and the *Admin VMs* page:
  - Open the VM details page for any VM.
  - Use the same lifecycle and resource actions exposed to end‑users (subject to Proxmox permissions).

## Feature comparison: PVMSS vs Proxmox VE

PVMSS is a focused self‑service interface on top of Proxmox VE. The table below summarizes where common actions are performed:

| Action | PVMSS | Proxmox VE GUI |
| --- | --- | --- |
| Create KVM/QEMU VM | Yes (self‑service, constrained by admin‑defined limits and presets) | Yes (full configuration options) |
| Create LXC container | No | Yes |
| Edit basic VM resources (CPU, RAM, disk count/size, network cards, ISO) | Yes (within UI and policy limits; some disk operations are not exposed) | Yes (full set of options) |
| Manage snapshots | No | Yes |
| Run backups / restores | No | Yes |
| Live migrate VMs between nodes | No | Yes |
| Configure advanced networking (VLANs, firewall rules, etc.) | Partially (choose bridge and NIC model only) | Yes (full networking stack) |
| Manage VM templates / cloning | No | Yes |
| Configure cloud-init | No | Yes |
| Manage users and permissions | Yes (create/delete PVMSS users and pools; relies on Proxmox roles) | Yes (full RBAC, realms, roles, ACLs) |

This comparison is not exhaustive but highlights that PVMSS intentionally exposes only a safe subset of Proxmox features for end‑users.

## Logging and diagnostics

PVMSS uses its own logging system (configured with the `LOG_LEVEL` environment variable). These logs are written locally by the PVMSS backend process and are **not** forwarded to Proxmox or any external logging system by default.

- **Scope of PVMSS logs**:
  - Application startup and shutdown events.
  - HTTP request handling and internal errors.
  - Proxmox API calls made by PVMSS and any associated failures.
  - Background workers (cache refresh, guest agent cache, etc.).
- **Not a Proxmox monitoring solution**:
  - Only errors and warnings returned by Proxmox during API calls appear in PVMSS logs.
  - PVMSS does **not** collect or expose full Proxmox cluster logs, syslog, or task history.
  - You must continue to use the Proxmox interface and its own logs to debug Proxmox issues, nodes, storage, or cluster services.

When investigating issues, always combine `/admin/appinfo`, the **Nodes** and **Limits** pages, and the local PVMSS logs on the host machine. For deeper Proxmox problems (cluster status, storage errors, HA, etc.), refer directly to Proxmox tools and logs.

## QEMU Guest Agent integration

PVMSS does not manage the installation of the **QEMU Guest Agent** inside VMs, but it exposes and consumes its status in several places:

- On the **VM details** page, a small badge labelled *“QEMU Guest Agent”* shows the last known agent state (Available / Unavailable / Unknown / Offline).
- The **Shutdown** action uses a short health check against the agent:
  - If the agent is clearly unavailable, PVMSS fails fast with a user-friendly message and suggests using **Stop** instead of waiting for long timeouts.
  - If the agent is available, PVMSS sends a graceful shutdown request and briefly polls the VM status to confirm that it stops.
- When `PVMSS_OFFLINE=true` or Proxmox is unreachable, PVMSS does not attempt any agent calls and shows an **Offline (PVMSS)** state in the badge.

From a logging perspective:

- Health checks against the QEMU agent are logged with structured fields (operation, node, VMID, result, duration, error message).
- Shutdown attempts record whether the agent pre-check passed, whether the shutdown completed within the expected window, or whether it was aborted because of offline mode.

Administrators should monitor these logs to detect recurring problems with guest configurations (for example missing or misconfigured agents on a specific OS image).

## Operational runbooks

### Runbook: a user cannot create a VM

1. Ask the user for the exact error message and which node or storage they selected.
2. Open `/admin/limits` and check:
   - The global **`max_vm_per_user`** value and the number of VMs already owned by this user (using `/admin/vms` or the Search page).
   - The per‑VM limits (CPU, RAM, disk) to confirm the requested configuration is within the allowed range.
   - The per‑node aggregate limits to see whether a node is already at or above its configured capacity.
3. Open the **Nodes** and **Storage** pages to verify that the selected node and storage are enabled and not marked offline or over capacity.
4. Open `/admin/appinfo` to confirm that PVMSS is **not** running in `offline` mode and that the Proxmox cluster is reachable.
5. Check the PVMSS logs on the host for quota, permission, or Proxmox API errors related to the user or to VM creation.
6. If necessary, adjust limits or node/storage configuration, then ask the user to retry the creation with the updated policy.

### Runbook: console does not work for multiple users

1. Confirm that the affected VMs are **running** and reachable directly from the Proxmox interface.
2. Check `/admin/appinfo` to ensure that PVMSS is not in `offline` mode and that Proxmox is reachable.
3. Verify your reverse proxy / TLS configuration and that WebSocket traffic to PVMSS is allowed according to your infrastructure standards.
4. Inspect PVMSS logs for console‑related or WebSocket‑related errors.
5. If the problem is limited to a single VM or user, review their Proxmox permissions and the VM's networking from the Proxmox GUI.

## Deployment and upgrade checklist

- Confirm that the target Proxmox VE version is supported (9.0 or newer).
- Configure mandatory environment variables: `PROXMOX_URL`, `PROXMOX_VERIFY_SSL`, `PVMSS_ENV`, `PVMSS_OFFLINE`, `PVMSS_SETTINGS_PATH`, `ADMIN_PASSWORD_HASH`, `LOG_LEVEL`.
- Start PVMSS and open `/admin/appinfo` to verify:
  - Environment mode (`development`, `production`, `offline`).
  - Proxmox URL and SSL verification settings.
  - Cluster vs standalone detection and visible node count.
- Configure, in order:
  - Nodes and storages to expose on the *Create VM* page.
  - ISO images available to users.
  - Network bridges (VMBR) and maximum number of network cards.
  - VM, node, and user limits on the **Limits** page.
- Create a temporary self‑service user in `/admin/userpool` and run an end‑to‑end test:
  - Sign in as this user.
  - Create a VM, start/stop it, open the console, then delete the VM.
- After upgrades, repeat `/admin/appinfo` checks and a short end‑to‑end test with a non‑critical test user.

## Security and access control recommendations

- Expose PVMSS only over HTTPS (typically behind a reverse proxy) and restrict access to trusted networks.
- Use dedicated Proxmox accounts for PVMSS administrators; avoid sharing the built‑in administrator password.
- Keep Proxmox permissions simple:
  - End‑users with `PVEVMUser` on their dedicated pool.
  - PVMSS administrators with `PVEAdmin` at the root of the Proxmox tree.
- Do not grant `PVEAdmin` to regular users or to accounts used only for self‑service.
- Regularly review user pools and disable or delete unused accounts and pools.

## Glossary

- **Node**: A Proxmox VE host that runs virtual machines.
- **VM (Virtual Machine)**: A KVM/QEMU guest managed by Proxmox and exposed in PVMSS.
- **Pool**: A Proxmox resource container (for example `pvmss_johndoe`) grouping a user's VMs.
- **Bridge / VMBR**: A Proxmox network bridge used to connect VMs to a network.
- **Tag**: A label applied to VMs (for example `env:prod`, `team:ml`, `promo:2025`) that can be used for filtering and search.
- **Offline mode**: A PVMSS mode where Proxmox API calls are disabled (for example when `PVMSS_OFFLINE=true`); self‑service operations like VM creation are unavailable.
- **Limits**: Configuration in `/admin/limits` that defines per‑VM, per‑node and per‑user resource boundaries.
- **Self‑service user**: A Proxmox user with a dedicated `pvmss_<username>` pool managed through `/admin/userpool`.

## Known limitations

- The PVMSS application is designed to work on Proxmox VE 9.0 servers and higher
- It is not possible to connect an external authentication system to the PVMSS application (OIDC, SAML, etc.)
- PVMSS supports both standalone Proxmox servers and Proxmox clusters, but advanced cluster operations (such as live migration, high availability configuration, or backup orchestration) must still be performed directly from the Proxmox interface.
- There is **no built-in VM templating system** or catalog of reusable VM models in PVMSS. VM templates and advanced cloning workflows must be managed directly from Proxmox.
- PVMSS does **not** currently integrate with **cloud-init**. Cloud-init user data, SSH keys injection, and similar guest customization features must be configured directly in Proxmox or inside the guest operating system.
- Administrators **cannot create VMs from the admin interface**; VM creation is only available in the self‑service user interface or directly in Proxmox.
