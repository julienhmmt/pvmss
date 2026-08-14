<script lang="ts">
	/**
	 * AdminNav — persistent side navigation for the /admin section.
	 *
	 * The admin area has 15 sub-pages with no in-area navigation before this:
	 * every page was a lone <section> and the only way between them was the
	 * global navbar, which crammed a subset of links into one row. This gives
	 * the admin surface a proper information architecture (product register:
	 * standard side-nav, don't reinvent). The server RequireAdmin middleware
	 * remains the real guard; this is UX only.
	 *
	 * Active state is exact-match on the current pathname. Exact match is
	 * correct here because the only nested route (/admin/policy/nodes) must not
	 * also light up its parent (/admin/policy).
	 */
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';

	interface NavItem {
		href: string;
		label: () => string;
	}

	interface NavGroup {
		heading: () => string;
		items: NavItem[];
	}

	const groups: NavGroup[] = [
		{
			heading: () => m['chrome.adminNav.overview'](),
			items: [{ href: resolve('/admin'), label: () => m['chrome.adminNav.dashboard']() }]
		},
		{
			heading: () => m['chrome.adminNav.infrastructure'](),
			items: [
				{ href: resolve('/admin/nodes'), label: () => m['chrome.adminNav.nodes']() },
				{ href: resolve('/admin/clusters'), label: () => m['chrome.adminNav.clusters']() },
				{ href: resolve('/admin/pools'), label: () => m['chrome.adminNav.pools']() }
			]
		},
		{
			heading: () => m['chrome.adminNav.catalog'](),
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
			items: [
				{ href: resolve('/admin/policy'), label: () => m['chrome.adminNav.limits']() },
				{ href: resolve('/admin/policy/nodes'), label: () => m['chrome.adminNav.nodeCapacity']() }
			]
		},
		{
			heading: () => m['chrome.adminNav.system'](),
			items: [
				{ href: resolve('/admin/appinfo'), label: () => m['chrome.adminNav.appInfo']() },
				{ href: resolve('/admin/settings'), label: () => m['chrome.adminNav.settings']() }
			]
		}
	];
</script>

<nav
	aria-label={m['chrome.adminNav.ariaLabel']()}
	class="w-full shrink-0 border-b border-border px-2 py-3 md:w-52 md:border-b-0 md:border-r md:px-3 md:py-6"
>
	<ul class="flex flex-col gap-4">
		{#each groups as group (group.heading())}
			<li>
				<p class="px-2 pb-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
					{group.heading()}
				</p>
				<ul class="flex flex-col gap-0.5">
					{#each group.items as item (item.href)}
						{@const active = page.url.pathname === item.href}
						<li>
							<a
								href={item.href}
								aria-current={active ? 'page' : undefined}
								class="block rounded-md px-2 py-1.5 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring {active
									? 'bg-accent font-medium text-accent-foreground'
									: 'text-muted-foreground hover:bg-accent/50 hover:text-foreground'}"
							>
								{item.label()}
							</a>
						</li>
					{/each}
				</ul>
			</li>
		{/each}
	</ul>
</nav>
