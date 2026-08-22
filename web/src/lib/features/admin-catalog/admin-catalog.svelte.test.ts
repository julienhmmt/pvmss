import { afterEach, describe, expect, it, vi } from 'vitest';
import { AdminCatalogStore, type AdminBridge, type AdminISO, type AdminNode, type AdminStorage } from './admin-catalog.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

const bridges: AdminBridge[] = [
	{ name: 'vmbr0', node: 'node-a', active: true, comment: '', enabled: false },
	{ name: 'vmbr0', node: 'node-b', active: true, comment: '', enabled: false }
];

const isos: AdminISO[] = [
	{ node: 'node-a', storage: 'local', file: 'debian-12.iso', sizeBytes: 1000, enabled: true },
	{ node: 'node-a', storage: 'local', file: 'ubuntu-24.iso', sizeBytes: 2000, enabled: false },
	{ node: 'node-b', storage: 'nfs', file: 'rocky-9.iso', sizeBytes: 3000, enabled: true }
];

const nodes: AdminNode[] = [
	{
		name: 'node-a',
		status: 'online',
		cpuCores: 8,
		cpuUsage: 0.25,
		memoryTotal: 32_000_000_000,
		memoryUsed: 8_000_000_000,
		storageTotal: 1_000_000_000_000,
		storageUsed: 200_000_000_000,
		vmCount: 5,
		enabled: true
	},
	{
		name: 'node-b',
		status: 'offline',
		cpuCores: 16,
		cpuUsage: 0,
		memoryTotal: 64_000_000_000,
		memoryUsed: 0,
		storageTotal: 2_000_000_000_000,
		storageUsed: 0,
		vmCount: 0,
		enabled: false
	},
	{
		name: 'node-c',
		status: 'unknown',
		cpuCores: 4,
		cpuUsage: 0.9,
		memoryTotal: 16_000_000_000,
		memoryUsed: 15_000_000_000,
		storageTotal: 500_000_000_000,
		storageUsed: 100_000_000_000,
		vmCount: 1,
		enabled: false
	}
];

