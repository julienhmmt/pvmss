# 0003 - A VM outside PVMSS scope answers 404, never 403

Status: accepted (implemented)

## Context

Users must only see and act on their own VMs. A 403 for "exists but not yours"
confirms that a given VMID exists on the cluster to someone who should not know.
An earlier version also resolved the target VM in each write handler on its own,
which is how an ownership check was once missed on one action path.

## Decision

- `vm.Resolve(source, actor, cluster, vmid)` is the single gate every VM
  endpoint, read or write, goes through before touching Proxmox.
- It returns `vm.ErrNotFound` both when the VM is absent from the index and when
  it exists without the `pvmss` tag. The two are indistinguishable to the caller.
- A VM that is in scope but belongs to another user's pool is `vm.ErrForbidden`
  (403): the caller is inside the portal's scope and may know it exists.
- Handlers identify the target VM and its node from the resolved `Entity` only.

## Consequences

- Every new VM handler must call `Resolve` first; the authorization contract
  tests (`httpapi/authz_contract_test.go`) exist to catch a handler that does not.
- Failed resolutions are recorded for audit (`vm.resolve_failed`) without
  distinguishing the two cases to the caller.
