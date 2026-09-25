<script lang="ts">
	/**
	 * Sidebar - the Calm workspace rail (DESIGN.md §5). Pool users get the
	 * three-item workspace nav (My machines, Activity, Help & guides) under a
	 * "Personal workspace" label, and a bottom block with a reassurance note,
	 * preferences and an account link. Admins keep their Dashboard / Search /
	 * About items and the collapsible admin groups; they have no personal
	 * pool, so no machines, activity or account link. Active nav uses
	 * aria-current="page" + tint fill.
	 *
	 * Below 900px the same markup becomes a drawer (T035): the parent layout
	 * mounts it inside a Dialog-style overlay driven by ChromeState.sidebarOpen.
	 */
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { ADMIN_NAV_GROUPS, type SidebarIconName } from './admin-nav-items.svelte';
	import { SidebarNavigationState } from './sidebar-navigation.svelte';
	import { getChromeContext } from './chrome.svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { getPowerActionsContext } from '$lib/features/tasks/power-actions.svelte';
	import { get } from '$lib/shared/api/client';
	import type { VmListItem, VmListResult } from '$lib/features/vms/list.svelte';
	import { goto } from '$app/navigation';
	import { m } from '$lib/paraglide/messages.js';
	import SidebarIcon from './SidebarIcon.svelte';
	import Logo from '$lib/shared/ui/Logo.svelte';
	import LanguageSwitcher from './LanguageSwitcher.svelte';
	import ThemeToggle from './ThemeToggle.svelte';

	const session = getSessionContext();
	const chrome = getChromeContext();
	const tray = getTaskTrayContext();
	const powerActions = getPowerActionsContext();

	// "My machines" count chip: every machine the user owns (the list API's
	// total). Failed creations never exist server-side, so they are not in it.
	let machineCount = $state.raw<number | null>(null);
	async function loadMachineCount(): Promise<void> {
		if (session.principal === null || session.isAdmin) return;
		try {
			const result = await get<VmListResult>('/api/v1/vms?pageSize=1');
			machineCount = result.total;
		} catch {
			machineCount = null;
		}
	}
	onMount(() => {
		void loadMachineCount();
	});

	// "Activity" count chip: operations in flight right now - creations and
	// snapshot work the tray polls, plus power actions converging.
	const activityCount = $derived(tray.tasks.length + powerActions.size);

	const initials = $derived(accountInitials(session.principal?.displayName || session.principal?.username || ''));

	function accountInitials(name: string): string {
		const parts = name.trim().split(/[\s._-]+/).filter(Boolean);
		if (parts.length === 0) return '?';
		const first = parts[0]?.[0] ?? '';
		const second = parts.length > 1 ? (parts[1]?.[0] ?? '') : (parts[0]?.[1] ?? '');
		return `${first}${second}`.toUpperCase();
	}

	// Machines drawer (below the "Machines" nav link): a small owned-VMs list
	// a pool user can pop open without leaving whatever page they're on.
	// Admins have no personal pool (never own VMs), so the link is hidden.
	let machinesOpen = $state(false);
	let machinesVms = $state.raw<VmListItem[]>([]);
	let machinesLoading = $state.raw(false);
	let machinesLoaded = $state.raw(false);

	async function loadMachines(): Promise<void> {
		machinesLoading = true;
		try {
			const result = await get<VmListResult>('/api/v1/vms?pageSize=8&sortBy=name');
			machinesVms = result.items;
			machinesLoaded = true;
		} catch {
			machinesVms = [];
		} finally {
			machinesLoading = false;
		}
	}

	function toggleMachines(): void {
		machinesOpen = !machinesOpen;
		if (machinesOpen && !machinesLoaded) void loadMachines();
	}

	onMount(() =>
		tray.onTaskOk(() => {
			void loadMachineCount();
			if (machinesOpen) void loadMachines();
			else machinesLoaded = false;
		})
	);

	interface MainNavItem {
		href: string;
		label: () => string;
		icon: SidebarIconName;
		/** Prefix match: /vms also lights up for /vms/create and a detail page. */
		prefix?: boolean;
		/** Count chip, hidden when null or zero. */
		count?: () => number | null;
		/** Accent the count chip (in-flight operations). */
		countAccent?: boolean;
		countLabel?: (count: number) => string;
	}

	// Pool users: the three workspace destinations (DESIGN.md §5). Create and
	// detail are states of "My machines", not items of their own. Admins keep
	// Dashboard (they are redirected from / to /admin), Search and About.
	const mainNav = $derived<MainNavItem[]>(
		session.isAdmin
			? [
					{ href: resolve('/admin'), label: () => m['chrome.sidebar.navDashboard'](), icon: 'home' },
					{ href: resolve('/search'), label: () => m['chrome.sidebar.navSearch'](), icon: 'search' },
					{ href: resolve('/about'), label: () => m['chrome.sidebar.navAbout'](), icon: 'info' }
				]
			: [
					{
						href: resolve('/vms'),
						label: () => m['chrome.sidebar.navMyMachines'](),
						icon: 'vm',
						prefix: true,
						count: () => machineCount,
						countLabel: (count) => m['chrome.sidebar.machinesCount']({ count })
					},
					{
						href: resolve('/activity'),
						label: () => m['chrome.sidebar.navActivity'](),
						icon: 'clock',
						count: () => activityCount,
						countAccent: true,
						countLabel: (count) => m['chrome.sidebar.activityCount']({ count })
					},
					{ href: resolve('/docs'), label: () => m['chrome.sidebar.navHelp'](), icon: 'help', prefix: true }
				]
	);

	const navigation: SidebarNavigationState = new SidebarNavigationState(ADMIN_NAV_GROUPS.length);

	function isActive(href: string, exact = false): boolean {
		return navigation.isItemActive({ pathname: page.url.pathname, href, exact });
	}

	function isActiveGroup(group: (typeof ADMIN_NAV_GROUPS)[number]): boolean {
		return group.items.some((item) => isActive(item.href, true));
	}

	function isGroupOpen(index: number): boolean {
		const group = ADMIN_NAV_GROUPS[index];
		if (!group) return false;
		return navigation.isGroupOpen({ index, active: isActiveGroup(group) });
	}

	function toggleGroup(index: number): void {
		const group = ADMIN_NAV_GROUPS[index];
		if (!group) return;
		navigation.toggleGroup({ index, active: isActiveGroup(group) });
	}

	const docsHref = resolve('/docs');

	function closeDrawer(): void {
		chrome.closeSidebar();
	}

	async function handleLogout(): Promise<void> {
		await session.logout();
		await goto(resolve('/login'));
	}
