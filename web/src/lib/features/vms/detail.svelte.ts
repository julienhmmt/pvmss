import { getContext, setContext } from 'svelte';
import { get, post, del, patch, ApiRequestError } from '$lib/shared/api/client';
import type { VmStatus } from './list.svelte';

export type VmAction = 'start' | 'stop' | 'shutdown' | 'reboot' | 'reset';

export interface VmDetailEntity {
	vmid: number;
	name: string;
	node: string;
	pool: string;
	status: VmStatus;
	tags: string[];
	cpuCores: number;
	memoryTotal: number;
	diskTotal: number;
	uptimeSeconds?: number;
	description?: string;
}

interface ActionResponse {
	status: string;
}

interface DeleteResponse {
	status: string;
}

/**
 * State for the single-VM detail view (V15). One store instance per consuming
 * screen (constitution VII: no module singletons). The entity is `$state.raw`
 * because it is replaced wholesale on load/reload, not mutated field-by-field
 * — except for the optimistic status flip during a power action (V12), which
 * is a deliberate local mutation reconciled by reload().
 */
export class VmDetailStore {
	readonly cluster: string;
	readonly vmid: number;

	entity = $state.raw<VmDetailEntity | null>(null);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);

	/** True while a power action is in flight; the UI shows an optimistic status. */
	actionInFlight = $state.raw(false);
	actionError = $state.raw<string | null>(null);

	/** True while a delete is in flight; the UI disables the button. */
	deleteInFlight = $state.raw(false);
	deleteError = $state.raw<string | null>(null);

	/** True while a patch (rename/description) is in flight. */
	patchInFlight = $state.raw(false);
	patchError = $state.raw<string | null>(null);

	/** Set after a successful delete so the page can navigate away. */
	deleted = $state.raw(false);

	#basePath: string;

	constructor(cluster: string, vmid: number) {
		this.cluster = cluster;
		this.vmid = vmid;
		this.#basePath = `/api/v1/vms/${encodeURIComponent(cluster)}/${vmid}`;
	}

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.entity = await get<VmDetailEntity>(this.#basePath);
		} catch (err) {
			this.error = errorMessage(err, 'failed to load VM');
		} finally {
			this.loading = false;
		}
	}

	/**
	 * Triggers a power action (V12). The status flips optimistically before the
	 * server responds, then reload() reconciles with the authoritative state.
	 * `aria-live` on the status element (constitution XII) announces the flip.
	 */
	async action(kind: VmAction): Promise<void> {
		if (this.actionInFlight || this.entity === null) return;
		this.actionError = null;
		this.actionInFlight = true;

		const previousStatus = this.entity.status;
		this.entity = { ...this.entity, status: optimisticStatus(kind) };

		try {
			await post<ActionResponse>(`${this.#basePath}/actions`, { action: kind });
			await this.load();
		} catch (err) {
			// Revert the optimistic flip on failure.
			if (this.entity !== null) {
				this.entity = { ...this.entity, status: previousStatus };
			}
			this.actionError = errorMessage(err, 'action failed');
		} finally {
			this.actionInFlight = false;
		}
	}

	/** Permanently deletes the VM (V14: no soft-delete, no undo). */
	async delete(): Promise<void> {
		if (this.deleteInFlight || this.entity === null) return;
		this.deleteError = null;
		this.deleteInFlight = true;
		try {
			await del<DeleteResponse>(this.#basePath);
			this.deleted = true;
		} catch (err) {
			this.deleteError = errorMessage(err, 'delete failed');
		} finally {
			this.deleteInFlight = false;
		}
	}

	/**
	 * Renames and/or updates the description (V16/V17). Returns true on success
	 * so the caller can exit inline-edit mode. Empty values are omitted from the
	 * request body — the server treats an absent field as "no change".
	 */
	async patch(name: string | null, description: string | null): Promise<boolean> {
		if (this.patchInFlight || this.entity === null) return false;
		this.patchError = null;
		this.patchInFlight = true;
		try {
			const body: Record<string, string> = {};
			if (name !== null) body.name = name;
			if (description !== null) body.description = description;
			this.entity = await patch<VmDetailEntity>(this.#basePath, body);
			return true;
		} catch (err) {
			this.patchError = errorMessage(err, 'update failed');
			return false;
		} finally {
			this.patchInFlight = false;
		}
	}
}

/** optimisticStatus returns the status a VM is expected to show after kind. */
function optimisticStatus(kind: VmAction): VmStatus {
	switch (kind) {
		case 'start':
		case 'reboot':
		case 'reset':
			return 'running';
		case 'stop':
		case 'shutdown':
			return 'stopped';
	}
}

function errorMessage(err: unknown, fallback: string): string {
	return err instanceof ApiRequestError ? err.message : fallback;
}

const VM_DETAIL_CONTEXT_KEY = Symbol('vm-detail');

/** Called once, by the route that owns this state (constitution VII). */
export function setVmDetailContext(cluster: string, vmid: number): VmDetailStore {
	const store = new VmDetailStore(cluster, vmid);
	setContext(VM_DETAIL_CONTEXT_KEY, store);
	return store;
}

export function getVmDetailContext(): VmDetailStore {
	return getContext<VmDetailStore>(VM_DETAIL_CONTEXT_KEY);
}
