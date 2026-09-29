import { describe, it, expect } from 'vitest';
import { SshKeySelection } from './ssh-key-selection.svelte';
import { PROFILE_SSH_KEY_MAX, type ProfileSshKey } from './types';
import { m } from '$lib/paraglide/messages.js';

function profileKey(id: string, blob: string, comment = '', label?: string): ProfileSshKey {
	return {
		id,
		label: label ?? `label-${id}`,
		publicKey: comment === '' ? `ssh-ed25519 ${blob}` : `ssh-ed25519 ${blob} ${comment}`,
		fingerprint: `SHA256:${id}`,
		createdAt: '2026-09-29T10:00:00Z'
	};
}

describe('SshKeySelection.setProfileKeys', () => {
	it('selects all keys when the profile has 3 or fewer', () => {
		const selection = new SshKeySelection();
		selection.setProfileKeys([profileKey('a', 'BA'), profileKey('b', 'BB'), profileKey('c', 'BC')]);
		expect(selection.selectedIds).toEqual(['a', 'b', 'c']);
	});

	it('selects none when the profile has more than 3', () => {
		const selection = new SshKeySelection();
		selection.setProfileKeys([
			profileKey('a', 'BA'),
			profileKey('b', 'BB'),
			profileKey('c', 'BC'),
			profileKey('d', 'BD')
		]);
		expect(selection.selectedIds).toEqual([]);
	});

	it('selects none in add-only mode regardless of count', () => {
		const selection = new SshKeySelection({ addOnly: true });
		selection.setProfileKeys([profileKey('a', 'BA'), profileKey('b', 'BB')]);
		expect(selection.selectedIds).toEqual([]);
	});

	it('keeps an empty selection for an empty profile', () => {
		const selection = new SshKeySelection();
		selection.setProfileKeys([]);
		expect(selection.selectedIds).toEqual([]);
	});

	it('marks a profile key already on the machine and never selects it', () => {
		const selection = new SshKeySelection({ existing: ['ssh-ed25519 BA other-comment'] });
		selection.setProfileKeys([profileKey('a', 'BA'), profileKey('b', 'BB')]);

		expect(selection.isOnVm(selection.profileKeys[0]!)).toBe(true);
		expect(selection.isOnVm(selection.profileKeys[1]!)).toBe(false);
		expect(selection.selectedIds).toEqual(['b']);
	});
});

describe('SshKeySelection.pastedKeys', () => {
	it('splits on newlines, trims, and drops blanks', () => {
		const selection = new SshKeySelection();
		selection.pasted = '  ssh-ed25519 BA c1\n\n ssh-rsa BB c2  \n   \n';
		expect(selection.pastedKeys()).toEqual(['ssh-ed25519 BA c1', 'ssh-rsa BB c2']);
	});
});

describe('SshKeySelection.finalKeys', () => {
	it('unions existing, selected profile keys and pasted keys in order', () => {
		const selection = new SshKeySelection({ existing: ['ssh-ed25519 BEXIST vm'] });
		selection.setProfileKeys([profileKey('a', 'BA'), profileKey('b', 'BB')]);
		selection.selectedIds = ['b'];
		selection.pasted = 'ssh-ed25519 BPASTED new';

		expect(selection.finalKeys()).toEqual([
			'ssh-ed25519 BEXIST vm',
			'ssh-ed25519 BB',
			'ssh-ed25519 BPASTED new'
		]);
	});

	it('dedupes by identity, first occurrence wins', () => {
		const selection = new SshKeySelection({ existing: ['ssh-ed25519 BA original'] });
		selection.setProfileKeys([profileKey('a', 'BA', 'same-key-other-comment')]);
		selection.selectedIds = ['a'];
		selection.pasted = 'ssh-ed25519 BA yet-another-comment';

		expect(selection.finalKeys()).toEqual(['ssh-ed25519 BA original']);
	});

	it('returns only pasted keys for an empty profile without existing', () => {
		const selection = new SshKeySelection();
		selection.pasted = 'ssh-ed25519 BA\nssh-ed25519 BA dup\nssh-rsa BB';
		expect(selection.finalKeys()).toEqual(['ssh-ed25519 BA', 'ssh-rsa BB']);
	});
});

