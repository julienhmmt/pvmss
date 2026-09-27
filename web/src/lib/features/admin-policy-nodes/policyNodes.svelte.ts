import { get, post, put, ApiRequestError } from '$lib/shared/api/client';
import { fetchClusterOptions, type ClusterOption } from '$lib/shared/clusters';
import { getContext, setContext } from 'svelte';
import { m } from '$lib/paraglide/messages.js';
import type { NodeStatus } from '../cluster/nodes.svelte';

export interface NodeCapacity {
	node: string;
	status: NodeStatus;
	maxVms: number;
	maxVcpus: number;
	maxRamGb: number;
	maxDiskGb: number;
	usedVms: number;
	usedVcpus: number;
	usedRamGb: number;
	usedDiskGb: number;
	physicalVcpus: number;
	physicalRamGb: number;
	/** Live node load covering every VM, not only pvmss-tagged ones. */
	nodeCpuUsage: number;
	nodeMemoryUsedGb: number;
	nodeStorageUsedGb: number;
	nodeStorageTotalGb: number;
	totalVms: number;
}

export interface NodeCapacityPatch {
	maxVms: number;
	maxVcpus: number;
	maxRamGb: number;
	maxDiskGb: number;
}

interface NodeCapacityListResponse {
	nodes: NodeCapacity[];
	refreshedAt: string;
}

export type NodeCapacitySortColumn = 'node' | 'vms' | 'vcpus' | 'ram' | 'disk' | 'load';

/** Manages live node discovery joined with server-owned capacité values. */
export class AdminPolicyNodesStore {
	nodes = $state.raw<NodeCapacity[]>([]);
	refreshedAt = $state.raw<string | null>(null);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	errorCode = $state.raw<string | null>(null);
	saving = $state.raw(false);
	saveError = $state.raw<string | null>(null);
	saveErrorCode = $state.raw<string | null>(null);
	clusterOptions = $state.raw<ClusterOption[]>([]);
	cluster = $state('');
	refreshing = $state.raw(false);
	refreshError = $state.raw<string | null>(null);
	refreshDisabled = $state.raw(false);

	#reenableTimer: ReturnType<typeof setTimeout> | null = null;

	sortBy: NodeCapacitySortColumn = $state('node');
	sortDir: 'asc' | 'desc' = $state('asc');

	sortedNodes = $derived(sortNodeCapacities(this.nodes, this.sortBy, this.sortDir));

	setSort(column: NodeCapacitySortColumn): void {
		if (this.sortBy === column) {
			this.sortDir = this.sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			this.sortBy = column;
			this.sortDir = 'asc';
		}
	}

	/** Resolves the real cluster name (matches admin-catalog's pattern) so
	 *  requests never send the literal "default" against a deployment whose
	 *  cluster is named something else. */
	async loadClusters(): Promise<void> {
		try {
			this.clusterOptions = await fetchClusterOptions();
			const first = this.clusterOptions[0];
			if (first && (this.clusterOptions.length === 1 || !this.clusterOptions.some((option) => option.name === this.cluster))) {
				this.cluster = first.name;
			}
		} catch (error: unknown) {
			this.errorCode = error instanceof ApiRequestError ? error.code : null;
			this.error = error instanceof ApiRequestError ? error.message : m['policy.nodesLoadError']();
		}
	}

	setCluster(value: string): void {
		this.cluster = value;
		void this.load();
	}

	async load(): Promise<void> {
		await this.loadClusters();
		this.loading = true;
		this.error = null;
		this.errorCode = null;
		try {
			const response = await get<NodeCapacityListResponse>(`/api/v1/admin/policy/nodes?cluster=${encodeURIComponent(this.cluster)}`);
			this.nodes = response.nodes;
			this.refreshedAt = response.refreshedAt || null;
		} catch (error: unknown) {
			this.errorCode = error instanceof ApiRequestError ? error.code : null;
			this.error = error instanceof ApiRequestError ? error.message : m['policy.nodesLoadError']();
		} finally {
			this.loading = false;
		}
	}

