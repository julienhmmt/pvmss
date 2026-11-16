import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AdminPolicyStore, buildPolicyExport, parsePolicyExport } from './policy.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('AdminPolicyStore', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	it('loads the server policy without local ceilings', async () => {
		const policy = {
			cluster: 'default',
			gabarit: { maxSockets: 4, maxCores: 8, maxMemoryMB: 16384, maxDiskPerVmGb: 500, maxNetworkCards: 4, maxSnapshots: 5, isolationVlanTag: 0, vmidMin: 0, vmidMax: 0 },
			quota: { maxVmPerUser: -1 }
		};
		vi.stubGlobal('fetch', vi.fn().mockImplementation(() => jsonResponse(200, policy)));
		const store = new AdminPolicyStore();
		await store.load();
		expect(store.policy).toEqual(policy);
	});

	it('saves a partial policy and reconciles the full server response', async () => {
		const response = {
			cluster: 'default',
			gabarit: { maxSockets: 4, maxCores: 8, maxMemoryMB: 16384, maxDiskPerVmGb: 10, maxNetworkCards: 4, maxSnapshots: 5, isolationVlanTag: 0, vmidMin: 0, vmidMax: 0 },
			quota: { maxVmPerUser: 1 }
		};
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, response));
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminPolicyStore();
		store.cluster = 'default';
		await store.save({ gabarit: { maxDiskPerVmGb: 10 }, quota: { maxVmPerUser: 1 } });
		expect(store.policy).toEqual(response);
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined;
		expect(JSON.parse(init?.body as string)).toEqual({
			cluster: 'default',
			gabarit: { maxDiskPerVmGb: 10 },
			quota: { maxVmPerUser: 1 }
		});
	});
});

describe('AdminPolicyStore quota overrides', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	it('loads the selected cluster overrides after the policy', async () => {
		const policy = {
			cluster: 'pve',
			gabarit: { maxSockets: 4, maxCores: 8, maxMemoryMB: 16384, maxDiskPerVmGb: 500, maxNetworkCards: 4, maxSnapshots: 5, isolationVlanTag: 0, vmidMin: 0, vmidMax: 0 },
			quota: { maxVmPerUser: 3 }
		};
		const overrides = [{ cluster: 'pve', pool: 'lab', maxVms: 12, updatedBy: 'admin', updatedAt: '2026-11-01T00:00:00Z' }];
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, [{ name: 'pve', displayName: 'pve' }]))
			.mockResolvedValueOnce(jsonResponse(200, policy))
			.mockResolvedValueOnce(jsonResponse(200, { overrides }));
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminPolicyStore();

		await store.load();

		expect(store.overrides).toEqual(overrides);
		expect(store.overridesError).toBeNull();
		expect(fetchMock.mock.calls[2]?.[0]).toBe(`/api/v1/admin/quota-overrides?cluster=${encodeURIComponent(store.cluster)}`);
	});

	it('keeps the policy when the overrides request fails', async () => {
		const policy = {
			cluster: 'pve',
			gabarit: { maxSockets: 4, maxCores: 8, maxMemoryMB: 16384, maxDiskPerVmGb: 500, maxNetworkCards: 4, maxSnapshots: 5, isolationVlanTag: 0, vmidMin: 0, vmidMax: 0 },
			quota: { maxVmPerUser: 3 }
		};
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, [{ name: 'pve', displayName: 'pve' }]))
			.mockResolvedValueOnce(jsonResponse(200, policy))
			.mockResolvedValueOnce(jsonResponse(500, { code: 'internal', message: 'boom' }));
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminPolicyStore();

		await store.load();

		expect(store.policy).toEqual(policy);
		expect(store.error).toBeNull();
		expect(store.overrides).toEqual([]);
		expect(store.overridesError).not.toBeNull();
	});
});

describe('policy export file', () => {
	const policy = {
		cluster: 'pve',
		gabarit: { maxSockets: 4, maxCores: 8, maxMemoryMB: 16384, maxDiskPerVmGb: 500, maxNetworkCards: 4, maxSnapshots: 5, isolationVlanTag: 0, vmidMin: 0, vmidMax: 0, leaseDays: 0 },
		quota: { maxVmPerUser: 3 }
	};

	it('round-trips the loaded policy', () => {
		expect(parsePolicyExport(buildPolicyExport(policy))).toEqual({ gabarit: policy.gabarit, quota: policy.quota });
	});

	it('rejects another export kind', () => {
		expect(() => parsePolicyExport(JSON.stringify({ kind: 'pvmss-profiles', version: 1, profiles: [] }))).toThrow();
	});

	it('rejects a file missing a gabarit field', () => {
		const file = JSON.parse(buildPolicyExport(policy)) as { gabarit: Record<string, number> };
		delete file.gabarit.maxSnapshots;
		expect(() => parsePolicyExport(JSON.stringify(file))).toThrow();
	});
});

describe('AdminPolicyStore.importPolicy', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	const policy = {
		cluster: 'pve',
		gabarit: { maxSockets: 4, maxCores: 8, maxMemoryMB: 16384, maxDiskPerVmGb: 500, maxNetworkCards: 4, maxSnapshots: 5, isolationVlanTag: 0, vmidMin: 0, vmidMax: 0, leaseDays: 0 },
		quota: { maxVmPerUser: 3 }
	};

	function stubFetch(importResponse: () => Response): ReturnType<typeof vi.fn> {
		const fetchMock = vi.fn().mockImplementation((url: string) => {
			if (url === '/api/v1/admin/policy/import') return Promise.resolve(importResponse());
			if (url === '/api/v1/auth/clusters') return Promise.resolve(jsonResponse(200, [{ name: 'pve', displayName: 'pve' }]));
			if (url.startsWith('/api/v1/admin/policy?')) return Promise.resolve(jsonResponse(200, policy));
			if (url.startsWith('/api/v1/admin/quota-overrides')) return Promise.resolve(jsonResponse(200, { overrides: [] }));
			return Promise.resolve(jsonResponse(404, { code: 'not_found', message: `unexpected ${url}` }));
		});
		vi.stubGlobal('fetch', fetchMock);
		return fetchMock;
	}

	it('posts the policy to every target cluster and reloads the current one', async () => {
		const outcomes = [{ cluster: 'pve', result: 'updated' }];
		const fetchMock = stubFetch(() => jsonResponse(200, outcomes));
		const store = new AdminPolicyStore();
		store.cluster = 'pve';

		const result = await store.importPolicy(['pve', 'lab'], { gabarit: policy.gabarit, quota: policy.quota });

		expect(result).toEqual(outcomes);
		expect(store.policy).toEqual(policy);
		const call = fetchMock.mock.calls.find(([url]) => url === '/api/v1/admin/policy/import');
		const init = call?.[1] as RequestInit | undefined;
		expect(JSON.parse(init?.body as string)).toEqual({ clusters: ['pve', 'lab'], gabarit: policy.gabarit, quota: policy.quota });
	});

	it('reports a failed import through saveError', async () => {
		stubFetch(() => jsonResponse(400, { code: 'invalid_request', message: 'invalid request body' }));
		const store = new AdminPolicyStore();
		store.cluster = 'pve';

		const result = await store.importPolicy(['pve'], { gabarit: policy.gabarit, quota: policy.quota });

		expect(result).toBeNull();
		expect(store.saveError).not.toBeNull();
	});
});