describe('SshKeySelection.saveOffer', () => {
	it('offers the single unknown pasted key', () => {
		const selection = new SshKeySelection();
		selection.setProfileKeys([profileKey('a', 'BA')]);
		selection.pasted = 'ssh-ed25519 BNEW laptop comment';

		expect(selection.saveOffer()).toBe('ssh-ed25519 BNEW laptop comment');
	});

	it('returns null when the pasted key is already in the profile, even with a different comment', () => {
		const selection = new SshKeySelection();
		selection.setProfileKeys([profileKey('a', 'BA', 'saved-comment')]);
		selection.pasted = 'ssh-ed25519 BA another-comment';

		expect(selection.saveOffer()).toBeNull();
	});

	it('returns null for several unknown pasted keys', () => {
		const selection = new SshKeySelection();
		selection.pasted = 'ssh-ed25519 BA\nssh-ed25519 BB';

		expect(selection.saveOffer()).toBeNull();
	});

	it('returns null when the pasted key is already on the machine', () => {
		const selection = new SshKeySelection({ existing: ['ssh-ed25519 BA'] });
		selection.pasted = 'ssh-ed25519 BA';

		expect(selection.saveOffer()).toBeNull();
	});

	it('returns null at the profile limit', () => {
		const selection = new SshKeySelection();
		selection.setProfileKeys(Array.from({ length: PROFILE_SSH_KEY_MAX }, (_, i) => profileKey(`k${i}`, `B${i}`)));
		selection.pasted = 'ssh-ed25519 BNEW';

		expect(selection.saveOffer()).toBeNull();
	});

	it('prefills the save label from the key comment', () => {
		const selection = new SshKeySelection();
		selection.pasted = 'ssh-ed25519 BNEW my laptop';

		expect(selection.saveOffer()).not.toBeNull();
		expect(selection.saveLabel).toBe('my laptop');
	});

	it('falls back to the default label when the key has no comment', () => {
		const selection = new SshKeySelection();
		selection.pasted = 'ssh-ed25519 BNEW';

		expect(selection.saveOffer()).not.toBeNull();
		expect(selection.saveLabel).toBe(m['profileSshKeys.defaultLabel']());
	});

	it('updates the prefilled label when the offer key changes', () => {
		const selection = new SshKeySelection();
		selection.pasted = 'ssh-ed25519 BA first';
		expect(selection.saveLabel).toBe('first');

		selection.pasted = 'ssh-ed25519 BB second';
		expect(selection.saveLabel).toBe('second');
	});

	it('keeps a user-edited label when the offer key changes', () => {
		const selection = new SshKeySelection();
		selection.pasted = 'ssh-ed25519 BA first';
		selection.saveLabel = 'my custom label';
		selection.saveLabelEdited = true;

		selection.pasted = 'ssh-ed25519 BB second';

		expect(selection.saveLabel).toBe('my custom label');
	});

	it('suffixes the prefilled label until it is unique among profile labels', () => {
		const selection = new SshKeySelection();
		const mine = m['profileSshKeys.defaultLabel']();
		selection.setProfileKeys([
			profileKey('a', 'BA', '', mine),
			profileKey('b', 'BB', '', `${mine} 2`)
		]);
		selection.pasted = 'ssh-ed25519 BNEW';

		expect(selection.saveLabel).toBe(`${mine} 3`);
	});

	it('truncates a long comment so the prefilled label fits 64 code points', () => {
		const selection = new SshKeySelection();
		const comment = 'k'.repeat(63) + '🔑🔑';
		selection.pasted = `ssh-ed25519 BNEW ${comment}`;

		expect([...selection.saveLabel].length).toBe(64);
		expect(selection.saveLabel.endsWith('🔑')).toBe(true);
	});

	it('truncates the base so a suffix still fits under 64 code points', () => {
		const selection = new SshKeySelection();
		const comment = 'k'.repeat(100);
		selection.setProfileKeys([profileKey('a', 'BA', '', 'k'.repeat(64))]);
		selection.pasted = `ssh-ed25519 BNEW ${comment}`;

		expect(selection.saveLabel).toBe(`${'k'.repeat(62)} 2`);
	});

	it('re-prefills the label when the profile loads after the paste', () => {
		const selection = new SshKeySelection();
		const mine = m['profileSshKeys.defaultLabel']();
		selection.pasted = 'ssh-ed25519 BNEW';
		expect(selection.saveLabel).toBe(mine);

		selection.setProfileKeys([profileKey('a', 'BA', '', mine)]);

		expect(selection.saveLabel).toBe(`${mine} 2`);
	});

	it('keeps a user-edited label when the profile loads after the paste', () => {
		const selection = new SshKeySelection();
		const mine = m['profileSshKeys.defaultLabel']();
		selection.pasted = 'ssh-ed25519 BNEW';
		selection.saveLabel = 'my custom label';

		selection.setProfileKeys([profileKey('a', 'BA', '', mine)]);

		expect(selection.saveLabel).toBe('my custom label');
	});
});

