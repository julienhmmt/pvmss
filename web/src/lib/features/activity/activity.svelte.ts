import { get } from '#lib/shared/api/client.js';
import { m } from '#lib/paraglide/messages.js';
import type { VmListItem, VmListResult } from '#lib/features/vms/list.svelte.js';
import type { VmAuditEntry } from '#lib/features/vms/detail.svelte.js';

/** One row of the "Recent updates" timeline. */
export interface ActivityEntry {
	id: string;
	cluster: string;
	vmid: number;
	machineName: string;
	action: string;
	actor: string;
	timestamp: string;
}

/** Machines whose history is read. The list is quota-bounded for pool
 *  users; the cap keeps an unlimited allowance from fanning out forever. */
export const MAX_MACHINES = 20;
/** Rows shown in the timeline. */
export const MAX_ENTRIES = 25;

/**
 * Merges per-machine audit pages into one timeline, newest first. Pure so
 * the ordering and the cap are unit-tested without a server.
 */
export function mergeActivity(
	perMachine: readonly { machine: Pick<VmListItem, 'cluster' | 'vmid' | 'name'>; items: readonly VmAuditEntry[] }[],
	limit = MAX_ENTRIES
): ActivityEntry[] {
	const rows: ActivityEntry[] = [];
	for (const { machine, items } of perMachine) {
		for (const item of items) {
			rows.push({
				id: `${machine.cluster}:${item.id}`,
				cluster: machine.cluster,
				vmid: machine.vmid,
				machineName: machine.name,
				action: item.action,
				actor: item.actor,
				timestamp: item.timestamp
			});
		}
	}
	rows.sort((a, b) => Date.parse(b.timestamp) - Date.parse(a.timestamp));
	return rows.slice(0, limit);
}

const ACTION_LABELS: Record<string, () => string> = {
	start: () => m['activity.action.start'](),
	stop: () => m['activity.action.stop'](),
	shutdown: () => m['activity.action.shutdown'](),
	reboot: () => m['activity.action.reboot'](),
	reset: () => m['activity.action.reset'](),
	pause: () => m['activity.action.pause'](),
	resume: () => m['activity.action.resume'](),
	vm_create: () => m['activity.action.vm_create'](),
	rename: () => m['activity.action.rename'](),
	console_open: () => m['activity.action.console_open'](),
	hardware_update: () => m['activity.action.hardware_update'](),
	network_update: () => m['activity.action.network_update'](),
	add_disk: () => m['activity.action.add_disk'](),
	resize_disk: () => m['activity.action.resize_disk'](),
	delete_disk: () => m['activity.action.delete_disk'](),
	delete: () => m['activity.action.delete'](),
	edit_description: () => m['activity.action.edit_description'](),
	boot_cdrom: () => m['activity.action.boot_cdrom'](),
	cdrom_mount: () => m['activity.action.cdrom_mount'](),
	cdrom_disconnect: () => m['activity.action.cdrom_disconnect'](),
	cdrom_remove: () => m['activity.action.cdrom_remove'](),
	vm_snapshot_create: () => m['activity.action.vm_snapshot_create'](),
	vm_snapshot_rollback: () => m['activity.action.vm_snapshot_rollback'](),
	vm_snapshot_delete: () => m['activity.action.vm_snapshot_delete'](),
	set_console_password: () => m['activity.action.set_console_password'](),
	serial_enable: () => m['activity.action.serial_enable'](),
	vm_create_rollback: () => m['activity.action.vm_create_rollback'](),
	retrofit_seabios: () => m['activity.action.retrofit_seabios'](),
	'vm.migrate': () => m['activity.action.vm_migrate'](),
	'vm.resolve_failed': () => m['activity.action.vm_resolve_failed'](),
	'admin.clusters.create': () => m['activity.action.admin_clusters_create'](),
	'admin.clusters.update': () => m['activity.action.admin_clusters_update'](),
	'admin.clusters.delete': () => m['activity.action.admin_clusters_delete'](),
	'admin.isos.toggle': () => m['activity.action.admin_isos_toggle'](),
	'admin.isos.delete': () => m['activity.action.admin_isos_delete'](),
	'admin.storages.toggle': () => m['activity.action.admin_storages_toggle'](),
	'admin.storages.delete': () => m['activity.action.admin_storages_delete'](),
	'admin.bridges.toggle': () => m['activity.action.admin_bridges_toggle'](),
	'admin.bridges.delete': () => m['activity.action.admin_bridges_delete'](),
	'admin.nodes.toggle': () => m['activity.action.admin_nodes_toggle'](),
	'admin.nodes.delete': () => m['activity.action.admin_nodes_delete'](),
	'admin.templates.toggle': () => m['activity.action.admin_templates_toggle'](),
	'admin.templates.update': () => m['activity.action.admin_templates_update'](),
	'admin.templates.delete': () => m['activity.action.admin_templates_delete'](),
	'admin.policy.update': () => m['activity.action.admin_policy_update'](),
	'admin.policy.nodes.update': () => m['activity.action.admin_policy_nodes_update'](),
	'admin.pools.create': () => m['activity.action.admin_pools_create'](),
	'admin.pools.update': () => m['activity.action.admin_pools_update'](),
	'admin.pools.delete': () => m['activity.action.admin_pools_delete'](),
	'admin.profiles.create': () => m['activity.action.admin_profiles_create'](),
	'admin.profiles.update': () => m['activity.action.admin_profiles_update'](),
	'admin.profiles.delete': () => m['activity.action.admin_profiles_delete'](),
	'admin.profiles.toggle': () => m['activity.action.admin_profiles_toggle'](),
	'admin.tags.create': () => m['activity.action.admin_tags_create'](),
	'admin.tags.color': () => m['activity.action.admin_tags_color'](),
	'admin.tags.delete': () => m['activity.action.admin_tags_delete'](),
	'admin.log_level.update': () => m['activity.action.admin_log_level_update'](),
	'admin.audit_config.update': () => m['activity.action.admin_audit_config_update'](),
	'admin.db_export': () => m['activity.action.admin_db_export'](),
	'admin.db_import': () => m['activity.action.admin_db_import'](),
	'admin.db_import.rejected': () => m['activity.action.admin_db_import_rejected'](),
	'auth.login_failed': () => m['activity.action.auth_login_failed'](),
	'auth.admin_denied': () => m['activity.action.auth_admin_denied'](),
	'auth.csrf_rejected': () => m['activity.action.auth_csrf_rejected'](),
	'auth.rate_limited': () => m['activity.action.auth_rate_limited'](),
	'quota.exceeded': () => m['activity.action.quota_exceeded']()
};

