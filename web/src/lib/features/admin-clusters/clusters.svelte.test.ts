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
	removedAt: null,
	lastTestStatus: 'ok' as const,
	lastTestAt: null,
	lastTestMessage: null,
	proxmoxVersion: '8.2.4',
	nodeCount: 2,
	vmCount: 18,
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

	it('creates a cluster through the admin route', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, cluster));
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminClustersStore();
		await store.create({
			name: 'secondary',
			tls: {},
			url: 'https://secondary.invalid',
			tlsInsecureSkipVerify: false,
			tokenId: 'pvmss@pve!service',
			tokenSecret: 'secret',
		});
		expect(fetchMock).toHaveBeenCalled();
	});
});
