# 0001 - Live status reads, not the projection, after a power action

Status: accepted (implemented)

## Context

The inventory projection is refreshed on a timer (30s by default), so it can be
up to 30s stale. The first VM action UI called `load()` right after the action
POST and overwrote the optimistic status flip with that stale projection: the
buttons appeared to do nothing. The two reference portals (ProxMate, pegaprox)
do not poll the Proxmox task (UPID) for interactive power actions either; they
fire the action, treat HTTP 200 as success, and converge on live state.

## Decision

- Keep an optimistic status in the UI after a successful action POST, then poll
  live status until it matches the target or a timeout is reached
  (`web/src/lib/features/vms/converge.ts`: 1.5s interval, 30s timeout).
- Add live reads that never touch the projection: `GET /vms/{cluster}/{vmid}/status`
  (detail) and `POST /vms/status` (batch, one call per tick for every flipping
  row), both through `cluster.VMStatusReader` (`/nodes/{node}/qemu/{vmid}/status/current`).
- Every target still goes through `vm.Resolve`, so ownership is checked exactly
  as on the other VM endpoints (see ADR 0003).
- Do not poll UPIDs for power actions. Task polling stays only where ordering is
  real: create, clone and delete.

## Consequences

- The batch endpoint has its own limit of 120 requests/minute, not the 30/minute
  of the write path. One action converges in up to 20 polls (1.5s over 30s), and
  several actions can converge at once, so 30/minute would reject legitimate
  polling. The limit lives in `httpapi/router.go` (`vmStatusLimiter`).
- A target that fails resolution or the live read is omitted from the batch
  response instead of failing the whole request.
- Live reads cost one Proxmox call per VM per tick while a row is flipping, and
  nothing otherwise.
