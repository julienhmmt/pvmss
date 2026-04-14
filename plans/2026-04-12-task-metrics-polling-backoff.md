# Metrics Polling Backoff / Visibility-based Pause

Implement a reusable polling composable with visibility-based pause and exponential backoff to optimize client-side API load and improve resilience.

## Overview

This plan implements a comprehensive polling optimization feature that:

- Pauses polling when the browser tab is inactive using the Page Visibility API
- Implements exponential backoff on API errors (5xx, timeouts, network failures)
- Provides UI feedback for backoff states
- Creates a reusable Svelte composable for consistency across all polling locations

## Current Polling Locations

| File | Endpoint | Interval | Visibility API | Backoff |
| ---- | -------- | -------- | -------------- | ------- |
| `tasks.svelte.ts` | `/api/v1/tasks/status` | 2500ms | No | No |
| `vm/[id]/+page.svelte` | `/api/v1/vms/{vmid}/metrics` | 5000ms | No | No |
| `admin/vms/+page.svelte` | `/api/v1/admin/vms` | 30000ms | Yes | No |
| `admin/nodes/+page.svelte` | `/api/v1/admin/nodes` | 30000ms | Yes | No |
| `home/+page.svelte` | `/api/v1/vms` | None (manual) | N/A | N/A |
| `profile/+page.svelte` | `/api/v1/vms` | None (manual) | N/A | N/A |

## Implementation Tasks

### Phase 1: Core Polling Utility

Task 1.1: Create reusable polling composable

- File: `frontend/src/lib/composables/usePolling.ts`
- Create a Svelte composable that encapsulates:
  - Polling state management (isPolling, isPaused, backoffState)
  - Visibility API integration (document.hidden listener)
  - Exponential backoff logic
  - Error classification (5xx, network, timeout vs 401/403)
  - Configurable parameters (baseInterval, maxInterval, multiplier)
- Export types for polling configuration and state

Task 1.2: Implement exponential backoff algorithm

- Base interval: 5000ms (configurable)
- Multiplier: 2x (configurable)
- Max interval: 60000ms (configurable)
- Reset on successful 2xx response
- Backoff on: 5xx errors, network failures, timeouts
- Stop polling on: 401/403 auth errors
- Ignore: other 4xx client errors

Task 1.3: Implement visibility-based pause

- Listen to `visibilitychange` event
- When `document.hidden`: pause polling, clear intervals
- When visible again: immediately fetch data, resume polling with current interval
- Store paused state for UI feedback

Task 1.4: Add UI feedback state

- Expose reactive state: `backoffInterval`, `nextRetryIn`, `isBackedOff`
- Provide formatted time string for UI (e.g., "Reconnecting in 12s...")
- Expose `isPaused` for visibility state

### Phase 2: Update Existing Polling Locations

Task 2.1: Update tasks.svelte.ts

- Replace manual `setInterval` with `usePolling` composable
- Configure: baseInterval=2500ms, maxInterval=60000ms
- Integrate with existing error handling
- Keep existing MAX_CONSECUTIVE_ERRORS logic as additional safeguard
- Update type definitions if needed

**Task 2.2: Update vm/[id]/+page.svelte**

- Replace manual `setInterval` with `usePolling` composable
- Configure: baseInterval=5000ms, maxInterval=60000ms
- Add UI feedback indicator near refresh button
- Keep existing retry logic for initial VM provisioning
- Test error handling for metrics endpoint

**Task 2.3: Update admin/vms/+page.svelte**

- Replace existing `document.hidden` check with `usePolling` composable
- Configure: baseInterval=30000ms, maxInterval=60000ms
- Remove manual visibility check (handled by composable)
- Add UI feedback indicator
- Test empty state handling

**Task 2.4: Update admin/nodes/+page.svelte**

- Replace existing `document.hidden` check with `usePolling` composable
- Configure: baseInterval=30000ms, maxInterval=60000ms
- Integrate empty retry logic with backoff (keep EMPTY_RETRY_MAX limit)
- Remove manual visibility check
- Add UI feedback indicator
- Test node refresh behavior

### Phase 3: Add Polling to New Pages

**Task 3.1: Add polling to home/+page.svelte**

- Add `usePolling` composable for VM list
- Configure: baseInterval=30000ms, maxInterval=60000ms
- Add UI feedback indicator near manual refresh button
- Keep manual refresh button for immediate updates
- Test pagination with polling

**Task 3.2: Keep profile/+page.svelte manual**

- No changes (manual refresh is sufficient for profile page)
- Document decision in code comments

### Phase 4: UI Components

**Task 4.1: Create PollingStatus component**

- File: `frontend/src/lib/components/data/PollingStatus.svelte`
- Display small icon (Phosphor: `ArrowsClockwise` or `WifiSlash`)
- Show "Reconnecting in Xs..." when in backoff
- Show "Paused" when tab is hidden (optional, for debugging)
- Minimal visual footprint, subtle styling
- Accept props: `isBackedOff`, `nextRetryIn`, `isPaused`

**Task 4.2: Integrate PollingStatus into pages**

