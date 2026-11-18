import type { NodeSummary } from './dashboard.svelte';

export interface NodeClusterGroup {
	clusterKey: string;
	label: string;
	nodes: NodeSummary[];
	vmCount: number;
}

/** Groups dashboard nodes by cluster in first-seen order. The server already
 * sorts nodes worst state first inside each cluster, so node order is kept. */
export function groupNodesByCluster(nodes: readonly NodeSummary[]): NodeClusterGroup[] {
	const groups = new Map<string, NodeClusterGroup>();
	for (const node of nodes) {
		const group = groups.get(node.clusterKey) ?? { clusterKey: node.clusterKey, label: node.cluster, nodes: [], vmCount: 0 };
		groups.set(node.clusterKey, { ...group, nodes: [...group.nodes, node], vmCount: group.vmCount + node.vmCount });
	}
	return [...groups.values()];
}
