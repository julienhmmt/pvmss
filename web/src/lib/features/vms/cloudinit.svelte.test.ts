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

	it('loads the VM document and the published templates of its cluster', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, { templateId: 'web', filename: 'pvmss-tpl-web-abc.yml', legacy: false, updatedAt: null, updatedBy: 'alice' }))
			.mockResolvedValueOnce(jsonResponse(200, { cloudInitTemplates: [{ id: 'web', label: 'Web' }], cloudInitWriteEnabled: true }));
		vi.stubGlobal('fetch', fetchMock);
		const store = new CloudInitStore('default', 101);

		await store.loadDocument();

		expect(store.document?.templateId).toBe('web');
		expect(store.templates).toEqual([{ id: 'web', label: 'Web' }]);
		expect(store.publishingEnabled).toBe(true);
		expect(String(fetchMock.mock.calls[1]?.[0])).toContain('/api/v1/vm-create/catalog?cluster=default');
	});

	it('switches the document with a template id, never YAML', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, { status: 'saved' }))
			.mockResolvedValueOnce(jsonResponse(200, { templateId: 'web', filename: 'f.yml', legacy: false, updatedAt: null, updatedBy: null }));
		vi.stubGlobal('fetch', fetchMock);
		const store = new CloudInitStore('default', 101);

		const saved = await store.saveDocument('web');

		expect(saved).toBe(true);
		expect(JSON.parse(fetchMock.mock.calls[0]?.[1]?.body as string)).toEqual({ templateId: 'web' });
		expect(store.document?.templateId).toBe('web');
	});

	it('maps cloudinit_not_published to a localized error', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(409, { code: 'cloudinit_not_published', message: 'x' })));
		const store = new CloudInitStore('default', 101);

		expect(await store.saveDocument('web')).toBe(false);
		expect(store.documentErrorCode).toBe('cloudinit_not_published');
		expect(store.documentError).not.toBe('x');
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
