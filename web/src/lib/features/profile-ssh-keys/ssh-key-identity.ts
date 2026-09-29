import { m } from '$lib/paraglide/messages.js';
import { PROFILE_SSH_KEY_MAX } from './types';

/** The identity of an SSH public key: its first two whitespace-separated
 *  fields (key type + base64 blob) joined by one space. The comment does
 *  not participate, so two lines with the same blob are the same key -
 *  which is exactly what the server-side SHA256 fingerprint compares.
 *  All client-side dedupe and "known key" checks use this instead of
 *  computing fingerprints in the browser. */
export function keyIdentity(key: string): string {
	const fields = key.trim().split(/\s+/).filter((field) => field !== '');
	return fields.slice(0, 2).join(' ');
}

/** The comment part of an SSH public key line: everything after the
 *  type and blob fields, joined and trimmed ('' when absent). */
export function keyComment(key: string): string {
	const fields = key.trim().split(/\s+/).filter((field) => field !== '');
	return fields.slice(2).join(' ').trim();
}

/** Maps a profile SSH-key API error code to its translated message.
 *  Unknown codes and null fall back to the generic message; the server's
 *  raw English text is never shown. */
export function profileKeyErrorMessage(code: string | null): string {
	switch (code) {
		case 'invalid_label':
			return m['profileSshKeys.error.invalidLabel']();
		case 'invalid_request':
			return m['profileSshKeys.error.invalidRequest']();
		case 'ssh_key_empty':
			return m['profileSshKeys.error.empty']();
		case 'ssh_key_multiline':
			return m['profileSshKeys.error.multiline']();
		case 'ssh_key_type':
			return m['profileSshKeys.error.type']();
		case 'ssh_key_format':
			return m['profileSshKeys.error.format']();
		case 'ssh_key_private':
			return m['profileSshKeys.error.private']();
		case 'ssh_key_too_long':
			return m['profileSshKeys.error.tooLong']({ max: 1024 });
		case 'duplicate_label':
			return m['profileSshKeys.error.duplicateLabel']();
		case 'duplicate_key':
			return m['profileSshKeys.error.duplicateKey']();
		case 'limit_reached':
			return m['profileSshKeys.error.limit']({ max: PROFILE_SSH_KEY_MAX });
		default:
			return m['profileSshKeys.error.generic']();
	}
}