/** A plain-language message for an audit action; unknown ones keep the raw
 *  action name so nothing is hidden. */
export function activityMessage(action: string): string {
	return ACTION_LABELS[action]?.() ?? m['activity.action.other']({ action });
}

/** Every audit code the catalog names, for filters that match exact codes. */
export function knownActions(): string[] {
	return Object.keys(ACTION_LABELS);
}

/** Whether the catalog has a label for this action, so a surface can flag a
 *  missing translation instead of hiding it. */
export function isKnownAction(action: string): boolean {
	return action in ACTION_LABELS;
}

const TARGET_LABELS: Record<string, () => string> = {
	vm: () => m['activity.target.vm'](),
	cluster: () => m['activity.target.cluster'](),
	node: () => m['activity.target.node'](),
	node_capacity: () => m['activity.target.node_capacity'](),
	storage: () => m['activity.target.storage'](),
	bridge: () => m['activity.target.bridge'](),
	iso: () => m['activity.target.iso'](),
	template: () => m['activity.target.template'](),
	pool: () => m['activity.target.pool'](),
	profile: () => m['activity.target.profile'](),
	tag: () => m['activity.target.tag'](),
	policy: () => m['activity.target.policy'](),
	quota: () => m['activity.target.quota'](),
	auth: () => m['activity.target.auth'](),
	db: () => m['activity.target.db'](),
	audit_config: () => m['activity.target.audit_config'](),
	log_level: () => m['activity.target.log_level']()
};

/** An audit target as words ("VM 123"), not the stored "vm:123" pair. Unknown
 *  types keep the raw pair. */
export function auditTarget(type: string, id: string): string {
	const label = TARGET_LABELS[type]?.();
	if (!label) return type ? `${type}:${id}` : '';
	return id ? `${label} ${id}` : label;
}

/**
 * Recent updates for the Activity screen. There is no cross-machine audit
 * endpoint for pool users, so the store reads the user's machines, then each
 * machine's own audit page (GET /api/v1/vms/{cluster}/{vmid}/audit, the same
 * source as the detail Activity tab), and merges them. Nothing is inferred:
 * every row is a recorded audit entry.
 */
export class ActivityStore {
	entries = $state.raw<ActivityEntry[] | null>(null);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			const list = await get<VmListResult>(`/api/v1/vms?pageSize=${MAX_MACHINES}&sortBy=name`);
			const pages = await Promise.all(
				list.items.map(async (machine) => {
					try {
						const page = await get<{ items: VmAuditEntry[] }>(
							`/api/v1/vms/${encodeURIComponent(machine.cluster)}/${machine.vmid}/audit`
						);
						return { machine, items: page.items };
					} catch {
						// One unreachable machine must not blank the whole timeline.
						return { machine, items: [] };
					}
				})
			);
			this.entries = mergeActivity(pages);
		} catch {
			this.error = m['activity.loadError']();
		} finally {
			this.loading = false;
		}
	}
}
