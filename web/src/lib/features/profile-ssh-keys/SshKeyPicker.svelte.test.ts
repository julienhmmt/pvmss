import { afterEach, describe, expect, it } from 'vitest';
import { mount, flushSync } from 'svelte';
import SshKeyPicker from './SshKeyPicker.svelte';
import { SshKeySelection } from './ssh-key-selection.svelte';
import { m } from '$lib/paraglide/messages.js';
import type { ProfileSshKey } from './types';

function profileKey(id: string, label: string, blob: string): ProfileSshKey {
	return {
		id,
		label,
		publicKey: `ssh-ed25519 ${blob} ${label}`,
		fingerprint: `SHA256:${id}`,
		createdAt: '2026-09-29T10:00:00Z'
	};
}

// The picker is controlled: driving the model is the same signal the
// textarea binding produces. (happy-dom + Textarea's autogrow action
// swallows the synthetic input event, so a DOM-level paste cannot be
// used here.)
function paste(selection: SshKeySelection, text: string): void {
	selection.pasted = text;
	flushSync();
}

afterEach(() => {
	document.body.innerHTML = '';
});

describe('SshKeyPicker', () => {
	it('renders only the textarea for an empty profile', () => {
		const selection = new SshKeySelection();
		mount(SshKeyPicker, { target: document.body, props: { selection } });

		expect(document.querySelector('[data-testid="ssh-key-picker"]')).not.toBeNull();
		expect(document.querySelector('[data-testid="ssh-key-picker-paste"]')).not.toBeNull();
		expect(document.querySelector('[data-testid^="ssh-key-picker-option-"]')).toBeNull();
		expect(document.querySelector('[data-testid="ssh-key-picker-save"]')).toBeNull();
	});

	it('renders one row per profile key with its fingerprint', () => {
		const selection = new SshKeySelection();
		selection.setProfileKeys([profileKey('k1', 'laptop', 'BLOB1'), profileKey('k2', 'desktop', 'BLOB2')]);
		mount(SshKeyPicker, { target: document.body, props: { selection } });

		const row = document.querySelector('[data-testid="ssh-key-picker-option-k1"]');
		expect(row?.textContent).toContain('laptop');
		expect(row?.textContent).toContain('SHA256:k1');
		expect(document.querySelector('[data-testid="ssh-key-picker-option-k2"]')).not.toBeNull();
	});

	it('reflects the default selection and toggles keys on change', () => {
		const selection = new SshKeySelection();
		selection.setProfileKeys([profileKey('k1', 'laptop', 'BLOB1')]);
		mount(SshKeyPicker, { target: document.body, props: { selection } });

		const checkbox = document.querySelector<HTMLInputElement>(
			'[data-testid="ssh-key-picker-option-k1"] input[type="checkbox"]'
		)!;
		expect(checkbox.checked).toBe(true);

		checkbox.click();
		flushSync();
		expect(selection.selectedIds).toEqual([]);
	});

	it('shows the save controls when a pasted key is unknown, with a prefilled label', () => {
		const selection = new SshKeySelection();
		selection.setProfileKeys([profileKey('k1', 'laptop', 'BLOB1')]);
		mount(SshKeyPicker, { target: document.body, props: { selection } });
		expect(document.querySelector('[data-testid="ssh-key-picker-save"]')).toBeNull();

		paste(selection, 'ssh-ed25519 BNEW a fresh key');

		const save = document.querySelector<HTMLInputElement>('[data-testid="ssh-key-picker-save"]');
		const label = document.querySelector<HTMLInputElement>('[data-testid="ssh-key-picker-save-label"]');
		expect(save).not.toBeNull();
		expect(save!.checked).toBe(true);
		expect(label!.value).toBe('a fresh key');
	});

	it('hides the save offer when the pasted key is already in the profile', () => {
		const selection = new SshKeySelection();
		selection.setProfileKeys([profileKey('k1', 'laptop', 'BLOB1')]);
		mount(SshKeyPicker, { target: document.body, props: { selection } });

		paste(selection, 'ssh-ed25519 BLOB1 a different comment');

		expect(document.querySelector('[data-testid="ssh-key-picker-save"]')).toBeNull();
	});

	it('marks a key already on the machine as present and disabled', () => {
		const selection = new SshKeySelection({ addOnly: true, existing: ['ssh-ed25519 BLOB1 comment'] });
		selection.setProfileKeys([profileKey('k1', 'laptop', 'BLOB1'), profileKey('k2', 'desktop', 'BLOB2')]);
		mount(SshKeyPicker, { target: document.body, props: { selection } });

		const row = document.querySelector('[data-testid="ssh-key-picker-option-k1"]')!;
		const checkbox = row.querySelector<HTMLInputElement>('input[type="checkbox"]')!;
		expect(checkbox.disabled).toBe(true);
		expect(row.textContent).toContain(m['profileSshKeys.alreadyOnVm']());
	});
});
