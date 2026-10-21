import { m } from '#lib/paraglide/messages.js';

/**
 * Bounded, explicitly-listed set of server error `code` values already shipped
 * by T05/T06/T09/T12/T15's contracts, mapped to localized Paraglide messages.
 * A `code` not present here returns the generic localized fallback (with the
 * request reference when known) - never the server's raw `message` string
 * (FR-006), except validation codes, which frame it (see DETAIL_CODES). This map grows incrementally as later
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
	snapshot_name_exists: () => m['error.snapshot_name_exists'](),
	unauthenticated: () => m['error.unauthenticated'](),
	invalid_credentials: () => m['error.invalid_credentials'](),
	rate_limited: () => m['error.rate_limited'](),
	refresh_too_soon: () => m['error.refresh_too_soon'](),
	internal_error: () => m['error.internal_error'](),
	cluster_unreachable: () => m['error.cluster_unreachable'](),
	cluster_unavailable: () => m['error.cluster_unavailable'](),
	cluster_not_found: () => m['error.cluster_not_found'](),
	cluster_error: () => m['error.cluster_error'](),
	inventory_not_ready: () => m['error.inventory_not_ready'](),
	policy_unavailable: () => m['error.policy_unavailable'](),
	vm_running: () => m['error.vm_running'](),
	vm_not_running: () => m['error.vm_not_running'](),
	vm_not_stopped: () => m['error.vm_not_stopped'](),
	not_approved: () => m['error.not_approved'](),
	max_snapshots_reached: () => m['error.max_snapshots_reached'](),
	snapshot_not_found: () => m['error.snapshot_not_found'](),
	disk_not_found: () => m['error.disk_not_found'](),
	guest_agent_unreachable: () => m['error.guest_agent_unreachable'](),
	guest_agent_disabled: () => m['error.guest_agent_disabled'](),
	console_unavailable: () => m['error.console_unavailable'](),
	insufficient_disk_space: () => m['error.insufficient_disk_space'](),
	members_remain: () => m['error.members_remain'](),
	last_cluster: () => m['error.last_cluster'](),
	protected_tag: () => m['error.protected_tag'](),
	boot_disk_protected: () => m['error.boot_disk_protected'](),
	shutdown_timeout: () => m['error.shutdown_timeout'](),
	expired: () => m['error.expired'](),
	empty_targets: () => m['error.empty_targets'](),
	import_failed: () => m['error.import_failed'](),
	iso_not_approved: () => m['error.not_approved'](),
	bridge_not_approved: () => m['error.not_approved'](),
	storage_not_approved: () => m['error.not_approved'](),
	name_taken: () => m['error.duplicate']()
};

/**
 * Codes whose server message names the exact field or limit at fault
 * ("memory 8192 exceeds the 4096 limit"). They keep that detail inside a
 * localized sentence: losing it would leave the user guessing what to fix.
 */
const DETAIL_CODES: ReadonlySet<string> = new Set([
	'hardware_exceeds_limit',
	'disk_size_exceeds_limit',
	'network_cards_exceed_limit',
	'capacity_exceeded',
	'limit_reached',
	'too_many_targets',
	'disk_size_not_greater',
	'disk_reduction',
	'disk_below_image',
	'node_limit_below_usage',
	'node_limit_above_capacity',
	'page_size_too_large',
	'out_of_range',
	'empty_patch',
	'bus_full',
	// Proxmox's own exit message / the blocking reason is the content here.
	'task_failed',
	'migration_blocked'
]);

function isDetailCode(code: string): boolean {
	return code.startsWith('invalid_') || DETAIL_CODES.has(code);
}

/** Every code currently in the map - exported for table-driven tests. */
export const KNOWN_ERROR_CODES: readonly string[] = Object.keys(ERROR_CODE_MAP);

/**
 * Resolves a server error `code` to a localized user-facing message. Unknown
 * codes fall back to the generic localized message, never the raw server text.
 *
 * One deliberate exception (ADR 0002): a `cluster_rejected` code carries
 * Proxmox's own message as its fallback - that message is the content the
 * user needs ("storage does not support snapshots", "VM is locked"), it
 * describes the VM's storage or state, and 401/403 rejections carry the
 * generic fallback anyway, so surfacing it never leaks an auth error body.
 *
 * @param code - the `code` field from a `{code, message}` error response.
 * @param fallback - the server's raw `message` string, accepted but normally
 *   not surfaced directly to the user (FR-006).
 * @returns a localized string for display.
 */
export function resolveErrorMessage(
	code: string,
	fallback: string,
	details: { readonly privilege?: string | undefined; readonly path?: string | undefined; readonly requestId?: string | undefined } = {}
): string {
	if (code === 'cluster_rejected' && fallback.trim() !== '') return fallback;
	if (code === 'cluster_permission_denied') {
		// Server fields: privilege and path identify the missing ACL grant.
		if (typeof details.privilege === 'string' && typeof details.path === 'string' && details.privilege !== '' && details.path !== '') {
			return m['error.cluster_permission_denied']({ privilege: details.privilege, path: details.path });
		}
	}
	const resolver = ERROR_CODE_MAP[code] ?? (code.startsWith('duplicate_') ? () => m['error.duplicate']() : undefined);
	if (resolver !== undefined) return resolver();
	if (isDetailCode(code) && fallback.trim() !== '') return m['error.validation']({ detail: fallback });
	// Unknown code: never the raw English text; the reference finds it in the server log.
	if (details.requestId) return m['error.genericRef']({ ref: details.requestId });
	return m['error.generic']();
}
