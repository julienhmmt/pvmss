import { afterEach, describe, expect, it, vi } from 'vitest';
import { AdminBaselineStore } from './baseline.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('AdminBaselineStore', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('loads the generated baseline without a document when unchecked', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(jsonResponse(200, { generated: '#cloud-config\npackages:\n  - qemu-guest-agent\n', document: null, documentError: 'off' }))
		);
		const store = new AdminBaselineStore();
		await store.load('default');
		expect(store.state?.generated).toContain('qemu-guest-agent');
		expect(store.state?.document).toBeNull();
		expect(store.error).toBeNull();
	});

	it('loads the baseline document with its command and per-node presence', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(
				jsonResponse(200, {
					generated: '#cloud-config\n',
					document: { filename: 'pvmss-baseline-abc.yml', command: 'F=...', nodes: [{ node: 'pve-node-01', present: true }] }
				})
			)
		);
		const store = new AdminBaselineStore();
		await store.load('default');
		expect(store.state?.document?.filename).toBe('pvmss-baseline-abc.yml');
		expect(store.state?.document?.nodes[0]?.present).toBe(true);
	});

	it('surfaces a load error', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(500, { error: 'internal' })));
		const store = new AdminBaselineStore();
		await store.load('default');
		expect(store.state).toBeNull();
		expect(store.error).not.toBeNull();
	});

	function requestUrl(input: RequestInfo | URL): string {
		if (typeof input === 'string') return input;
		return input instanceof URL ? input.href : input.url;
	}

	it('init resolves the real cluster name and loads its baseline', async () => {
		const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL) => {
			const url = requestUrl(input);
			if (url.includes('/api/v1/auth/clusters')) {
				return Promise.resolve(jsonResponse(200, [{ name: 'prod', displayName: 'Production' }]));
			}
			return Promise.resolve(jsonResponse(200, { generated: '#cloud-config\n', document: null, documentError: 'off' }));
		});
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminBaselineStore();
		await store.init();
		expect(store.cluster).toBe('prod');
		expect(store.clusterOptions[0]?.name).toBe('prod');
		expect(fetchMock).toHaveBeenCalledWith(expect.stringContaining('/api/v1/admin/baseline?cluster=prod'), expect.anything());
		expect(store.state?.generated).toContain('cloud-config');
	});

	it('keeps a matching cluster selection when multiple clusters exist', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockImplementation((input: RequestInfo | URL) => {
				const url = requestUrl(input);
				if (url.includes('/api/v1/auth/clusters')) {
					return Promise.resolve(
						jsonResponse(200, [
							{ name: 'default', displayName: 'Default' },
							{ name: 'prod', displayName: 'Production' }
						])
					);
				}
				return Promise.resolve(jsonResponse(200, { generated: '#cloud-config\n', document: null, documentError: 'off' }));
			})
		);
		const store = new AdminBaselineStore();
		await store.init();
		expect(store.cluster).toBe('default');
	});

	it('setCluster reloads the baseline for the new cluster', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { generated: '#cloud-config\n', document: null, documentError: 'off' }));
		vi.stubGlobal('fetch', fetchMock);
		const store = new AdminBaselineStore();
		store.setCluster('prod');
		expect(store.cluster).toBe('prod');
		await vi.waitFor(() => {
			expect(fetchMock).toHaveBeenCalledWith(expect.stringContaining('cluster=prod'), expect.anything());
		});
	});
});