- Add to vm/[id]/+page.svelte near refresh button
- Add to admin/vms/+page.svelte near refresh button
- Add to admin/nodes/+page.svelte near refresh button
- Add to home/+page.svelte near refresh button
- Add to tasks display areas (if applicable)

## Testing Tasks

### Unit Tests

**Test 1.1: Polling composable initialization**

- File: `frontend/src/lib/composables/usePolling.test.ts`
- Test: Composable initializes with default parameters
- Test: Composable accepts custom configuration
- Test: State is reactive and updates correctly

**Test 1.2: Exponential backoff logic**

- Test: Backoff increases interval exponentially (5s → 10s → 20s → 40s → 60s)
- Test: Backoff caps at maxInterval
- Test: Backoff resets on successful response
- Test: Backoff counter increments correctly
- Test: nextRetryIn countdown works correctly

**Test 1.3: Error classification**

- Test: 5xx errors trigger backoff
- Test: Network errors trigger backoff
- Test: Timeout errors trigger backoff
- Test: 401 errors stop polling
- Test: 403 errors stop polling
- Test: Other 4xx errors are ignored (no backoff)
- Test: 2xx responses reset backoff

**Test 1.4: Visibility API integration**

- Test: Polling pauses when document.hidden is true
- Test: Polling resumes when document.hidden is false
- Test: Immediate fetch on resume
- Test: isPaused state updates correctly
- Test: Multiple visibility changes work correctly

**Test 1.5: Cleanup and lifecycle**

- Test: Polling stops on unmount/composable cleanup
- Test: All intervals are cleared
- Test: Event listeners are removed
- Test: No memory leaks on repeated mount/unmount

### Integration Tests

**Test 2.1: tasks.svelte.ts integration**

- Test: Task status polling works with composable
- Test: Error handling integrates correctly
- Test: Completion callbacks still work
- Test: Cleanup logic still works

**Test 2.2: vm/[id]/+page.svelte integration**

- Test: Metrics polling works with composable
- Test: UI feedback displays correctly
- Test: Visibility pause/resume works
- Test: Backoff state shows in UI
- Test: Manual refresh still works

**Test 2.3: Admin pages integration**

- Test: Auto-refresh works with composable
- Test: Visibility checks are consistent
- Test: Empty retry logic integrates correctly
- Test: UI feedback displays correctly

**Test 2.4: home/+page.svelte integration**

- Test: New polling works correctly
- Test: Manual refresh still works
- Test: Pagination works with polling
- Test: Activity log updates correctly

### Manual Testing Checklist

**Test 3.1: Visibility-based pause**

- [ ] Open VM details page, switch to another tab, verify polling pauses
- [ ] Return to tab, verify immediate refresh and polling resumes
- [ ] Minimize browser, verify polling pauses
- [ ] Restore browser, verify immediate refresh and polling resumes
- [ ] Test on multiple pages (VM details, admin, home)

**Test 3.2: Exponential backoff**

- [ ] Simulate network failure (disconnect network), verify backoff increases
- [ ] Verify UI shows "Reconnecting in Xs..."
- [ ] Reconnect network, verify immediate refresh and backoff resets
- [ ] Simulate server error (block endpoint), verify backoff increases
- [ ] Restore endpoint, verify backoff resets

**Test 3.3: Auth error handling**

- [ ] Log out, verify polling stops on 401/403
- [ ] Verify task shows error status
- [ ] Log back in, verify polling can restart

**Test 3.4: UI feedback**

- [ ] Verify disconnected icon appears during backoff
- [ ] Verify countdown timer works correctly
- [ ] Verify icon disappears when connection restored
- [ ] Verify icon placement is consistent across pages

**Test 3.5: Performance**

- [ ] Open DevTools Network tab, verify reduced requests when tab hidden
- [ ] Verify no requests when tab hidden for 30+ seconds
- [ ] Verify request rate decreases during backoff
- [ ] Verify no memory leaks after tab switching 10+ times

**Test 3.6: Edge cases**

- [ ] Rapid tab switching (10+ times), verify no errors
- [ ] Page navigation while polling, verify cleanup
- [ ] Multiple tabs open, verify independent polling states
- [ ] Browser sleep/wake, verify polling resumes correctly

## Dependencies

- None (pure client-side implementation)
- Existing Phosphor icons for UI feedback
- Existing svelte-sonner for toasts (if needed for errors)

## Success Criteria

- [ ] All polling locations use the reusable composable
- [ ] Visibility API pauses polling when tab is hidden
- [ ] Exponential backoff activates on 5xx, network, timeout errors
- [ ] Backoff resets on successful responses
- [ ] Auth errors (401/403) stop polling
- [ ] UI feedback displays backoff state correctly
- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] Manual testing checklist completed
- [ ] No performance regression in DevTools
- [ ] No memory leaks in prolonged use

## Rollout Plan

1. Implement core polling composable (Phase 1)
2. Write unit tests for composable
3. Update one existing page (vm/[id]) as pilot
4. Test pilot thoroughly
5. Update remaining existing pages (Phase 2)
6. Add polling to home page (Phase 3)
7. Create UI component (Phase 4)
8. Run full test suite
9. Manual testing across all pages
10. Code review and merge
