# Proxmox permissions for PVMSS

This page describes the Proxmox roles and API token PVMSS needs, and the
privileges each role requires. The exact `pveum` commands are kept in sync with
the project README. These are the recommended roles; always validate the
privilege list against the Proxmox documentation for your PVE version.

## Roles overview

PVMSS relies on dedicated Proxmox roles and ACLs for three actors:

- **PVMSS_Service** — the backend service account, used through an API token
  (`PROXMOX_API_TOKEN_NAME` / `PROXMOX_API_TOKEN_VALUE`). It performs cluster,
  node, VM, storage, network, and user/pool operations on behalf of the
  application.
- **PVMSS_Admin** — human administrators of PVMSS. They manage the VMs, users,
  and pools PVMSS created, and view cluster and node resources.
- **PVMSSUser** — the per-pool role automatically assigned to every
  self-service user. PVMSS provisions this role, the user, the pool, and the
  ACL when you create a pool from `/admin/pools`. You do not create it manually.

The `PVEVMUser` default role is not used by PVMSS; self-service users get the
scoped `PVMSSUser` role on their own pool instead.

## Service account (PVMSS_Service)

Run as `root` on the Proxmox node. Create the role, then the user and its API
token. The token secret must be stored in `PROXMOX_API_TOKEN_VALUE`.

```bash
pveum roleadd PVMSS_Service -privs "Sys.Audit VM.Audit VM.Allocate VM.PowerMgmt VM.Console VM.Config.CDROM VM.Config.CPU VM.Config.HWType VM.Config.Memory VM.Config.Disk VM.Config.Network VM.Config.Options VM.Config.Cloudinit VM.Snapshot VM.Snapshot.Rollback Datastore.Audit Datastore.AllocateSpace Pool.Allocate Pool.Audit User.Modify Permissions.Modify Realm.AllocateUser SDN.Allocate SDN.Audit SDN.Use"

pveum useradd pvmss-svc@pve \
  -comment "PVMSS service account" \
  -enable 1

pveum aclmod / -user pvmss-svc@pve -role PVMSS_Service -propagate 1

pveum user token add pvmss-svc@pve pvmss-service-token --privsep 0
```

## Administrators (PVMSS_Admin)

```bash
pveum roleadd PVMSS_Admin -privs "Sys.Audit VM.Audit VM.PowerMgmt VM.Console VM.Config.CPU VM.Config.Memory VM.Config.Disk VM.Config.Network VM.Config.HWType VM.GuestAgent.Audit VM.Migrate VM.Config.CDROM VM.Config.Options VM.Config.Cloudinit Datastore.Audit Datastore.AllocateSpace Pool.Allocate Pool.Audit User.Modify Permissions.Modify Realm.AllocateUser SDN.Audit Group.Allocate"

pveum useradd pvmss-admin1@pve \
  -comment "PVMSS administrator" -password "strong_password" \
  -enable 1

pveum aclmod / -user pvmss-admin1@pve -role PVMSS_Admin -propagate 1
```

## Per-user pools (PVMSSUser)

PVMSS creates the `PVMSSUser` role, the user, the dedicated pool, and the ACL
automatically when an administrator provisions a pool from `/admin/pools`. No
manual `pveum` steps are required for end users. Each user only sees the VMs
inside their own pool.

## Security recommendations

- Use dedicated Proxmox accounts for PVMSS; never share the built-in
  administrator password.
- Keep the service token secret; it carries high privileges across the cluster.
- Restrict service and admin accounts to automated and human operations
  respectively; do not reuse them for unrelated tasks.
- Prefer a dedicated service token over `root@pam` tokens in production.

## Functional scope

PVMSS does not manage the following Proxmox features through its own UI: backups
and restore, LXC containers, SDN orchestration beyond what is read for bridges,
Proxmox firewall, high availability, storage replication, and Ceph/RBD-specific
management. These remain administered directly in Proxmox.

## API families used by PVMSS

PVMSS calls the standard Proxmox API families:

- **Authentication & users**: `/access/ticket`, `/access/users`, `/access/password`, `/access/roles`, `/access/acl`, `/pools`.
- **Cluster & nodes**: `/cluster/status`, `/nodes`, `/nodes/{node}/status`.
- **VM management (QEMU)**: `/nodes/{node}/qemu`, its `config` and `status` subpaths, and VM deletion.
- **Storage**: `/storage`, `/nodes/{node}/storage`, `/nodes/{node}/storage/{storage}/content`.
- **Network**: `/nodes/{node}/network`.
- **Console / VNC**: `/nodes/{node}/qemu/{vmid}/vncproxy` and `vncwebsocket`.

The privileges listed per role above are the minimum needed for these calls.
