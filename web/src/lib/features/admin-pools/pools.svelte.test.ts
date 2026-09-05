import { afterEach, describe, expect, it, vi } from 'vitest';
import { PoolsStore } from './pools.svelte';
import { m } from '$lib/paraglide/messages.js';

const SEARCH_DEBOUNCE_MS = 300;

const clusterOptions = [{ name: 'musclegrid', displayName: 'musclegrid', oidcEnabled: false }];

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

function stubFetch(...responses: Response[]): ReturnType<typeof vi.fn> {
	const fetchMock = vi.fn();
	for (const response of responses) {
		fetchMock.mockResolvedValueOnce(response);
	}
	vi.stubGlobal('fetch', fetchMock);
	return fetchMock;
}

describe('PoolsStore', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('loads pools and applies case-insensitive server search', async () => {
		const pools = [
			{ name: 'alice', comment: '', total: 7, running: 3, stopped: 4, managed: false },
			{ name: 'carol', comment: '', total: 0, running: 0, stopped: 0, managed: true }
		];
		const fetchMock = stubFetch(
			jsonResponse(200, clusterOptions),
			jsonResponse(200, pools),
			jsonResponse(200, [pools[1]])
		);
		const store = new PoolsStore();

		await store.load();
		store.applySearch('CAR');
		await vi.waitFor(() => {
			expect(store.pools).toEqual([pools[1]]);
		}, { timeout: SEARCH_DEBOUNCE_MS + 100 });
		expect(fetchMock).toHaveBeenLastCalledWith(
			'/api/v1/admin/pools?cluster=musclegrid&search=CAR',
			expect.objectContaining({ headers: { Accept: 'application/json' } })
		);
	});

	it('creates and deletes through the API while exposing status', async () => {
		stubFetch(
			jsonResponse(201, { name: 'pvmss-carol', username: 'pvmss-carol', password: 'gen-pw-123', comment: '', managed: true }),
			jsonResponse(200, { status: 'deleted', userDeleted: true })
		);
		const store = new PoolsStore();
		store.cluster = 'musclegrid';

		await store.create('carol', '');
		expect(store.pools).toHaveLength(1);
		expect(store.pools[0]?.managed).toBe(true);
		expect(store.pools[0]?.name).toBe('pvmss-carol');
		expect(store.createdCredentials).not.toBeNull();
		expect(store.createdCredentials?.password).toBe('gen-pw-123');
		await store.remove('pvmss-carol');
		expect(store.pools).toHaveLength(0);
		expect(store.announce).toBe(m['admin.pools.deleted']({ name: 'pvmss-carol' }));
	});

	it('dismissCredentials clears the createdCredentials state', async () => {
		stubFetch(
			jsonResponse(201, { name: 'pvmss-carol', username: 'pvmss-carol', password: 'gen-pw', comment: '', managed: true })
		);
		const store = new PoolsStore();
		store.cluster = 'musclegrid';
		await store.create('carol', '');
		expect(store.createdCredentials).not.toBeNull();
		store.dismissCredentials();
		expect(store.createdCredentials).toBeNull();
	});
});
