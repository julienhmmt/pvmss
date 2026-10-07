import { afterEach, describe, expect, it, vi } from 'vitest';
import { VmCreateStore } from './create.svelte';
import type { ProfileSshKey } from '$lib/features/profile-ssh-keys/types';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

function profileKey(id: string, label: string, blob: string): ProfileSshKey {
	return { id, label, publicKey: `ssh-ed25519 ${blob} ${label}`, fingerprint: `SHA256:${id}`, createdAt: '2026-09-29T10:00:00Z' };
}

afterEach(() => vi.unstubAllGlobals());

describe('VmCreateStore SSH key selection', () => {
	it('keeps ciSshKeysInput as a proxy over the picker pasted text', () => {
		const store = new VmCreateStore();
		store.ciSshKeysInput = 'ssh-ed25519 AAAA one';
		expect(store.sshSelection.pasted).toBe('ssh-ed25519 AAAA one');
		store.sshSelection.pasted = 'ssh-ed25519 BBBB two';
		expect(store.ciSshKeysInput).toBe('ssh-ed25519 BBBB two');
	});

	it('sends picked profile keys and pasted keys, deduped by identity', () => {
		const store = new VmCreateStore();
		store.sshSelection.setProfileKeys([
			profileKey('k1', 'laptop', 'BLOB1'),
			profileKey('k2', 'desktop', 'BLOB2')
		]);
		// <= 3 keys: all preselected (D8).
		store.ciSshKeysInput = 'ssh-ed25519 BLOB1 pasted-same-key\nssh-ed25519 NEW key';

		expect(store.sshKeys()).toEqual([
			'ssh-ed25519 BLOB1 laptop',
			'ssh-ed25519 BLOB2 desktop',
			'ssh-ed25519 NEW key'
		]);
	});

	it('sends only pasted keys when the profile is empty', () => {
		const store = new VmCreateStore();
		store.ciSshKeysInput = 'ssh-ed25519 AAAA x';
		expect(store.sshKeys()).toEqual(['ssh-ed25519 AAAA x']);
	});

	it('loads the profile keys once, even when requested repeatedly', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { keys: [profileKey('k1', 'laptop', 'BLOB')] }));
		vi.stubGlobal('fetch', fetchMock);
		const store = new VmCreateStore();

		await store.loadProfileKeysOnce();
		await store.loadProfileKeysOnce();

		expect(fetchMock).toHaveBeenCalledTimes(1);
		expect(store.sshSelection.profileKeys).toHaveLength(1);
		expect(store.sshSelection.selectedIds).toEqual(['k1']);
	});

	it('treats a profile load failure as an empty profile', async () => {
		vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('down')));
		const store = new VmCreateStore();

		await store.loadProfileKeysOnce();

		expect(store.sshSelection.profileKeys).toEqual([]);
		expect(store.submitError).toBeNull();
	});

	it('exposes the key to save for the post-submit flow', () => {
		const store = new VmCreateStore();
		store.sshSelection.setProfileKeys([]);
		store.ciSshKeysInput = 'ssh-ed25519 NEW office-laptop';

		expect(store.sshSelection.keyToSave()).toEqual({ label: 'office-laptop', publicKey: 'ssh-ed25519 NEW office-laptop' });
	});

	it('does not offer to save a pasted key already in the profile', () => {
		const store = new VmCreateStore();
		store.sshSelection.setProfileKeys([profileKey('k1', 'laptop', 'BLOB1')]);
		store.ciSshKeysInput = 'ssh-ed25519 BLOB1 same-key-different-comment';

		expect(store.sshSelection.saveOffer()).toBeNull();
		expect(store.sshSelection.keyToSave()).toBeNull();
	});
});
