import { m } from '$lib/paraglide/messages.js';

/**
 * Bounded, explicitly-listed set of server error `code` values already shipped
 * by T05/T06/T09/T12/T15's contracts, mapped to localized Paraglide messages.
 * A `code` not present here returns the generic localized fallback — never the
 * server's raw `message` string (FR-006). This map grows incrementally as later
 * tranches surface error codes users actually see.
 *
 * Ticket 02 (ADR 0002) added the snapshot cluster-rejection codes; their
 * `cluster_rejected` sibling is handled in resolveErrorMessage (the raw
 * Proxmox message is the content) and is deliberately absent here.
 */
const ERROR_CODE_MAP: Record<string, () => string> = {
	forbidden: () => m['error.forbidden'](),
	not_found: () => m['error.not_found'](),
	invalid_action: () => m['error.invalid_action'](),
	quota_exceeded: () => m['error.quota_exceeded'](),
	gabarit_exceeded: () => m['error.gabarit_exceeded'](),
	snapshot_storage_unsupported: () => m['error.snapshot_storage_unsupported'](),
	vm_locked: () => m['error.vm_locked'](),
	snapshot_name_exists: () => m['error.snapshot_name_exists']()
};

/** Every code currently in the map — exported for table-driven tests. */
export const KNOWN_ERROR_CODES: readonly string[] = Object.keys(ERROR_CODE_MAP);

/**
 * Resolves a server error `code` to a localized user-facing message. Unknown
 * codes fall back to the generic localized message, never the raw server text.
 *
 * One deliberate exception (ADR 0002): a `cluster_rejected` code carries
 * Proxmox's own message as its fallback — that message is the content the
 * user needs ("storage does not support snapshots", "VM is locked"), it
 * describes the VM's storage or state, and 401/403 rejections carry the
 * generic fallback anyway, so surfacing it never leaks an auth error body.
 *
 * @param code - the `code` field from a `{code, message}` error response.
 * @param fallback - the server's raw `message` string, accepted but normally
 *   not surfaced directly to the user (FR-006).
 * @returns a localized string for display.
 */
export function resolveErrorMessage(code: string, fallback: string): string {
	if (code === 'cluster_rejected' && fallback.trim() !== '') return fallback;
	const resolver = ERROR_CODE_MAP[code];
	if (resolver !== undefined) return resolver();
	return m['error.generic']();
}