describe('SshKeySelection.keyToSave', () => {
	it('returns the offer with the trimmed label when saving is enabled', () => {
		const selection = new SshKeySelection();
		selection.pasted = 'ssh-ed25519 BNEW laptop';
		selection.saveLabel = '  laptop  ';

		expect(selection.keyToSave()).toEqual({ label: 'laptop', publicKey: 'ssh-ed25519 BNEW laptop' });
	});

	it('returns null when saving is unchecked', () => {
		const selection = new SshKeySelection();
		selection.pasted = 'ssh-ed25519 BNEW laptop';
		selection.saveToProfile = false;

		expect(selection.keyToSave()).toBeNull();
	});

	it('returns null when the label is blank', () => {
		const selection = new SshKeySelection();
		selection.pasted = 'ssh-ed25519 BNEW laptop';
		selection.saveLabel = '   ';

		expect(selection.keyToSave()).toBeNull();
	});

	it('returns null without an offer', () => {
		const selection = new SshKeySelection();
		expect(selection.keyToSave()).toBeNull();
	});
});

describe('SshKeySelection.reset', () => {
	it('clears the pasted text, the selection and the save form', () => {
		const selection = new SshKeySelection({ addOnly: true });
		selection.setProfileKeys([profileKey('a', 'BA')]);
		selection.selectedIds = ['a'];
		selection.pasted = 'ssh-ed25519 BNEW k';
		selection.saveToProfile = false;
		selection.saveLabel = 'custom';
		selection.saveLabelEdited = true;

		selection.reset();

		expect(selection.pasted).toBe('');
		expect(selection.selectedIds).toEqual([]);
		expect(selection.saveToProfile).toBe(true);
		expect(selection.saveLabelEdited).toBe(false);
		expect(selection.saveLabel).not.toBe('custom');
		// Profile keys stay loaded - reset only clears the picks.
		expect(selection.profileKeys).toHaveLength(1);
	});
});

describe('SshKeySelection.toggle', () => {
	it('adds and removes ids from the selection', () => {
		const selection = new SshKeySelection();
		selection.setProfileKeys([profileKey('a', 'BA'), profileKey('b', 'BB')]);
		selection.selectedIds = [];

		selection.toggle('a', true);
		expect(selection.selectedIds).toEqual(['a']);
		selection.toggle('a', false);
		expect(selection.selectedIds).toEqual([]);
	});
});
