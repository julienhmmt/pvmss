import { afterEach, describe, expect, it, vi } from 'vitest';
import { mount } from 'svelte';
import CloudInitForm from './CloudInitForm.svelte';
import { CloudInitStore } from './cloudinit.svelte';
import type { CloudInitConfigUpdate } from './cloudinit.types';
import { SshKeySelection } from '$lib/features/profile-ssh-keys/ssh-key-selection.svelte';
import type { ProfileSshKey } from '$lib/features/profile-ssh-keys/types';
import { m } from '$lib/paraglide/messages.js';

function profileKey(id: string, label: string, blob: string): ProfileSshKey {
	return { id, label, publicKey: `ssh-ed25519 ${blob} ${label}`, fingerprint: `SHA256:${id}`, createdAt: '2026-09-29T10:00:00Z' };
}

function storeWithKeys(sshKeys: string[]): CloudInitStore {
	const store = new CloudInitStore('default', 101);
	store.config = { user: 'debian', sshKeys, ipMode: 'dhcp' };
	return store;
}

const tick = (): Promise<void> => new Promise((resolve) => setTimeout(resolve, 0));

afterEach(() => {
	document.body.innerHTML = '';
	vi.unstubAllGlobals();
});

describe('CloudInitForm SSH-key picker (add-only)', () => {
	it('renders the picker below the editable key list', async () => {
		const store = storeWithKeys(['ssh-ed25519 OLD on-vm']);
		const sshSelection = new SshKeySelection({ addOnly: true });
		sshSelection.setProfileKeys([profileKey('k1', 'laptop', 'BLOB1')]);
		mount(CloudInitForm, { target: document.body, props: { store, sshSelection, onRequestSave: () => {} } });
		await tick();

		expect(document.querySelector('[data-testid="cloudinit-ssh-keys"]')).not.toBeNull();
		expect(document.querySelector('[data-testid="ssh-key-picker"]')).not.toBeNull();
		expect(document.querySelector('[data-testid="ssh-key-picker-option-k1"]')).not.toBeNull();
		// Add-only mode: nothing is preselected.
		expect(sshSelection.selectedIds).toEqual([]);
	});

	it('appends a picked profile key to the keys sent on save', async () => {
		const store = storeWithKeys(['ssh-ed25519 OLD on-vm']);
		const sshSelection = new SshKeySelection({ addOnly: true });
		sshSelection.setProfileKeys([profileKey('k1', 'laptop', 'BLOB1')]);
		const onRequestSave = vi.fn<(update: CloudInitConfigUpdate) => void>();
		mount(CloudInitForm, { target: document.body, props: { store, sshSelection, onRequestSave } });
		await tick();

		sshSelection.toggle('k1', true);
		document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));

		expect(onRequestSave).toHaveBeenCalledTimes(1);
		expect(vi.mocked(onRequestSave).mock.calls[0]?.[0].sshKeys).toEqual([
			'ssh-ed25519 OLD on-vm',
			'ssh-ed25519 BLOB1 laptop'
		]);
	});

	it('never duplicates a profile key already on the machine', async () => {
		const store = storeWithKeys(['ssh-ed25519 BLOB1 on-vm-comment']);
		const sshSelection = new SshKeySelection({ addOnly: true });
		sshSelection.setProfileKeys([profileKey('k1', 'laptop', 'BLOB1')]);
		const onRequestSave = vi.fn<(update: CloudInitConfigUpdate) => void>();
		mount(CloudInitForm, { target: document.body, props: { store, sshSelection, onRequestSave } });
		await tick();

		const option = document.querySelector('[data-testid="ssh-key-picker-option-k1"]');
		expect(option?.textContent).toContain(m['profileSshKeys.alreadyOnVm']());
		expect(option?.querySelector('input')?.disabled).toBe(true);

		document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
		expect(vi.mocked(onRequestSave).mock.calls[0]?.[0].sshKeys).toEqual(['ssh-ed25519 BLOB1 on-vm-comment']);
	});

	it('appends a pasted one-off key after the existing keys', async () => {
		const store = storeWithKeys(['ssh-ed25519 OLD on-vm']);
		const sshSelection = new SshKeySelection({ addOnly: true });
		const onRequestSave = vi.fn<(update: CloudInitConfigUpdate) => void>();
		mount(CloudInitForm, { target: document.body, props: { store, sshSelection, onRequestSave } });
		await tick();

		sshSelection.pasted = 'ssh-ed25519 NEW pasted';
		document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));

		expect(vi.mocked(onRequestSave).mock.calls[0]?.[0].sshKeys).toEqual([
			'ssh-ed25519 OLD on-vm',
			'ssh-ed25519 NEW pasted'
		]);
	});

	it('shows the save offer only for an unknown pasted key', async () => {
		const store = storeWithKeys(['ssh-ed25519 OLD on-vm']);
		const sshSelection = new SshKeySelection({ addOnly: true });
		sshSelection.setProfileKeys([profileKey('k1', 'laptop', 'BLOB1')]);
		mount(CloudInitForm, { target: document.body, props: { store, sshSelection, onRequestSave: () => {} } });
		await tick();

		sshSelection.pasted = 'ssh-ed25519 BLOB1 same-key-different-comment';
		await tick();
		expect(document.querySelector('[data-testid="ssh-key-picker-save"]')).toBeNull();

		sshSelection.pasted = 'ssh-ed25519 NEW pasted';
		await tick();
		expect(document.querySelector('[data-testid="ssh-key-picker-save"]')).not.toBeNull();
	});
});
