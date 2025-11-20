# Proxmox – Permissions & APIs used by PVMSS

## 1. Document goals

- **Main purpose**
  - Describe precisely which Proxmox APIs are used by PVMSS.
  - Link these APIs to the required **Proxmox privileges**.
  - Define **recommended roles** (`PVMSS_Service`, `PVMSS_Admin`, `PVMSS_User`, etc.).
- **Target audience**
  - Proxmox administrators who need to configure permissions for PVMSS.
  - LLMs/agents that need to answer questions like “Can this role do X?”.
- **Sources of truth**
  - PVMSS code, especially:
    - `backend/proxmox/*.go` (Proxmox API access)
    - `backend/handlers/*.go` (business handlers: VM creation, actions, console, pools, users, etc.)
  - Proxmox VE documentation:
    - User Management: [https://pve.proxmox.com/pve-docs/chapter-pveum.html](https://pve.proxmox.com/pve-docs/chapter-pveum.html)
    - Privileges & Roles: [https://pve.proxmox.com/wiki/User_Management](https://pve.proxmox.com/wiki/User_Management)
    - Proxmox API Viewer: [https://pve.proxmox.com/pve-docs/api-viewer/](https://pve.proxmox.com/pve-docs/api-viewer/)

> API names and privilege names are aligned with the Proxmox API Viewer, but the exact list of minimal privileges must always be validated against the Proxmox documentation, as some combinations depend on the PVE version. We use **Proxmox 9.1**.

---

## 2. Actors and roles on the Proxmox side

### 2.1 PVMSS service account (`PVMSS_Service`)

- Used by the PVMSS backend through an **API token** (variables `PROXMOX_API_TOKEN_NAME` / `PROXMOX_API_TOKEN_VALUE`).
- Used to:
  - List and monitor **cluster, nodes, VMs, storages**.
  - Create / modify / delete **VMs**.
  - Read **network bridges** (vmbr and SDN) and **ISOs**.
  - Manage **users and pools** on the Proxmox side.

Sensitive account with high privileges on the whole cluster. It is recommended to restrict its usage to **automated operations only** and never use it for manual tasks.

On Proxmox, this account is created with the following privileges:

```bash
pveum roleadd PVMSS_Service -privs "Sys.Audit \
  VM.Audit VM.Allocate VM.PowerMgmt VM.Console \
  VM.Config.CPU VM.Config.Memory VM.Config.Disk VM.Config.Network VM.Config.Options VM.Config.Cloudinit \
  Datastore.Audit Datastore.AllocateSpace \
  Pool.Allocate User.Modify Permissions.Modify"

pveum useradd pvmss-svc@pve \
  -comment "PVMSS service account" \
  -enable 1

pveum user token add pvmss-svc@pve pvmss-service-token --privsep 0
┌──────────────┬──────────────────────────────────────┐
│ key          │ value                                │
╞══════════════╪══════════════════════════════════════╡
│ full-tokenid │ pvmss-svc@pve!pvmss-service-token    │
├──────────────┼──────────────────────────────────────┤
│ info         │ {"privsep":"0"}                      │
├──────────────┼──────────────────────────────────────┤
│ value        │ secret_value_to_store_in_env         │
└──────────────┴──────────────────────────────────────┘
```

### 2.2 PVMSS administrators (`PVMSS_Admin`)

- Human administrators of PVMSS.
- Typical access:
  - Manage VMs for all PVMSS users.
  - View cluster/node resources.
  - Manage Proxmox users created by PVMSS.
  - Manage Proxmox pools associated with users created by PVMSS.
  - Do **not** manage existing user accounts outside the scope of PVMSS.
  - Do **not** access global system settings unrelated to VMs.

On Proxmox, this account is created with the following privileges:

```bash
pveum roleadd PVMSS_Admin -privs "Sys.Audit VM.Audit VM.PowerMgmt VM.Console VM.Config.CPU VM.Config.Memory VM.Config.Disk VM.Config.Network VM.Config.Options VM.Config.Cloudinit Datastore.Audit Datastore.AllocateSpace Pool.Allocate User.Modify Permissions.Modify"

pveum useradd pvmss-admin1@pve \
  -comment "PVMSS administrator <name>" \
  -enable 1

pveum aclmod / -user pvmss-admin1@pve -role PVMSS_Admin -propagate 1
```

### 2.3 PVMSS end users (`PVMSS_User`)

- End users consuming VMs through PVMSS.
- Typical access:
  - See **only** the VMs in their pools.
  - Start / stop / reboot their own VMs.
  - Access the **noVNC console**.
  - Edit a limited subset of VM settings (e.g. description, ISO, cloud-init options).
  - Do **not** access global system settings unrelated to VMs.
  - Do **not** access cluster or node configuration.

On Proxmox, the `PVMSS_User` role can be created with this line:

```bash
pveum roleadd PVMSSUser -privs "VM.Audit VM.PowerMgmt VM.Console \
  VM.Config.CDROM Datastore.Audit Pool.Audit"
```

---

## 3. Proxmox API families used by PVMSS

This section gives a synthetic view of the API domains used by PVMSS operations.

> **Functional scope**: as of now, PVMSS **does not handle** the following Proxmox features: backups/snapshots, LXC/containers, SDN via `/cluster/sdn`, Proxmox firewall, High Availability (HA), storage replication, or specific Ceph/RBD management. These domains must be documented and administered separately on the Proxmox side.

- **Authentication & user management**
  - `/access/ticket`, `/access/users`, `/access/password`, `/access/roles`, `/access/acl`, `/pools`.
- **Cluster & nodes**
  - `/cluster/status`, `/nodes`, `/nodes/{node}/status`.
- **VM management (QEMU)**
  - `/nodes/{node}/qemu`, `/nodes/{node}/qemu/{vmid}/config`, `/status/current`, `/status/{action}`, VM deletion.
  - Guest agent: `/nodes/{node}/qemu/{vmid}/agent/network-get-interfaces`.
- **Storage**
  - `/storage`, `/nodes/{node}/storage`, `/nodes/{node}/storage/{storage}/content`.
- **Network (vmbr bridges)**
  - `/nodes/{node}/network` (bridges created by SDN appear here as `vmbrX` interfaces).
- **Console / VNC**
  - `/nodes/{node}/qemu/{vmid}/vncproxy`, `/nodes/{node}/qemu/{vmid}/vncwebsocket`.

The following sections detail these usages per **PVMSS feature**, with a stable structure that is easy for an LLM to consume.

---

## 4. Authentication & user management

### 4.1 PVMSS user authentication

- **Feature ID**: `auth.user_ticket`
- **Description**: Authenticate a Proxmox user from PVMSS (PVMSS login) and retrieve a ticket + CSRF token.
- **PVMSS files**:
  - `backend/proxmox/access.go` → `CreateTicket`
  - `backend/handlers/auth.go`
- **Proxmox endpoints**:
  - `POST /access/ticket`
- **Main parameters** (see API Viewer):
  - `username` (string, e.g. `user@pve` or `user@pam`)
  - `password` (string)
  - `otp` (string, optional)
  - `realm` (string, optional)
  - `path` / `privs` (optional, for privilege checks)
- **Required privileges**:
  - No explicit privilege is required to log in, but the user must exist and be **enabled=1**.
- **Roles involved**:
  - `PVMSS_User`, `PVMSS_Admin` (human accounts).

### 4.2 Proxmox user creation / update

- **Feature ID**: `auth.manage_users`
- **Description**: Create a Proxmox user corresponding to a PVMSS user, and update its password.
- **PVMSS files**:
  - `backend/proxmox/access.go` → `EnsureUser`, `UpdateUserPassword`
  - `backend/handlers/user_pool.go`, `backend/handlers/admin.go` (depending on integration)
- **Proxmox endpoints**:
  - `GET /access/users/{userid}` (existence check)
  - `POST /access/users` (creation)
  - `PUT /access/password` (password change)
- **Likely Proxmox privileges** (to confirm against PVE docs):
  - `User.Modify` (user management)
  - `Sys.Audit` (list/view users)
- **Roles involved**:
  - `PVMSS_Service` (if user creation is automated by the backend).
  - `PVMSS_Admin` (if PVMSS admins manage users directly in Proxmox).

### 4.3 Pools & ACL management

- **Feature ID**: `auth.manage_pools_acl`
- **Description**: Create Proxmox pools and assign users to those pools with a dedicated PVMSS role.
- **PVMSS files**:
  - `backend/proxmox/access.go` → `EnsurePool`, `EnsurePoolACL`, `EnsureRole`
  - `backend/handlers/user_pool.go`
- **Proxmox endpoints**:
  - `GET /pools/{poolid}`
  - `POST /pools`
  - `GET /access/roles/{roleid}`
  - `POST /access/roles`
  - `PUT /access/acl`
- **Likely Proxmox privileges**:
  - `Pool.Allocate` (pool creation & management)
  - `Permissions.Modify` (ACL management)
  - `Sys.Audit` (global view of roles & permissions)
- **Roles involved**:
  - `PVMSS_Service` (backend preparing pools + ACLs).
  - `PVMSS_Admin` (manual administration on top).

---

## 5. Cluster & nodes

### 5.1 Cluster vs standalone detection

- **Feature ID**: `cluster.status`
- **Description**: Determine if Proxmox is running in cluster mode, cluster name, number of nodes.
- **PVMSS files**:
  - `backend/proxmox/cluster.go` → `GetClusterStatus`, `GetClusterStatusResty`
  - `backend/handlers/admin_appinfo.go`
- **Proxmox endpoints**:
  - `GET /cluster/status`
- **Likely Proxmox privileges**:
  - `Sys.Audit` (read cluster and node information).
- **Roles involved**:
  - `PVMSS_Service`, `PVMSS_Admin`.

### 5.2 List and details of nodes

- **Feature ID**: `nodes.list_and_details`
- **Description**: List nodes and retrieve their status & capacity (CPU/RAM/disk) for admin screens and VM creation.
- **PVMSS files**:
  - `backend/proxmox/nodes.go` → `GetNodeNames{,Resty}`, `GetOnlineNodeNamesResty`, `GetNodeDetails{,Resty}`
  - `backend/handlers/admin.go`, `backend/handlers/vm_create.go`, `backend/handlers/admin_vms.go`
- **Proxmox endpoints**:
  - `GET /nodes`
  - `GET /nodes/{node}/status`
- **Likely Proxmox privileges**:
  - `Sys.Audit`
- **Roles involved**:
  - `PVMSS_Service`, `PVMSS_Admin`.

---

## 6. VM management (QEMU)

### 6.1 Listing and reading VM configuration

- **Feature ID**: `vm.list_and_read_config`
- **Description**:
  - List all VMs (user profile, admin views).
  - Read a VM’s detailed configuration (CPU, RAM, disks, network, tags, EFI, TPM, etc.).
- **PVMSS files**:
  - `backend/proxmox/vms.go` → `GetVMsResty`, `GetVMsForNodeResty`, `GetVMConfigResty`, `GetVMCurrentResty`, `GetGuestAgentNetworkInterfaces`
  - `backend/handlers/profile.go`, `backend/handlers/admin_vms.go`, `backend/handlers/vm_details.go`, `backend/state/manager.go`
- **Proxmox endpoints**:
  - `GET /nodes` (to iterate over nodes)
  - `GET /nodes/{node}/qemu` (VM list for a node)
  - `GET /nodes/{node}/qemu/{vmid}/config`
  - `GET /nodes/{node}/qemu/{vmid}/status/current`
  - `GET /nodes/{node}/qemu/{vmid}/agent/network-get-interfaces`
- **Likely Proxmox privileges**:
  - `VM.Audit` (read VMs)
  - `Sys.Audit` (read nodes)
  - Guest agent: no extra privilege, but the VM must have QEMU guest agent configured.
- **Roles involved**:
  - `PVMSS_Service` (global read, RAM cache, aggregation).
  - `PVMSS_Admin` (full admin view).
  - `PVMSS_User` (limited to VMs in their pools via ACLs).

### 6.2 VM creation

- **Feature ID**: `vm.create`
- **Description**: Create a new VM with CPU, RAM, disks, network, ISO, EFI, TPM, etc.
- **PVMSS files**:
  - `backend/handlers/vm_create.go`
  - `backend/proxmox/vms.go` → `GetNextVMIDResty` (determine next VMID)
- **Typical Proxmox endpoints** (based on `chapter-qm.html` in the admin guide):
  - `GET /nodes/{node}/qemu` (compute next VMID through VM list)
  - `POST /nodes/{node}/qemu` (VM creation, initial configuration: cores, memory, disks, net0, bios=ovmf, tpmstate0, etc.)
  - Optionally follow-up `POST /nodes/{node}/qemu/{vmid}/config` calls to fine-tune options.
- **Likely Proxmox privileges**:
  - `VM.Allocate` (VM creation)
  - `VM.Config.CPU`, `VM.Config.Memory`, `VM.Config.Disk`, `VM.Config.Network`, `VM.Config.Options`, `VM.Config.Cloudinit` (depending on which options PVMSS exposes)
  - `Datastore.AllocateSpace` (disk creation on selected storages)
  - `Sys.Audit` (read nodes)
- **Roles involved**:
  - `PVMSS_Service` (platform-level creation).
  - `PVMSS_Admin` (if admins can create VMs manually via PVMSS).

### 6.3 VM power / lifecycle actions

- **Feature ID**: `vm.power_actions`
- **Description**: Start, stop, shutdown, reboot, reset VMs.
- **PVMSS files**:
  - `backend/proxmox/vms.go` → `VMActionResty`
  - `backend/handlers/vm_actions.go`, `backend/handlers/vm_delete.go`
- **Proxmox endpoints**:
  - `POST /nodes/{node}/qemu/{vmid}/status/start`
  - `POST /nodes/{node}/qemu/{vmid}/status/stop`
  - `POST /nodes/{node}/qemu/{vmid}/status/shutdown`
  - `POST /nodes/{node}/qemu/{vmid}/status/reboot`
  - `POST /nodes/{node}/qemu/{vmid}/status/reset`
- **Likely Proxmox privileges**:
  - `VM.PowerMgmt`
  - `VM.Audit` (VM read)
- **Roles involved**:
  - `PVMSS_Service`, `PVMSS_Admin`, `PVMSS_User` (restricted to VMs they are allowed to see via ACLs).

### 6.4 VM configuration updates (resources, tags, network, EFI/TPM)

- **Feature ID**: `vm.update_config`
- **Description**: Update description, tags, CPU, RAM, disks, network (net0..netN), EFI, TPM, etc.
- **PVMSS files**:
  - `backend/proxmox/vms.go` → `UpdateVMConfigResty`
  - `backend/handlers/vm_details.go`, `backend/handlers/vm_actions.go`
- **Proxmox endpoint**:
  - `POST /nodes/{node}/qemu/{vmid}/config`
- **Likely Proxmox privileges**:
  - `VM.Config.CPU`
  - `VM.Config.Memory`
  - `VM.Config.Disk`
  - `VM.Config.Network`
  - `VM.Config.Options`
  - `VM.Config.Cloudinit` (if used)
- **Roles involved**:
  - `PVMSS_Service`, `PVMSS_Admin`.
  - Optionally `PVMSS_User` if you want to allow limited changes (e.g. only `VM.Config.Options`).

### 6.5 VM deletion

- **Feature ID**: `vm.delete`
- **Description**: Cleanly stop a VM and delete it.
- **PVMSS files**:
  - `backend/handlers/vm_delete.go`
  - `backend/proxmox/vms.go` → `VMActionResty`, `DeleteVMResty`
- **Proxmox endpoints**:
  - `POST /nodes/{node}/qemu/{vmid}/status/stop` (or `shutdown`)
  - `DELETE /nodes/{node}/qemu/{vmid}`
- **Likely Proxmox privileges**:
  - `VM.Allocate` (deletion)
  - `VM.PowerMgmt`
  - `Datastore.AllocateSpace` (disk space cleanup, depending on Proxmox behavior)
- **Roles involved**:
  - `PVMSS_Service`, `PVMSS_Admin`.

---

## 7. Storage & ISOs

### 7.1 Storage listing and status

- **Feature ID**: `storage.list`
- **Description**: List available storages, their type, usage, and the ones visible from a given node.
- **PVMSS files**:
  - `backend/proxmox/storage.go` → `GetStoragesResty`, `GetNodeStoragesResty`
  - `backend/handlers/storage.go`, `backend/handlers/vm_create.go`
- **Proxmox endpoints**:
  - `GET /storage`
  - `GET /nodes/{node}/storage`
- **Likely Proxmox privileges**:
  - `Datastore.Audit`
  - `Sys.Audit` (read nodes)
- **Roles involved**:
  - `PVMSS_Service`, `PVMSS_Admin`.

### 7.2 ISO and storage content listing

- **Feature ID**: `storage.list_content`
- **Description**: List ISOs and other content (images, templates) of a storage.
- **PVMSS files**:
  - `backend/proxmox/iso.go` → `GetISOListResty`, `GetAllStorageContentResty`
  - `backend/handlers/settings_iso.go`, `backend/handlers/vm_create.go`
- **Proxmox endpoint**:
  - `GET /nodes/{node}/storage/{storage}/content`
- **Likely Proxmox privileges**:
  - `Datastore.Audit`
- **Roles involved**:
  - `PVMSS_Service`, `PVMSS_Admin`.

### 7.3 Volume create/delete (optional / future)

- **Feature ID**: `storage.create_delete_volume`
- **Description**: Create or delete volumes (disks) for VMs managed by PVMSS.
  - **Status**: as of **PVMSS 0.2.0**, these endpoints are not called by the code. This section is a documentation base for future features (ISO upload, direct volume management).
- **Typical Proxmox endpoints**:
  - `POST /nodes/{node}/storage/{storage}/content` (upload/create volume)
  - `DELETE /nodes/{node}/storage/{storage}/content/{volid}` (delete volume)
- **Likely Proxmox privileges**:
  - `Datastore.AllocateSpace`
  - Optionally `Datastore.Allocate` (depending on exact use case)
- **Roles involved**:
  - `PVMSS_Service`, `PVMSS_Admin`.

---

## 8. Network (vmbr bridges)

### 8.1 Reading network interfaces and bridges

- **Feature ID**: `network.list_bridges`
- **Description**: List network interfaces of a node and filter bridge interfaces (`vmbrX`) that can be used when creating/updating VMs.
- **PVMSS files**:
  - `backend/proxmox/vmbr.go` → `GetVMBRsResty`, `GetAllNetworkInterfacesResty`
  - `backend/handlers/vmbr.go`, `backend/handlers/vm_create.go`, `backend/handlers/vm_details.go`
- **Proxmox endpoint**:
  - `GET /nodes/{node}/network`
- **Likely Proxmox privileges**:
  - `Sys.Audit` (read node network configuration)
- **Roles involved**:
  - `PVMSS_Service`, `PVMSS_Admin`.

### 8.2 Advanced network management (optional)

If PVMSS were to create/modify bridges:

- **Proxmox endpoints**:
  - `POST /nodes/{node}/network`
  - `DELETE /nodes/{node}/network/{iface}`
- **Likely Proxmox privileges**:
  - `Sys.Modify` or specific network privileges (to be checked in Proxmox docs).
- **Roles involved**:
  - Most likely **restricted** to a very privileged role (equivalent to `PVEAdmin`).

---

## 9. noVNC console / VNC proxy

### 9.1 Getting a VNC ticket and WebSocket port

- **Feature ID**: `console.vncproxy`
- **Description**: Generate a VNC ticket and WebSocket port in order to open the noVNC console for a VM.
- **PVMSS files**:
  - `backend/proxmox/vnc.go` → `GetVNCProxy`
  - `backend/handlers/vm_console_api.go`, `vm_console_websocket.go`, `vm_console_helpers.go`
- **Proxmox endpoint**:
  - `POST /nodes/{node}/qemu/{vmid}/vncproxy`
- **Likely Proxmox privileges**:
  - `VM.Console` (console access)
  - `VM.Audit` (VM read)
- **Roles involved**:
  - `PVMSS_Service` (if the platform provides an HTTP/WS proxy on the backend).
  - `PVMSS_Admin`, `PVMSS_User` (console access to their own VMs via temporary tickets).

### 9.2 noVNC WebSocket connection

- **Feature ID**: `console.websocket`
- **Description**: Establish the WebSocket connection between the browser and Proxmox (either directly or via a PVMSS HTTP proxy).
- **Proxmox endpoint**:
  - `GET /nodes/{node}/qemu/{vmid}/vncwebsocket?port={port}&vncticket={ticket}` (called by noVNC)
- **Proxmox privileges**:
  - Same as VNC ticket creation (`VM.Console`).
- **Roles involved**:
  - `PVMSS_Admin`, `PVMSS_User` (through their own authentication and temporary tickets).

---

## 10. Recommended roles and privilege mapping

> **IMPORTANT**: The lists below are recommendations that must be refined using official Proxmox documentation (User Management + pveum). The goal is to cover PVMSS features with a reasonable minimum set of privileges.

### 10.1 Role `PVMSS_Service` (technical API account)

- **Goal**: non-interactive account used by the backend.
- **Recommended scope**: path `/` with propagation (simpler), or a restricted subtree depending on your multi-tenant architecture.
- **Typical privileges**:
  - Cluster / nodes
    - `Sys.Audit`
  - VMs
    - `VM.Audit`
    - `VM.Allocate`
    - `VM.PowerMgmt`
    - `VM.Console` (useful for debugging console through proxy)
    - `VM.Config.CPU`, `VM.Config.Memory`, `VM.Config.Disk`, `VM.Config.Network`, `VM.Config.Options`, `VM.Config.Cloudinit`
  - Storage
    - `Datastore.Audit`
    - `Datastore.AllocateSpace`
  - Pools / ACL / users (if managed by the platform)
    - `Pool.Allocate`
    - `User.Modify`
    - `Permissions.Modify`

### 10.2 Role `PVMSS_Admin`

- **Goal**: human administrators of PVMSS.
- **Recommended scope**: `/` or a subtree dedicated to PVMSS resources.
- **Typical privileges**:
  - All `PVMSS_Service` privileges related to VMs + storage + cluster read.
  - `Pool.Allocate`, `User.Modify`, `Permissions.Modify` if admins manage pools/users.
  - Avoid very broad system privileges (e.g. `Sys.Modify`, `Realm.Allocate`) unless strictly necessary.

### 10.3 Role `PVMSS_User`

- **Goal**: PVMSS end users, limited to their own VMs.
- **Recommended scope**: apply the role on `/pool/{poolid}` with `propagate=1` for each dedicated pool.
- **Typical privileges**:
  - `VM.Audit` (see their own VMs)
  - `VM.Console` (console)
  - `VM.PowerMgmt` (start/stop/reboot their VMs)
  - (Optional) some `VM.Config.*` if you want to allow limited configuration (for example `VM.Config.Options`).

---

## 11. Machine-friendly summary (for LLMs)

This section provides a more structured format that agents can easily parse.

### 11.1 Dictionary `features → endpoints → privileges`

- **Feature** `auth.user_ticket`
  - `endpoints`: `POST /access/ticket`
  - `min_privileges`: `[]` (simple login)
  - `roles`: `[PVMSS_User, PVMSS_Admin]`

- **Feature** `auth.manage_users`
  - `endpoints`: `GET /access/users/{userid}`, `POST /access/users`, `PUT /access/password`
  - `min_privileges`: `[User.Modify, Sys.Audit]`
  - `roles`: `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `auth.manage_pools_acl`
  - `endpoints`: `GET /pools/{poolid}`, `POST /pools`, `GET /access/roles/{roleid}`, `POST /access/roles`, `PUT /access/acl`
  - `min_privileges`: `[Pool.Allocate, Permissions.Modify, Sys.Audit]`
  - `roles`: `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `cluster.status`
  - `endpoints`: `GET /cluster/status`
  - `min_privileges`: `[Sys.Audit]`
  - `roles`: `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `nodes.list_and_details`
  - `endpoints`: `GET /nodes`, `GET /nodes/{node}/status`
  - `min_privileges`: `[Sys.Audit]`
  - `roles`: `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `vm.list_and_read_config`
  - `endpoints`: `GET /nodes/{node}/qemu`, `GET /nodes/{node}/qemu/{vmid}/config`, `GET /nodes/{node}/qemu/{vmid}/status/current`, `GET /nodes/{node}/qemu/{vmid}/agent/network-get-interfaces`
  - `min_privileges`: `[VM.Audit, Sys.Audit]`
  - `roles`: `[PVMSS_Service, PVMSS_Admin, PVMSS_User]`

- **Feature** `vm.create`
  - `endpoints`: `GET /nodes/{node}/qemu`, `POST /nodes/{node}/qemu`
  - `min_privileges`: `[VM.Allocate, VM.Config.*, Datastore.AllocateSpace, Sys.Audit]`
  - `roles`: `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `vm.power_actions`
  - `endpoints`: `POST /nodes/{node}/qemu/{vmid}/status/{action}`
  - `min_privileges`: `[VM.PowerMgmt, VM.Audit]`
  - `roles`: `[PVMSS_Service, PVMSS_Admin, PVMSS_User]`

- **Feature** `vm.update_config`
  - `endpoints`: `POST /nodes/{node}/qemu/{vmid}/config`
  - `min_privileges`: `[VM.Config.*]`
  - `roles`: `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `vm.delete`
  - `endpoints`: `POST /nodes/{node}/qemu/{vmid}/status/stop`, `DELETE /nodes/{node}/qemu/{vmid}`
  - `min_privileges`: `[VM.Allocate, VM.PowerMgmt, Datastore.AllocateSpace]`
  - `roles`: `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `storage.list`
  - `endpoints`: `GET /storage`, `GET /nodes/{node}/storage`
  - `min_privileges`: `[Datastore.Audit, Sys.Audit]`
  - `roles`: `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `storage.list_content`
  - `endpoints`: `GET /nodes/{node}/storage/{storage}/content`
  - `min_privileges`: `[Datastore.Audit]`
  - `roles`: `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `storage.create_delete_volume`
  - `endpoints`: `POST /nodes/{node}/storage/{storage}/content`, `DELETE /nodes/{node}/storage/{storage}/content/{volid}`
  - `min_privileges`: `[Datastore.AllocateSpace]`
  - `roles`: `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `network.list_bridges`
  - `endpoints`: `GET /nodes/{node}/network`
  - `min_privileges`: `[Sys.Audit]`
  - `roles`: `[PVMSS_Service, PVMSS_Admin]`

- **Feature** `console.vncproxy`
  - `endpoints`: `POST /nodes/{node}/qemu/{vmid}/vncproxy`
  - `min_privileges`: `[VM.Console, VM.Audit]`
  - `roles`: `[PVMSS_Service, PVMSS_Admin, PVMSS_User]`

- **Feature** `console.websocket`
  - `endpoints`: `GET /nodes/{node}/qemu/{vmid}/vncwebsocket`
  - `min_privileges`: `[VM.Console]`
  - `roles`: `[PVMSS_Admin, PVMSS_User]`

> For any fine-grained privilege tuning (for example separating `VM.Config.Disk` from `VM.Config.CPU`), always refer to Proxmox documentation sections “Privileges” and “Predefined Roles”, then adapt this dictionary accordingly.
