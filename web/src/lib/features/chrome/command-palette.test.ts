import { beforeEach, describe, expect, it } from 'vitest';
import { filterItems, matches, navigationItems, readRecent, rememberRecent, type PaletteItem } from './command-palette';

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

describe('navigationItems', () => {
	it('never offers admin pages to a pool user', () => {
		const hrefs = navigationItems(false).map((entry) => entry.href);
		expect(hrefs).toContain('/vms');
		expect(hrefs.some((href) => href.startsWith('/admin'))).toBe(false);
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
