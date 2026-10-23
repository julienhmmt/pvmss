import { describe, expect, it } from 'vitest';
import { locales } from '#lib/paraglide/runtime.js';

// A key missing from a locale silently falls back to the base locale, so
// every message file must carry exactly the base locale's keys and the same
// {placeholders}.
const files = import.meta.glob<Record<string, unknown>>('../../messages/*.json', { eager: true, import: 'default' });
const byLocale: Record<string, Record<string, unknown>> = Object.fromEntries(
	Object.entries(files).map(([path, messages]) => [path.replace(/^.*\/(\w+)\.json$/, '$1'), messages])
);
const placeholders = (value: unknown): string[] => [...JSON.stringify(value).matchAll(/\{(\w+)\}/g)].map((match) => match[1] ?? '').sort();

describe('message files', () => {
	const base = byLocale['fr'] ?? {};

	it('exist for every configured locale', () => {
		expect(Object.keys(byLocale).sort()).toEqual([...locales].sort());
	});

	for (const locale of locales) {
		it(`${locale} has the same keys and placeholders as fr`, () => {
			const messages = byLocale[locale] ?? {};
			expect(Object.keys(messages).sort()).toEqual(Object.keys(base).sort());
			const mismatched = Object.keys(base).filter(
				(key) => placeholders(messages[key]).join() !== placeholders(base[key]).join()
			);
			expect(mismatched).toEqual([]);
		});
	}
});
