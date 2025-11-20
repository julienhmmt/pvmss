# Fix QEMU agent

## Context

Currently, the QEMU guest agent integration is not working reliably. PVMSS forces the agent option to be enabled when creating VMs in Proxmox, but some guest OSes:

- Do not have `qemu-guest-agent` installed by default
- Install it but do not enable/start the service at boot
- Start it very late during boot, leading to long timeouts

As a result, PVMSS cannot reliably use the QEMU agent for guest discovery and guest-side operations (e.g. graceful shutdown), and the user experience is poor when the agent is missing or unresponsive.

This task is about designing and implementing a robust, observable integration with the QEMU guest agent on top of the Proxmox API.

## Problem

When the QEMU guest agent is not installed or not running and the user triggers an agent-based operation (for example a graceful shutdown from PVMSS), Proxmox eventually returns an error like:

> "QEMU Guest Agent is not running - VM 100 qmp command 'guest-ping' failed - got timeout"

This error appears in Proxmox after a long delay (for example around 60–65 seconds). During this time, PVMSS has poor or no feedback to present to the user. From the PVMSS point of view, the operation seems to hang.

Consequences:

- PVMSS cannot reliably know whether the guest agent is available for a specific VM
- Users do not understand why a "Shutdown" is slow or failing
- Some guests can only be managed by "Stop" (power off), but the UI does not clearly guide users
- Long-running timeouts in the HTTP handlers are undesirable and may consume resources

## High-level objectives

- **Reliability**: PVMSS should know, with a bounded latency, whether the QEMU guest agent for a given VM is:
  - `available` (responding to a simple probe)
  - `unavailable` (not installed, not running, or blocked)
  - `unknown` (no recent information / offline mode)
- **Good UX**: For operations that depend on the agent (mainly graceful shutdown), PVMSS should:
  - Avoid hanging for more than a small, configurable timeout
  - Expose clear error messages when the agent is not usable
  - Suggest the appropriate fallback operation (e.g. "Stop" instead of "Shutdown")
- **Observability**: Failures and timeouts when talking to the agent should be logged with enough context (VMID, node, OS info if available) to diagnose problems.
- **Minimal Proxmox load**: Reuse existing state/cache where possible and avoid polling too aggressively.

## Functional requirements (for an LLM)

### FR-1 – Keep forcing agent enablement on VM creation

- When creating a VM through PVMSS, continue to set the Proxmox VM config option that enables the QEMU guest agent.
- This behaviour must be preserved; we only add better detection and handling when the guest does not actually run the agent.

### FR-2 – Agent status model

- Introduce a simple internal status model for the QEMU guest agent per VM, for example:
  - `AgentStatusUnknown`
  - `AgentStatusAvailable`
  - `AgentStatusUnavailable`
  - (Optionally) `AgentStatusError` with details
- The status object should carry:
  - VMID and node
  - `status` (one of the enum values above)
  - `lastCheckedAt` timestamp
  - `lastErrorMessage` (if any, for logging and optional display)
- This status can be:
  - Stored in memory (e.g. attached to the existing VM cache / state manager)
  - Recomputed on demand via a Proxmox API call when stale

### FR-3 – Agent health check via Proxmox API

- Implement a helper that, given a VM (context, node, VMID), checks whether the QEMU agent is responsive by calling the appropriate Proxmox API endpoint (for example an `agent ping`-style command).
- This helper must:
  - Use a short per-request timeout (for example 3–5 seconds)
  - Normalize all responses into the internal status model:
    - Success → `AgentStatusAvailable`
    - Explicit "agent not running" / "agent not installed" responses → `AgentStatusUnavailable` with error message
    - Network / HTTP / JSON / timeout errors → `AgentStatusUnavailable` or `AgentStatusError` depending on what makes sense
  - Never block longer than the configured timeout
  - Log failures with structured fields (VMID, node, error type, duration)

### FR-4 – Agent-aware shutdown behaviour

- Before issuing an agent-based shutdown for a VM from PVMSS:
  - Check the last known agent status if it is recent enough; otherwise perform a fresh health check.
- Behaviour when status is:
  - `AgentStatusAvailable`:
    - Proceed with the existing agent-based shutdown call to Proxmox.
    - After sending the shutdown request, poll the VM status for a bounded period (for example up to 15–30 seconds, with a small interval) to see if the VM transitions away from `running`.
    - If the VM does not stop within that time window, surface a clear message to the user (see FR-5).
  - `AgentStatusUnavailable` or `AgentStatusError`:
    - Avoid waiting 60+ seconds for Proxmox to time out.
    - Fail fast and present to the user a warning that the QEMU agent is not available for this VM.
    - Suggest using the "Stop" action instead.
  - `AgentStatusUnknown`:
    - Option 1: perform a single health check, then behave as above based on the result.
    - Option 2: treat as `Unavailable` and show a warning; the exact choice should be documented in the code.

- All code paths must use contexts with deadlines so that HTTP handlers do not block indefinitely.

### FR-5 – User feedback (UI / toasts)

