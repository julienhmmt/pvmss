import { del, get, post, ApiRequestError } from '$lib/shared/api/client';
import { m } from '$lib/paraglide/messages.js';
import type { TaskKind, TaskTrayStore } from '$lib/features/tasks/tasks.svelte';

export interface VmSnapshot {
	name: string;
	description: string;
	createdAt: string;
	vmstate: boolean;
}

interface SnapshotListResponse {
	snapshots: VmSnapshot[];
	maxSnapshots: number;
}

interface SnapshotTaskResponse {
	cluster: string;
	vmid: number;
	name: string;
	upid: string;
}

/** Owns live snapshot data and registers accepted writes with the global task tray. */
export class VmSnapshotsStore {
	readonly cluster: string;
	readonly vmid: number;
	readonly #tray: TaskTrayStore;
	snapshots = $state.raw<readonly VmSnapshot[]>([]);
	maxSnapshots = $state.raw<number | null>(null);
	loading = $state.raw(false);
	inFlight = $state.raw(false);
	error = $state.raw<string | null>(null);
	readonly #basePath: string;

	constructor(cluster: string, vmid: number, tray: TaskTrayStore) {
		this.cluster = cluster;
		this.vmid = vmid;
		this.#tray = tray;
		this.#basePath = `/api/v1/vms/${encodeURIComponent(cluster)}/${vmid}/snapshots`;
	}

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			const response = await get<SnapshotListResponse>(this.#basePath);
			this.snapshots = response.snapshots;
			this.maxSnapshots = response.maxSnapshots;
		} catch (error: unknown) {
			this.error = errorMessage(error, () => m['vms.snapshots.errorLoad']());
		} finally {
			this.loading = false;
		}
	}

	async create(name: string, description: string, vmstate: boolean): Promise<boolean> {
		return this.dispatch('vm_snapshot_create', name, () => post<SnapshotTaskResponse>(this.#basePath, { name, description, vmstate }));
	}

	async rollback(name: string): Promise<boolean> {
		return this.dispatch('vm_snapshot_rollback', name, () => post<SnapshotTaskResponse>(`${this.#basePath}/${encodeURIComponent(name)}/rollback`));
	}

	async delete(name: string): Promise<boolean> {
		return this.dispatch('vm_snapshot_delete', name, () => del<SnapshotTaskResponse>(`${this.#basePath}/${encodeURIComponent(name)}`));
	}

	clearError(): void {
		this.error = null;
	}

	async dispatch(kind: TaskKind, name: string, request: () => Promise<SnapshotTaskResponse>): Promise<boolean> {
		if (this.inFlight) return false;
		this.inFlight = true;
		this.error = null;
		try {
			const task = await request();
			this.#tray.track({ upid: task.upid, kind, vmid: this.vmid, name, cluster: this.cluster });
			return true;
		} catch (error: unknown) {
			this.error = errorMessage(error, () => m['vms.snapshots.errorOperation']());
			return false;
		} finally {
			this.inFlight = false;
		}
	}
}

function errorMessage(error: unknown, fallback: () => string): string {
	return error instanceof ApiRequestError ? error.message : fallback();
}
