import { describe, it, expect } from 'vitest';
import { keyIdentity, keyComment, profileKeyErrorMessage } from './ssh-key-identity';
import { m } from '$lib/paraglide/messages.js';

describe('keyIdentity', () => {
	it('returns the type and blob fields joined by one space', () => {
		expect(keyIdentity('ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMe comment')).toBe(
			'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMe'
		);
	});

	it('matches the same key pasted with different comments', () => {
		const a = keyIdentity('ssh-ed25519 AAAAB3Nza laptop');
		const b = keyIdentity('ssh-ed25519 AAAAB3Nza a different comment');
		expect(a).toBe(b);
	});

	it('tolerates extra whitespace between fields', () => {
		expect(keyIdentity('  ssh-rsa   AAAABlob   comment ')).toBe('ssh-rsa AAAABlob');
	});

	it('returns the single field when there is no blob', () => {
		expect(keyIdentity('ssh-ed25519')).toBe('ssh-ed25519');
	});

	it('returns an empty string for a blank key', () => {
		expect(keyIdentity('   ')).toBe('');
	});
});

describe('keyComment', () => {
	it('returns the fields after the blob, joined', () => {
		expect(keyComment('ssh-ed25519 AAAABlob user@host extra words')).toBe('user@host extra words');
	});

	it('returns an empty string when there is no comment', () => {
		expect(keyComment('ssh-ed25519 AAAABlob')).toBe('');
	});

	it('returns an empty string for a single-field input', () => {
		expect(keyComment('ssh-ed25519')).toBe('');
	});
});

describe('profileKeyErrorMessage', () => {
	const codes = [
		'invalid_label',
		'invalid_request',
		'ssh_key_empty',
		'ssh_key_multiline',
		'ssh_key_type',
		'ssh_key_format',
		'ssh_key_private',
		'ssh_key_too_long',
		'duplicate_label',
		'duplicate_key',
		'limit_reached'
	];

	it.each(codes)('maps %s to a non-empty translated message', (code) => {
		const message = profileKeyErrorMessage(code);
		expect(message.length).toBeGreaterThan(0);
		expect(message).not.toBe(code);
	});

	it('falls back to the generic message for an unknown code', () => {
		expect(profileKeyErrorMessage('unknown_code')).toBe(m['profileSshKeys.error.generic']());
	});

	it('falls back to the generic message for null', () => {
		expect(profileKeyErrorMessage(null)).toBe(m['profileSshKeys.error.generic']());
	});
});
