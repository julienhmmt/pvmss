import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AdminPolicyNodesStore } from './policyNodes.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('AdminPolicyNodesStore', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	it('loads discovered nodes with usage, physical capacity, and configured capacity', async () => {
		const nodes = [{ node: 'pve-node-01', maxVms: 0, maxVcpus: 0, maxRamGb: 0, maxDiskGb: 0, usedVms: 4, usedVcpus: 12, usedRamGb: 24, physicalVcpus: 32, physicalRamGb: 128 }];
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, nodes)));
		const store = new AdminPolicyNodesStore();
		await store.load();
		expect(store.nodes).toEqual(nodes);
	});

	it('saves a node capacity through the shared API client', async () => {
		const node = { node: 'pve-node-02', maxVms: 6, maxVcpus: 4, maxRamGb: 16, maxDiskGb: 0, usedVms: 2, usedVcpus: 2, usedRamGb: 6, physicalVcpus: 16, physicalRamGb: 64 };
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, node));
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminPolicyNodesStore();
		await store.save('pve-node-02', { maxVms: 6, maxVcpus: 4, maxRamGb: 16, maxDiskGb: 0 });
		expect(store.nodes).toContainEqual(node);
		expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/v1/admin/policy/nodes/pve-node-02');
	});

	describe('sortedNodes', () => {
		const nodes = [
			{ node: 'pve-node-02', maxVms: 6, maxVcpus: 4, maxRamGb: 16, maxDiskGb: 0, usedVms: 2, usedVcpus: 2, usedRamGb: 6, physicalVcpus: 16, physicalRamGb: 64 },
			{ node: 'pve-node-01', maxVms: 0, maxVcpus: 0, maxRamGb: 0, maxDiskGb: 0, usedVms: 4, usedVcpus: 12, usedRamGb: 24, physicalVcpus: 32, physicalRamGb: 128 },
			{ node: 'pve-node-03', maxVms: 10, maxVcpus: 8, maxRamGb: 32, maxDiskGb: 100, usedVms: 1, usedVcpus: 1, usedRamGb: 2, physicalVcpus: 8, physicalRamGb: 32 }
		];

		it('sorts by node name ascending then descending', () => {
			const store = new AdminPolicyNodesStore();
			store.nodes = nodes;
			store.sortBy = 'node';
			store.sortDir = 'asc';
			expect(store.sortedNodes.map((n) => n.node)).toEqual(['pve-node-01', 'pve-node-02', 'pve-node-03']);
			store.sortDir = 'desc';
			expect(store.sortedNodes.map((n) => n.node)).toEqual(['pve-node-03', 'pve-node-02', 'pve-node-01']);
		});

		it('sorts by maxVms with node as tiebreaker', () => {
			const store = new AdminPolicyNodesStore();
			store.nodes = nodes;
			store.sortBy = 'maxVms';
			store.sortDir = 'asc';
			expect(store.sortedNodes.map((n) => n.maxVms)).toEqual([0, 6, 10]);
		});

		it('sorts by usedVms', () => {
			const store = new AdminPolicyNodesStore();
			store.nodes = nodes;
			store.sortBy = 'usedVms';
			store.sortDir = 'asc';
			expect(store.sortedNodes.map((n) => n.usedVms)).toEqual([1, 2, 4]);
		});

		it('sorts by physicalVcpus', () => {
			const store = new AdminPolicyNodesStore();
			store.nodes = nodes;
			store.sortBy = 'physicalVcpus';
			store.sortDir = 'asc';
			expect(store.sortedNodes.map((n) => n.physicalVcpus)).toEqual([8, 16, 32]);
		});

		it('setSort toggles direction on same column and resets on new column', () => {
			const store = new AdminPolicyNodesStore();
			store.sortBy = 'node';
			store.sortDir = 'asc';
			store.setSort('node');
			expect(store.sortDir).toBe('desc');
			store.setSort('maxVms');
			expect(store.sortBy).toBe('maxVms');
			expect(store.sortDir).toBe('asc');
		});
	});
});
