import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { SearchStore } from './search.svelte';
import type { VmListResult } from '$lib/features/vms/list.svelte';

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

		expect(fetchMock).toHaveBeenCalledWith('/api/v1/vms?search=web-01&scope=all&sortBy=name&sortDir=asc&pageSize=15', expect.anything());
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

		expect(store.error).toBe('inventory has not been populated yet');
		expect(store.result).toBeNull();
	});
});
