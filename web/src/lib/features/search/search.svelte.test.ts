import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { SearchStore } from './search.svelte';
import type { VmListResult } from '#lib/features/vms/list.svelte.js';
import { m } from '#lib/paraglide/messages.js';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

const oneVmResult: VmListResult = {
	items: [
		{
			cluster: 'default',
			clusterDisplayName: 'default',
			vmid: 100,
			name: 'web-01',
			node: 'pve-node-01',
			status: 'running',
			pool: 'pool-alice',
			tags: ['pvmss', 'web'],
			ostype: 'l26',
			cpuCores: 2,
			memoryTotal: 4294967296
		}
	],
	total: 1,
	page: 1,
	pageSize: 15,
	availableNodes: ['pve-node-01'],
	quota: { used: 1, allowed: -1 }
};

const emptyResult: VmListResult = {
	items: [],
	total: 0,
	page: 1,
	pageSize: 15,
	availableNodes: [],
	emptyReason: 'no_match'
};

describe('SearchStore', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.unstubAllGlobals();
		vi.useRealTimers();
	});

	it('starts with an empty query and no result', () => {
		const store = new SearchStore();
		expect(store.query).toBe('');
		expect(store.result).toBeNull();
		expect(store.loading).toBe(false);
		expect(store.error).toBeNull();
	});

	it('loads results through the shared VM list endpoint', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult));
		vi.stubGlobal('fetch', fetchMock);

		const store = new SearchStore();
		store.applySearch('web-01');
		await vi.advanceTimersByTimeAsync(300);

		expect(fetchMock).toHaveBeenCalledWith('/api/v1/vms?search=web-01&scope=all&sortBy=name&sortDir=asc&page=1&pageSize=15', expect.anything());
		expect(store.result?.items).toHaveLength(1);
		expect(store.result?.items[0]?.name).toBe('web-01');
		expect(store.error).toBeNull();
	});

	it('shows an empty state when no VMs match', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, emptyResult));
		vi.stubGlobal('fetch', fetchMock);

		const store = new SearchStore();
		store.applySearch('unknown');
		await vi.advanceTimersByTimeAsync(300);

		expect(store.result?.items).toHaveLength(0);
		expect(store.result?.emptyReason).toBe('no_match');
	});

	it('clears the result when the query is empty', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult));
		vi.stubGlobal('fetch', fetchMock);

		const store = new SearchStore();
		store.applySearch('web-01');
		await vi.advanceTimersByTimeAsync(300);
		expect(store.result).not.toBeNull();

		store.applySearch('');
		await vi.advanceTimersByTimeAsync(300);

		expect(store.result).toBeNull();
	});

	it('sets an error when the API fails', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(jsonResponse(503, { code: 'inventory_not_ready', message: 'inventory has not been populated yet' }))
		);

		const store = new SearchStore();
		store.applySearch('web-01');
		await vi.advanceTimersByTimeAsync(300);

		expect(store.error).toBe(m['error.inventory_not_ready']());
		expect(store.result).toBeNull();
	});
});

describe('SearchStore URL query (q)', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
		vi.useRealTimers();
	});

	it('restores an earlier URL query and cancels pending typing without rewriting history', async () => {
		vi.useFakeTimers();
		const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(jsonResponse(200, oneVmResult)));
		vi.stubGlobal('fetch', fetchMock);
		const navigated: string[] = [];
		const store = new SearchStore({ initialQuery: '?q=web', navigate: (query: string) => { navigated.push(query); } });
		store.applySearch('db');
		await vi.advanceTimersByTimeAsync(300);
		store.applySearch('unfinished');
		store.restoreQuery('?q=web');
		await vi.advanceTimersByTimeAsync(300);
		expect(store.query).toBe('web');
		expect(navigated).toEqual(['q=db']);
		expect(fetchMock).toHaveBeenLastCalledWith(expect.stringContaining('search=web'), expect.anything());
		store.restoreQuery('');
		expect(store.query).toBe('');
		expect(store.result).toBeNull();
	});

	it('dispose cancels a pending search navigation', async () => {
		vi.useFakeTimers();
		const navigate = vi.fn();
		const store = new SearchStore({ navigate });
		store.applySearch('web');
		store.dispose();
		await vi.advanceTimersByTimeAsync(300);
		expect(navigate).not.toHaveBeenCalled();
	});

	it('starts from ?q= and writes the debounced query back', async () => {
		vi.useFakeTimers();
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult)));
		const navigated: string[] = [];

		const store = new SearchStore({ initialQuery: '?q=pvmss-test', navigate: (qs) => navigated.push(qs) });
		expect(store.query).toBe('pvmss-test');

		store.applySearch('web');
		await vi.advanceTimersByTimeAsync(400);
		expect(navigated.at(-1)).toBe('q=web');

		store.applySearch('');
		await vi.advanceTimersByTimeAsync(400);
		expect(navigated.at(-1)).toBe('');
	});
});

