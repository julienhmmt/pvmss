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
	/** Third sidebar level: the detail page the admin is on, if any. */
	detail?: (pathname: string) => AdminNavDetail | null;
}

export interface AdminNavDetail {
	href: string;
	label: string;
	title?: string;
}

/**
 * Reads `<base>/<seg1>/.../<segN>` (exactly `depth` segments after `base`) and
 * returns the decoded segments, or null when the path is not a detail route.
 */
function detailSegments(pathname: string, base: string, depth: number): string[] | null {
	if (!pathname.startsWith(`${base}/`)) return null;
	const raw = pathname.slice(base.length + 1).split('/');
	if (raw.length !== depth || raw.some((seg) => seg === '')) return null;
	try {
		return raw.map((seg) => decodeURIComponent(seg));
	} catch {
		return null;
	}
}

export function nodeDetail(pathname: string): AdminNavDetail | null {
	const base = resolve('/admin/nodes');
	const segs = detailSegments(pathname, base, 2);
	if (!segs) return null;
	const [cluster = '', node = ''] = segs;
	return { href: pathname, label: node, title: `${node} · ${cluster}` };
}

export function poolDetail(pathname: string): AdminNavDetail | null {
	const segs = detailSegments(pathname, resolve('/admin/pools'), 1);
	if (!segs) return null;
	const [name = ''] = segs;
	return { href: pathname, label: name };
}

// The VM URL only carries the vmid: the detail page publishes the loaded name here.
const vmDetailName = $state({ pathname: '', name: '' });

/** Called by the admin VM page; returns a cleanup that clears the name. */
export function publishVmDetailName(pathname: string, name: string): () => void {
	vmDetailName.pathname = pathname;
	vmDetailName.name = name;
	return () => {
		if (vmDetailName.pathname === pathname) vmDetailName.name = '';
	};
}

export function vmDetail(pathname: string): AdminNavDetail | null {
	const segs = detailSegments(pathname, resolve('/admin/vms'), 2);
	if (!segs) return null;
	const [cluster = '', vmid = ''] = segs;
	const fallback = `VM ${vmid}`;
	const name = vmDetailName.pathname === pathname && vmDetailName.name ? vmDetailName.name : fallback;
	return { href: pathname, label: name, title: `${name} (${vmid}) · ${cluster}` };
}

export function clusterDetail(pathname: string): AdminNavDetail | null {
	const base = resolve('/admin/clusters');
	const segs = detailSegments(pathname, base, 1);
	if (!segs) return null;
	const [name = ''] = segs;
	return { href: pathname, label: name };
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
			{ href: resolve('/admin/nodes'), label: () => m['chrome.adminNav.nodes'](), exact: false, detail: nodeDetail },
			{ href: resolve('/admin/clusters'), label: () => m['chrome.adminNav.clusters'](), exact: false, detail: clusterDetail },
			{ href: resolve('/admin/pools'), label: () => m['chrome.adminNav.pools'](), exact: false, detail: poolDetail },
			{ href: resolve('/admin/vms'), label: () => m['chrome.adminNav.vms'](), exact: false, detail: vmDetail }
		]
	},
	{
		heading: () => m['chrome.adminNav.resources'](),
		icon: 'catalog',
		items: [
			{ href: resolve('/admin/storages'), label: () => m['chrome.adminNav.storages']() },
			{ href: resolve('/admin/isos'), label: () => m['chrome.adminNav.isos']() },
			// Cloud images (.img) are disabled for now: they do not work reliably yet.
			// { href: resolve('/admin/images'), label: () => m['chrome.adminNav.images']() },
			{ href: resolve('/admin/bridges'), label: () => m['chrome.adminNav.bridges']() },
			{ href: resolve('/admin/docs'), label: () => m['chrome.adminNav.documentation']() }
		]
	},
	{
		heading: () => m['chrome.adminNav.provisioning'](),
		icon: 'shield',
		items: [
			{ href: resolve('/admin/templates'), label: () => m['chrome.adminNav.templates']() },
			{ href: resolve('/admin/profiles'), label: () => m['chrome.adminNav.profiles']() },
			{ href: resolve('/admin/tags'), label: () => m['chrome.adminNav.tags']() }
		]
	},
	{
		heading: () => m['chrome.adminNav.policy'](),
		icon: 'policy',
		items: [
			{ href: resolve('/admin/policy'), label: () => m['chrome.adminNav.limits']() },
			{ href: resolve('/admin/policy/nodes'), label: () => m['chrome.adminNav.nodeCapacity']() },
			{ href: resolve('/admin/quota-requests'), label: () => m['chrome.adminNav.quotaRequests']() }
		]
	},
	{
		heading: () => m['chrome.adminNav.system'](),
		icon: 'system',
		items: [
			{ href: resolve('/admin/appinfo'), label: () => m['chrome.adminNav.appInfo']() },
			{ href: resolve('/admin/recovery'), label: () => m['chrome.adminNav.recovery']() },
			{ href: resolve('/admin/settings'), label: () => m['chrome.adminNav.settings']() }
		]
	}
];
