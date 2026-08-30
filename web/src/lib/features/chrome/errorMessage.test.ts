import { describe, expect, it } from 'vitest';
import { resolveErrorMessage, KNOWN_ERROR_CODES } from './errorMessage';
import { m } from '$lib/paraglide/messages.js';
import { getLocale, setLocale } from '$lib/paraglide/runtime.js';

// T012 (US1): resolveErrorMessage — a known `code` resolves to the matching
// localized message; an unlisted `code` resolves to the generic localized
// fallback. Table-driven over every code currently in the map, in both
// locales (FR-006: bounded client-side mapping, never the raw server message).

describe('resolveErrorMessage', () => {
	for (const code of KNOWN_ERROR_CODES) {
		describe(`code "${code}"`, () => {
			it('resolves to the matching localized message in French', () => {
				setLocale('fr', { reload: false });
				expect(getLocale()).toBe('fr');
				const result = resolveErrorMessage(code, 'raw server text');
				expect(result).not.toBe(m['error.generic']()); // sanity: specific differs from generic
				expect(result).toBe(localizedFor(code, 'fr'));
				expect(result).not.toBe('raw server text');
			});

			it('resolves to the matching localized message in English', () => {
				setLocale('en', { reload: false });
				expect(getLocale()).toBe('en');
				const result = resolveErrorMessage(code, 'raw server text');
				expect(result).toBe(localizedFor(code, 'en'));
				expect(result).not.toBe('raw server text');
			});
		});
	}

	// Ticket 02 (ADR 0002): cluster_rejected carries Proxmox's own message as
	// its content — surfaced as-is, never replaced by the generic fallback.
	describe('code "cluster_rejected"', () => {
		it('surfaces the raw Proxmox message when present', () => {
			setLocale('fr', { reload: false });
			expect(resolveErrorMessage('cluster_rejected', "snapshot feature not available for storage 'local'")).toBe(
				"snapshot feature not available for storage 'local'"
			);
		});

		it('falls back to the generic localized message when the fallback is empty', () => {
			setLocale('en', { reload: false });
			expect(resolveErrorMessage('cluster_rejected', '')).toBe(m['error.generic']());
		});
	});

	it('an unlisted code resolves to the generic localized fallback, never the raw server message', () => {
		setLocale('fr', { reload: false });
		const result = resolveErrorMessage('some_unknown_code_xyz', 'raw server text');
		expect(result).toBe(m['error.generic']());
		expect(result).not.toBe('raw server text');
	});

	it('an empty code resolves to the generic localized fallback', () => {
		setLocale('en', { reload: false });
		expect(resolveErrorMessage('', 'raw server text')).toBe(m['error.generic']());
	});
});

function localizedFor(code: string, locale: 'fr' | 'en'): string {
	const messages: Record<string, { fr: () => string; en: () => string }> = {
		forbidden: { fr: () => m['error.forbidden'](), en: () => m['error.forbidden']({}, { locale: 'en' }) },
		not_found: { fr: () => m['error.not_found'](), en: () => m['error.not_found']({}, { locale: 'en' }) },
		invalid_action: { fr: () => m['error.invalid_action'](), en: () => m['error.invalid_action']({}, { locale: 'en' }) },
		quota_exceeded: { fr: () => m['error.quota_exceeded'](), en: () => m['error.quota_exceeded']({}, { locale: 'en' }) },
		gabarit_exceeded: { fr: () => m['error.gabarit_exceeded'](), en: () => m['error.gabarit_exceeded']({}, { locale: 'en' }) },
		snapshot_storage_unsupported: {
			fr: () => m['error.snapshot_storage_unsupported'](),
			en: () => m['error.snapshot_storage_unsupported']({}, { locale: 'en' })
		},
		vm_locked: { fr: () => m['error.vm_locked'](), en: () => m['error.vm_locked']({}, { locale: 'en' }) },
		snapshot_name_exists: {
			fr: () => m['error.snapshot_name_exists'](),
			en: () => m['error.snapshot_name_exists']({}, { locale: 'en' })
		}
	};
	const entry = messages[code];
	if (!entry) throw new Error(`unknown code in test helper: ${code}`);
	return locale === 'fr' ? entry.fr() : entry.en();
}
