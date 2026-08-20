import { afterEach, describe, expect, it, vi } from 'vitest';
import { LoginForm } from './login.svelte';
import { m } from '$lib/paraglide/messages.js';

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

	it('appends the @pve realm to a bare username', async () => {
		const fetchMock = vi.fn().mockResolvedValueOnce(jsonResponse(200, { username: 'alice@pve', pool: 'pool-alice', isAdmin: false, cluster: '', clusterDisplayName: '' }));
		vi.stubGlobal('fetch', fetchMock);
		const form = new LoginForm();
		form.username = 'alice';
		form.password = 'pvmss-alice';
		await form.submit();
		expect(JSON.parse(fetchMock.mock.calls[0]?.[1]?.body as string).username).toBe('alice@pve');
	});

	it('does not double the realm when the user already typed one', async () => {
		const fetchMock = vi.fn().mockResolvedValueOnce(jsonResponse(200, { username: 'alice@pam', pool: 'pool-alice', isAdmin: false, cluster: '', clusterDisplayName: '' }));
		vi.stubGlobal('fetch', fetchMock);
		const form = new LoginForm();
		form.username = 'alice@pam';
		form.password = 'pvmss-alice';
		await form.submit();
		expect(JSON.parse(fetchMock.mock.calls[0]?.[1]?.body as string).username).toBe('alice@pam');
	});

	it('translates a server invalid_credentials error instead of showing the raw English message', async () => {
		const fetchMock = vi.fn().mockResolvedValueOnce(
			jsonResponse(401, { code: 'invalid_credentials', message: 'invalid credentials' })
		);
		vi.stubGlobal('fetch', fetchMock);
		const form = new LoginForm();
		form.username = 'alice';
		form.password = 'wrong';
		const principal = await form.submit();
		expect(principal).toBeNull();
		expect(form.error).toBe(m['login.error.invalidCredentials']());
		expect(form.error).not.toBe('invalid credentials');
	});
});
