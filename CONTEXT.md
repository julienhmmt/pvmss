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
