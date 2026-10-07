import { describe, it, expect, vi, afterEach } from 'vitest';
import { VmListStore, INCOMPLETE_RETRY_MS, type VmListResult } from './list.svelte';
import { markVmDeleted } from './recently-deleted';

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
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult)));
		const { store } = makeStore('', 'all');
		store.setSort('cpu');
		expect(store.queryString()).toBe('sortBy=cpu&scope=all');
	});

	it('parses and serializes the optional cluster filter', () => {
		const { store } = makeStore('?cluster=secondary');
		expect(store.cluster).toBe('secondary');
		expect(store.queryString()).toBe('cluster=secondary');
	});

	it('re-reads a just-created row until Proxmox reports it (ghost row)', async () => {
		vi.useFakeTimers();
		const ghost: VmListResult = {
			...oneVmResult,
			items: [{ ...oneVmResult.items[0]!, name: '', cpuCores: 0, memoryTotal: 0, status: 'stopped' }]
		};
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, ghost))
			// The forced refresh is throttled: 429 is not proof of freshness.
			.mockResolvedValueOnce(jsonResponse(429, { error: { code: 'refresh_too_soon', message: 'later' } }))
			.mockResolvedValueOnce(jsonResponse(200, oneVmResult));
		vi.stubGlobal('fetch', fetchMock);

		const { store } = makeStore('');
		const done = store.loadUntilComplete({ cluster: 'default', vmid: 100, expectedStatus: 'running' });
		await vi.advanceTimersByTimeAsync(INCOMPLETE_RETRY_MS);
		await done;

		expect(store.result?.items[0]).toMatchObject({ name: 'web-01', status: 'running' });
		expect(fetchMock).toHaveBeenCalledWith('/api/v1/cluster/refresh', expect.anything());
	});

	it('coalesces overlapping equivalent loads into one fetch', async () => {
		const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(jsonResponse(200, oneVmResult)));
		vi.stubGlobal('fetch', fetchMock);
		const { store } = makeStore();
		await Promise.all([store.load(), store.load(), store.load()]);
		expect(fetchMock).toHaveBeenCalledTimes(1);
		expect(store.result).toEqual(oneVmResult);
	});

	it('allows a fresh load after a coalesced failure', async () => {
		const fetchMock = vi.fn().mockRejectedValueOnce(new Error('offline'))
			.mockResolvedValueOnce(jsonResponse(200, oneVmResult));
		vi.stubGlobal('fetch', fetchMock);
		const { store } = makeStore();
		await Promise.all([store.load(), store.load()]);
		expect(fetchMock).toHaveBeenCalledTimes(1);
		expect(store.error).not.toBeNull();
		await store.load();
		expect(fetchMock).toHaveBeenCalledTimes(2);
		expect(store.error).toBeNull();
	});

	it('keeps the newest query result when an older load finishes last', async () => {
		let finishOld: (response: Response) => void = () => {};
		const oldResponse: Promise<Response> = new Promise((resolve) => { finishOld = resolve; });
		const filtered: VmListResult = { ...oneVmResult, items: [], total: 0 };
		const fetchMock = vi.fn().mockReturnValueOnce(oldResponse)
			.mockResolvedValueOnce(jsonResponse(200, filtered));
		vi.stubGlobal('fetch', fetchMock);
		const { store } = makeStore();
		const oldLoad: Promise<void> = store.load();
		store.search = 'missing';
		await store.load();
		finishOld(jsonResponse(200, oneVmResult));
		await oldLoad;
		expect(fetchMock).toHaveBeenCalledTimes(2);
		expect(store.result).toEqual(filtered);
	});

	it.each(['running', 'stopped'] as const)('re-reads a complete created row until its status is %s', async (expectedStatus) => {
		vi.useFakeTimers();
		const stale: VmListResult = { ...oneVmResult, items: [{ ...oneVmResult.items[0]!, status: expectedStatus === 'running' ? 'stopped' : 'running' }] };
		const fresh: VmListResult = { ...oneVmResult, items: [{ ...oneVmResult.items[0]!, status: expectedStatus }] };
		const fetchMock = vi.fn().mockResolvedValueOnce(jsonResponse(200, stale))
			.mockResolvedValueOnce(jsonResponse(200, {})).mockResolvedValueOnce(jsonResponse(200, fresh));
		vi.stubGlobal('fetch', fetchMock);
		const { store } = makeStore();
		const done: Promise<void> = store.loadUntilComplete({ cluster: 'default', vmid: 100, expectedStatus });
		await vi.advanceTimersByTimeAsync(INCOMPLETE_RETRY_MS);
		await done;
		expect(fetchMock).toHaveBeenCalledTimes(3);
		expect(store.result?.items[0]?.status).toBe(expectedStatus);
	});

	it('does not retry an unrelated incomplete row after creation', async () => {
		const otherGhost: VmListResult = { ...oneVmResult, items: [...oneVmResult.items, { ...oneVmResult.items[0]!, vmid: 101, name: '', cpuCores: 0 }] };
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, otherGhost));
		vi.stubGlobal('fetch', fetchMock);
		const { store } = makeStore();
		await store.loadUntilComplete({ cluster: 'default', vmid: 100, expectedStatus: 'running' });
		expect(fetchMock).toHaveBeenCalledTimes(1);
	});

	it.each(['vm_snapshot_create', 'vm_snapshot_rollback', 'vm_snapshot_delete', 'vm_migrate'] as const)('only loads once after %s, even with an incomplete row', async (kind) => {
		const ghost: VmListResult = { ...oneVmResult, items: [{ ...oneVmResult.items[0]!, name: '', cpuCores: 0 }] };
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, ghost));
		vi.stubGlobal('fetch', fetchMock);
		const { store } = makeStore();
		await store.loadAfterTask({ kind, cluster: 'default', vmid: 100, name: 'web', upid: 'task', deadline: 0 });
		expect(fetchMock).toHaveBeenCalledTimes(1);
	});

	it('bounds retries when the created VM is missing from the projection', async () => {
		vi.useFakeTimers();
		const empty: VmListResult = { ...oneVmResult, items: [], total: 0 };
		const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(jsonResponse(200, empty)));
		vi.stubGlobal('fetch', fetchMock);
		const { store } = makeStore();
		const done: Promise<void> = store.loadUntilComplete({ cluster: 'default', vmid: 100, expectedStatus: 'running' });
		await vi.runAllTimersAsync();
		await done;
		const calls: number = fetchMock.mock.calls.length;
		expect(calls).toBeGreaterThan(1);
		await vi.advanceTimersByTimeAsync(INCOMPLETE_RETRY_MS);
		expect(fetchMock).toHaveBeenCalledTimes(calls);
	});

	it('stops creation retries after disposal', async () => {
		vi.useFakeTimers();
		const ghost: VmListResult = { ...oneVmResult, items: [{ ...oneVmResult.items[0]!, name: '' }] };
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, ghost));
		vi.stubGlobal('fetch', fetchMock);
		const { store } = makeStore();
		const done: Promise<void> = store.loadUntilComplete({ cluster: 'default', vmid: 100, expectedStatus: 'running' });
		await vi.advanceTimersByTimeAsync(0);
		store.dispose();
		await vi.advanceTimersByTimeAsync(INCOMPLETE_RETRY_MS);
		await done;
		expect(fetchMock).toHaveBeenCalledTimes(1);
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

	it('sets error and errorCode on failure', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(jsonResponse(503, { code: 'inventory_not_ready', message: 'inventory has not been populated yet' }))
		);

		const { store } = makeStore('');
		await store.load();

		expect(store.error).toBe('inventory has not been populated yet');
		expect(store.errorCode).toBe('inventory_not_ready');
		expect(store.result).toBeNull();
	});

	it('clears errorCode on a successful reload after a failure', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(503, { code: 'inventory_not_ready', message: 'inventory has not been populated yet' }))
			.mockResolvedValueOnce(jsonResponse(200, oneVmResult));
		vi.stubGlobal('fetch', fetchMock);

		const { store } = makeStore('');
		await store.load();
		expect(store.errorCode).toBe('inventory_not_ready');

		await store.load();
		expect(store.errorCode).toBeNull();
		expect(store.error).toBeNull();
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

	it('dispose cancels a pending search so it cannot navigate after unmount', async () => {
		vi.useFakeTimers();
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult)));
		const { store, navigated } = makeStore('');

		store.applySearch('web');
		store.dispose();
		await vi.advanceTimersByTimeAsync(1000);
		expect(navigated).toHaveLength(0);
	});

	it('clearFilters drops search, status and node and returns to page 1', () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult)));
		const { store, navigated } = makeStore('?search=web&status=stopped&node=n1&page=2');

		store.clearFilters();
		expect(store.search).toBe('');
		expect(store.status).toBe('');
		expect(store.node).toBe('');
		expect(store.page).toBe(1);
		expect(navigated.at(-1)).toBe('');
	});

	it('toggling the active sort column reverses direction', () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult)));
		const { store, navigated } = makeStore('');

		store.setSort('name');
		expect(store.sortDir).toBe('desc');
		expect(navigated.at(-1)).toBe('sortDir=desc');

		store.setSort('cpu');
		expect(store.sortBy).toBe('cpu');
		expect(store.sortDir).toBe('asc');
	});

	it('changing a filter resets to page one', () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult)));
		const { store, navigated } = makeStore('?page=4');

		store.setStatus('running');
		expect(store.page).toBe(1);
		expect(navigated.at(-1)).toBe('status=running');
	});

	it('page navigation syncs the URL', () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult)));
		const { store, navigated } = makeStore('');

		store.setPage(2);
		expect(navigated.at(-1)).toBe('page=2');
	});

	describe('refreshIfStale', () => {
		it('records lastLoadedAt when load() completes', async () => {
			vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult)));
			const { store } = makeStore('');

			expect(store.lastLoadedAt).toBe(0);
			await store.load();
			expect(store.lastLoadedAt).toBeGreaterThan(0);
		});

		it('reloads when the last load is older than the stale window', async () => {
			vi.useFakeTimers();
			const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult));
			vi.stubGlobal('fetch', fetchMock);
			const { store } = makeStore('');

			await store.load();
			vi.setSystemTime(Date.now() + 31_000);
			await store.refreshIfStale();

			expect(fetchMock).toHaveBeenCalledTimes(2);
		});

		it('does not reload while the data is still fresh', async () => {
			vi.useFakeTimers();
			const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult));
			vi.stubGlobal('fetch', fetchMock);
			const { store } = makeStore('');

			await store.load();
			vi.setSystemTime(Date.now() + 29_000);
			await store.refreshIfStale();

			expect(fetchMock).toHaveBeenCalledTimes(1);
		});

		it('does not reload while a load is in flight', async () => {
			const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult));
			vi.stubGlobal('fetch', fetchMock);
			const { store } = makeStore('');

			store.loading = true;
			await store.refreshIfStale(0);

			expect(fetchMock).not.toHaveBeenCalled();
		});

		it('does not reload while a search debounce is pending', async () => {
			vi.useFakeTimers();
			const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult));
			vi.stubGlobal('fetch', fetchMock);
			const { store } = makeStore('');

			store.applySearch('web');
			await store.refreshIfStale(0);
			expect(fetchMock).not.toHaveBeenCalled();

			// The debounced load still fires on its own schedule.
			await vi.advanceTimersByTimeAsync(300);
			expect(fetchMock).toHaveBeenCalledTimes(1);
		});

		it('does not reload while a row action is converging', async () => {
			const pending: { release?: (response: Response) => void } = {};
			const fetchMock = vi.fn().mockImplementation((url: string) => {
				if (url.endsWith('/actions')) {
					return new Promise<Response>((resolve) => {
						pending.release = resolve;
					});
				}
				if (url === '/api/v1/vms/status') {
					return Promise.resolve(
						jsonResponse(200, [{ cluster: 'default', vmid: 100, status: 'running', uptime: 0 }])
					);
				}
				return Promise.resolve(jsonResponse(200, oneVmResult));
			});
			vi.stubGlobal('fetch', fetchMock);

			const { store } = makeStore('');
			store.result = oneVmResult;

			const actionPromise = store.rowAction('default', 100, 'start');
			await store.refreshIfStale(0);

			// Only the action POST was issued - no list GET.
			expect(fetchMock).toHaveBeenCalledTimes(1);
			expect(fetchMock).not.toHaveBeenCalledWith('/api/v1/vms', expect.anything());

			pending.release?.(jsonResponse(200, { status: 'ok' }));
			await actionPromise;
		});
	});
	describe('recently deleted suppression', () => {
		afterEach(() => sessionStorage.clear());

		it('reports the true empty state when the stale cache only holds a just-deleted VM', async () => {
			markVmDeleted('default', 100);
			vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult)));
			const { store } = makeStore();

			await store.load();

			expect(store.result?.items).toEqual([]);
			expect(store.result?.total).toBe(0);
			expect(store.result?.quota?.used).toBe(0);
			expect(store.result?.emptyReason).toBe('no_vms_owned');
		});

		it('leaves counts untouched when nothing was deleted', async () => {
			vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult)));
			const { store } = makeStore();

			await store.load();

			expect(store.result?.total).toBe(1);
			expect(store.result?.quota?.used).toBe(1);
			expect(store.result?.emptyReason).toBeUndefined();
		});
	});
});

