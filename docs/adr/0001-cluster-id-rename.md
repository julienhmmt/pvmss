# 0001 - Cluster id rename rewrites every cluster-keyed table

Status: accepted (implemented)

## Context

The cluster id (`clusters.name`, e.g. `default`) keys every per-cluster row:
sessions, audit_log, the seven catalog tables, managed_pools, vm_limits,
node_limits, profile_ssh_keys, quota_requests, vm_leases,
vm_snapshot_schedules and pool_quota_overrides - none of them through a
foreign key. The id is also the `:cluster` route param, the client and
inventory registry key, and the persisted active-cluster selection (session
rows server-side, the admin's sessionStorage pick client-side). Only the
display label was editable; a wrong id forced a create-new-delete-old
workaround that loses history and forces re-seeding catalog and policy data.

## Decision

- `store.RenameCluster` runs one transaction: update `clusters.name`, then
  `UPDATE <t> SET cluster = ? WHERE cluster = ?` for every cluster-keyed
  table. The table list lives in `store.clusterScopedTables` and the store
  test iterates it, so a new cluster-keyed table cannot be forgotten.
- `audit_log.cluster` is rewritten too: history follows the new id for one
  coherent view instead of staying split across a dead id.
- `sessions.cluster` is rewritten: logged-in users keep working.
- The handler (`PUT /api/v1/admin/clusters/{name}/rename`) re-keys the
  registries right after commit: `clients.Remove(old)` plus registration of
  the re-fetched row, `inventories.Remove(old)` plus the new entry, then the
  first refresh is awaited like create and update already do.
- Validation: the new id must match `[a-z0-9-]+` and differ from the old
  (400); the old row must be active (404); any clusters row - active or
  soft-deleted - using the new id refuses the rename (409), and a uniqueness
  violation from orphan rows under the new id is also mapped to 409.

## Consequences

- No locking bridges the commit-to-rekey window: requests that already
  resolved the old client finish; afterwards the old id answers
  cluster-not-found (404). Rows written under the old id inside the window
  are orphaned - the window is one synchronous call, so this is accepted.
- Open VNC websockets live until closed; task-tracker entries under the old
  id are lost though the Proxmox task itself continues.
- `?cluster=<id>` query links and `/admin/clusters/[name]` bookmarks under
  the old id stop working; the admin UI rewrites its saved selection to the
  new id on success.
- Fake-source caveat: `ensureSeedClusters` recreates `default`, `secondary`
  and `offline-demo` at startup when absent, so renaming a demo cluster
  revives the seeded id on the next restart. Fake-only, accepted.

## Rejected alternative

Create-new-delete-old: register a new cluster under the wanted id and remove
the old one. Rejected because nothing follows the rows keyed by the old id:
catalog approvals, policy, managed pools, leases and the whole audit trail
stay under a dead id, and the operator re-seeds all of it by hand.