	async save(node: string, patch: NodeCapacityPatch): Promise<void> {
		this.saving = true;
		this.saveError = null;
		this.saveErrorCode = null;
		try {
			const updated = await put<NodeCapacity>(`/api/v1/admin/policy/nodes/${encodeURIComponent(node)}`, { cluster: this.cluster, ...patch });
			this.nodes = this.nodes.some((item) => item.node === node)
				? this.nodes.map((item) => (item.node === node ? updated : item))
				: [...this.nodes, updated];
		} catch (error: unknown) {
			this.saveErrorCode = error instanceof ApiRequestError ? error.code : null;
			this.saveError = error instanceof ApiRequestError ? error.message : m['policy.nodesSaveError']();
			throw error;
		} finally {
			this.saving = false;
		}
	}

	clearSaveError(): void {
		this.saveError = null;
		this.saveErrorCode = null;
	}

	async refresh(): Promise<void> {
		this.refreshing = true;
		this.refreshError = null;
		try {
			const response = await post<{ refreshedAt: string }>('/api/v1/cluster/refresh');
			this.refreshedAt = response.refreshedAt;
			this.clearRefreshDisabled();
			await this.load();
		} catch (error: unknown) {
			if (error instanceof ApiRequestError && error.code === 'refresh_too_soon') {
				this.refreshDisabled = true;
				this.refreshError = error.message;
				this.#scheduleReenable(error.retryAfterSeconds ?? 5);
			} else {
				this.refreshError = error instanceof ApiRequestError ? error.message : m['nodes.errorRefresh']();
			}
		} finally {
			this.refreshing = false;
		}
	}

	async retryConnection(): Promise<void> {
		try {
			await post('/api/v1/cluster/refresh');
		} catch {
			// Ignore; the next load will surface the current state.
		}
		await this.load();
	}

	#scheduleReenable(seconds: number): void {
		if (this.#reenableTimer !== null) clearTimeout(this.#reenableTimer);
		this.#reenableTimer = setTimeout(() => {
			this.#reenableTimer = null;
			this.clearRefreshDisabled();
		}, seconds * 1000);
	}

	clearRefreshDisabled(): void {
		if (this.#reenableTimer !== null) {
			clearTimeout(this.#reenableTimer);
			this.#reenableTimer = null;
		}
		this.refreshDisabled = false;
		this.refreshError = null;
	}
}

const POLICY_NODES_CONTEXT_KEY = Symbol('admin-policy-nodes');

export function setAdminPolicyNodesContext(): AdminPolicyNodesStore {
	const store = new AdminPolicyNodesStore();
	setContext(POLICY_NODES_CONTEXT_KEY, store);
	return store;
}

export function getAdminPolicyNodesContext(): AdminPolicyNodesStore {
	return getContext<AdminPolicyNodesStore>(POLICY_NODES_CONTEXT_KEY);
}

/** Saturation of one dimension: distance to the cap that refuses creation.
 *  Uncapped dimensions report 0 - they can never block. */
function saturation(used: number, cap: number): number {
	return cap > 0 ? used / cap : 0;
}

function dimensionSaturation(node: NodeCapacity, dimension: 'vms' | 'vcpus' | 'ram' | 'disk'): number {
	switch (dimension) {
		case 'vms':
			return saturation(node.usedVms, node.maxVms);
		case 'vcpus':
			return saturation(node.usedVcpus, node.maxVcpus);
		case 'ram':
			return saturation(node.usedRamGb, node.maxRamGb);
		case 'disk':
			return saturation(node.usedDiskGb, node.maxDiskGb);
	}
}

/** Worst live-load ratio across CPU, memory, and storage (0 when unknown). */
function loadScore(node: NodeCapacity): number {
	const memory = node.physicalRamGb > 0 ? node.nodeMemoryUsedGb / node.physicalRamGb : 0;
	const storage = node.nodeStorageTotalGb > 0 ? node.nodeStorageUsedGb / node.nodeStorageTotalGb : 0;
	return Math.max(node.nodeCpuUsage, memory, storage);
}

function sortNodeCapacities(nodes: NodeCapacity[], sortBy: NodeCapacitySortColumn, dir: 'asc' | 'desc'): NodeCapacity[] {
	const sorted = [...nodes].sort((a, b) => {
		let cmp: number;
		if (sortBy === 'node') {
			cmp = a.node.localeCompare(b.node);
		} else if (sortBy === 'load') {
			cmp = loadScore(a) - loadScore(b) || a.node.localeCompare(b.node);
		} else {
			cmp = dimensionSaturation(a, sortBy) - dimensionSaturation(b, sortBy) || a.node.localeCompare(b.node);
		}
		return cmp;
	});
	return dir === 'asc' ? sorted : sorted.reverse();
}
