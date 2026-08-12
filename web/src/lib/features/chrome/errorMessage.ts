import { m } from '$lib/paraglide/messages.js';

/**
 * Bounded, explicitly-listed set of server error `code` values already shipped
 * by T05/T06/T09/T12/T15's contracts, mapped to localized Paraglide messages.
 * A `code` not present here returns the generic localized fallback — never the
 * server's raw `message` string (FR-006). This map grows incrementally as later
 * tranches surface error codes users actually see.
 */
const ERROR_CODE_MAP: Record<string, () => string> = {
	forbidden: () => m['error.forbidden'](),
	not_found: () => m['error.not_found'](),
	invalid_action: () => m['error.invalid_action'](),
	quota_exceeded: () => m['error.quota_exceeded'](),
	gabarit_exceeded: () => m['error.gabarit_exceeded']()
};

/** Every code currently in the map — exported for table-driven tests. */
export const KNOWN_ERROR_CODES: readonly string[] = Object.keys(ERROR_CODE_MAP);

/**
 * Resolves a server error `code` to a localized user-facing message. Unknown
 * codes fall back to the generic localized message, never the raw server text.
 *
 * @param code - the `code` field from a `{code, message}` error response.
 * @param fallback - the server's raw `message` string, accepted but never
 *   surfaced directly to the user (FR-006).
 * @returns a localized string for display.
 */
export function resolveErrorMessage(code: string, fallback: string): string {
	const resolver = ERROR_CODE_MAP[code];
	if (resolver !== undefined) return resolver();
	return m['error.generic']();
}
