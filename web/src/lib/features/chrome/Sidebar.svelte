<script lang="ts">
	/**
	 * Sidebar — Layer B app-shell sidebar (mockup 236px sticky column). Brand +
	 * cluster, "New machine" CTA (hidden for admins), Home / Machines / Nodes,
	 * admin group only when session.isAdmin, user chip, version. Active nav uses
	 * aria-current="page" + tint fill.
	 *
	 * Below 900px the same markup becomes a drawer (T035): the parent layout
	 * mounts it inside a Dialog-style overlay driven by ChromeState.sidebarOpen.
	 */
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { ADMIN_NAV_GROUPS } from './admin-nav-items.svelte';
	import { goto } from '$app/navigation';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		/** Public version string (fetched once by the shell layout). */
		version?: string | null;
	}

	let { version = null }: Props = $props();

	const session = getSessionContext();

	const mainNav: { href: string; label: () => string }[] = [
		{ href: resolve('/'), label: () => m['chrome.sidebar.navHome']() },
		{ href: resolve('/vms'), label: () => m['chrome.sidebar.navMachines']() },
		{ href: resolve('/nodes'), label: () => m['chrome.sidebar.navNodes']() }
	];

	function isActive(href: string): boolean {
		const path = page.url.pathname;
		if (href === resolve('/')) return path === resolve('/');
		return path === href || path.startsWith(href + '/');
	}

	async function handleLogout(): Promise<void> {
		await session.logout();
		await goto(resolve('/login'));
	}
</script>

<aside
	class="flex h-full w-[236px] shrink-0 flex-col gap-5 border-r border-sidebar-border bg-sidebar px-3 py-5 text-sidebar-foreground"
	aria-label={m['chrome.sidebar.ariaLabel']()}
	data-testid="app-sidebar"
>
	<div class="flex items-baseline gap-2 px-2">
		<span class="text-base font-bold tracking-tight">{m['chrome.sidebar.brand']()}</span>
		{#if session.principal}
			<span class="font-mono text-xs text-muted-foreground">{session.principal.cluster}</span>
		{/if}
	</div>

	{#if session.principal && !session.isAdmin}
		<a
			href={resolve('/vms/create')}
			class="rounded-[0.625rem] bg-primary px-3 py-2.5 text-center text-sm font-semibold text-primary-foreground shadow-card transition-colors hover:bg-primary/90"
		>
			{m['chrome.sidebar.newMachine']()}
		</a>
	{/if}

	<nav class="flex flex-col gap-0.5">
		{#each mainNav as item (item.href)}
			{@const active = isActive(item.href)}
			<a
				href={item.href}
				aria-current={active ? 'page' : undefined}
				class="rounded-lg px-3 py-2 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring {active
					? 'bg-sidebar-accent text-sidebar-accent-foreground'
					: 'text-muted-foreground hover:bg-sidebar-accent/50 hover:text-foreground'}"
			>
				{item.label()}
			</a>
		{/each}
	</nav>

	{#if session.isAdmin}
		<nav class="flex flex-col gap-3" aria-label={m['chrome.sidebar.navAdmin']()}>
			<p class="px-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground-subtle">
				{m['chrome.sidebar.navAdmin']()}
			</p>
			{#each ADMIN_NAV_GROUPS as group (group.heading())}
				<div class="flex flex-col gap-0.5">
					<p class="px-3 pb-0.5 text-xs font-medium text-muted-foreground">{group.heading()}</p>
					{#each group.items as item (item.href)}
						{@const active = isActive(item.href)}
						<a
							href={item.href}
							aria-current={active ? 'page' : undefined}
							class="rounded-lg px-3 py-1.5 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring {active
								? 'bg-sidebar-accent font-medium text-sidebar-accent-foreground'
								: 'text-muted-foreground hover:bg-sidebar-accent/50 hover:text-foreground'}"
						>
							{item.label()}
						</a>
					{/each}
				</div>
			{/each}
		</nav>
	{/if}

	<div class="mt-auto flex flex-col gap-3">
		{#if session.principal}
			<p class="px-2 text-xs text-muted-foreground-subtle">
				{m['chrome.sidebar.userChip']({ username: session.principal.username })}
			</p>
			<button
				type="button"
				class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
				onclick={handleLogout}
				data-testid="sidebar-logout-button"
				aria-label={m['auth.logout']}
			>
				<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
					<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
					<polyline points="16 17 21 12 16 7" />
					<line x1="21" y1="12" x2="9" y2="12" />
				</svg>
				{m['auth.logout']()}
			</button>
		{/if}
		{#if version}
			<p class="px-2 font-mono text-xs text-muted-foreground-subtle">
				{m['chrome.sidebar.version']({ version })}
			</p>
		{/if}
	</div>
</aside>
