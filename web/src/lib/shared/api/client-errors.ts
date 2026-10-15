import { post } from './client';

let installed = false;

/**
 * Forwards one browser-side failure to the server log. Fire-and-forget:
 * telemetry must never throw back into the code path that just failed, and a
 * dropped report (offline, 429) is acceptable.
 */
export function reportClientError(error: unknown): void {
	const message = error instanceof Error ? error.message : String(error);
	const stack = error instanceof Error ? error.stack : undefined;
	void post('/api/v1/client-errors', { message, stack, path: globalThis.location?.pathname }).catch(() => {
		// Swallowed on purpose - see the docblock.
	});
}

/** Installs the global handlers SvelteKit does not route through
 * handleError (uncaught exceptions, unhandled rejections). Idempotent. */
export function installClientErrorReporting(): void {
	if (installed || globalThis.window === undefined) return;
	installed = true;
	globalThis.addEventListener('error', (event: ErrorEvent) => {
		reportClientError(event.error ?? event.message);
	});
	globalThis.addEventListener('unhandledrejection', (event: PromiseRejectionEvent) => {
		reportClientError(event.reason);
	});
}
