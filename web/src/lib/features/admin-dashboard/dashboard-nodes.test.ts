import { describe, expect, it } from 'vitest';
import { groupNodesByCluster } from './dashboard-nodes';
import type { NodeSummary } from './dashboard.svelte';

function node(clusterKey: string, name: string, vmCount: number): NodeSummary {
	return {
		clusterKey,
		cluster: clusterKey.toUpperCase(),
		name,
		status: 'online',
		vmCount,
		vmRunningCount: 0,
		pvmssVMCount: 0,
		pvmssRamGB: 0,
		policyMaxRamGB: 0,
		policyMaxVMs: 0,
		unadoptedVMCount: 0,
		cpuCores: 4,
		cpuUsage: 0,
		memoryTotalBytes: 1,
		memoryUsedBytes: 0
	};
}

describe('groupNodesByCluster', () => {
	it('groups in first-seen order, keeps node order and sums VMs', () => {
		const groups = groupNodesByCluster([node('east', 'b', 2), node('west', 'a', 1), node('east', 'a', 3)]);
		expect(groups.map((g) => [g.clusterKey, g.label, g.nodes.map((n) => n.name), g.vmCount])).toEqual([
			['east', 'EAST', ['b', 'a'], 5],
			['west', 'WEST', ['a'], 1]
		]);
	});

	it('returns no groups for no nodes', () => {
		expect(groupNodesByCluster([])).toEqual([]);
	});
});
