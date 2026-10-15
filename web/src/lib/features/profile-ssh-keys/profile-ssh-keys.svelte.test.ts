import { afterEach, describe, expect, it, vi } from 'vitest';
import { ProfileSshKeysStore, loadProfileSshKeys } from './profile-ssh-keys.svelte';
import { SshKeySelection } from './ssh-key-selection.svelte';
import { PROFILE_SSH_KEY_MAX, type ProfileSshKey } from './types';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

function key(id: string, label: string): ProfileSshKey {
	return {
		id,
		label,
		publicKey: `ssh-ed25519 BLOB${id} ${label}`,
		fingerprint: `SHA256:${id}`,
		createdAt: '2026-09-29T10:00:00Z'
	};
}

afterEach(() => vi.unstubAllGlobals());

describe('ProfileSshKeysStore', () => {
	it('loads the key list', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { keys: [key('k1', 'laptop')] })));
		const store = new ProfileSshKeysStore();

		await store.load();

		expect(store.keys).toHaveLength(1);
		expect(store.keys[0]?.label).toBe('laptop');
		expect(store.loadError).toBeNull();
		expect(store.loading).toBe(false);
		expect(vi.mocked(fetch).mock.calls[0]?.[0]).toBe('/api/v1/profile/ssh-keys');
	});

	it('sets loadError and leaves keys empty on failure', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(500, { code: 'internal_error', message: 'boom' })));
		const store = new ProfileSshKeysStore();

		await store.load();

		expect(store.keys).toEqual([]);
		expect(store.loadError).not.toBeNull();
	});

	it('adds a key and appends it to the list', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(201, key('k2', 'desktop'))));
		const store = new ProfileSshKeysStore();

		const result = await store.add('desktop', 'ssh-ed25519 BLOBk2 desktop');

		expect(result).toEqual({ ok: true });
		expect(store.keys.map((item) => item.id)).toEqual(['k2']);
		const init = vi.mocked(fetch).mock.calls[0]?.[1];
		expect(init?.method).toBe('POST');
		expect(JSON.parse(init?.body as string)).toEqual({ label: 'desktop', publicKey: 'ssh-ed25519 BLOBk2 desktop' });
	});

	it('returns the server error code on add failure', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(409, { code: 'duplicate_key', message: 'dup' })));
		const store = new ProfileSshKeysStore();

		const result = await store.add('desktop', 'ssh-ed25519 BLOBk2 desktop');

		expect(result).toEqual({ ok: false, code: 'duplicate_key' });
		expect(store.keys).toEqual([]);
	});

	it('returns a null code when the failure carries none', async () => {
		vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('network down')));
		const store = new ProfileSshKeysStore();

		const result = await store.add('x', 'ssh-ed25519 BLOB x');

		expect(result).toEqual({ ok: false, code: null });
	});

	it('removes a key by id', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, { keys: [key('k1', 'a'), key('k2', 'b')] }))
			.mockResolvedValueOnce(new Response(null, { status: 204 }));
		vi.stubGlobal('fetch', fetchMock);
		const store = new ProfileSshKeysStore();
		await store.load();

		const removed = await store.remove('k1');

		expect(removed).toBe(true);
		expect(store.keys.map((item) => item.id)).toEqual(['k2']);
		const req = fetchMock.mock.calls[1] as [string, RequestInit | undefined] | undefined;
		expect(String(req?.[0])).toContain('/api/v1/profile/ssh-keys/k1');
		expect(req?.[1]?.method).toBe('DELETE');
	});

	it('reports a failed removal and keeps the key', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, { keys: [key('k1', 'a')] }))
			.mockResolvedValueOnce(jsonResponse(404, { code: 'not_found', message: 'gone' }));
		vi.stubGlobal('fetch', fetchMock);
		const store = new ProfileSshKeysStore();
		await store.load();

		const removed = await store.remove('k1');

		expect(removed).toBe(false);
		expect(store.keys).toHaveLength(1);
	});

	it('reports limitReached at the maximum', async () => {
		const keys = Array.from({ length: PROFILE_SSH_KEY_MAX }, (_, i) => key(`k${i}`, `key ${i}`));
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { keys })));
		const store = new ProfileSshKeysStore();

		await store.load();

		expect(store.limitReached).toBe(true);
	});
});

describe('loadProfileSshKeys', () => {
	it('feeds the selection with the loaded keys', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { keys: [key('k1', 'laptop')] })));
		const selection = new SshKeySelection();

		await loadProfileSshKeys(selection);

		expect(selection.profileKeys).toHaveLength(1);
		expect(selection.selectedIds).toEqual(['k1']);
	});

	it('behaves as an empty profile on failure, without throwing', async () => {
		vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('down')));
		const selection = new SshKeySelection();

		await expect(loadProfileSshKeys(selection)).resolves.toBeUndefined();
		expect(selection.profileKeys).toEqual([]);
	});
});
