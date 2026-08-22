import { afterEach, describe, expect, it, vi } from 'vitest';
import { CloudInitStore } from './cloudinit.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

afterEach(() => vi.unstubAllGlobals());

describe('CloudInitStore', () => {
	it('loads structured config and keeps password out of state', async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			jsonResponse(200, { user: 'debian', sshKeys: [], ipMode: 'dhcp', password: 'must-not-exist' })
		);
		vi.stubGlobal('fetch', fetchMock);
		const store = new CloudInitStore('default', 101);

		await store.loadConfig();

		expect(store.config?.user).toBe('debian');
		expect(store.config).not.toHaveProperty('password');
		expect(store.configError).toBeNull();
	});

	it('saves partial config, sends rebootNow, reloads config, and reloads VM', async () => {
		const reloadVm = vi.fn().mockResolvedValue(undefined);
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, { status: 'updated', rebooted: true }))
			.mockResolvedValueOnce(jsonResponse(200, { user: 'ubuntu', sshKeys: [], ipMode: 'dhcp' }));
		vi.stubGlobal('fetch', fetchMock);
		const store = new CloudInitStore('default', 101, reloadVm);

		const saved = await store.saveConfig({ user: 'ubuntu', ipMode: 'dhcp' }, true);

		expect(saved).toBe(true);
		expect(reloadVm).toHaveBeenCalledOnce();
		expect(JSON.parse(fetchMock.mock.calls[0]?.[1]?.body as string)).toEqual({ user: 'ubuntu', ipMode: 'dhcp', rebootNow: true });
		expect(store.config?.user).toBe('ubuntu');
	});

	it('maps null snippet content to empty editor state while preserving API state', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { content: null, updatedAt: null, updatedBy: null })));
		const store = new CloudInitStore('default', 101);

		await store.loadSnippet();

		expect(store.snippet?.content).toBeNull();
	});

	it('keeps push_failed code for distinct stored-not-applied feedback', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(502, { code: 'push_failed', message: 'snippet saved, not yet applied to the VM' })));
		const store = new CloudInitStore('default', 101);

		const saved = await store.saveSnippet('#cloud-config\n');

		expect(saved).toBe(false);
		expect(store.snippetErrorCode).toBe('push_failed');
		expect(store.snippetError).toContain('not yet applied');
	});

	it('injects an ssh key via POST and reloads config', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, { status: 'injected' }))
			.mockResolvedValueOnce(jsonResponse(200, { user: 'debian', sshKeys: [], ipMode: 'dhcp' }));
		vi.stubGlobal('fetch', fetchMock);
		const store = new CloudInitStore('default', 101);

		const ok = await store.addSSHKey('ssh-ed25519 AAAA x', 'debian');

		expect(ok).toBe(true);
		const req = fetchMock.mock.calls[0];
		expect(req?.[0]).toBe('/api/v1/vms/default/101/cloudinit/ssh-keys');
		expect(req?.[1]?.method).toBe('POST');
		expect(JSON.parse(req?.[1]?.body as string)).toEqual({ key: 'ssh-ed25519 AAAA x', user: 'debian' });
	});

	it('surfaces an injection failure without throwing', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(502, { code: 'invalid_key', message: 'invalid ssh public key' })));
		const store = new CloudInitStore('default', 101);

		const ok = await store.addSSHKey('not-a-key');

		expect(ok).toBe(false);
		expect(store.sshKeyErrorCode).toBe('invalid_key');
		expect(store.sshKeyError).toContain('invalid ssh public key');
	});
});
