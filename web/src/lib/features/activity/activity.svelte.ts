import { get } from '$lib/shared/api/client';
import { m } from '$lib/paraglide/messages.js';
import type { VmListItem, VmListResult } from '$lib/features/vms/list.svelte';
import type { VmAuditEntry } from '$lib/features/vms/detail.svelte';

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
	delete_disk: () => m['activity.action.delete_disk']()
};

/** A plain-language message for an audit action; unknown ones keep the raw
 *  action name so nothing is hidden. */
export function activityMessage(action: string): string {
	return ACTION_LABELS[action]?.() ?? m['activity.action.other']({ action });
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
