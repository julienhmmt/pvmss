import { afterEach, describe, expect, it, vi } from 'vitest';
import { AdminBaselineStore } from './baseline.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('AdminBaselineStore', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('loads the generated baseline with no override (issue 07)', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(
				jsonResponse(200, {
					generated: '#cloud-config\npackages:\n  - qemu-guest-agent\n',
					overridePresent: false,
					overrideFilename: 'pvmss-baseline.yml'
				})
			)
		);
		const store = new AdminBaselineStore();
		await store.load('default');
		expect(store.state?.generated).toContain('qemu-guest-agent');
		expect(store.state?.overridePresent).toBe(false);
		expect(store.error).toBeNull();
	});

	it('loads the generated baseline with a present override (issue 07)', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(
				jsonResponse(200, {
					generated: '#cloud-config\npackages:\n  - qemu-guest-agent\n',
					overridePresent: true,
					overrideFilename: 'pvmss-baseline.yml',
					overrideContent: '#cloud-config\npackages:\n  - htop\n'
				})
			)
		);
		const store = new AdminBaselineStore();
		await store.load('default');
		expect(store.state?.overridePresent).toBe(true);
		expect(store.state?.overrideContent).toContain('htop');
	});

	it('surfaces a load error', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(500, { error: 'internal' })));
		const store = new AdminBaselineStore();
		await store.load('default');
		expect(store.state).toBeNull();
		expect(store.error).not.toBeNull();
	});
});
