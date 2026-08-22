import { resolve } from '$app/paths';
import { m } from '$lib/paraglide/messages.js';

/**
 * Sidebar icon names used for main and admin navigation.
 */
export type SidebarIconName =
	| 'home'
	| 'search'
	| 'vm'
	| 'dashboard'
	| 'infrastructure'
	| 'catalog'
	| 'policy'
	| 'system';

interface AdminNavItem {
	href: string;
	label: () => string;
}

interface AdminNavGroup {
	heading: () => string;
	icon: SidebarIconName;
	items: AdminNavItem[];
}

/**
 * Admin navigation items — the single source for the admin destination list.
 * Rendered as the "Administration" group inside the global Sidebar (T034:
 * the admin area no longer ships a second 52-width rail). The server-side
 * RequireAdmin middleware remains the real guard; this is IA only.
 *
 * Active state is exact-match on pathname (the only nested route,
 * /admin/policy/nodes, must not also light up /admin/policy).
 */
export const ADMIN_NAV_GROUPS: readonly AdminNavGroup[] = [
	{
		heading: () => m['chrome.adminNav.overview'](),
		icon: 'dashboard',
		items: [{ href: resolve('/admin'), label: () => m['chrome.adminNav.dashboard']() }]
	},
	{
		heading: () => m['chrome.adminNav.infrastructure'](),
		icon: 'infrastructure',
		items: [
			{ href: resolve('/admin/nodes'), label: () => m['chrome.adminNav.nodes']() },
			{ href: resolve('/admin/clusters'), label: () => m['chrome.adminNav.clusters']() },
			{ href: resolve('/admin/pools'), label: () => m['chrome.adminNav.pools']() }
		]
	},
	{
		heading: () => m['chrome.adminNav.catalog'](),
		icon: 'catalog',
		items: [
			{ href: resolve('/admin/storages'), label: () => m['chrome.adminNav.storages']() },
			{ href: resolve('/admin/isos'), label: () => m['chrome.adminNav.isos']() },
			{ href: resolve('/admin/bridges'), label: () => m['chrome.adminNav.bridges']() },
			{ href: resolve('/admin/cloudinit-templates'), label: () => m['chrome.adminNav.cloudinit']() },
			{ href: resolve('/admin/docs'), label: () => m['chrome.adminNav.documentation']() },
			{ href: resolve('/admin/profiles'), label: () => m['chrome.adminNav.profiles']() },
			{ href: resolve('/admin/tags'), label: () => m['chrome.adminNav.tags']() }
		]
	},
	{
		heading: () => m['chrome.adminNav.policy'](),
		icon: 'policy',
		items: [
			{ href: resolve('/admin/policy'), label: () => m['chrome.adminNav.limits']() },
			{ href: resolve('/admin/policy/nodes'), label: () => m['chrome.adminNav.nodeCapacity']() }
		]
	},
	{
		heading: () => m['chrome.adminNav.system'](),
		icon: 'system',
		items: [
			{ href: resolve('/admin/appinfo'), label: () => m['chrome.adminNav.appInfo']() },
			{ href: resolve('/admin/settings'), label: () => m['chrome.adminNav.settings']() }
		]
	}
];
