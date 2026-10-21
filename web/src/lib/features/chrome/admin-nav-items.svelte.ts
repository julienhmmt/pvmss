import { resolve } from '$app/paths';
import { m } from '#lib/paraglide/messages.js';

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
	| 'system'
	| 'info'
	| 'clock'
	| 'help'
	| 'shield';

interface AdminNavItem {
	href: string;
	label: () => string;
	exact?: boolean;
}

interface AdminNavGroup {
	heading: () => string;
	icon: SidebarIconName;
	items: AdminNavItem[];
}

/**
 * Admin navigation items - the single source for the admin destination list.
 * Rendered as the "Administration" group inside the global Sidebar (T034:
 * the admin area no longer ships a second 52-width rail). The server-side
 * RequireAdmin middleware remains the real guard; this is IA only.
 *
 * Admin links match exactly by default; a parent link can opt into matching its detail routes.
 */
export const ADMIN_NAV_GROUPS: readonly AdminNavGroup[] = [
	{
		heading: () => m['chrome.adminNav.infrastructure'](),
		icon: 'infrastructure',
		items: [
			{ href: resolve('/admin/nodes'), label: () => m['chrome.adminNav.nodes'](), exact: false },
			{ href: resolve('/admin/clusters'), label: () => m['chrome.adminNav.clusters']() },
			{ href: resolve('/admin/pools'), label: () => m['chrome.adminNav.pools'](), exact: false },
			{ href: resolve('/admin/vms'), label: () => m['chrome.adminNav.vms'](), exact: false }
		]
	},
	{
		heading: () => m['chrome.adminNav.resources'](),
		icon: 'catalog',
		items: [
			{ href: resolve('/admin/storages'), label: () => m['chrome.adminNav.storages']() },
			{ href: resolve('/admin/isos'), label: () => m['chrome.adminNav.isos']() },
			{ href: resolve('/admin/templates'), label: () => m['chrome.adminNav.templates']() },
			// Cloud images (.img) are disabled for now: they do not work reliably yet.
			// { href: resolve('/admin/images'), label: () => m['chrome.adminNav.images']() },
			{ href: resolve('/admin/bridges'), label: () => m['chrome.adminNav.bridges']() }
		]
	},
	{
		heading: () => m['chrome.adminNav.provisioning'](),
		icon: 'shield',
		items: [
			{ href: resolve('/admin/profiles'), label: () => m['chrome.adminNav.profiles']() },
			// Cloud-init is disabled for now: not reliable or viable yet, too complex to set up.
			// { href: resolve('/admin/cloudinit-templates'), label: () => m['chrome.adminNav.cloudinit']() },
			// { href: resolve('/admin/baseline'), label: () => m['chrome.adminNav.baseline']() },
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
			{ href: resolve('/admin/docs'), label: () => m['chrome.adminNav.documentation']() },
			{ href: resolve('/admin/settings'), label: () => m['chrome.adminNav.settings']() }
		]
	}
];
