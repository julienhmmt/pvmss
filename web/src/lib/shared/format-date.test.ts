import { afterAll, describe, expect, it } from 'vitest';
import { setLocale } from '#lib/paraglide/runtime.js';
import { formatDate, formatDateTime, formatTime } from './format-date';

const ISO = '2026-03-04T15:05:00Z';

afterAll(() => setLocale('en', { reload: false }));

describe('format-date', () => {
	it('follows the interface locale, not the browser', async () => {
		await setLocale('fr', { reload: false });
		expect(formatDate(ISO)).toMatch(/4 mars 2026/);
		await setLocale('en', { reload: false });
		expect(formatDate(ISO)).toMatch(/Mar 4, 2026/);
	});

	it('offers a compact style for table cells and a time-only form', async () => {
		await setLocale('en', { reload: false });
		expect(formatDateTime(ISO, 'short')).toMatch(/^3\/4\/26, /);
		expect(formatDateTime(ISO)).toMatch(/^Mar 4, 2026, /);
		expect(formatTime(ISO)).toMatch(/\d{1,2}:05/);
	});

	it('returns an unparseable value unchanged instead of throwing', () => {
		expect(formatDateTime('not a date')).toBe('not a date');
	});
});

describe('no browser-locale date formatting', () => {
	// Every date goes through format-date so it follows the interface locale.
	const sources = import.meta.glob(['/src/**/*.{svelte,ts}', '!/src/**/*.test.ts', '!/src/lib/paraglide/**'], { query: '?raw', import: 'default', eager: true });

	it('has no toLocale*String or Intl.DateTimeFormat(undefined) call sites', () => {
		const offenders = Object.entries(sources)
			.filter(([, text]) => /toLocale(?:Date|Time)?String\(|DateTimeFormat\(undefined/.test(String(text)))
			.map(([path]) => path);
		expect(offenders).toEqual([]);
	});
});
