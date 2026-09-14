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
	vmCount: 18,
	snippetDir: '/snippets',
	snippetStorage: 'shared',
	cloudInitWriteEnabled: true
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

	it('create() forwards the snippet write target (spec D8)', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, cluster));
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminClustersStore();
		await store.create({
			name: 'secondary',
			url: 'https://secondary.invalid',
			tlsInsecureSkipVerify: false,
			tokenId: 'pvmss@pve!service',
			tokenSecret: 'secret',
			snippetDir: '/snippets',
			snippetStorage: 'shared'
		});
		expect(JSON.parse(fetchMock.mock.calls[0]?.[1]?.body as string)).toMatchObject({
			snippetDir: '/snippets',
			snippetStorage: 'shared'
		});
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

	it('loadSnippetStorages() queries the snippet-storages endpoint with the cluster name', async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			jsonResponse(200, [{ name: 'shared', node: 'pve-node-01', type: 'dir' }])
		);
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminClustersStore();
		const storages = await store.loadSnippetStorages('default');
		expect(storages).toEqual([{ name: 'shared', node: 'pve-node-01', type: 'dir' }]);
		expect(fetchMock.mock.calls[0]?.[0]).toContain('/api/v1/admin/snippet-storages?cluster=default');
	});
});