</script>

<aside
	class="flex h-full w-[236px] shrink-0 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground"
	aria-label={m['chrome.sidebar.ariaLabel']()}
	data-testid="app-sidebar"
>
	<div class="flex flex-col gap-1 px-3 pt-5">
		<div class="flex items-center gap-2 px-2">
			<Logo />
			{#if session.principal}
				<span class="font-mono text-xs text-muted-foreground">{session.principal.clusterDisplayName || session.principal.cluster}</span>
			{/if}
		</div>
		{#if !session.isAdmin}
			<p class="px-2 text-xs text-muted-foreground" data-testid="sidebar-tagline">{m['chrome.sidebar.tagline']()}</p>
		{/if}
	</div>

	<div class="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-3 py-4">
		<nav class="flex flex-col gap-0.5" aria-label={m['chrome.navbar.ariaLabel']()}>
			{#if !session.isAdmin}
				<p class="px-3 pb-1.5 text-[0.6875rem] font-semibold uppercase tracking-[0.08em] text-muted-foreground-subtle" data-testid="sidebar-workspace-label">
					{m['chrome.sidebar.workspaceLabel']()}
				</p>
			{/if}
			{#each mainNav as item (item.href)}
				{@const active = isActive(item.href, !item.prefix)}
				{@const count = item.count?.() ?? null}
				{@const isMachines = item.href === resolve('/vms')}
				<div class="flex flex-col">
					<div
						class="flex items-center rounded-lg text-sm font-medium transition-colors {active
							? 'bg-sidebar-accent text-sidebar-accent-foreground'
							: 'text-muted-foreground hover:bg-sidebar-accent/50 hover:text-foreground'}"
					>
						<a
							href={item.href}
							aria-current={active ? 'page' : undefined}
							class="flex flex-1 items-center gap-2 px-3 py-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
							onclick={closeDrawer}
						>
							<SidebarIcon name={item.icon} />
							<span class="flex-1">{item.label()}</span>
							{#if count !== null && count > 0}
								<span
									class="min-w-5 rounded-full px-1.5 text-center font-mono text-[0.6875rem] tabular-nums max-[369px]:hidden {item.countAccent
										? 'bg-primary-solid text-primary-foreground'
										: 'bg-muted text-muted-foreground'}"
									aria-label={item.countLabel?.(count)}
									data-testid="sidebar-count"
								>
									{count}
								</span>
							{/if}
						</a>
						{#if isMachines}
							<button
								type="button"
								aria-expanded={machinesOpen}
								aria-controls="sidebar-machines-drawer"
								aria-label={m['chrome.sidebar.machinesToggle']()}
								class="rounded-lg p-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
								onclick={toggleMachines}
							>
								<svg
									viewBox="0 0 24 24"
									class="h-4 w-4 transition-transform duration-200 {machinesOpen ? 'rotate-180' : ''}"
									fill="none"
									stroke="currentColor"
									stroke-width="2"
									stroke-linecap="round"
									stroke-linejoin="round"
									aria-hidden="true"
								>
									<polyline points="6 9 12 15 18 9" />
								</svg>
							</button>
						{/if}
					</div>
					{#if isMachines && machinesOpen}
						<ul id="sidebar-machines-drawer" class="flex flex-col gap-0.5 py-1">
							{#if machinesLoading}
								<li class="px-9 py-1.5 text-xs text-muted-foreground-subtle">{m['common.loading']()}</li>
							{:else if machinesVms.length === 0}
								<li class="px-9 py-1.5 text-xs text-muted-foreground-subtle">{m['chrome.sidebar.machinesEmpty']()}</li>
							{:else}
								{#each machinesVms as vm (vm.vmid + vm.cluster)}
									{@const href = resolve('/vms/[cluster]/[vmid]', { cluster: vm.cluster, vmid: String(vm.vmid) })}
									<li>
										<a
											{href}
											aria-current={isActive(href, true) ? 'page' : undefined}
											class="flex items-center gap-2 rounded-lg pl-9 pr-3 py-1.5 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring {isActive(href, true)
												? 'bg-sidebar-accent font-medium text-sidebar-accent-foreground'
												: 'text-muted-foreground hover:bg-sidebar-accent/50 hover:text-foreground'}"
											onclick={closeDrawer}
										>
											<span
												class="inline-block size-1.5 shrink-0 rounded-full {vm.status === 'running'
													? 'bg-success'
													: vm.status === 'paused'
														? 'bg-destructive'
														: 'bg-muted-foreground-subtle'}"
												aria-hidden="true"
											></span>
											<span class="truncate">{vm.name}</span>
										</a>
									</li>
								{/each}
							{/if}
						</ul>
					{/if}
				</div>
			{/each}
		</nav>

		{#if session.isAdmin}
			<nav class="flex flex-col gap-1" aria-label={m['chrome.sidebar.navAdmin']()}>
				<p class="px-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground-subtle">
					{m['chrome.sidebar.navAdmin']()}
				</p>
				{#each ADMIN_NAV_GROUPS as group, index (group.heading())}
					<div class="flex flex-col">
						<button
							type="button"
							aria-expanded={isGroupOpen(index)}
							aria-controls="admin-nav-group-{index}"
							class="w-full flex items-center gap-2 rounded-lg px-3 py-2 text-left text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring {isActiveGroup(group)
								? 'text-foreground'
								: 'text-muted-foreground hover:bg-sidebar-accent/50 hover:text-foreground'}"
							onclick={() => toggleGroup(index)}
						>
							<SidebarIcon name={group.icon} />
							<span class="flex-1">{group.heading()}</span>
							<svg
								viewBox="0 0 24 24"
								class="h-4 w-4 transition-transform duration-200 {isGroupOpen(index) ? 'rotate-180' : ''}"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
								aria-hidden="true"
							>
								<polyline points="6 9 12 15 18 9" />
							</svg>
						</button>
						{#if isGroupOpen(index)}
							<ul id="admin-nav-group-{index}" class="flex flex-col gap-0.5 py-1">
								{#each group.items as item (item.href)}
									{@const active = isActive(item.href, true)}
									<li>
										<a
											href={item.href}
											aria-current={active ? 'page' : undefined}
											class="flex items-center rounded-lg pl-9 pr-3 py-1.5 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring {active
												? 'bg-sidebar-accent font-medium text-sidebar-accent-foreground'
												: 'text-muted-foreground hover:bg-sidebar-accent/50 hover:text-foreground'}"
											onclick={closeDrawer}
										>
											{item.label()}
										</a>
									</li>
								{/each}
							</ul>
						{/if}
					</div>
				{/each}
			</nav>
		{/if}
	</div>

	<div class="mt-auto flex flex-col gap-3 border-t border-sidebar-border px-3 pb-5 pt-4">
		{#if session.isAdmin}
			<a
				href={docsHref}
				aria-current={isActive(docsHref) ? 'page' : undefined}
				class="flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring {isActive(docsHref)
					? 'bg-sidebar-accent text-sidebar-accent-foreground'
					: 'text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground'}"
			>
				<SidebarIcon name="info" />
				{m['chrome.header.docs']()}
			</a>
		{:else}
			<p class="flex items-start gap-2 px-2 text-xs text-muted-foreground" data-testid="sidebar-reassurance">
				<SidebarIcon name="shield" class="mt-px h-3.5 w-3.5 shrink-0 text-success" />
				{m['chrome.sidebar.reassurance']()}
			</p>
		{/if}
		<div class="flex flex-wrap items-center justify-between gap-2" role="group" aria-label={m['chrome.sidebar.preferences']()}>
			<LanguageSwitcher />
			<ThemeToggle />
		</div>

		{#if session.principal}
			{#if session.isAdmin}
				<p class="px-2 text-xs text-muted-foreground-subtle">
					{m['chrome.sidebar.userChip']({ username: session.principal.displayName || session.principal.username })}
				</p>
			{:else}
				{@const accountHref = resolve('/profile')}
				<a
					href={accountHref}
					aria-current={isActive(accountHref, true) ? 'page' : undefined}
					aria-label={m['chrome.sidebar.accountLink']({ name: session.principal.displayName || session.principal.username })}
					class="flex items-center gap-2.5 rounded-lg px-2 py-2 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring {isActive(accountHref, true)
						? 'bg-sidebar-accent'
						: 'hover:bg-sidebar-accent/50'}"
					onclick={closeDrawer}
					data-testid="sidebar-account-link"
				>
					<span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-sidebar-accent text-xs font-semibold text-sidebar-accent-foreground" aria-hidden="true">
						{initials}
					</span>
					<span class="flex min-w-0 flex-1 flex-col">
						<span class="truncate text-sm font-medium text-foreground">{session.principal.displayName || session.principal.username}</span>
						<span class="truncate text-xs text-muted-foreground">{m['chrome.sidebar.accountLabel']()}</span>
					</span>
					<svg viewBox="0 0 24 24" class="h-4 w-4 shrink-0 text-muted-foreground" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
						<polyline points="9 18 15 12 9 6" />
					</svg>
				</a>
			{/if}
			<button
				type="button"
				class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
				onclick={handleLogout}
				data-testid="sidebar-logout-button"
				aria-label={m['auth.logout']()}
			>
				<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
					<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
					<polyline points="16 17 21 12 16 7" />
					<line x1="21" y1="12" x2="9" y2="12" />
				</svg>
				{m['auth.logout']()}
			</button>
		{/if}
	</div>
</aside>
