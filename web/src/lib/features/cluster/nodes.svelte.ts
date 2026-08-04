import { getContext, setContext } from 'svelte';
import { get, ApiRequestError } from '$lib/shared/api/client';

export type NodeStatus = 'online' | 'offline' | 'unknown';

export interface ClusterNode {
	name: string;
	status: NodeStatus;
	cpuCores: number;
	cpuUsage: number;
	memoryTotal: number;
	memoryUsed: number;
	storageTotal: number;
	storageUsed: number;
}

interface NodesResponse {
	nodes: ClusterNode[];
}

export class NodesStore {
	nodes = $state.raw<ClusterNode[]>([]);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			const response = await get<NodesResponse>('/api/v1/cluster/nodes');
			this.nodes = response.nodes;
		} catch (err) {
			this.error = err instanceof ApiRequestError ? err.message : 'failed to load cluster nodes';
		} finally {
			this.loading = false;
		}
	}
}

const NODES_CONTEXT_KEY = Symbol('nodes');

/** Called once, by the route that owns this state (constitution VII: no module singletons). */
export function setNodesContext(): NodesStore {
	const store = new NodesStore();
	setContext(NODES_CONTEXT_KEY, store);
	return store;
}

export function getNodesContext(): NodesStore {
	return getContext<NodesStore>(NODES_CONTEXT_KEY);
}
