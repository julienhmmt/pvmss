import { resolve } from '$app/paths';
import { m } from '#lib/paraglide/messages.js';
import { ADMIN_NAV_GROUPS } from './admin-nav-items.svelte';

/** One destination in the command palette (ticket 06: navigation only). */
export interface PaletteItem {
	id: string;
	group: 'recent' | 'goto' | 'admin' | 'machines';
	label: string;
	/** Secondary line: node and cluster for a machine, section for a page. */
	hint?: string;
	href: string;
}

// Per user: a shared browser must not show one account another's pages or machines.
const recentKey = (user: string): string => `pvmss-palette-recent-v1:${user}`;
const RECENT_LIMIT = 5;

/** Pages the principal may open: the sidebar's destinations, nothing more. */
export function navigationItems(isAdmin: boolean): PaletteItem[] {
	const page = (href: string, label: string): PaletteItem => ({ id: href, group: 'goto', label, href });
	const account = page(resolve('/profile'), m['account.heading']());
	if (!isAdmin) {
		return [
			page(resolve('/vms'), m['chrome.sidebar.navMyMachines']()),
			page(resolve('/vms/create'), m['vms.create.heading']()),
			page(resolve('/activity'), m['chrome.sidebar.navActivity']()),
			page(resolve('/docs'), m['chrome.sidebar.navHelp']()),
			account
		];
	}
	const adminPages = ADMIN_NAV_GROUPS.flatMap((group) =>
		group.items.map((entry): PaletteItem => ({ id: entry.href, group: 'admin', label: entry.label(), hint: group.heading(), href: entry.href }))
	);
	return [
		page(resolve('/admin'), m['chrome.sidebar.navDashboard']()),
		page(resolve('/search'), m['chrome.sidebar.navSearch']()),
		page(resolve('/about'), m['chrome.sidebar.navAbout']()),
		account,
		...adminPages
	];
}

function normalize(text: string): string {
	return text.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase();
}

/** Loose match: the query's letters appear in order (spaces ignored). */
export function matches(label: string, query: string): boolean {
	const needle = normalize(query).replace(/\s+/g, '');
	const haystack = normalize(label);
	let at = 0;
	for (const char of needle) {
		at = haystack.indexOf(char, at);
		if (at === -1) return false;
		at += 1;
	}
	return true;
}

/** Matching items, the ones whose label starts with the query first. */
export function filterItems(items: readonly PaletteItem[], query: string): PaletteItem[] {
	const q = normalize(query.trim());
	if (q === '') return [...items];
	const hits = items.filter((item) => matches(item.label, q));
	return [...hits.filter((item) => normalize(item.label).startsWith(q)), ...hits.filter((item) => !normalize(item.label).startsWith(q))];
}

/** The last destinations `user` opened from the palette, newest first. */
export function readRecent(user: string): PaletteItem[] {
	try {
		const parsed: unknown = JSON.parse(localStorage.getItem(recentKey(user)) ?? '[]');
		if (!Array.isArray(parsed)) return [];
		return (parsed as unknown[])
			.filter((entry): entry is PaletteItem => {
				const candidate = entry as Partial<PaletteItem> | null;
				return typeof candidate?.href === 'string' && typeof candidate.label === 'string';
			})
			.map((entry) => ({ ...entry, group: 'recent' as const }));
	} catch {
		return [];
	}
}

export function rememberRecent(item: PaletteItem, user: string): void {
	const next = [{ ...item, group: 'recent' as const }, ...readRecent(user).filter((entry) => entry.href !== item.href)].slice(0, RECENT_LIMIT);
	try {
		localStorage.setItem(recentKey(user), JSON.stringify(next));
	} catch {
		// Storage blocked: recents are a convenience, the palette works without them.
	}
}
