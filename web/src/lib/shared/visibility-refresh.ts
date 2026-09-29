/**
 * Refresh-on-visibility wiring (PLAN-ui-quickwins W3).
 *
 * The server refreshes its VM inventory every `PVMSS_INVENTORY_REFRESH_INTERVAL`
 * (30s default), but an open tab never re-reads on its own - a VM stopped
 * outside PVMSS would stay "running" forever. This helper fires a callback
 * when the document becomes visible and on a timer while it is; the callback
 * decides whether a reload is actually due (see `VmListStore.refreshIfStale`
 * and `VmDetailStore.refreshIfStale`), keeping the DOM plumbing shared.
 */

/**
 * Minimum age of the last `load()` before a visibility change triggers a
 * reload - matches the server's default inventory refresh interval (30s).
 */
export const STALE_REFRESH_MS = 30_000;

/**
 * Poll cadence while the tab is visible. Half the stale threshold so a tick
 * always lands after `STALE_REFRESH_MS` has elapsed: a VM stopped outside
 * PVMSS (guest shutdown, Proxmox UI) shows up within ~one inventory tick.
 */
export const POLL_REFRESH_MS = STALE_REFRESH_MS / 2;

/**
 * Calls `callback` when the document becomes visible and every
 * `POLL_REFRESH_MS` while it stays visible. Returns the unsubscribe function
 * - call it on component destroy (or return it from `onMount`).
 */
export function onVisibleRefresh(callback: () => void): () => void {
	const handler = (): void => {
		if (document.visibilityState === 'visible') callback();
	};
	document.addEventListener('visibilitychange', handler);
	const timer = setInterval(handler, POLL_REFRESH_MS);
	return () => {
		document.removeEventListener('visibilitychange', handler);
		clearInterval(timer);
	};
}
