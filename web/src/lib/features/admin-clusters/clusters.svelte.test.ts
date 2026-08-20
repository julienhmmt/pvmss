import { afterEach, describe, expect, it, vi } from 'vitest';
import { AdminClustersStore } from './clusters.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

const cluster = {
	name: 'secondary',
	displayName: 'prod-pve',
	url: 'https://secondary.invalid',
	tlsInsecureSkipVerify: false,
	tokenId: 'pvmss@pve!service',
	tokenSet: true,
	oidcEnabled: false,
	removedAt: null,
	lastTestStatus: 'ok' as const,
	lastTestAt: null,
	lastTestMessage: null,
	proxmoxVersion: '8.2.4',
	nodeCount: 2,
	vmCount: 18
};

describe('AdminClustersStore', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('loads cluster status without a secret field', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, [cluster])));
		const store = new AdminClustersStore();
		await store.load();
		expect(store.clusters).toEqual([cluster]);
		expect(store.error).toBeNull();
	});

	it('toggles OIDC locally from the server acknowledgement', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { name: 'secondary', oidcEnabled: true }));
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminClustersStore();
		store.clusters = [cluster];
		await store.toggleOIDC('secondary', true);
		expect(store.clusters[0]?.oidcEnabled).toBe(true);
		expect(JSON.parse(fetchMock.mock.calls[0]?.[1]?.body as string)).toEqual({ enabled: true });
	});
});
