import { ApiRequestError, post } from './client';

let installed = false;

function formatClientErrorMessage(error: unknown): string {
	if (error instanceof ApiRequestError) {
		const suffix: string = error.requestId ? ` (requestId ${error.requestId})` : '';
		return `${error.code}: ${error.serverMessage}${suffix}`;
	}
	if (error instanceof Error) return error.message;
	return String(error);
}

/**
 * Forwards one browser-side failure to the server log. Fire-and-forget:
 * telemetry must never throw back into the code path that just failed, and a
 * dropped report (offline, 429) is acceptable.
 */
export function reportClientError(error: unknown): void {
	// The log wants the server's words and reference, not the localized text.
	const message: string = formatClientErrorMessage(error);
	const stack: string | undefined = error instanceof Error ? error.stack : undefined;
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
