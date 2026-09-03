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
			gabarit: { maxSockets: 4, maxCores: 8, maxMemoryMB: 16384, maxDiskPerVmGb: 500, maxNetworkCards: 4, maxSnapshots: 5, allowCustomYaml: true, isolationVlanTag: 0 },
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
			gabarit: { maxSockets: 4, maxCores: 8, maxMemoryMB: 16384, maxDiskPerVmGb: 10, maxNetworkCards: 4, maxSnapshots: 5, allowCustomYaml: true, isolationVlanTag: 0 },
			quota: { maxVmPerUser: 1 }
		};
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, response));
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminPolicyStore();
		store.cluster = 'default';
		await store.save({ gabarit: { maxDiskPerVmGb: 10 }, quota: { maxVmPerUser: 1 } });
		expect(store.policy).toEqual(response);
		expect(JSON.parse(fetchMock.mock.calls[0]?.[1]?.body as string)).toEqual({
			cluster: 'default',
			gabarit: { maxDiskPerVmGb: 10 },
			quota: { maxVmPerUser: 1 }
		});
	});
});
