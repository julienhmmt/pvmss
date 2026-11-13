import { beforeEach, describe, expect, it } from 'vitest';
import { m } from '#lib/paraglide/messages.js';
import { docItems, filterItems, matches, navigationItems, readRecent, rememberRecent, type PaletteItem } from './command-palette';
import type { DocSummary } from '#lib/features/docs/docs.svelte.js';

const item = (label: string, href = `/${label}`): PaletteItem => ({ id: href, group: 'goto', label, href });

describe('matches', () => {
	it('is a case- and accent-insensitive subsequence match', () => {
		expect(matches('Capacité des nœuds', 'capa nœ')).toBe(true);
		expect(matches('Node capacity', 'ndcap')).toBe(true);
		expect(matches('Activité', 'activite')).toBe(true);
		expect(matches('Activity', 'xyz')).toBe(false);
		expect(matches('Activity', '')).toBe(true);
	});
});

describe('filterItems', () => {
	it('keeps matches, prefix hits first', () => {
		const result = filterItems([item('Search'), item('Activity'), item('Settings')], 'se');
		expect(result.map((entry) => entry.label)).toEqual(['Search', 'Settings']);
	});
});

const doc = (id: string, title: string, lang = 'en', audience: DocSummary['audience'] = 'user'): DocSummary => ({
	id,
	lang,
	title,
	category: 'guides',
	audience
});

describe('docItems', () => {
	const pages = [doc('leases', 'Leases and expiry'), doc('quotas', 'Quota basics'), doc('fr-only', 'Guide', 'fr'), doc('admin-ops', 'Ops runbook', 'en', 'admin')];

	it('maps current-locale user articles to docs-group items with the article href', () => {
		const items = docItems(pages, false, 'en');
		expect(items.map((entry) => entry.href)).toEqual(['/docs/leases?lang=en', '/docs/quotas?lang=en']);
		expect(items.every((entry) => entry.group === 'docs')).toBe(true);
		expect(items[0]).toMatchObject({ label: 'Leases and expiry' });
	});

	it('offers admin-audience articles only to an admin', () => {
		expect(docItems(pages, true, 'en').map((entry) => entry.label)).toContain('Ops runbook');
	});

	it('returns nothing when the index is empty', () => {
		expect(docItems([], true, 'fr')).toEqual([]);
	});
});

describe('navigationItems', () => {
	it('never offers admin pages to a pool user', () => {
		const hrefs = navigationItems(false).map((entry) => entry.href);
		expect(hrefs).toContain('/vms');
		expect(hrefs.some((href) => href.startsWith('/admin'))).toBe(false);
	});

	it('offers quota requests to an admin under the policy group', () => {
		const entry = navigationItems(true).find((item) => item.href === '/admin/quota-requests');
		expect(entry?.group).toBe('admin');
		expect(entry?.hint).toBe(m['chrome.adminNav.policy']());
	});

	it('gives an admin the dashboard and every admin page, grouped as admin', () => {
		const items = navigationItems(true);
		expect(items.map((entry) => entry.href)).toContain('/admin/policy/nodes');
		expect(items.find((entry) => entry.href === '/admin/settings')?.group).toBe('admin');
		expect(items.some((entry) => entry.href === '/vms/create')).toBe(false);
	});
});

describe('recent items', () => {
	beforeEach(() => localStorage.clear());

	it('keeps the last five, newest first, without duplicates', () => {
		for (const label of ['a', 'b', 'c', 'd', 'e', 'f', 'b']) rememberRecent(item(label), 'alice');
		expect(readRecent('alice').map((entry) => entry.label)).toEqual(['b', 'f', 'e', 'd', 'c']);
		expect(readRecent('alice').every((entry) => entry.group === 'recent')).toBe(true);
	});

	it('never shows one user the destinations of another on the same browser', () => {
		rememberRecent(item('Node capacity', '/admin/policy/nodes'), 'admin');
		expect(readRecent('alice')).toEqual([]);
	});

	it('survives a corrupt stored value', () => {
		localStorage.setItem('pvmss-palette-recent-v1:alice', '{not json');
		expect(readRecent('alice')).toEqual([]);
	});
});
