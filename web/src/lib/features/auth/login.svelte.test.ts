import { afterEach, describe, expect, it, vi } from 'vitest';
import { LoginForm } from './login.svelte';
import { m } from '#lib/paraglide/messages.js';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('LoginForm', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('loads clusters and submits the selected cluster', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, [{ name: 'default', displayName: 'prod-pve' }, { name: 'secondary', displayName: 'dr-pve' }]))
			.mockResolvedValueOnce(jsonResponse(200, { username: 'alice@pve', pool: 'pool-alice', isAdmin: false, cluster: 'secondary', clusterDisplayName: 'dr-pve' }));
		vi.stubGlobal('fetch', fetchMock);
		const form = new LoginForm();
		await form.loadClusters();
		form.cluster = 'secondary';
		form.username = 'alice';
		form.password = 'pvmss-alice';
		const principal = await form.submit();
		expect(principal?.cluster).toBe('secondary');
		const init = fetchMock.mock.calls[1]?.[1] as RequestInit | undefined;
		expect((JSON.parse(init?.body as string) as { cluster: string }).cluster).toBe('secondary');
	});

	it('sends a bare username with the default pve realm', async () => {
		const fetchMock = vi.fn().mockResolvedValueOnce(jsonResponse(200, { username: 'alice@pve', pool: 'pool-alice', isAdmin: false, cluster: '', clusterDisplayName: '' }));
		vi.stubGlobal('fetch', fetchMock);
		const form = new LoginForm();
		form.username = 'alice';
		form.password = 'pvmss-alice';
		await form.submit();
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined;
		const body = JSON.parse(init?.body as string) as { username: string; realm: string };
		expect(body.username).toBe('alice');
		expect(body.realm).toBe('pve');
	});

	it('sends the selected pam realm', async () => {
		const fetchMock = vi.fn().mockResolvedValueOnce(jsonResponse(200, { username: 'root@pam', pool: '', isAdmin: true, cluster: '', clusterDisplayName: '' }));
		vi.stubGlobal('fetch', fetchMock);
		const form = new LoginForm();
		form.username = 'root';
		form.realm = 'pam';
		form.password = 'root-password';
		await form.submit();
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined;
		const body = JSON.parse(init?.body as string) as { username: string; realm: string };
		expect(body.username).toBe('root');
		expect(body.realm).toBe('pam');
	});

	it('does not send a realm when the user already typed one', async () => {
		const fetchMock = vi.fn().mockResolvedValueOnce(jsonResponse(200, { username: 'alice@pam', pool: 'pool-alice', isAdmin: false, cluster: '', clusterDisplayName: '' }));
		vi.stubGlobal('fetch', fetchMock);
		const form = new LoginForm();
		form.username = 'alice@pam';
		form.password = 'pvmss-alice';
		await form.submit();
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined;
		const body = JSON.parse(init?.body as string) as { username: string; realm?: string };
		expect(body.username).toBe('alice@pam');
		expect(body.realm).toBeUndefined();
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

	it('translates a cluster_unavailable error', async () => {
		const fetchMock = vi.fn().mockResolvedValueOnce(
			jsonResponse(503, { code: 'cluster_unavailable', message: 'cluster is unavailable' })
		);
		vi.stubGlobal('fetch', fetchMock);
		const form = new LoginForm();
		form.username = 'alice';
		form.password = 'pvmss-alice';
		const principal = await form.submit();
		expect(principal).toBeNull();
		expect(form.error).toBe(m['login.error.clusterUnavailable']());
		expect(form.error).not.toBe('cluster is unavailable');
	});

	it('refuses PVE login when the cluster is disabled without calling the server', async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);
		const form = new LoginForm();
		form.setPveDisabled(true);
		form.username = 'alice';
		form.password = 'pvmss-alice';
		const principal = await form.submit();
		expect(principal).toBeNull();
		expect(form.error).toBe(m['login.error.clusterUnavailable']());
		expect(fetchMock).not.toHaveBeenCalled();
	});

	it('allows local admin login even when PVE login is disabled', async () => {
		const fetchMock = vi.fn().mockResolvedValueOnce(
			jsonResponse(200, { username: 'admin', displayName: 'admin', isAdmin: true, cluster: '', clusterDisplayName: '', provider: 'bootstrap', isOwner: true })
		);
		vi.stubGlobal('fetch', fetchMock);
		const form = new LoginForm();
		form.setPveDisabled(true);
		form.provider = 'local';
		form.password = 'admin-password';
		const principal = await form.submit();
		expect(principal?.isAdmin).toBe(true);
		expect(form.error).toBeNull();
	});

	it('sends the optional local account username', async () => {
		const fetchMock = vi.fn().mockResolvedValueOnce(
			jsonResponse(200, { username: 'albert', displayName: 'albert', isAdmin: true, cluster: '', clusterDisplayName: '', provider: 'local', isOwner: false })
		);
		vi.stubGlobal('fetch', fetchMock);
		const form = new LoginForm();
		form.provider = 'local';
		form.localUsername = 'albert';
		form.password = 'albert-password';
		await form.submit();
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined;
		const body = JSON.parse(init?.body as string) as { username?: string };
		expect(body.username).toBe('albert');
	});

	it('omits the username so the server falls back to the bootstrap admin', async () => {
		const fetchMock = vi.fn().mockResolvedValueOnce(
			jsonResponse(200, { username: 'admin', displayName: 'admin', isAdmin: true, cluster: '', clusterDisplayName: '', provider: 'bootstrap', isOwner: true })
		);
		vi.stubGlobal('fetch', fetchMock);
		const form = new LoginForm();
		form.provider = 'local';
		form.password = 'admin-password';
		await form.submit();
		const init = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined;
		const body = JSON.parse(init?.body as string) as { username?: string };
		expect(body.username).toBeUndefined();
	});
});
