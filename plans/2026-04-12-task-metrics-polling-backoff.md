# Feature Plan: Metrics Polling Backoff / Visibility-based Pause

## Goal
Optimize client-side polling to reduce unnecessary API load on both the PVMSS backend and the Proxmox API when the user isn't actively looking at the page, or when the network connection is degraded.

## Current State
The frontend polls for VM details, statuses, and metrics at fixed intervals (e.g., every 5 seconds) regardless of whether the browser tab is active or in the background. If the server goes down, the client relentlessly hammers the API.

## Frontend Implementation (Client-Side Only)

1. **Visibility-based Pause**:
   - Utilize the standard browser Page Visibility API (`document.addEventListener("visibilitychange", handler)`).
   - **Pause Logic**: When `document.hidden` becomes `true` (user switches tabs, minimizes browser), clear the data polling `setInterval`.
   - **Resume Logic**: When `document.hidden` becomes `false` (user returns to tab), immediately trigger a data fetch to refresh stale data, and restart the standard polling interval.

2. **Exponential Backoff on Errors**:
   - Implement a wrapper around the `setInterval` fetching logic to handle failures gracefully.
   - If an API request fails (returns a `5xx` error, times out, or network is offline):
     - Increase the polling interval exponentially (e.g., base `5s` -> `10s` -> `20s` -> `40s` -> max `60s`).
   - If an API request succeeds (returns `2xx`):
     - Reset the polling interval back to the default base rate (e.g., `5s`).
   - This prevents the frontend from overwhelming a recovering server.

3. **UI Feedback**:
   - If polling goes into a backoff state due to errors, display a subtle UI indicator (e.g., a disconnected icon or "Reconnecting in Xs...") so the user knows data might be stale.

## Backend Implementation
- No significant backend changes are required. This is purely a client-side optimization.
- Ensure the backend properly sets CORS and cache-control headers so the client isn't aggressively caching dynamic metric endpoints.

## Challenges & Considerations
- **Multiple Timers**: Ensure that global polling (e.g., global notifications or task tracking) and page-specific polling (e.g., specific VM metrics) are both hooked into the visibility API to prevent any stray timers from running in the background.
- **WebSocket Alternative**: If real-time metrics become a core requirement, consider replacing polling entirely with WebSockets or Server-Sent Events (SSE), though polling with backoff/visibility checks is often simpler and highly effective.
