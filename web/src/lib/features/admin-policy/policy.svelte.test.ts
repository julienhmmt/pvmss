import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AdminPolicyStore } from './policy.svelte';

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
