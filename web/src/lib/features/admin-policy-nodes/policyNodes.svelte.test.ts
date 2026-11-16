import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
	AdminPolicyNodesStore,
	buildAllClustersNodeCapacityExport,
	buildNodeCapacityExport,
	NODE_CAPACITY_EXPORT_KIND,
	parseNodeCapacityExport,
	translateNodeCapacityError,
	type NodeCapacity
} from './policyNodes.svelte';

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

	it('refresh reloads until the background refresh has completed', async () => {
		vi.useFakeTimers();
		const before = [fixture({ node: 'pve-node-01', usedVms: 1 })];
		const after = [fixture({ node: 'pve-node-01', usedVms: 5 })];
		let gets = 0;
		vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string, init?: RequestInit) => {
			if (init?.method === 'POST') return Promise.resolve(jsonResponse(202, { refreshedAt: '2026-09-27T10:05:00Z' }));
			if (url.startsWith('/api/v1/admin/policy/nodes')) {
				gets += 1;
				return Promise.resolve(gets <= 2
					? jsonResponse(200, { nodes: before, refreshedAt: '2026-09-27T10:00:00Z' })
					: jsonResponse(200, { nodes: after, refreshedAt: '2026-09-27T10:05:01Z' }));
			}
			return Promise.resolve(jsonResponse(200, []));
		}));
		const store = new AdminPolicyNodesStore();
		await store.load();
		const done = store.refresh();
		await vi.runAllTimersAsync();
		await done;
		vi.useRealTimers();
		expect(store.refreshedAt).toBe('2026-09-27T10:05:01Z');
		expect(store.nodes).toEqual(after);
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

describe('node capacity export file', () => {
	it('exports only the nodes carrying a cap', () => {
		const nodes = [
			fixture({ node: 'pve-node-01', maxVms: 4, maxVcpus: 8, maxRamGb: 16, maxDiskGb: 100 }),
			fixture({ node: 'pve-node-02' })
		];
		const file = JSON.parse(buildNodeCapacityExport('pve', nodes)) as { kind: string; cluster: string; nodes: unknown[] };

		expect(file.kind).toBe(NODE_CAPACITY_EXPORT_KIND);
		expect(file.cluster).toBe('pve');
		expect(file.nodes).toEqual([{ node: 'pve-node-01', maxVms: 4, maxVcpus: 8, maxRamGb: 16, maxDiskGb: 100 }]);
	});

	it('round-trips through parse as a single unnamed section', () => {
		const capped = fixture({ node: 'pve-node-01', maxVms: 4, maxVcpus: 8, maxRamGb: 16, maxDiskGb: 100 });
		expect(parseNodeCapacityExport(buildNodeCapacityExport('pve', [capped]))).toEqual([
			{ cluster: null, nodes: [{ node: 'pve-node-01', maxVms: 4, maxVcpus: 8, maxRamGb: 16, maxDiskGb: 100 }] }
		]);
	});

	it('round-trips a multi-cluster bundle with per-cluster sections', () => {
		const sections = [
			{ cluster: 'pve-a', nodes: [{ node: 'pve-node-01', maxVms: 4, maxVcpus: 8, maxRamGb: 16, maxDiskGb: 100 }] },
			{ cluster: 'pve-b', nodes: [{ node: 'pve-node-02', maxVms: 2, maxVcpus: 4, maxRamGb: 8, maxDiskGb: 50 }] }
		];
		expect(parseNodeCapacityExport(buildAllClustersNodeCapacityExport(sections))).toEqual(sections);
	});

	it('rejects a file with a missing cap', () => {
		const file = JSON.parse(buildNodeCapacityExport('pve', [fixture({ node: 'pve-node-01', maxVms: 1 })])) as { nodes: Record<string, number>[] };
		delete file.nodes[0]?.maxDiskGb;
		expect(() => parseNodeCapacityExport(JSON.stringify(file))).toThrow();
	});
});

describe('AdminPolicyNodesStore.importNodes', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	it('posts the caps to the current cluster with the overwrite flag', async () => {
		const outcomes = [{ node: 'pve-node-01', result: 'updated' }];
		const nodes = [fixture({ node: 'pve-node-01', maxVms: 4 })];
		const fetchMock = vi.fn().mockImplementation((url: string) => {
			if (url === '/api/v1/admin/policy/nodes/import') return Promise.resolve(jsonResponse(200, outcomes));
			if (url === '/api/v1/auth/clusters') return Promise.resolve(jsonResponse(200, [{ name: 'pve', displayName: 'pve' }]));
			if (url.startsWith('/api/v1/admin/policy/nodes?')) return Promise.resolve(jsonResponse(200, { nodes, refreshedAt: '' }));
			return Promise.resolve(jsonResponse(404, { code: 'not_found', message: `unexpected ${url}` }));
		});
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminPolicyNodesStore();
		store.cluster = 'pve';
		const items = [{ node: 'pve-node-01', maxVms: 4, maxVcpus: 8, maxRamGb: 16, maxDiskGb: 100 }];

		const result = await store.importNodes([{ cluster: null, nodes: items }], true);

		expect(result).toEqual(outcomes.map((outcome) => ({ ...outcome, cluster: 'pve' })));
		expect(store.nodes).toEqual(nodes);
		const call = fetchMock.mock.calls.find(([url]) => url === '/api/v1/admin/policy/nodes/import');
		const init = call?.[1] as RequestInit | undefined;
		expect(JSON.parse(init?.body as string)).toEqual({ cluster: 'pve', overwrite: true, nodes: items });
	});

	it('posts each named section to its own cluster', async () => {
		const calls: Array<{ cluster: string; nodes: unknown }> = [];
		const nodes = [fixture({ node: 'pve-node-01', maxVms: 4 })];
		const fetchMock = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
			if (url === '/api/v1/admin/policy/nodes/import') {
				const body = JSON.parse(init?.body as string) as { cluster: string; nodes: unknown };
				calls.push(body);
				return Promise.resolve(jsonResponse(200, [{ node: 'n', result: 'updated' }]));
			}
			if (url === '/api/v1/auth/clusters') return Promise.resolve(jsonResponse(200, [{ name: 'pve', displayName: 'pve' }]));
			if (url.startsWith('/api/v1/admin/policy/nodes?')) return Promise.resolve(jsonResponse(200, { nodes, refreshedAt: '' }));
			return Promise.resolve(jsonResponse(404, { code: 'not_found', message: `unexpected ${url}` }));
		});
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminPolicyNodesStore();
		store.cluster = 'pve';
		const caps = { maxVms: 1, maxVcpus: 1, maxRamGb: 1, maxDiskGb: 1 };

		const result = await store.importNodes(
			[
				{ cluster: 'pve-a', nodes: [{ node: 'n-a', ...caps }] },
				{ cluster: 'pve-b', nodes: [{ node: 'n-b', ...caps }] }
			],
			false
		);

		expect(calls.map((call) => call.cluster)).toEqual(['pve-a', 'pve-b']);
		expect(result?.map((outcome) => outcome.cluster)).toEqual(['pve-a', 'pve-b']);
	});
});
