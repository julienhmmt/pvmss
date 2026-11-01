import { afterEach, describe, expect, it, vi } from 'vitest';
import { RecoveryStore } from './recovery.svelte';
import { m } from '#lib/paraglide/messages.js';

const clusterOptions = [{ name: 'musclegrid', displayName: 'musclegrid' }];

const orphanReport = {
	cluster: 'musclegrid',
	pools: [
		{
			name: 'pvmss-old',
			comment: 'previous install',
			members: [
				{ vmid: 900, name: 'web-old-01', node: 'n1', status: 'running', pool: 'pvmss-old', tags: ['pvmss', 'oldtag'] }
			]
		}
	],
	looseVMs: [{ vmid: 902, name: 'solo-old', node: 'n2', status: 'stopped', tags: ['pvmss', 'oldtag'] }],
	tags: [{ name: 'oldtag', vmCount: 2, color: '#3b82f6' }]
};

const emptyReport = { cluster: 'musclegrid', pools: [], looseVMs: [], tags: [] };

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

describe('RecoveryStore', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('loads the report for the resolved cluster and selects everything', async () => {
		const fetchMock = stubFetch(jsonResponse(200, clusterOptions), jsonResponse(200, orphanReport));
		const store = new RecoveryStore();

		await store.load();

		expect(fetchMock).toHaveBeenLastCalledWith(
			'/api/v1/admin/recovery?cluster=musclegrid',
			expect.objectContaining({ headers: { Accept: 'application/json' } })
		);
		expect(store.report?.pools).toHaveLength(1);
		expect(store.selectedPools.has('pvmss-old')).toBe(true);
		expect(store.selectedTags.has('oldtag')).toBe(true);
	});

	it('posts only the still-selected orphans and reloads', async () => {
		const fetchMock = stubFetch(
			jsonResponse(200, clusterOptions),
			jsonResponse(200, orphanReport),
			jsonResponse(200, { pools: ['pvmss-old'], tags: [] }),
			jsonResponse(200, emptyReport)
		);
		const store = new RecoveryStore();
		await store.load();
		store.toggleTag('oldtag');

		await store.adopt();

		expect(fetchMock).toHaveBeenNthCalledWith(
			3,
			'/api/v1/admin/recovery',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({ cluster: 'musclegrid', pools: ['pvmss-old'], tags: [] })
			})
		);
		expect(store.report?.pools).toHaveLength(0);
		expect(store.announce).toBe(m['admin.recovery.adopted']({ pools: 1, tags: 0 }));
	});

	it('forces a fresh scan via the rescan endpoint and reselects orphans', async () => {
		const refreshedReport = {
			...orphanReport,
			pools: [...orphanReport.pools, { name: 'pvmss-late', comment: 'second install', members: [] }]
		};
		const fetchMock = stubFetch(
			jsonResponse(200, clusterOptions),
			jsonResponse(200, orphanReport),
			jsonResponse(200, refreshedReport)
		);
		const store = new RecoveryStore();
		await store.load();
		store.toggleTag('oldtag');

		await store.rescan();

		expect(fetchMock).toHaveBeenNthCalledWith(
			3,
			'/api/v1/admin/recovery/scan',
			expect.objectContaining({ method: 'POST', body: JSON.stringify({ cluster: 'musclegrid' }) })
		);
		expect(store.report?.pools).toHaveLength(2);
		expect(store.selectedPools.has('pvmss-late')).toBe(true);
		expect(store.selectedTags.has('oldtag')).toBe(true);
	});

	it('reports nothing to adopt when the selection is empty', async () => {
		const store = new RecoveryStore();
		await store.adopt();
		expect(store.adoptError).toBe(m['admin.recovery.nothingSelected']());
	});

	it('surfaces the API error on load', async () => {
		stubFetch(jsonResponse(200, clusterOptions), jsonResponse(500, { code: 'internal_error', message: 'boom' }));
		const store = new RecoveryStore();
		await store.load();
		expect(store.error).toBe(m['error.internal_error']());
	});
});
