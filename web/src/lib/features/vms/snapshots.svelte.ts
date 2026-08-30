import { del, get, post, ApiRequestError } from '$lib/shared/api/client';
import { m } from '$lib/paraglide/messages.js';
import { SvelteSet } from 'svelte/reactivity';
import type { TaskKind, TaskTrayStore } from '$lib/features/tasks/tasks.svelte';

export interface VmSnapshot {
	name: string;
	description: string;
	createdAt: string;
	vmstate: boolean;
}

/** Server-computed snapshot capability of the VM (ticket 07). Warnings are
 *  English strings generated server-side; they inform rather than block. */
export interface VmSnapshotCapability {
	canSnapshot: boolean;
	canVMState: boolean;
	warnings: string[];
}

interface SnapshotListResponse {
	snapshots: VmSnapshot[];
	maxSnapshots: number;
	/** Absent on older servers — callers fall back to the `!running` rule. */
	capability?: VmSnapshotCapability;
}

interface SnapshotTaskResponse {
	cluster: string;
	vmid: number;
	name: string;
	upid: string;
}

export interface SnapshotConfigResponse {
	name: string;
	config: Record<string, string>;
}

/** One changed config key between the VM's current config and a snapshot's. */
export interface RollbackDiffEntry {
	key: string;
	before: string;
	after: string;
}

/** Owns live snapshot data and registers accepted writes with the global task tray. */
export class VmSnapshotsStore {
	readonly cluster: string;
	readonly vmid: number;
	readonly #tray: TaskTrayStore;
	snapshots = $state.raw<readonly VmSnapshot[]>([]);
	maxSnapshots = $state.raw<number | null>(null);
	capability = $state.raw<VmSnapshotCapability | null>(null);
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
			this.capability = response.capability ?? null;
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

	/** Diffs a snapshot's stored config against the VM's current config so a
	 *  rollback can be previewed before it happens (ticket 08). `name` may be
	 *  `current` for the live pseudo-entry; the diff is computed over the
	 *  union of keys, listing every key whose value would change. */
	async rollbackDiff(name: string): Promise<RollbackDiffEntry[]> {
		const [snapshot, current] = await Promise.all([
			get<SnapshotConfigResponse>(`${this.#basePath}/${encodeURIComponent(name)}/config`),
			get<SnapshotConfigResponse>(`${this.#basePath}/current/config`)
		]);
		const keys = new SvelteSet([...Object.keys(snapshot.config), ...Object.keys(current.config)]);
		const entries: RollbackDiffEntry[] = [];
		for (const key of keys) {
			const before = current.config[key] ?? '';
			const after = snapshot.config[key] ?? '';
			if (before !== after) entries.push({ key, before, after });
		}
		return entries.sort((a, b) => a.key.localeCompare(b.key));
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
