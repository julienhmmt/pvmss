import { afterEach, describe, expect, it, vi } from 'vitest';
import { AdminBaselineStore } from './baseline.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('AdminBaselineStore', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('loads the generated baseline and no publication yet', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(jsonResponse(200, { generated: '#cloud-config\npackages:\n  - qemu-guest-agent\n', publication: null }))
		);
		const store = new AdminBaselineStore();
		await store.load('default');
		expect(store.state?.generated).toContain('qemu-guest-agent');
		expect(store.state?.publication).toBeNull();
		expect(store.error).toBeNull();
	});

	it('loads the baseline publication with its per-node outcome', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(
				jsonResponse(200, {
					generated: '#cloud-config\n',
					publication: { filename: 'pvmss-baseline-abc.yml', publishedAt: '2026-09-23T00:00:00Z', nodes: [{ node: 'pve-node-01', ok: true }] }
				})
			)
		);
		const store = new AdminBaselineStore();
		await store.load('default');
		expect(store.state?.publication?.filename).toBe('pvmss-baseline-abc.yml');
	});

	it('surfaces a load error', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(500, { error: 'internal' })));
		const store = new AdminBaselineStore();
		await store.load('default');
		expect(store.state).toBeNull();
		expect(store.error).not.toBeNull();
	});
});
