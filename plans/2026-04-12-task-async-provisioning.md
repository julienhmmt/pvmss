# Feature Plan: Async Task Tracking for VM Provisioning

## Goal
Prevent the UI and HTTP requests from blocking or timing out during long-running operations like VM creations, cloning, or large disk manipulations.

## Current State
When a user initiates a VM creation, the frontend makes an HTTP request that blocks until the Proxmox task finishes. This can lead to browser timeouts, bad UX, and hanging UI states if the operation takes several minutes.

## Backend Implementation

1. **Return UPID Instead of Blocking**:
   - When triggering long operations (like `POST /nodes/{node}/qemu` for creation or cloning), the Proxmox API immediately returns a Task `UPID` (Unique Process ID) instead of waiting for completion.
   - Modify the backend handler to return this `UPID` to the frontend immediately with an HTTP `202 Accepted` status code.

2. **Task Status Endpoint**:
   - Create a new endpoint: `GET /api/tasks/{node}/{upid}/status`.
   - This endpoint proxies the request to `GET /nodes/{node}/tasks/{upid}/status`.
   - Also expose `GET /api/tasks/{node}/{upid}/log` to stream or fetch the task log for detailed progress.

## Frontend Implementation

1. **Operation Initiation**:
   - When a user submits the "Create VM" form, the frontend receives the `202 Accepted` response with the `UPID`.
   - Transition the UI away from the form to a "Task Progress" view, or show a persistent, non-blocking toast/tray notification.

2. **Polling Mechanism**:
   - Implement a polling service that calls `GET /api/tasks/{node}/{upid}/status` every 2-3 seconds.
   - Display the status (running, stopped) and optionally tail the logs using the log endpoint to show what Proxmox is currently doing (e.g., "Formatting disk...", "Copying data...").

3. **Completion Handling**:
   - Once the task status changes to `stopped`:
     - If `exitstatus` is `OK`: Display a success message and automatically redirect the user to the new VM's detail page.
     - If `exitstatus` contains an error: Display the error message extracted from the logs and allow the user to acknowledge it.

## Challenges & Considerations
- **Orphaned Tasks**: If the user closes the browser tab while a task is running, the task continues on Proxmox. We might need a global "Active Tasks" tray in the navigation bar to allow users to re-attach to running tasks.
- **Node Specificity**: UPIDs are bound to specific nodes. The frontend must keep track of which node the task is running on to query the status correctly.
