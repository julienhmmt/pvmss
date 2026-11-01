import { get, post, ApiRequestError } from '#lib/shared/api/client.js';
import {
	fetchClusterOptions,
	resolveInitialAdminCluster,
	saveAdminCluster,
	type ClusterOption
} from '#lib/shared/clusters.js';
import { getContext, setContext } from 'svelte';
import { SvelteSet } from 'svelte/reactivity';
import { m } from '#lib/paraglide/messages.js';
import type { MachineDisplayStatus } from '#lib/features/vms/display-status.js';

export interface RecoveryVM {
	vmid: number;
	name: string;
	node: string;
	status: MachineDisplayStatus;
	pool?: string;
	tags: string[];
}

export interface RecoveryPool {
	name: string;
	comment: string;
	members: RecoveryVM[];
}

export interface RecoveryTag {
	name: string;
	vmCount: number;
	color: string;
}

export interface RecoveryReport {
	cluster: string;
	scannedAt: string;
	pools: RecoveryPool[];
	looseVMs: RecoveryVM[];
	tags: RecoveryTag[];
}

interface AdoptResult {
	pools: string[];
	tags: { name: string; color: string }[];
}

const ADMIN_RECOVERY_CONTEXT_KEY = Symbol('admin-recovery');

/** Manages the orphan-adoption report and the adopt selection per cluster. */
export class RecoveryStore {
	report = $state.raw<RecoveryReport | null>(null);
	loading = $state.raw(false);
	rescanning = $state.raw(false);
	error = $state.raw<string | null>(null);
	adopting = $state.raw(false);
	adoptError = $state.raw<string | null>(null);
	announce = $state.raw<string | null>(null);
	clusterOptions = $state.raw<ClusterOption[]>([]);
	cluster = $state('default');
	selectedPools = new SvelteSet<string>();
	selectedTags = new SvelteSet<string>();

	async loadClusters(): Promise<void> {
		try {
			this.clusterOptions = await fetchClusterOptions();
			if (this.clusterOptions.length > 0) {
				this.cluster = resolveInitialAdminCluster(this.clusterOptions, this.cluster);
			}
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : m['admin.recovery.loadError']();
		}
	}

	setCluster(value: string): void {
		saveAdminCluster(value);
		this.cluster = value;
		void this.load();
	}

	async load(): Promise<void> {
		if (this.clusterOptions.length === 0) {
			await this.loadClusters();
		}
		this.loading = true;
		this.error = null;
		try {
			this.report = await get<RecoveryReport>(`/api/v1/admin/recovery?cluster=${encodeURIComponent(this.cluster)}`);
			this.selectAllOrphans();
		} catch (error) {
			this.error = error instanceof ApiRequestError ? error.message : m['admin.recovery.loadError']();
		} finally {
			this.loading = false;
		}
	}

	// rescan is the explicit "scan again" action: the GET report is cached
	// server-side and an empty report is pinned, so a fresh Proxmox read only
	// happens through this POST.
	async rescan(): Promise<void> {
		this.rescanning = true;
		this.error = null;
		try {
			this.report = await post<RecoveryReport>('/api/v1/admin/recovery/scan', { cluster: this.cluster });
			this.selectAllOrphans();
		} catch (error) {
			this.error = error instanceof ApiRequestError ? error.message : m['admin.recovery.loadError']();
		} finally {
			this.rescanning = false;
		}
	}

	private selectAllOrphans(): void {
		// Fresh report: default to adopting everything found - the admin
		// unchecks what should stay as-is on the cluster.
		this.selectedPools.clear();
		this.selectedTags.clear();
		for (const pool of this.report?.pools ?? []) this.selectedPools.add(pool.name);
		for (const tag of this.report?.tags ?? []) this.selectedTags.add(tag.name);
	}

	togglePool(name: string): void {
		if (this.selectedPools.has(name)) {
			this.selectedPools.delete(name);
		} else {
			this.selectedPools.add(name);
		}
	}

	toggleTag(name: string): void {
		if (this.selectedTags.has(name)) {
			this.selectedTags.delete(name);
		} else {
			this.selectedTags.add(name);
		}
	}

	async adopt(): Promise<void> {
		this.adoptError = null;
		this.announce = null;
		const pools = [...this.selectedPools];
		const tags = [...this.selectedTags];
		if (pools.length === 0 && tags.length === 0) {
			this.adoptError = m['admin.recovery.nothingSelected']();
			return;
		}
		this.adopting = true;
		try {
			const result = await post<AdoptResult>('/api/v1/admin/recovery', {
				cluster: this.cluster,
				pools,
				tags
			});
			this.announce = m['admin.recovery.adopted']({ pools: result.pools.length, tags: result.tags.length });
			await this.load();
		} catch (error) {
			this.adoptError = error instanceof ApiRequestError ? error.message : m['admin.recovery.adoptError']();
		} finally {
			this.adopting = false;
		}
	}
}

export function setAdminRecoveryContext(): RecoveryStore {
	const store = new RecoveryStore();
	setContext(ADMIN_RECOVERY_CONTEXT_KEY, store);
	return store;
}

export function getAdminRecoveryContext(): RecoveryStore {
	return getContext<RecoveryStore>(ADMIN_RECOVERY_CONTEXT_KEY);
}
