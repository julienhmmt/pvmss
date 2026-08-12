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
});
