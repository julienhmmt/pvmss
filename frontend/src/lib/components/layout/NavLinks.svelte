<script lang="ts">
	import { page } from '$app/stores';
	import type { NavLink } from '$lib/types/navbar';

	interface Props {
		navLinks: NavLink[];
	}

	let { navLinks }: Props = $props();

	function isActive(href: string, pathname: string): boolean {
		if (href === '/') return pathname === '/';
		return pathname.startsWith(href);
	}
</script>

<div class="pv-nav-links">
	{#each navLinks as link, i (i)}
		{@const active = isActive(link.href, $page.url.pathname)}
		<a
			href={link.href}
			class="pv-navbar-link {active ? 'pv-navbar-link--active' : ''}"
		>
			<link.icon weight={active ? 'fill' : 'regular'} class="h-4 w-4 flex-shrink-0" />
			{link.label}
		</a>
	{/each}
</div>

<style>
	.pv-nav-links {
		display: flex;
		align-items: center;
		gap: 2px;
	}

	.pv-navbar-link {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 12px;
		border-radius: 8px;
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--muted-foreground);
		text-decoration: none;
		transition: background 0.12s, color 0.12s;
		white-space: nowrap;
	}

	.pv-navbar-link:hover:not(.pv-navbar-link--active) {
		background: var(--accent);
		color: var(--accent-foreground);
	}

	.pv-navbar-link--active {
		color: hsl(var(--blaze-orange-600));
		background: hsl(var(--blaze-orange-50));
		font-weight: 600;
	}

	:global(.dark .pv-navbar-link--active) {
		background: hsl(var(--blaze-orange-950) / 0.5);
		color: hsl(var(--blaze-orange-400));
	}
</style>
