import { describe, expect, it } from 'vitest';
import { resolveErrorMessage, KNOWN_ERROR_CODES } from './errorMessage';
import { m } from '#lib/paraglide/messages.js';
import { setLocale } from '#lib/paraglide/runtime.js';

// T012 (US1): resolveErrorMessage - a known `code` resolves to the matching
// localized message; an unlisted `code` resolves to the generic localized
// fallback. Table-driven over every code currently in the map, in both
// locales (FR-006: bounded client-side mapping, never the raw server message).

describe('resolveErrorMessage', () => {
	for (const code of KNOWN_ERROR_CODES) {
		it.each(['fr', 'en'] as const)(`code "${code}" resolves to its own localized message in %s`, async (locale) => {
			await setLocale(locale, { reload: false });
			const result = resolveErrorMessage(code, 'raw server text');
			expect(result).not.toBe(m['error.generic']());
			expect(result).not.toContain('raw server text');
		});
	}

	it.each(['fr', 'en'] as const)('maps aliases and specific codes to their message in %s', async (locale) => {
		await setLocale(locale, { reload: false });
		expect(resolveErrorMessage('not_found', 'gone')).toBe(m['error.not_found']());
		expect(resolveErrorMessage('iso_not_approved', 'x')).toBe(m['error.not_approved']());
		expect(resolveErrorMessage('duplicate_tag', 'x')).toBe(m['error.duplicate']());
		expect(resolveErrorMessage('invalid_credentials', 'x')).toBe(m['error.invalid_credentials']());
	});

	it.each(['fr', 'en'] as const)('frames a validation detail in a localized sentence in %s', async (locale) => {
		await setLocale(locale, { reload: false });
		expect(resolveErrorMessage('invalid_request', 'name must be 3-32 characters')).toBe(
			m['error.validation']({ detail: 'name must be 3-32 characters' })
		);
		expect(resolveErrorMessage('hardware_exceeds_limit', 'memory 8192 exceeds 4096')).toContain('memory 8192 exceeds 4096');
		// Proxmox's own reason is the content of a failed task.
		expect(resolveErrorMessage('task_failed', 'storage full')).toContain('storage full');
	});

	it('gives an unknown code a generic sentence with the request reference, never the raw text', async () => {
		await setLocale('fr', { reload: false });
		const result = resolveErrorMessage('some_unknown_code_xyz', 'raw server text', { requestId: 'req-42' });
		expect(result).toBe(m['error.genericRef']({ ref: 'req-42' }));
		expect(result).not.toContain('raw server text');
	});

	// Ticket 02 (ADR 0002): cluster_rejected carries Proxmox's own message as
	// its content - surfaced as-is, never replaced by the generic fallback.
	describe('code "cluster_rejected"', () => {
		it('surfaces the raw Proxmox message when present', async () => {
			await setLocale('fr', { reload: false });
			expect(resolveErrorMessage('cluster_rejected', "snapshot feature not available for storage 'local'")).toBe(
				"snapshot feature not available for storage 'local'"
			);
		});

		it('falls back to the generic localized message when the fallback is empty', async () => {
			await setLocale('en', { reload: false });
			expect(resolveErrorMessage('cluster_rejected', '')).toBe(m['error.generic']());
		});
	});

	it.each(['en', 'fr'] as const)('cluster_permission_denied uses structured fields in %s', async (locale) => {
		await setLocale(locale, { reload: false });
		const message = resolveErrorMessage(
			'cluster_permission_denied',
			'reworded server message',
			{ privilege: 'VM.GuestAgent.Unrestricted', path: '/vms/999102' }
		);
		expect(message).toContain('VM.GuestAgent.Unrestricted');
		expect(message).toContain('/vms/999102');
		expect(message).toBe(m['error.cluster_permission_denied']({ privilege: 'VM.GuestAgent.Unrestricted', path: '/vms/999102' }));
	});

	it('an unlisted code resolves to the generic localized fallback, never the raw server message', async () => {
		await setLocale('fr', { reload: false });
		const result = resolveErrorMessage('some_unknown_code_xyz', 'raw server text');
		expect(result).toBe(m['error.generic']());
		expect(result).not.toBe('raw server text');
	});

	it('an empty code resolves to the generic localized fallback', async () => {
		await setLocale('en', { reload: false });
		expect(resolveErrorMessage('', 'raw server text')).toBe(m['error.generic']());
	});
});
