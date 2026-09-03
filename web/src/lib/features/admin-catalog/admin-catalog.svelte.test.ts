import { afterEach, describe, expect, it, vi } from 'vitest';
import { AdminCatalogStore, type AdminBridge, type AdminISO, type AdminNode, type AdminStorage, type AdminTemplate } from './admin-catalog.svelte';

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

const templates: AdminTemplate[] = [
	{ vmid: 9000, node: 'pve-node-02', name: 'debian-12-cloud', cloudInitCapable: true, diskStorage: 'local-lvm', diskSizeGB: 8, diskBus: 'scsi', enabled: true, missing: false, diskUnreadable: false },
	{ vmid: 9001, node: 'pve-node-02', name: 'alpine-appliance', cloudInitCapable: false, diskStorage: 'local', diskSizeGB: 2, diskBus: 'scsi', enabled: false, missing: false, diskUnreadable: false },
	{ vmid: 9002, node: 'pve-node-01', name: 'rocky-9-base', cloudInitCapable: false, diskStorage: 'local-lvm', diskSizeGB: 32, diskBus: 'virtio', enabled: false, missing: false, diskUnreadable: false }
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

		it('resetISOFilters clears all filters and sort', () => {
			const store = new AdminCatalogStore();
			store.isos = isos;
			store.isoSearch = 'deb';
			store.isoStorageFilter = 'local';
			store.isoEnabledFilter = 'enabled';
			store.isoSortBy = 'size';
			store.isoSortDir = 'desc';
			store.resetISOFilters();
			expect(store.filteredIsos.length).toBe(3);
			expect(store.isoSortBy).toBe('file');
			expect(store.isoSortDir).toBe('asc');
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

	describe('filteredStorages', () => {
		const testStorages: AdminStorage[] = [
			{ name: 'nfs-share', node: 'node-b', type: 'nfs', totalBytes: 2000, usedBytes: 500, enabled: false },
			{ name: 'local-lvm', node: 'node-a', type: 'lvm', totalBytes: 1000, usedBytes: 100, enabled: true },
			{ name: 'local-lvm', node: 'node-b', type: 'lvm', totalBytes: 3000, usedBytes: 200, enabled: false }
		];

		it('returns all storages when no filters are set', () => {
			const store = new AdminCatalogStore();
			store.storages = testStorages;
			expect(store.filteredStorages.length).toBe(3);
		});

		it('filters by search term on name, node, or type', () => {
			const store = new AdminCatalogStore();
			store.storages = testStorages;
			store.storageSearch = 'node-a';
			expect(store.filteredStorages.length).toBe(1);
			expect(store.filteredStorages[0]?.node).toBe('node-a');

			store.storageSearch = 'lvm';
			expect(store.filteredStorages.length).toBe(2);
		});

		it('filters by node', () => {
			const store = new AdminCatalogStore();
			store.storages = testStorages;
			store.storageNodeFilter = 'node-b';
			expect(store.filteredStorages.length).toBe(2);
			expect(store.filteredStorages.every((s) => s.node === 'node-b')).toBe(true);
		});

		it('filters by type', () => {
			const store = new AdminCatalogStore();
			store.storages = testStorages;
			store.storageTypeFilter = 'nfs';
			expect(store.filteredStorages.length).toBe(1);
			expect(store.filteredStorages[0]?.type).toBe('nfs');
		});

		it('filters by enabled state', () => {
			const store = new AdminCatalogStore();
			store.storages = testStorages;
			store.storageEnabledFilter = 'enabled';
			expect(store.filteredStorages.length).toBe(1);
			expect(store.filteredStorages[0]?.enabled).toBe(true);
		});

		it('sorts by name ascending then descending', () => {
			const store = new AdminCatalogStore();
			store.storages = testStorages;
			store.storageSortBy = 'name';
			store.storageSortDir = 'asc';
			expect(store.filteredStorages.map((s) => s.name)).toEqual(['local-lvm', 'local-lvm', 'nfs-share']);
			store.storageSortDir = 'desc';
			expect(store.filteredStorages.map((s) => s.name)).toEqual(['nfs-share', 'local-lvm', 'local-lvm']);
		});

		it('sorts by node with name as tiebreaker', () => {
			const store = new AdminCatalogStore();
			store.storages = testStorages;
			store.storageSortBy = 'node';
			store.storageSortDir = 'asc';
			expect(store.filteredStorages.map((s) => `${s.node}/${s.name}`)).toEqual([
				'node-a/local-lvm',
				'node-b/local-lvm',
				'node-b/nfs-share'
			]);
		});

		it('sorts by usage percentage', () => {
			const store = new AdminCatalogStore();
			store.storages = testStorages;
			store.storageSortBy = 'usage';
			store.storageSortDir = 'asc';
			// local-lvm@node-b: 6.7%, local-lvm@node-a: 10%, nfs-share: 25%
			expect(store.filteredStorages.map((s) => `${s.name}@${s.node}`)).toEqual([
				'local-lvm@node-b',
				'local-lvm@node-a',
				'nfs-share@node-b'
			]);
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

		it('resetStorageFilters clears all filters and sort', () => {
			const store = new AdminCatalogStore();
			store.storages = testStorages;
			store.storageSearch = 'local';
			store.storageNodeFilter = 'node-a';
			store.storageTypeFilter = 'lvm';
			store.storageEnabledFilter = 'disabled';
			store.storageSortBy = 'usage';
			store.storageSortDir = 'desc';
			store.resetStorageFilters();
			expect(store.filteredStorages.length).toBe(3);
			expect(store.storageSearch).toBe('');
			expect(store.storageNodeFilter).toBe('');
			expect(store.storageTypeFilter).toBe('');
			expect(store.storageEnabledFilter).toBe('all');
			expect(store.storageSortBy).toBe('name');
			expect(store.storageSortDir).toBe('asc');
		});
	});

	describe('filteredBridges', () => {
		const testBridges: AdminBridge[] = [
			{ name: 'vmbr1', node: 'node-b', active: true, comment: 'storage', enabled: false },
			{ name: 'vmbr0', node: 'node-a', active: true, comment: '', enabled: true },
			{ name: 'vmbr0', node: 'node-b', active: false, comment: 'wan', enabled: false }
		];

		it('returns all bridges when no filters are set', () => {
			const store = new AdminCatalogStore();
			store.bridges = testBridges;
			expect(store.filteredBridges.length).toBe(3);
		});

		it('filters by search term on name, node, or comment', () => {
			const store = new AdminCatalogStore();
			store.bridges = testBridges;
			store.bridgeSearch = 'vmbr0';
			expect(store.filteredBridges.length).toBe(2);

			store.bridgeSearch = 'wan';
			expect(store.filteredBridges.length).toBe(1);
			expect(store.filteredBridges[0]?.comment).toBe('wan');
		});

		it('filters by node', () => {
			const store = new AdminCatalogStore();
			store.bridges = testBridges;
			store.bridgeNodeFilter = 'node-a';
			expect(store.filteredBridges.length).toBe(1);
			expect(store.filteredBridges[0]?.node).toBe('node-a');
		});

		it('filters by active state', () => {
			const store = new AdminCatalogStore();
			store.bridges = testBridges;
			store.bridgeActiveFilter = 'active';
			expect(store.filteredBridges.length).toBe(2);
			expect(store.filteredBridges.every((b) => b.active)).toBe(true);
		});

		it('filters by enabled state', () => {
			const store = new AdminCatalogStore();
			store.bridges = testBridges;
			store.bridgeEnabledFilter = 'enabled';
			expect(store.filteredBridges.length).toBe(1);
			expect(store.filteredBridges[0]?.enabled).toBe(true);
		});

		it('sorts by name ascending then descending', () => {
			const store = new AdminCatalogStore();
			store.bridges = testBridges;
			store.bridgeSortBy = 'name';
			store.bridgeSortDir = 'asc';
			expect(store.filteredBridges.map((b) => `${b.name}@${b.node}`)).toEqual([
				'vmbr0@node-a',
				'vmbr0@node-b',
				'vmbr1@node-b'
			]);
		});

		it('sorts by node with name as tiebreaker', () => {
			const store = new AdminCatalogStore();
			store.bridges = testBridges;
			store.bridgeSortBy = 'node';
			store.bridgeSortDir = 'asc';
			expect(store.filteredBridges.map((b) => `${b.node}/${b.name}`)).toEqual([
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
			expect(store.filteredBridges.map((b) => `${b.name}@${b.node}`)).toEqual([
				'vmbr0@node-a',
				'vmbr1@node-b',
				'vmbr0@node-b'
			]);
		});

		it('sorts by active state', () => {
			const store = new AdminCatalogStore();
			store.bridges = testBridges;
			store.bridgeSortBy = 'active';
			store.bridgeSortDir = 'asc';
			expect(store.filteredBridges.map((b) => `${b.name}@${b.node}`)).toEqual([
				'vmbr0@node-b',
				'vmbr0@node-a',
				'vmbr1@node-b'
			]);
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

		it('resetBridgeFilters clears all filters and sort', () => {
			const store = new AdminCatalogStore();
			store.bridges = testBridges;
			store.bridgeSearch = 'vmbr';
			store.bridgeNodeFilter = 'node-a';
			store.bridgeActiveFilter = 'active';
			store.bridgeEnabledFilter = 'enabled';
			store.bridgeSortBy = 'comment';
			store.bridgeSortDir = 'desc';
			store.resetBridgeFilters();
			expect(store.filteredBridges.length).toBe(3);
			expect(store.bridgeSearch).toBe('');
			expect(store.bridgeNodeFilter).toBe('');
			expect(store.bridgeActiveFilter).toBe('all');
			expect(store.bridgeEnabledFilter).toBe('all');
			expect(store.bridgeSortBy).toBe('name');
			expect(store.bridgeSortDir).toBe('asc');
		});
	});

	describe('templates', () => {
		it('loadTemplates fetches and maps the DTO for the selected cluster', async () => {
			const fetchMock = vi.fn().mockImplementation((url: string) => {
				if (url.includes('/auth/clusters')) {
					return Promise.resolve(jsonResponse(200, [{ name: 'default', displayName: 'Default', oidcEnabled: false }]));
				}
				return Promise.resolve(jsonResponse(200, templates));
			});
			vi.stubGlobal('fetch', fetchMock);
			const store = new AdminCatalogStore();

			await store.loadTemplates();

			expect(store.templates).toEqual(templates);
			expect(store.error).toBeNull();
			const calls = fetchMock.mock.calls as [string][];
			expect(calls.at(-1)?.[0]).toBe('/api/v1/admin/templates?cluster=default');
		});

		it('loadTemplates keeps other catalog lists untouched (separate from loadAll)', async () => {
			vi.stubGlobal(
				'fetch',
				vi.fn().mockImplementation((url: string) => {
					if (url.includes('/auth/clusters')) {
						return Promise.resolve(jsonResponse(200, [{ name: 'default', displayName: 'Default', oidcEnabled: false }]));
					}
					return Promise.resolve(jsonResponse(200, templates));
				})
			);
			const store = new AdminCatalogStore();

			await store.loadTemplates();

			expect(store.nodes).toEqual([]);
			expect(store.storages).toEqual([]);
			expect(store.bridges).toEqual([]);
			expect(store.isos).toEqual([]);
		});

		it('toggleTemplate posts the right body and optimistically flips the row', async () => {
			let resolvePost: (value: Response) => void = () => undefined;
			const fetchMock = vi.fn().mockImplementation(
				() =>
					new Promise<Response>((resolve) => {
						resolvePost = resolve;
					})
			);
			vi.stubGlobal('fetch', fetchMock);
			const store = new AdminCatalogStore();
			store.templates = templates;

			const pending = store.toggleTemplate(9001, true);

			// Optimistic: the row flips before the response arrives.
			expect(store.templates[1]?.enabled).toBe(true);

			resolvePost(jsonResponse(200, { vmid: 9001, enabled: true }));
			await pending;

			expect(store.templates[1]?.enabled).toBe(true);
			expect(JSON.parse(fetchMock.mock.calls[0]?.[1]?.body as string)).toEqual({
				cluster: 'default',
				vmid: 9001,
				enabled: true
			});
		});

		it('toggleTemplate failure rolls the optimistic flip back and sets toggleError', async () => {
			vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(500, { message: 'boom' })));
			const store = new AdminCatalogStore();
			store.templates = templates;

			await expect(store.toggleTemplate(9000, false)).rejects.toBeTruthy();

			expect(store.templates[0]?.enabled).toBe(true);
			expect(store.toggleError).not.toBeNull();
		});

		it('filteredTemplates filters by search, node, and storage', () => {
			const store = new AdminCatalogStore();
			store.templates = templates;
			expect(store.filteredTemplates.length).toBe(3);

			store.templateSearch = 'debian';
			expect(store.filteredTemplates.map((t) => t.vmid)).toEqual([9000]);

			store.resetTemplateFilters();
			store.templateNodeFilter = 'pve-node-01';
			expect(store.filteredTemplates.map((t) => t.vmid)).toEqual([9002]);

			store.resetTemplateFilters();
			store.templateStorageFilter = 'local';
			expect(store.filteredTemplates.map((t) => t.vmid)).toEqual([9001]);
		});

		it('filteredTemplates sorts by vmid, name, and node', () => {
			const store = new AdminCatalogStore();
			store.templates = [...templates].reverse();

			store.templateSortBy = 'vmid';
			expect(store.filteredTemplates.map((t) => t.vmid)).toEqual([9000, 9001, 9002]);

			store.templateSortBy = 'name';
			expect(store.filteredTemplates.map((t) => t.name)).toEqual(['alpine-appliance', 'debian-12-cloud', 'rocky-9-base']);

			store.templateSortBy = 'node';
			expect(store.filteredTemplates.map((t) => t.node)).toEqual(['pve-node-01', 'pve-node-02', 'pve-node-02']);

			store.templateSortDir = 'desc';
			expect(store.filteredTemplates.map((t) => t.node)).toEqual(['pve-node-02', 'pve-node-02', 'pve-node-01']);
		});

		it('templateStorageOptions and templateNodeOptions derive from the rows', () => {
			const store = new AdminCatalogStore();
			store.templates = templates;
			expect(store.templateStorageOptions).toEqual(['local', 'local-lvm']);
			expect(store.templateNodeOptions).toEqual(['pve-node-01', 'pve-node-02']);
		});

		it('removeTemplate deletes the approval row and drops it from the list', async () => {
			const fetchMock = vi.fn().mockImplementation((url: string) => {
				if (url.includes('/auth/clusters')) {
					return Promise.resolve(jsonResponse(200, [{ name: 'default', displayName: 'Default', oidcEnabled: false }]));
				}
				return Promise.resolve(new Response(null, { status: 204 }));
			});
			vi.stubGlobal('fetch', fetchMock);
			const store = new AdminCatalogStore();
			store.templates = templates;

			await store.removeTemplate(9002);

			expect(store.templates.map((t) => t.vmid)).toEqual([9000, 9001]);
			const calls = fetchMock.mock.calls as [string, RequestInit?][];
			const [url, init] = calls.at(-1)!;
			expect(url).toBe('/api/v1/admin/templates/default/9002');
			expect(init?.method).toBe('DELETE');
		});

		it('removeTemplate failure keeps the row and sets toggleError', async () => {
			vi.stubGlobal(
				'fetch',
				vi.fn().mockImplementation((url: string) => {
					if (url.includes('/auth/clusters')) {
						return Promise.resolve(jsonResponse(200, [{ name: 'default', displayName: 'Default', oidcEnabled: false }]));
					}
					return Promise.resolve(jsonResponse(500, { message: 'boom' }));
				})
			);
			const store = new AdminCatalogStore();
			store.templates = templates;

			await expect(store.removeTemplate(9002)).rejects.toBeTruthy();

			expect(store.templates.length).toBe(3);
			expect(store.toggleError).not.toBeNull();
		});

		it('resetTemplateFilters clears all filters and sort', () => {
			const store = new AdminCatalogStore();
			store.templates = templates;
			store.templateSearch = 'deb';
			store.templateStorageFilter = 'local';
			store.templateNodeFilter = 'pve-node-01';
			store.templateSortBy = 'name';
			store.templateSortDir = 'desc';

			store.resetTemplateFilters();

			expect(store.filteredTemplates.length).toBe(3);
			expect(store.templateSearch).toBe('');
			expect(store.templateStorageFilter).toBe('');
			expect(store.templateNodeFilter).toBe('');
			expect(store.templateSortBy).toBe('vmid');
			expect(store.templateSortDir).toBe('asc');
		});
	});
});
