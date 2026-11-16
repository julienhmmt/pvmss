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
	tags: [{ name: 'oldtag', vmCount: 2, color: '#3b82f6' }],
	hiddenPools: [],
	hiddenLooseVMs: [],
	hiddenTags: []
};

const emptyReport = {
	cluster: 'musclegrid',
	pools: [],
	looseVMs: [],
	tags: [],
	hiddenPools: [],
	hiddenLooseVMs: [],
	hiddenTags: []
};

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

	it('does not auto-select loose VMs (hide-only selection)', async () => {
		stubFetch(jsonResponse(200, clusterOptions), jsonResponse(200, orphanReport));
		const store = new RecoveryStore();
		await store.load();
		expect(store.selectedVMs.size).toBe(0);
	});

	it('hideSelected posts pools, tags and loose VMs then reloads', async () => {
		const fetchMock = stubFetch(
			jsonResponse(200, clusterOptions),
			jsonResponse(200, orphanReport),
			jsonResponse(200, { pools: 1, tags: 0, vms: 1 }),
			jsonResponse(200, emptyReport)
		);
		const store = new RecoveryStore();
		await store.load();
		store.toggleTag('oldtag');
		store.toggleVM(902);

		await store.hideSelected();

		expect(fetchMock).toHaveBeenNthCalledWith(
			3,
			'/api/v1/admin/recovery/hide',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({ cluster: 'musclegrid', pools: ['pvmss-old'], tags: [], vms: [902] })
			})
		);
		expect(store.announce).toBe(m['admin.recovery.hidden']());
	});

	it('unhideItems posts to the unhide endpoint and reloads', async () => {
		const fetchMock = stubFetch(
			jsonResponse(200, clusterOptions),
			jsonResponse(200, orphanReport),
			jsonResponse(200, { pools: 1, tags: 0, vms: 0 }),
			jsonResponse(200, orphanReport)
		);
		const store = new RecoveryStore();
		await store.load();

		await store.unhideItems({ pools: ['pvmss-old'] });

		expect(fetchMock).toHaveBeenNthCalledWith(
			3,
			'/api/v1/admin/recovery/unhide',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({ cluster: 'musclegrid', pools: ['pvmss-old'], tags: [], vms: [] })
			})
		);
		expect(store.announce).toBe(m['admin.recovery.unhidden']());
	});

	it('adoptHiddenItem posts a single-item adopt and reloads', async () => {
		const fetchMock = stubFetch(
			jsonResponse(200, clusterOptions),
			jsonResponse(200, orphanReport),
			jsonResponse(200, { pools: ['pvmss-hidden'], tags: [] }),
			jsonResponse(200, emptyReport)
		);
		const store = new RecoveryStore();
		await store.load();

		await store.adoptHiddenItem({ pool: 'pvmss-hidden' });

		expect(fetchMock).toHaveBeenNthCalledWith(
			3,
			'/api/v1/admin/recovery',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({ cluster: 'musclegrid', pools: ['pvmss-hidden'], tags: [] })
			})
		);
	});

	it('reports nothing to hide when the selection is empty', async () => {
		const store = new RecoveryStore();
		await store.hideSelected();
		expect(store.hideError).toBe(m['admin.recovery.nothingToHide']());
	});

	it('filters pools on name, comment and member VM names', async () => {
		const report = {
			...orphanReport,
			pools: [
				{ name: 'pvmss-old', comment: 'previous install', members: [{ vmid: 900, name: 'web-old-01', node: 'n1', status: 'running' as const, tags: [] }] },
				{ name: 'pvmss-jhmt', comment: '', members: [] },
				{ name: 'pvmss-lily', comment: 'staging leftovers', members: [] }
			]
		};
		stubFetch(jsonResponse(200, clusterOptions), jsonResponse(200, report));
		const store = new RecoveryStore();
		await store.load();

		store.search = 'staging';
		expect(store.filteredPools.map((pool) => pool.name)).toEqual(['pvmss-lily']);

		store.search = 'web-old-01';
		expect(store.filteredPools.map((pool) => pool.name)).toEqual(['pvmss-old']);
	});

	it('filters tags by name and loose VMs by name or vmid', async () => {
		const report = {
			...orphanReport,
			tags: [
				{ name: 'oldtag', vmCount: 2, color: '#3b82f6' },
				{ name: 'other', vmCount: 1, color: '#10b981' }
			],
			looseVMs: [
				{ vmid: 902, name: 'solo-old', node: 'n2', status: 'stopped' as const, tags: [] },
				{ vmid: 903, name: 'stray', node: 'n2', status: 'stopped' as const, tags: [] }
			]
		};
		stubFetch(jsonResponse(200, clusterOptions), jsonResponse(200, report));
		const store = new RecoveryStore();
		await store.load();

		store.search = 'other';
		expect(store.filteredTags.map((tag) => tag.name)).toEqual(['other']);
		expect(store.filteredLooseVMs).toHaveLength(0);

		store.search = '903';
		expect(store.filteredLooseVMs.map((vm) => vm.vmid)).toEqual([903]);
	});

	it('select-all applies to the filtered rows only', async () => {
		const report = {
			...orphanReport,
			pools: [
				{ name: 'pvmss-a', comment: '', members: [] },
				{ name: 'pvmss-b', comment: '', members: [] },
				{ name: 'other-pool', comment: '', members: [] }
			]
		};
		stubFetch(jsonResponse(200, clusterOptions), jsonResponse(200, report));
		const store = new RecoveryStore();
		await store.load();
		store.togglePool('other-pool'); // deselect it so only a,b stay selected

		store.search = 'pvmss-';
		store.toggleAllPools(); // filtered a and b are all selected: clears them
		expect(store.selectedPools.has('pvmss-a')).toBe(false);
		expect(store.selectedPools.has('pvmss-b')).toBe(false);
		expect(store.selectedPools.has('other-pool')).toBe(false);

		store.toggleAllPools(); // reselect filtered a and b
		expect(store.selectedPools.has('pvmss-a')).toBe(true);
		expect(store.selectedPools.has('pvmss-b')).toBe(true);
		expect(store.selectedPools.has('other-pool')).toBe(false);

		store.search = 'other';
		expect(store.allPoolsSelected).toBe(false);
		store.toggleAllPools(); // selects only the filtered other-pool
		expect(store.selectedPools.has('other-pool')).toBe(true);
		expect(store.selectedPools.has('pvmss-a')).toBe(true);
	});

	it('surfaces the API error on load', async () => {
		stubFetch(jsonResponse(200, clusterOptions), jsonResponse(500, { code: 'internal_error', message: 'boom' }));
		const store = new RecoveryStore();
		await store.load();
		expect(store.error).toBe(m['error.internal_error']());
	});
});
