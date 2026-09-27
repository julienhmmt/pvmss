import { m } from '$lib/paraglide/messages.js';

/** The two halves of the context header: where the user is, and what screen. */
export interface ContextLabel {
	section: string;
	screen: string;
}

/**
 * Maps a pathname to the context header's "Workspace / <screen>" pair
 * (DESIGN.md §5). Pure so it can be unit-tested without a router. Unknown
 * paths fall back to the section name alone, never to a raw URL segment.
 */
export function contextLabel(pathname: string): ContextLabel {
	const path: string = trimTrailingSlashes(pathname) || '/';
	if (path === '/admin' || path.startsWith('/admin/')) {
		return { section: m['chrome.context.administration'](), screen: path === '/admin' ? m['chrome.context.dashboard']() : adminScreen(path) };
	}
	const workspace = m['chrome.context.workspace']();
	if (path === '/vms') return { section: workspace, screen: m['chrome.context.machines']() };
	if (path === '/vms/create') return { section: workspace, screen: m['chrome.context.create']() };
	if (/^\/vms\/[^/]+\/[^/]+\/console$/.test(path)) return { section: workspace, screen: m['chrome.context.console']() };
	if (/^\/vms\/[^/]+\/[^/]+$/.test(path)) return { section: workspace, screen: m['chrome.context.machine']() };
	if (path === '/activity') return { section: workspace, screen: m['chrome.context.activity']() };
	if (path === '/docs' || path.startsWith('/docs/')) return { section: workspace, screen: m['chrome.context.help']() };
	if (path === '/profile') return { section: workspace, screen: m['chrome.context.account']() };
	if (path === '/profile/tokens') return { section: workspace, screen: m['chrome.context.tokens']() };
	if (path === '/search') return { section: workspace, screen: m['chrome.context.search']() };
	if (path === '/about') return { section: workspace, screen: m['chrome.context.about']() };
	if (path === '/nodes') return { section: workspace, screen: m['chrome.context.nodes']() };
	return { section: workspace, screen: '' };
}

function trimTrailingSlashes(value: string): string {
	let end: number = value.length;
	while (end > 0 && value[end - 1] === '/') end--;
	return value.slice(0, end);
}

function adminScreen(path: string): string {
	// Admin screens name themselves in their own page header; the context
	// line only needs to say "Administration", so reuse the last segment's
	// sidebar label when it is one of the known groups, else stay silent.
	const segment = path.split('/')[2] ?? '';
	const labels: Record<string, () => string> = {
		nodes: () => m['chrome.adminNav.nodes'](),
		clusters: () => m['chrome.adminNav.clusters'](),
		pools: () => m['chrome.adminNav.pools'](),
		storages: () => m['chrome.adminNav.storages'](),
		isos: () => m['chrome.adminNav.isos'](),
		templates: () => m['chrome.adminNav.templates'](),
		images: () => m['chrome.adminNav.images'](),
		bridges: () => m['chrome.adminNav.bridges'](),
		'cloudinit-templates': () => m['chrome.adminNav.cloudinit'](),
		baseline: () => m['chrome.adminNav.baseline'](),
		docs: () => m['chrome.adminNav.documentation'](),
		profiles: () => m['chrome.adminNav.profiles'](),
		tags: () => m['chrome.adminNav.tags'](),
		policy: () => (path === '/admin/policy/nodes' ? m['chrome.adminNav.nodeCapacity']() : m['chrome.adminNav.limits']()),
		appinfo: () => m['chrome.adminNav.appInfo'](),
		settings: () => m['chrome.adminNav.settings']()
	};
	return labels[segment]?.() ?? '';
}
