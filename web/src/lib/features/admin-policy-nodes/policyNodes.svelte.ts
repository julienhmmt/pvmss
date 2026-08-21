import { get, put, ApiRequestError } from '$lib/shared/api/client';
import { getContext, setContext } from 'svelte';
import { m } from '$lib/paraglide/messages.js';

export interface NodeCapacity {
	node: string;
	maxVms: number;
	maxVcpus: number;
	maxRamGb: number;
	maxDiskGb: number;
	usedVms: number;
	usedVcpus: number;
	usedRamGb: number;
	physicalVcpus: number;
	physicalRamGb: number;
}

export interface NodeCapacityPatch {
	maxVms: number;
	maxVcpus: number;
	maxRamGb: number;
	maxDiskGb: number;
}

/** Manages live node discovery joined with server-owned capacité values. */
export class AdminPolicyNodesStore {
	nodes = $state.raw<NodeCapacity[]>([]);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);
	saving = $state.raw(false);
	saveError = $state.raw<string | null>(null);

	sortBy: 'node' | 'maxVms' | 'maxVcpus' | 'maxRamGb' | 'maxDiskGb' | 'usedVms' | 'usedVcpus' | 'usedRamGb' | 'physicalVcpus' | 'physicalRamGb' = $state('node');
	sortDir: 'asc' | 'desc' = $state('asc');

	sortedNodes = $derived(sortNodeCapacities(this.nodes, this.sortBy, this.sortDir));

	setSort(column: 'node' | 'maxVms' | 'maxVcpus' | 'maxRamGb' | 'maxDiskGb' | 'usedVms' | 'usedVcpus' | 'usedRamGb' | 'physicalVcpus' | 'physicalRamGb'): void {
		if (this.sortBy === column) {
			this.sortDir = this.sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			this.sortBy = column;
			this.sortDir = 'asc';
		}
	}

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.nodes = await get<NodeCapacity[]>('/api/v1/admin/policy/nodes?cluster=default');
		} catch (error: unknown) {
			this.error = error instanceof ApiRequestError ? error.message : m['policy.nodesLoadError']();
		} finally {
			this.loading = false;
		}
	}

	async save(node: string, patch: NodeCapacityPatch): Promise<void> {
		this.saving = true;
		this.saveError = null;
		try {
			const updated = await put<NodeCapacity>(`/api/v1/admin/policy/nodes/${encodeURIComponent(node)}`, { cluster: 'default', ...patch });
			this.nodes = this.nodes.some((item) => item.node === node)
				? this.nodes.map((item) => (item.node === node ? updated : item))
				: [...this.nodes, updated];
		} catch (error: unknown) {
			this.saveError = error instanceof ApiRequestError ? error.message : m['policy.nodesSaveError']();
			throw error;
		} finally {
			this.saving = false;
		}
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

type NodeCapacitySortColumn = 'node' | 'maxVms' | 'maxVcpus' | 'maxRamGb' | 'maxDiskGb' | 'usedVms' | 'usedVcpus' | 'usedRamGb' | 'physicalVcpus' | 'physicalRamGb';

function sortNodeCapacities(nodes: NodeCapacity[], sortBy: NodeCapacitySortColumn, dir: 'asc' | 'desc'): NodeCapacity[] {
	const sorted = [...nodes].sort((a, b) => {
		let cmp: number;
		if (sortBy === 'node') {
			cmp = a.node.localeCompare(b.node);
		} else {
			cmp = a[sortBy] - b[sortBy] || a.node.localeCompare(b.node);
		}
		return cmp;
	});
	return dir === 'asc' ? sorted : sorted.reverse();
}
