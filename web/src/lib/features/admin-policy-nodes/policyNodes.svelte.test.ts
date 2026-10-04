import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AdminPolicyNodesStore, translateNodeCapacityError, type NodeCapacity } from './policyNodes.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

function fixture(overrides: Partial<NodeCapacity>): NodeCapacity {
	return {
		node: 'pve-node-01',
		status: 'online',
		maxVms: 0,
		maxVcpus: 0,
		maxRamGb: 0,
		maxDiskGb: 0,
		usedVms: 0,
		usedVcpus: 0,
		usedRamGb: 0,
		usedDiskGb: 0,
		physicalVcpus: 0,
		physicalRamGb: 0,
		nodeCpuUsage: 0,
		nodeMemoryUsedGb: 0,
		nodeStorageUsedGb: 0,
		nodeStorageTotalGb: 0,
		totalVms: 0,
		approved: true,
		...overrides
	};
}

describe('AdminPolicyNodesStore', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	it('loads discovered nodes with usage, physical capacity, configured capacity, and live load', async () => {
		const nodes = [fixture({ node: 'pve-node-01', usedVms: 4, usedVcpus: 12, usedRamGb: 24, physicalVcpus: 32, physicalRamGb: 128, nodeCpuUsage: 0.4, totalVms: 7 })];
		vi.stubGlobal('fetch', vi.fn().mockImplementation(() => jsonResponse(200, { nodes, refreshedAt: '2026-09-27T10:00:00Z' })));
		const store = new AdminPolicyNodesStore();
		await store.load();
		expect(store.nodes).toEqual(nodes);
		expect(store.refreshedAt).toBe('2026-09-27T10:00:00Z');
	});

	it('saves a node capacity through the shared API client', async () => {
		const node = fixture({ node: 'pve-node-02', maxVms: 6, maxVcpus: 4, maxRamGb: 16, usedVms: 2, usedVcpus: 2, usedRamGb: 6, physicalVcpus: 16, physicalRamGb: 64 });
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, node));
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminPolicyNodesStore();
		await store.save('pve-node-02', { maxVms: 6, maxVcpus: 4, maxRamGb: 16, maxDiskGb: 0 });
		expect(store.nodes).toContainEqual(node);
		expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/v1/admin/policy/nodes/pve-node-02');
	});

	describe('sortedNodes', () => {
		// Saturation ratios: node-01 uncapped (0), node-02 at 50% vCPU,
		// node-03 at 90% vCPU - usage numbers alone would rank them wrong.
		const nodes = [
			fixture({ node: 'pve-node-02', maxVms: 6, maxVcpus: 4, maxRamGb: 16, maxDiskGb: 100, usedVms: 2, usedVcpus: 2, usedRamGb: 6, usedDiskGb: 90, physicalVcpus: 16, physicalRamGb: 64 }),
			fixture({ node: 'pve-node-01', usedVms: 4, usedVcpus: 12, usedRamGb: 24, physicalVcpus: 32, physicalRamGb: 128 }),
			fixture({ node: 'pve-node-03', maxVms: 10, maxVcpus: 8, maxRamGb: 32, maxDiskGb: 100, usedVms: 1, usedVcpus: 7, usedRamGb: 2, usedDiskGb: 10, physicalVcpus: 8, physicalRamGb: 32 })
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

		it('sorts a dimension by usage-to-cap ratio, uncapped last', () => {
			const store = new AdminPolicyNodesStore();
			store.nodes = nodes;
			store.sortBy = 'vcpus';
			store.sortDir = 'desc';
			// node-03: 7.2/8 = 90%, node-02: 2/4 = 50%, node-01 uncapped = 0
			expect(store.sortedNodes.map((n) => n.node)).toEqual(['pve-node-03', 'pve-node-02', 'pve-node-01']);
		});

		it('sorts the disk dimension by ratio', () => {
			const store = new AdminPolicyNodesStore();
			store.nodes = nodes;
			store.sortBy = 'disk';
			store.sortDir = 'desc';
			// node-02: 90/100, node-03: 10/100, node-01 uncapped
			expect(store.sortedNodes.map((n) => n.node)).toEqual(['pve-node-02', 'pve-node-03', 'pve-node-01']);
		});

		it('sorts the load column by worst live ratio', () => {
			const store = new AdminPolicyNodesStore();
			store.nodes = [
				fixture({ node: 'idle', nodeCpuUsage: 0.1 }),
				fixture({ node: 'hot-disk', nodeCpuUsage: 0.1, nodeStorageUsedGb: 90, nodeStorageTotalGb: 100 }),
				fixture({ node: 'hot-ram', physicalRamGb: 64, nodeMemoryUsedGb: 60 })
			];
			store.sortBy = 'load';
			store.sortDir = 'desc';
			// hot-ram: 60/64 = 93.75%, hot-disk: 90/100 = 90%, idle: 10%
			expect(store.sortedNodes.map((n) => n.node)).toEqual(['hot-ram', 'hot-disk', 'idle']);
		});

		it('setSort toggles direction on same column and resets on new column', () => {
			const store = new AdminPolicyNodesStore();
			store.sortBy = 'node';
			store.sortDir = 'asc';
			store.setSort('node');
			expect(store.sortDir).toBe('desc');
			store.setSort('vcpus');
			expect(store.sortBy).toBe('vcpus');
			expect(store.sortDir).toBe('asc');
		});
	});

	describe('translateNodeCapacityError', () => {
		// Locale-independent assertions: the tests run under the base locale
		// (fr), so only interpolated values and pass-through are checked.
		it('localizes node_limit_above_capacity with dimension, requested, node, and physical', () => {
			const message = "vcpu cap (3133) exceeds miniquarium's physical capacity (12)";
			const translated = translateNodeCapacityError('node_limit_above_capacity', message);
			expect(translated).not.toBe(message);
			expect(translated).toContain('3133');
			expect(translated).toContain('12');
			expect(translated).toContain('miniquarium');
		});

		it('localizes node_limit_below_usage with dimension, requested, node, and used', () => {
			const message = "ram cap (4) is below pve-node-01's current usage (8)";
			const translated = translateNodeCapacityError('node_limit_below_usage', message);
			expect(translated).not.toBe(message);
			expect(translated).toContain('4');
			expect(translated).toContain('8');
			expect(translated).toContain('pve-node-01');
		});

		it('localizes invalid_policy "must not be negative" without echoing the server field name', () => {
			const translated = translateNodeCapacityError('invalid_policy', 'maxVcpus must not be negative');
			expect(translated).not.toContain('maxVcpus');
		});

		it('passes unparseable messages through unchanged', () => {
			expect(translateNodeCapacityError('node_limit_above_capacity', 'something unexpected')).toBe('something unexpected');
			expect(translateNodeCapacityError('other_code', "vcpu cap (3133) exceeds n's physical capacity (12)")).toBe("vcpu cap (3133) exceeds n's physical capacity (12)");
		});
	});
});
