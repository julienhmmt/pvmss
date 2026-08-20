import { afterEach, describe, expect, it, vi } from 'vitest';
import { AdminCatalogStore, type AdminBridge, type AdminISO } from './admin-catalog.svelte';

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
});
