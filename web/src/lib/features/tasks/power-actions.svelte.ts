import { getContext, setContext } from 'svelte';
import { SvelteMap } from 'svelte/reactivity';
import type { VmAction } from '$lib/features/vms/detail.svelte';

/** One power action in flight on one machine. */
export interface PowerActionEntry {
	cluster: string;
	vmid: number;
	name: string;
	action: VmAction;
	/** Epoch ms when the action was sent. */
	startedAt: number;
}

const key = (cluster: string, vmid: number): string => `${cluster}:${vmid}`;

/**
 * PowerActionRegistry - app-wide record of power actions (start, shutdown,
 * reboot, ...) that are in flight. Power actions are not Proxmox tasks the
 * tray can poll (the action endpoint answers synchronously, then the caller
 * converges on the live status), so the list and the detail page register
 * them here while they wait. The registry feeds `displayStatus`
 * (starting / stopping), the Activity screen's "In progress" section and
 * the sidebar's Activity count. Session-scoped and in memory: a reload
 * forgets it, and the server status is the floor again.
 */
export class PowerActionRegistry {
	#entries = new SvelteMap<string, PowerActionEntry>();

	/** Marks an action as in flight. Replaces any earlier one on that VM. */
	begin(entry: Omit<PowerActionEntry, 'startedAt'>): void {
		this.#entries.set(key(entry.cluster, entry.vmid), { ...entry, startedAt: Date.now() });
	}

	/** Clears the in-flight action on a VM. Idempotent. */
	end(cluster: string, vmid: number): void {
		this.#entries.delete(key(cluster, vmid));
	}

	/** The action in flight on a VM, or null. */
	get(cluster: string, vmid: number): VmAction | null {
		return this.#entries.get(key(cluster, vmid))?.action ?? null;
	}

	/** Every in-flight action, oldest first. */
	get entries(): PowerActionEntry[] {
		return [...this.#entries.values()].sort((a, b) => a.startedAt - b.startedAt);
	}

	get size(): number {
		return this.#entries.size;
	}
}

const POWER_ACTIONS_CONTEXT_KEY = Symbol('power-actions');

/** Called once, by the app shell layout (alongside the task tray). */
export function setPowerActionsContext(): PowerActionRegistry {
	const registry = new PowerActionRegistry();
	setContext(POWER_ACTIONS_CONTEXT_KEY, registry);
	return registry;
}

/** Returns the shell's registry, or a detached one when rendered outside the
 *  shell (component tests), so callers never have to null-check. */
export function getPowerActionsContext(): PowerActionRegistry {
	return getContext<PowerActionRegistry | undefined>(POWER_ACTIONS_CONTEXT_KEY) ?? new PowerActionRegistry();
}