- When an agent-based shutdown operation fails, times out, or cannot be started because the agent is unavailable, PVMSS must show a toast notification or equivalent user-facing message. Example messages (to refine later):
  - "QEMU Guest Agent is not responding or not installed on this VM. Shutdown may fail. Please use the Stop action instead."
  - "QEMU Guest Agent shutdown did not complete within the expected time. The VM is still running. Consider using Stop."
- The message should be short but explicit, and must not expose raw Proxmox internal errors unless they are clearly understandable.
- If possible, the VM details page should display a small indicator of the last known agent state (e.g. a label or icon with a tooltip: `Agent: available / unavailable / unknown`).

### FR-6 – Offline mode compatibility

- When `PVMSS_OFFLINE=true` or when the Proxmox API client is not available, the QEMU agent logic must:
  - Skip any API calls to Proxmox
  - Treat the agent status as `AgentStatusUnknown`
  - Avoid presenting misleading messages about timeouts; instead, mention offline mode if relevant

### FR-7 – Logging and observability

- For every agent health check and agent-based shutdown attempt, log at least:
  - VMID and node
  - Operation type (health-check, shutdown)
  - Duration
  - Result (available, unavailable, timed out, error)
  - Error message or code when applicable
- Logs should make it easy to see recurring failures for a specific VM or OS.

### FR-8 – Documentation

- Add a short documentation section (for example in backend/docs/ or main README) that explains:
  - What the QEMU guest agent is and why PVMSS uses it
  - How PVMSS behaves when the agent is:
    - Available
    - Missing / not running
  - How to install and enable `qemu-guest-agent` on common OS families (Linux, Windows) at a high level
  - That PVMSS continues to force-enable the agent in Proxmox VM config, but guest-side installation/enabling still remains the admin’s responsibility.

## Behavioural scenarios (for testing and design)

### Scenario S1 – Agent installed and running

- VM created with agent enabled.
- OS has `qemu-guest-agent` installed and service running.
- Health check returns `AgentStatusAvailable` quickly.
- User clicks "Shutdown" in PVMSS:
  - PVMSS performs (or reuses) a health check → agent available.
  - Sends shutdown request via Proxmox API.
  - VM transitions to `stopped` within the polling window.
  - PVMSS shows a success message and the VM list/details reflect the stopped state.

### Scenario S2 – Agent not installed

- VM created with agent enabled in Proxmox, but guest OS has no `qemu-guest-agent` package.
- Health check either:
  - Returns a Proxmox error explicitly indicating agent is not running/installed, or
  - Times out.
- Status becomes `AgentStatusUnavailable`.
- User clicks "Shutdown" in PVMSS:
  - PVMSS detects `AgentStatusUnavailable` and **does not** wait 60+ seconds.
  - Operation fails fast.
  - User sees a toast like: "QEMU Guest Agent is not responding or not installed. Please use Stop instead."
  - VM remains running (as expected) until user uses "Stop".

### Scenario S3 – Agent service not started (or very slow startup)

- VM is booting; QEMU agent service is not yet up when PVMSS checks.
- First health check returns timeout or "agent not running".
- PVMSS may retry a small number of times with backoff (configurable) before deciding the agent is unavailable.
- If the user triggers "Shutdown" during this window, behaviour is the same as S2.

### Scenario S4 – Offline mode

- `PVMSS_OFFLINE=true` or Proxmox client initialization failed.
- Any attempt to inspect agent state or call agent-based shutdown must:
  - Avoid Proxmox calls
  - Return `AgentStatusUnknown`
  - Show a suitable message if the user attempts agent-based operations (for example: "Proxmox is offline, cannot perform graceful shutdown.")

## Non-goals / out of scope (for now)

- Implementing complex guest-side orchestration via the agent (file operations, script execution, etc.). This task focuses on presence detection and shutdown UX.
- Providing per-OS installation scripts; documentation should only give high-level guidance and point to official documentation.
- Persisting agent status across application restarts beyond what is already persisted for VM state.

## Acceptance criteria

An implementation can be considered complete when all of the following are true:

- **AC-1**: For a VM with a properly working QEMU agent, PVMSS can perform a graceful shutdown that completes within a reasonable time and reports success.
- **AC-2**: For a VM without QEMU agent installed or running, PVMSS:
  - Does not hang the HTTP request for ~60 seconds waiting for Proxmox timeouts
  - Fails fast with a clear UI message suggesting to use "Stop"
- **AC-3**: The last known QEMU agent status for a VM can be surfaced in the UI (even minimally, e.g. as a label or tooltip) and is updated when the user performs an operation that depends on it.
- **AC-4**: All new agent-related network calls are bounded by explicit timeouts and use the existing logging patterns.
- **AC-5**: Offline mode is respected: no agent-related Proxmox calls are made, and user feedback is not misleading.
- **AC-6**: There is at least basic test coverage (unit and/or integration) for the main scenarios S1–S4.

## Notes for future implementation work

Clearly separate concerns:

- Proxmox agent API client wrapper
- Agent status evaluation and caching
- Handler-layer logic for shutdown and UI messages

Prefer small, focused functions with clear inputs/outputs so that future LLMs can modify them safely.
