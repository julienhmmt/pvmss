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
});
