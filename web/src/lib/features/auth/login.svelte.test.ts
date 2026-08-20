import { afterEach, describe, expect, it, vi } from 'vitest';
import { LoginForm } from './login.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('LoginForm', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('loads clusters and submits the selected cluster', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, [{ name: 'default', displayName: 'prod-pve', oidcEnabled: false }, { name: 'secondary', displayName: 'dr-pve', oidcEnabled: true }]))
			.mockResolvedValueOnce(jsonResponse(200, { username: 'alice@pve', pool: 'pool-alice', isAdmin: false, cluster: 'secondary', clusterDisplayName: 'dr-pve' }));
		vi.stubGlobal('fetch', fetchMock);
		const form = new LoginForm();
		await form.loadClusters();
		form.cluster = 'secondary';
		form.username = 'alice';
		form.password = 'pvmss-alice';
		const principal = await form.submit();
		expect(principal?.cluster).toBe('secondary');
		expect(JSON.parse(fetchMock.mock.calls[1]?.[1]?.body as string).cluster).toBe('secondary');
	});
});
