import { beforeEach, describe, expect, it } from 'vitest';
import { resolveAdminPolicyCopy } from './admin-policy';

describe('resolveAdminPolicyCopy', () => {
	beforeEach(() => {
		localStorage.clear();
		document.documentElement.lang = 'en';
	});

	it.each([
		{ locale: 'en', title: 'Global policy' },
		{ locale: 'fr', title: 'Politique globale' }
	])('uses the configured $locale copy and document language', ({ locale, title }) => {
		localStorage.setItem('pvmss-locale', locale);
		expect(resolveAdminPolicyCopy().title).toBe(title);
		expect(document.documentElement.lang).toBe(locale);
	});
});
