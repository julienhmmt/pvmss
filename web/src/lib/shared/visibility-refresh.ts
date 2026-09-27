/**
 * Refresh-on-visibility wiring (PLAN-ui-quickwins W3).
 *
 * The server refreshes its VM inventory every `PVMSS_INVENTORY_REFRESH_INTERVAL`
 * (30s default), but an open tab never re-reads on its own - a user who
 * switches away and back can stare at arbitrarily stale data. This helper
 * fires a callback each time the document becomes visible again; the callback
 * decides whether a reload is actually due (see `VmListStore.refreshIfStale`
 * and `VmDetailStore.refreshIfStale`), keeping the DOM plumbing shared.
 */

/**
 * Minimum age of the last `load()` before a visibility change triggers a
 * reload - matches the server's default inventory refresh interval (30s).
 */
export const STALE_REFRESH_MS = 30_000;

/**
 * Registers a `visibilitychange` listener that calls `callback` when the
 * document becomes visible. Returns the unsubscribe function - call it on
 * component destroy (or return it from `onMount`).
 */
export function onVisibleRefresh(callback: () => void): () => void {
	const handler = (): void => {
		if (document.visibilityState === 'visible') callback();
	};
	document.addEventListener('visibilitychange', handler);
	return () => {
		document.removeEventListener('visibilitychange', handler);
	};
}