describe('AdminCatalogStore', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('toggles the exact bridge node and name pair', async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			jsonResponse(200, { node: 'node-a', name: 'vmbr0', enabled: true })
		);
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminCatalogStore();
		store.bridges = bridges;

		await store.toggleBridge('node-a', 'vmbr0', true);

		expect(store.bridges).toEqual([
			{ ...bridges[0], enabled: true },
			bridges[1]
		]);
		expect(JSON.parse(fetchMock.mock.calls[0]?.[1]?.body as string)).toEqual({
			cluster: 'default',
			node: 'node-a',
			name: 'vmbr0',
			enabled: true
		});
	});

	describe('filteredIsos', () => {
		it('returns all isos when no filters are set', () => {
			const store = new AdminCatalogStore();
			store.isos = isos;
			expect(store.filteredIsos.length).toBe(3);
		});

		it('filters by search term on file name', () => {
			const store = new AdminCatalogStore();
			store.isos = isos;
			store.isoSearch = 'debian';
			expect(store.filteredIsos.length).toBe(1);
			expect(store.filteredIsos[0]?.file).toBe('debian-12.iso');
		});

		it('filters by storage', () => {
			const store = new AdminCatalogStore();
			store.isos = isos;
			store.isoStorageFilter = 'nfs';
			expect(store.filteredIsos.length).toBe(1);
			expect(store.filteredIsos[0]?.storage).toBe('nfs');
		});

		it('filters by node', () => {
			const store = new AdminCatalogStore();
			store.isos = isos;
			store.isoNodeFilter = 'node-b';
			expect(store.filteredIsos.length).toBe(1);
			expect(store.filteredIsos[0]?.node).toBe('node-b');
		});

		it('filters by enabled state', () => {
			const store = new AdminCatalogStore();
			store.isos = isos;
			store.isoEnabledFilter = 'enabled';
			expect(store.filteredIsos.length).toBe(2);
			expect(store.filteredIsos.every((i) => i.enabled)).toBe(true);
		});

		it('sorts by file ascending then descending', () => {
			const store = new AdminCatalogStore();
			store.isos = isos;
			store.isoSortBy = 'file';
			store.isoSortDir = 'asc';
			expect(store.filteredIsos.map((i) => i.file)).toEqual(['debian-12.iso', 'rocky-9.iso', 'ubuntu-24.iso']);
			store.isoSortDir = 'desc';
			expect(store.filteredIsos.map((i) => i.file)).toEqual(['ubuntu-24.iso', 'rocky-9.iso', 'debian-12.iso']);
		});

		it('sorts by size', () => {
			const store = new AdminCatalogStore();
			store.isos = isos;
			store.isoSortBy = 'size';
			store.isoSortDir = 'asc';
			expect(store.filteredIsos.map((i) => i.sizeBytes)).toEqual([1000, 2000, 3000]);
		});

		it('resetISOFilters clears all filters', () => {
			const store = new AdminCatalogStore();
			store.isos = isos;
			store.isoSearch = 'deb';
			store.isoStorageFilter = 'local';
			store.isoEnabledFilter = 'enabled';
			store.resetISOFilters();
			expect(store.filteredIsos.length).toBe(3);
		});
	});

	describe('filteredNodes', () => {
		it('returns all nodes when no filters are set', () => {
			const store = new AdminCatalogStore();
			store.nodes = nodes;
			expect(store.filteredNodes.length).toBe(3);
		});

		it('filters by search term on node name', () => {
			const store = new AdminCatalogStore();
			store.nodes = nodes;
			store.nodeSearch = 'node-a';
			expect(store.filteredNodes.length).toBe(1);
			expect(store.filteredNodes[0]?.name).toBe('node-a');
		});

		it('filters by status', () => {
			const store = new AdminCatalogStore();
			store.nodes = nodes;
			store.nodeStatusFilter = 'offline';
			expect(store.filteredNodes.length).toBe(1);
			expect(store.filteredNodes[0]?.status).toBe('offline');
		});

		it('filters by enabled state', () => {
			const store = new AdminCatalogStore();
			store.nodes = nodes;
			store.nodeEnabledFilter = 'enabled';
			expect(store.filteredNodes.length).toBe(1);
			expect(store.filteredNodes[0]?.enabled).toBe(true);
		});

		it('sorts by name ascending then descending', () => {
			const store = new AdminCatalogStore();
			store.nodes = nodes;
			store.nodeSortBy = 'name';
			store.nodeSortDir = 'asc';
			expect(store.filteredNodes.map((n) => n.name)).toEqual(['node-a', 'node-b', 'node-c']);
			store.nodeSortDir = 'desc';
			expect(store.filteredNodes.map((n) => n.name)).toEqual(['node-c', 'node-b', 'node-a']);
		});

		it('sorts by status with name as tiebreaker', () => {
			const store = new AdminCatalogStore();
			store.nodes = nodes;
			store.nodeSortBy = 'status';
			store.nodeSortDir = 'asc';
			// Lexicographic: offline < online < unknown
		expect(store.filteredNodes.map((n) => n.name)).toEqual(['node-b', 'node-a', 'node-c']);
		});

		it('sorts by vm count', () => {
			const store = new AdminCatalogStore();
			store.nodes = nodes;
			store.nodeSortBy = 'vmCount';
			store.nodeSortDir = 'asc';
			expect(store.filteredNodes.map((n) => n.vmCount)).toEqual([0, 1, 5]);
		});

		it('sorts by cpu usage', () => {
			const store = new AdminCatalogStore();
			store.nodes = nodes;
			store.nodeSortBy = 'cpuUsage';
			store.nodeSortDir = 'asc';
			expect(store.filteredNodes.map((n) => n.cpuUsage)).toEqual([0, 0.25, 0.9]);
		});

		it('sorts by memory usage', () => {
			const store = new AdminCatalogStore();
			store.nodes = nodes;
			store.nodeSortBy = 'memoryUsage';
			store.nodeSortDir = 'asc';
			// node-b 0%, node-a 25%, node-c 93.75%
			expect(store.filteredNodes.map((n) => n.name)).toEqual(['node-b', 'node-a', 'node-c']);
		});

		it('sorts by enabled state', () => {
			const store = new AdminCatalogStore();
			store.nodes = nodes;
			store.nodeSortBy = 'enabled';
			store.nodeSortDir = 'asc';
			expect(store.filteredNodes.map((n) => Number(n.enabled))).toEqual([0, 0, 1]);
		});

		it('setNodeSort toggles direction on same column and resets on new column', () => {
			const store = new AdminCatalogStore();
			store.nodes = nodes;
			store.setNodeSort('name');
			expect(store.nodeSortDir).toBe('desc');
			store.setNodeSort('status');
			expect(store.nodeSortBy).toBe('status');
			expect(store.nodeSortDir).toBe('asc');
		});

		it('resetNodeFilters clears all filters and sort', () => {
			const store = new AdminCatalogStore();
			store.nodes = nodes;
			store.nodeSearch = 'node';
			store.nodeStatusFilter = 'online';
			store.nodeEnabledFilter = 'enabled';
			store.nodeSortBy = 'vmCount';
			store.nodeSortDir = 'desc';
			store.resetNodeFilters();
			expect(store.filteredNodes.length).toBe(3);
			expect(store.nodeSearch).toBe('');
			expect(store.nodeStatusFilter).toBe('');
			expect(store.nodeEnabledFilter).toBe('all');
			expect(store.nodeSortBy).toBe('name');
			expect(store.nodeSortDir).toBe('asc');
		});
	});

	describe('sortedStorages', () => {
		const storages: AdminStorage[] = [
			{ name: 'nfs-share', node: 'node-b', type: 'nfs', totalBytes: 2000, usedBytes: 500, enabled: false },
			{ name: 'local-lvm', node: 'node-a', type: 'lvm', totalBytes: 1000, usedBytes: 100, enabled: true },
			{ name: 'local-lvm', node: 'node-b', type: 'lvm', totalBytes: 3000, usedBytes: 200, enabled: false }
		];

		it('sorts by name ascending then descending', () => {
			const store = new AdminCatalogStore();
			store.storages = storages;
			store.storageSortBy = 'name';
			store.storageSortDir = 'asc';
			expect(store.sortedStorages.map((s) => s.name)).toEqual(['local-lvm', 'local-lvm', 'nfs-share']);
			store.storageSortDir = 'desc';
			expect(store.sortedStorages.map((s) => s.name)).toEqual(['nfs-share', 'local-lvm', 'local-lvm']);
		});

		it('sorts by node with name as tiebreaker', () => {
			const store = new AdminCatalogStore();
			store.storages = storages;
			store.storageSortBy = 'node';
			store.storageSortDir = 'asc';
			expect(store.sortedStorages.map((s) => `${s.node}/${s.name}`)).toEqual([
				'node-a/local-lvm',
				'node-b/local-lvm',
				'node-b/nfs-share'
			]);
		});

		it('sorts by usage', () => {
			const store = new AdminCatalogStore();
			store.storages = storages;
			store.storageSortBy = 'usage';
			store.storageSortDir = 'asc';
			expect(store.sortedStorages.map((s) => s.usedBytes)).toEqual([100, 200, 500]);
		});

		it('setStorageSort toggles direction on same column and resets on new column', () => {
			const store = new AdminCatalogStore();
			store.storageSortBy = 'name';
			store.storageSortDir = 'asc';
			store.setStorageSort('name');
			expect(store.storageSortDir).toBe('desc');
			store.setStorageSort('node');
			expect(store.storageSortBy).toBe('node');
			expect(store.storageSortDir).toBe('asc');
		});
	});

	describe('sortedBridges', () => {
		const testBridges: AdminBridge[] = [
			{ name: 'vmbr1', node: 'node-b', active: true, comment: 'storage', enabled: false },
			{ name: 'vmbr0', node: 'node-a', active: true, comment: '', enabled: true },
			{ name: 'vmbr0', node: 'node-b', active: true, comment: 'wan', enabled: false }
		];

		it('sorts by name ascending then descending', () => {
			const store = new AdminCatalogStore();
			store.bridges = testBridges;
			store.bridgeSortBy = 'name';
			store.bridgeSortDir = 'asc';
			expect(store.sortedBridges.map((b) => b.name)).toEqual(['vmbr0', 'vmbr0', 'vmbr1']);
			store.bridgeSortDir = 'desc';
			expect(store.sortedBridges.map((b) => b.name)).toEqual(['vmbr1', 'vmbr0', 'vmbr0']);
		});

		it('sorts by node with name as tiebreaker', () => {
			const store = new AdminCatalogStore();
			store.bridges = testBridges;
			store.bridgeSortBy = 'node';
			store.bridgeSortDir = 'asc';
			expect(store.sortedBridges.map((b) => `${b.node}/${b.name}`)).toEqual([
				'node-a/vmbr0',
				'node-b/vmbr0',
				'node-b/vmbr1'
			]);
		});

		it('sorts by comment', () => {
			const store = new AdminCatalogStore();
			store.bridges = testBridges;
			store.bridgeSortBy = 'comment';
			store.bridgeSortDir = 'asc';
			expect(store.sortedBridges.map((b) => b.comment || '')).toEqual(['', 'storage', 'wan']);
		});

		it('setBridgeSort toggles direction on same column and resets on new column', () => {
			const store = new AdminCatalogStore();
			store.bridgeSortBy = 'name';
			store.bridgeSortDir = 'asc';
			store.setBridgeSort('name');
			expect(store.bridgeSortDir).toBe('desc');
			store.setBridgeSort('node');
			expect(store.bridgeSortBy).toBe('node');
			expect(store.bridgeSortDir).toBe('asc');
		});
	});
});
