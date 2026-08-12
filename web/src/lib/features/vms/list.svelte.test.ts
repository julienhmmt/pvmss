import { describe, it, expect, vi, afterEach } from 'vitest';
import { VmListStore, type VmListResult } from './list.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

function makeStore(initialQuery = '', scope: 'mine' | 'all' = 'mine'): {
	store: VmListStore;
	navigated: string[];
} {
	const navigated: string[] = [];
	const store = new VmListStore({ scope, initialQuery, navigate: (qs) => navigated.push(qs) });
	return { store, navigated };
}

const oneVmResult: VmListResult = {
	items: [
		{
			cluster: 'default',
			vmid: 100,
			name: 'web-01',
			node: 'pve-node-01',
			status: 'running',
			pool: 'pool-alice',
			tags: ['pvmss', 'web'],
			cpuCores: 2,
			memoryTotal: 4294967296
		}
	],
	total: 1,
	page: 1,
	pageSize: 10,
	availableNodes: ['pve-node-01'],
	quota: { used: 1, allowed: -1 }
};

describe('VmListStore', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
		vi.useRealTimers();
	});

	it('parses the initial URL query into state (FR-007)', () => {
		const { store } = makeStore('?search=web&status=stopped&node=pve-node-02&sortBy=vmid&sortDir=desc&page=3&pageSize=25');
		expect(store.search).toBe('web');
		expect(store.status).toBe('stopped');
		expect(store.node).toBe('pve-node-02');
		expect(store.sortBy).toBe('vmid');
		expect(store.sortDir).toBe('desc');
		expect(store.page).toBe(3);
		expect(store.pageSize).toBe(25);
	});

	it('defaults state when the URL carries nothing', () => {
		const { store } = makeStore('');
		expect(store.search).toBe('');
		expect(store.sortBy).toBe('name');
		expect(store.sortDir).toBe('asc');
		expect(store.page).toBe(1);
		expect(store.pageSize).toBe(10);
	});

	it('omits default values from the query string', () => {
		const { store } = makeStore('');
		expect(store.queryString()).toBe('');
	});

	it('includes non-default values and the scope in the query string', () => {
		const { store } = makeStore('', 'all');
		store.setSort('cpu');
		expect(store.queryString()).toBe('sortBy=cpu&scope=all');
	});

	it('parses and serializes the optional cluster filter', () => {
		const { store } = makeStore('?cluster=secondary');
		expect(store.cluster).toBe('secondary');
		expect(store.queryString()).toBe('cluster=secondary');
	});

	it('loads the list through the shared API client', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult));
		vi.stubGlobal('fetch', fetchMock);

		const { store } = makeStore('');
		await store.load();

		expect(fetchMock).toHaveBeenCalledWith('/api/v1/vms', expect.anything());
		expect(store.loading).toBe(false);
		expect(store.error).toBeNull();
		expect(store.result?.items).toHaveLength(1);
		expect(store.result?.quota?.allowed).toBe(-1);
	});

	it('sets error on failure', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(jsonResponse(503, { code: 'inventory_not_ready', message: 'inventory has not been populated yet' }))
		);

		const { store } = makeStore('');
		await store.load();

		expect(store.error).toBe('inventory has not been populated yet');
		expect(store.result).toBeNull();
	});

	it('search is debounced, resets the page, syncs the URL, and reloads', async () => {
		vi.useFakeTimers();
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult)));
		const { store, navigated } = makeStore('?page=3');

		store.applySearch('we');
		store.applySearch('web');
		expect(navigated).toHaveLength(0);

		await vi.advanceTimersByTimeAsync(300);
		expect(store.page).toBe(1);
		expect(navigated).toEqual(['search=web']);
	});

	it('toggling the active sort column reverses direction', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult)));
		const { store, navigated } = makeStore('');

		store.setSort('name');
		expect(store.sortDir).toBe('desc');
		expect(navigated.at(-1)).toBe('sortDir=desc');

		store.setSort('cpu');
		expect(store.sortBy).toBe('cpu');
		expect(store.sortDir).toBe('asc');
	});

	it('changing a filter resets to page one', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult)));
		const { store, navigated } = makeStore('?page=4');

		store.setStatus('running');
		expect(store.page).toBe(1);
		expect(navigated.at(-1)).toBe('status=running');
	});

	it('page navigation syncs the URL', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult)));
		const { store, navigated } = makeStore('');

		store.setPage(2);
		expect(navigated.at(-1)).toBe('page=2');
	});
});