describe('SearchStore cluster & status filters', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.unstubAllGlobals();
		vi.useRealTimers();
	});

	it('reads cluster and status from the initial URL query', () => {
		const store = new SearchStore({ initialQuery: '?q=web&cluster=prod&status=running' });
		expect(store.query).toBe('web');
		expect(store.cluster).toBe('prod');
		expect(store.status).toBe('running');
	});

	it('ignores an invalid status query param', () => {
		const store = new SearchStore({ initialQuery: '?status=bogus' });
		expect(store.status).toBe('');
	});

	it('passes cluster and status to the API and syncs the URL', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult));
		vi.stubGlobal('fetch', fetchMock);
		const navigated: string[] = [];
		const store = new SearchStore({ initialQuery: '?q=web', navigate: (qs) => navigated.push(qs) });

		store.setCluster('prod');
		await vi.advanceTimersByTimeAsync(0);
		expect(fetchMock).toHaveBeenLastCalledWith(
			expect.stringContaining('search=web'),
			expect.anything()
		);
		expect(fetchMock).toHaveBeenLastCalledWith(
			expect.stringContaining('cluster=prod'),
			expect.anything()
		);
		expect(navigated.at(-1)).toBe('q=web&cluster=prod');

		store.setStatus('running');
		await vi.advanceTimersByTimeAsync(0);
		expect(fetchMock).toHaveBeenLastCalledWith(
			expect.stringContaining('status=running'),
			expect.anything()
		);
		expect(navigated.at(-1)).toBe('q=web&cluster=prod&status=running');
	});

	it('searches on filters alone when the query is empty', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult));
		vi.stubGlobal('fetch', fetchMock);
		const store = new SearchStore({ navigate: () => {} });

		store.setStatus('stopped');
		await vi.advanceTimersByTimeAsync(0);

		expect(fetchMock).toHaveBeenCalledWith(
			expect.stringContaining('status=stopped'),
			expect.anything()
		);
		expect(fetchMock).not.toHaveBeenCalledWith(expect.stringContaining('search='), expect.anything());
		expect(store.result?.items).toHaveLength(1);
	});

	it('restores filters when navigating back', () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult)));
		const store = new SearchStore({ navigate: () => {} });
		store.setCluster('prod');
		store.restoreQuery('?q=db&status=paused');
		expect(store.query).toBe('db');
		expect(store.cluster).toBe('');
		expect(store.status).toBe('paused');
	});

	it('loads cluster options via init', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(jsonResponse(200, [{ name: 'prod', displayName: 'Production' }]))
		);
		const store = new SearchStore();
		await store.init();
		expect(store.clusterOptions).toEqual([{ name: 'prod', displayName: 'Production' }]);
	});

	it('keeps search usable when cluster options fail to load', async () => {
		vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));
		const store = new SearchStore();
		await store.init();
		expect(store.clusterOptions).toEqual([]);
		expect(store.error).toBeNull();
	});

	it('loads the next page and appends it while matches remain', async () => {
		const vm = oneVmResult.items[0]!;
		const page1: VmListResult = { ...oneVmResult, items: [vm], total: 2, page: 1 };
		const page2: VmListResult = { ...oneVmResult, items: [{ ...vm, vmid: 101, name: 'web-02' }], total: 2, page: 2 };
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, page1))
			.mockResolvedValueOnce(jsonResponse(200, page2));
		vi.stubGlobal('fetch', fetchMock);

		const store = new SearchStore({ initialQuery: '?q=web' });
		await store.load();
		expect(store.hasMore).toBe(true);

		await store.loadMore();
		expect(String(fetchMock.mock.calls[1]![0])).toContain('page=2');
		expect(store.result?.items.map((item) => item.name)).toEqual(['web-01', 'web-02']);
		expect(store.hasMore).toBe(false);
	});
});