describe('VmListStore attention filter', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('is mutually exclusive with the server status filter', () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult)));
		const { store, navigated } = makeStore('?status=running');

		store.setAttention(true);
		expect(store.attention).toBe(true);
		expect(store.status).toBe('');
		expect(navigated.at(-1)).toContain('attention=1');

		store.setStatus('stopped');
		expect(store.attention).toBe(false);
		expect(navigated.at(-1)).not.toContain('attention=1');
	});

	it('loads every page, so a match on page 2 is not hidden', async () => {
		const [baseVm] = oneVmResult.items;
		if (baseVm === undefined) throw new Error('fixture is missing its VM');
		const pageResult = (page: number, count: number, total: number): VmListResult => ({
			items: Array.from({ length: count }, (_, i) => ({
				...baseVm,
				vmid: page * 100 + i,
				name: `vm-${page}-${i}`
			})),
			total,
			page,
			pageSize: 10,
			availableNodes: []
		});
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, pageResult(1, 10, 25)))
			.mockResolvedValueOnce(jsonResponse(200, pageResult(2, 10, 25)))
			.mockResolvedValueOnce(jsonResponse(200, pageResult(3, 5, 25)));
		vi.stubGlobal('fetch', fetchMock);

		const { store } = makeStore('?attention=1');
		await store.load();

		expect(fetchMock).toHaveBeenCalledTimes(3);
		expect(store.result?.items).toHaveLength(25);
		expect(store.result?.page).toBe(1);
		expect(store.result?.pageSize).toBe(25);
	});

	it('reads the filter back out of the URL', () => {
		const { store } = makeStore('?attention=1');
		expect(store.attention).toBe(true);
	});
});

