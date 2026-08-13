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

	interface NavItem {
		href: string;
		label: string;
	}

	interface NavGroup {
		heading: string;
		items: NavItem[];
	}

	const groups: NavGroup[] = [
		{
			heading: 'Overview',
			items: [{ href: resolve('/admin'), label: 'Dashboard' }]
		},
		{
			heading: 'Infrastructure',
			items: [
				{ href: resolve('/admin/nodes'), label: 'Nodes' },
				{ href: resolve('/admin/clusters'), label: 'Clusters' },
				{ href: resolve('/admin/pools'), label: 'Pools' }
			]
		},
		{
			heading: 'Catalog',
			items: [
				{ href: resolve('/admin/storages'), label: 'Storages' },
				{ href: resolve('/admin/isos'), label: 'ISOs' },
				{ href: resolve('/admin/bridges'), label: 'Bridges' },
				{ href: resolve('/admin/cloudinit-templates'), label: 'Cloud-init' },
				{ href: resolve('/admin/profiles'), label: 'VM Profiles' },
				{ href: resolve('/admin/tags'), label: 'Tags' }
			]
		},
		{
			heading: 'Policy',
			items: [
				{ href: resolve('/admin/policy'), label: 'Limits' },
				{ href: resolve('/admin/policy/nodes'), label: 'Node capacity' }
			]
		},
		{
			heading: 'System',
			items: [
				{ href: resolve('/admin/appinfo'), label: 'App Info' },
				{ href: resolve('/admin/settings'), label: 'Settings' }
			]
		}
	];
</script>

<nav
	aria-label="Admin"
	class="w-full shrink-0 border-b border-border px-2 py-3 md:w-52 md:border-b-0 md:border-r md:px-3 md:py-6"
>
	<ul class="flex flex-col gap-4">
		{#each groups as group (group.heading)}
			<li>
				<p class="px-2 pb-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
					{group.heading}
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
								{item.label}
							</a>
						</li>
					{/each}
				</ul>
			</li>
		{/each}
	</ul>
</nav>
