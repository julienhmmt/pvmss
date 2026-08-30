# PVMSS

A lightweight web portal that lets users manage Proxmox VMs without direct
Proxmox UI access. Go REST API over the Proxmox API + SQLite, SvelteKit SPA
served by the Go binary.

## Language

**Projection**:
The cached inventory snapshot stored in SQLite, refreshed by the background
inventory worker every 30s and by the per-cluster refresher after write
actions. Source for list views and bulk reads — never the source of truth for
an in-flight lifecycle transition.
_Avoid_: cache, inventory state, DB state

**Live status**:
An on-demand read of a single VM's current power state (`VMLiveStatus{Status,
Lock, Uptime}`), fetched from Proxmox `/nodes/{n}/qemu/{id}/status/current`.
The source of truth for lifecycle transitions: the front converge loop,
shutdown escalation, idempotence pre-checks, and lock-retry all consume it.
_Avoid_: real status, current status, fresh status

**Convergence**:
The process by which an optimistic status (posed by the front immediately on
click) reaches the live status via polling, bounded by a timeout. The
optimistic value survives until the live read confirms or contradicts it.
_Avoid_: sync, refresh, reconciliation

**UPID**:
The Proxmox task identifier returned by async POST endpoints (create, clone,
delete). PVMSS discards it on interactive power actions (start/stop/shutdown/
reboot/reset) — matching both reference implementations — and waits on it only
where post-task configuration is required (create → cloud-init attach, delete).
_Avoid_: task id, job id

**Tâche**:
An asynchronous Proxmox operation tracked by UPID and polled by the task tray
(create, snapshot, rollback, snapshot delete). Terminal states are
running / ok / error. A task that ends in `WARNINGS: N` is a **success** — the
warning is an attribute (`Warnings`), never a state: `Warnings` non-empty
implies state `ok`. When the tray stops following a task (deadline, repeated
errors) it says "we stopped following", never "it failed".
_Avoid_: job, workflow, "task with warnings" as a state

**Verrou Proxmox (lock)**:
A per-VM mutex PVE holds while an operation runs (snapshot, rollback, backup,
migrate, clone, create…). While set, PVE refuses other operations on the VM.
Snapshot operations retry on it (bounded, then fail naming the lock);
`lock = snapshot-delete` left behind by a failed delete is the NFS/ESTALE
signature. PVMSS reads it (`LiveStatus.Lock`, error parsing via
`extractLockName`) but cannot clear it under an API token — the operator runs
`qm unlock <vmid>` on the node.
_Avoid_: treating the lock as a VM power state

**Rollback**:
Restoring a VM to a snapshot. Proxmox stops the VM, reverts the disks, and
starts it again — with RAM state the VM resumes live at the snapshot point,
without it the VM boots fresh from the restored disks. The confirmation UI
must say « arrête puis redémarre la VM ».
_Avoid_: silent restore, "revert" (no PVMSS usage)

**Snapshot avec état RAM (vmstate)**:
A snapshot that also captures the VM's RAM, enabling resume-at-point
rollback. Requires the VM to be running and every disk on storage able to hold
the RAM state. The UI offers it as a checkbox and greys it out with a reason,
never silently.
_Avoid_: RAM snapshot (ambiguous with a memory dump)