describe('VmListStore fetchAllMatching', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
		sessionStorage.clear();
	});

	it('returns every page of the filtered set', async () => {
		const [baseVm] = oneVmResult.items;
		if (baseVm === undefined) throw new Error('fixture is missing its VM');
		const pageResult = (page: number, count: number, total: number): VmListResult => ({
			items: Array.from({ length: count }, (_, i) => ({
				...baseVm,
				vmid: page * 100 + i,
				name: `vm-${page}-${i}`
			})),
			total,
			page,
			pageSize: 10,
			availableNodes: []
		});
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, pageResult(1, 10, 15)))
			.mockResolvedValueOnce(jsonResponse(200, pageResult(2, 5, 15)));
		vi.stubGlobal('fetch', fetchMock);

		const { store } = makeStore('?status=running');
		const items = await store.fetchAllMatching();

		expect(fetchMock).toHaveBeenCalledTimes(2);
		expect(String(fetchMock.mock.calls[1]?.[0])).toContain('status=running');
		expect(String(fetchMock.mock.calls[1]?.[0])).toContain('page=2');
		expect(items).toHaveLength(15);
	});

	it('drops rows this tab just deleted, like load() does', async () => {
		markVmDeleted('default', 100);
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, oneVmResult)));
		const { store } = makeStore();

		expect(await store.fetchAllMatching()).toEqual([]);
	});
});
