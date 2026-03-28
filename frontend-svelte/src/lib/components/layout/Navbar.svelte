<script lang="ts">
	import { page } from '$app/stores';
	import { base } from '$app/paths';
	import { auth } from '$lib/stores/auth.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Sheet from '$lib/components/ui/sheet';
	import ThemeToggle from './ThemeToggle.svelte';
	import { t } from 'svelte-i18n';
	import { setLocale } from '$lib/i18n';
	import {
		House,
		PlusSquare,
		MagnifyingGlass,
		BookOpen,
		UserCircle,
		GearSix,
		SignOut,
		List,
		CaretDown,
		Globe
	} from 'phosphor-svelte';

	let mobileOpen = $state(false);

	const navLinks = $derived([
		{ href: '/', icon: House, label: $t('nav.home') },
		{ href: '/vm/create', icon: PlusSquare, label: $t('nav.createVm') },
		{ href: '/search', icon: MagnifyingGlass, label: $t('nav.searchVm') },
		{ href: '/docs/user', icon: BookOpen, label: $t('nav.documentation') }
	]);

	function isActive(href: string, pathname: string): boolean {
		if (href === '/') return pathname === '/' || pathname === '';
		return pathname.startsWith(href);
	}

	function navigate(url: string) {
		window.location.href = url;
	}
</script>

<nav class="pv-navbar">
	<div class="pv-navbar-inner">
		<!-- Brand -->
		<a href="/" class="pv-navbar-brand">
			<div class="pv-navbar-logo">PV</div>
			<span class="pv-navbar-brand-name">PVMSS</span>
		</a>

		<!-- Desktop navigation -->
		<div class="pv-navbar-links hidden md:flex">
			{#each navLinks as link}
				{@const active = isActive(link.href, $page.url.pathname)}
				<a href={link.href} class="pv-navbar-link {active ? 'pv-navbar-link--active' : ''}">
					<link.icon weight={active ? 'fill' : 'regular'} class="h-4 w-4 flex-shrink-0" />
					{link.label}
				</a>
			{/each}
		</div>

		<div class="flex-1"></div>

		<!-- Right side -->
		<div class="flex items-center gap-1">
			<!-- Language selector -->
			<div class="hidden items-center sm:flex">
				<DropdownMenu.Root>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<button class="pv-navbar-icon-btn" {...props} aria-label="Language">
								<Globe class="h-4 w-4" />
							</button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end" class="w-36">
						<DropdownMenu.Item onclick={() => setLocale('fr')}>
							🇫🇷 {$t('nav.languageFr')}
						</DropdownMenu.Item>
						<DropdownMenu.Item onclick={() => setLocale('en')}>
							🇬🇧 {$t('nav.languageEn')}
						</DropdownMenu.Item>
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			</div>

			<!-- Theme toggle -->
			<ThemeToggle />

			<!-- Vertical divider -->
			<div class="pv-navbar-divider hidden sm:block"></div>

			<!-- User dropdown -->
			{#if auth.initialized && auth.username}
				<DropdownMenu.Root>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<button class="pv-navbar-user-btn" {...props}>
								<div class="pv-navbar-avatar">
									{auth.username.slice(0, 1).toUpperCase()}
								</div>
								<span class="hidden sm:inline">{auth.username}</span>
								<CaretDown class="h-3 w-3 opacity-50" />
							</button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end" class="w-52">
						<DropdownMenu.Label class="font-normal">
							<div class="flex flex-col space-y-0.5">
								<p class="text-sm font-semibold leading-none">{auth.username}</p>
								<p class="text-muted-foreground text-xs leading-none">{$t('nav.administrator')}</p>
							</div>
						</DropdownMenu.Label>
						<DropdownMenu.Separator />
						{#if auth.isAdmin}
							<DropdownMenu.Item onclick={() => navigate(`${base}/`)}>
								<GearSix class="h-4 w-4" />
								{$t('common.admin')}
							</DropdownMenu.Item>
							<DropdownMenu.Separator />
						{/if}
						<DropdownMenu.Item
							class="text-destructive focus:text-destructive"
							onclick={() => navigate('/logout')}
						>
							<SignOut class="h-4 w-4" />
							{$t('common.logout')}
						</DropdownMenu.Item>
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			{/if}

			<!-- Mobile menu trigger -->
			<div class="md:hidden">
				<Sheet.Root bind:open={mobileOpen}>
					<Sheet.Trigger>
						{#snippet child({ props })}
							<button class="pv-navbar-icon-btn" {...props} aria-label={$t('nav.menu')}>
								<List class="h-5 w-5" />
							</button>
						{/snippet}
					</Sheet.Trigger>
					<Sheet.Content side="left" class="w-72 p-0">
						<div class="flex items-center gap-3 border-b px-5 py-4">
							<div class="pv-navbar-logo">PV</div>
							<span class="text-base font-bold tracking-tight">PVMSS</span>
						</div>
						<div class="flex flex-col gap-0.5 p-3">
							{#each navLinks as link}
								{@const active = isActive(link.href, $page.url.pathname)}
								<a
									href={link.href}
									class="pv-sidebar-item {active ? 'pv-sidebar-item--active' : ''}"
									onclick={() => (mobileOpen = false)}
								>
									<span class="pv-sidebar-icon-wrap {active ? 'pv-sidebar-icon-wrap--active' : ''}">
										<link.icon weight={active ? 'fill' : 'regular'} class="h-4 w-4" />
									</span>
									{link.label}
								</a>
							{/each}

							<div class="border-border my-2 border-t"></div>

							<button
								class="pv-sidebar-item"
								onclick={() => { setLocale('fr'); mobileOpen = false; }}
							>
								<span class="pv-sidebar-icon-wrap">🇫🇷</span>
								{$t('nav.languageFr')}
							</button>
							<button
								class="pv-sidebar-item"
								onclick={() => { setLocale('en'); mobileOpen = false; }}
							>
								<span class="pv-sidebar-icon-wrap">🇬🇧</span>
								{$t('nav.languageEn')}
							</button>

							<div class="border-border my-2 border-t"></div>

							{#if auth.isAdmin}
								<a
									href="{base}/"
									class="pv-sidebar-item"
									onclick={() => (mobileOpen = false)}
								>
									<span class="pv-sidebar-icon-wrap"><GearSix class="h-4 w-4" /></span>
									{$t('common.admin')}
								</a>
							{/if}
							<a
								href="/logout"
								class="pv-sidebar-item text-destructive"
							>
								<span class="pv-sidebar-icon-wrap"><SignOut class="h-4 w-4" /></span>
								{$t('common.logout')}
							</a>
						</div>
					</Sheet.Content>
				</Sheet.Root>
			</div>
		</div>
	</div>
</nav>

<style>
	:global(.pv-navbar) {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		z-index: 50;
		height: 56px;
		background: var(--background);
		border-bottom: 1px solid var(--border);
		backdrop-filter: blur(12px);
		-webkit-backdrop-filter: blur(12px);
	}

	:global(.pv-navbar-inner) {
		display: flex;
		align-items: center;
		height: 100%;
		max-width: 1400px;
		margin: 0 auto;
		padding: 0 20px;
		gap: 4px;
	}

	:global(.pv-navbar-brand) {
		display: flex;
		align-items: center;
		gap: 10px;
		text-decoration: none;
		margin-right: 20px;
		flex-shrink: 0;
	}

	:global(.pv-navbar-logo) {
		width: 30px;
		height: 30px;
		border-radius: 8px;
		background: linear-gradient(135deg, hsl(var(--blaze-orange-600)), hsl(var(--blaze-orange-800)));
		color: white;
		font-size: 0.7rem;
		font-weight: 800;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		letter-spacing: 0.02em;
	}

	:global(.pv-navbar-brand-name) {
		font-size: 0.95rem;
		font-weight: 700;
		letter-spacing: -0.01em;
		color: var(--foreground);
	}

	:global(.pv-navbar-links) {
		align-items: center;
		gap: 2px;
	}

	:global(.pv-navbar-link) {
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

	:global(.pv-navbar-link:hover:not(.pv-navbar-link--active)) {
		background: var(--accent);
		color: var(--accent-foreground);
	}

	:global(.pv-navbar-link--active) {
		color: hsl(var(--blaze-orange-600));
		background: hsl(var(--blaze-orange-50));
		font-weight: 600;
	}

	:global(.dark .pv-navbar-link--active) {
		background: hsl(var(--blaze-orange-950) / 0.5);
		color: hsl(var(--blaze-orange-400));
	}

	:global(.pv-navbar-icon-btn) {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 34px;
		height: 34px;
		border-radius: 8px;
		background: transparent;
		border: none;
		cursor: pointer;
		color: var(--muted-foreground);
		transition: background 0.12s, color 0.12s;
	}

	:global(.pv-navbar-icon-btn:hover) {
		background: var(--accent);
		color: var(--accent-foreground);
	}

	:global(.pv-navbar-divider) {
		width: 1px;
		height: 20px;
		background: var(--border);
		margin: 0 4px;
	}

	:global(.pv-navbar-user-btn) {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		padding: 5px 10px 5px 5px;
		border-radius: 8px;
		background: transparent;
		border: none;
		cursor: pointer;
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--foreground);
		transition: background 0.12s;
	}

	:global(.pv-navbar-user-btn:hover) {
		background: var(--accent);
	}

	:global(.pv-navbar-avatar) {
		width: 26px;
		height: 26px;
		border-radius: 6px;
		background: linear-gradient(135deg, hsl(var(--blaze-orange-500)), hsl(var(--blaze-orange-700)));
		color: white;
		font-size: 0.7rem;
		font-weight: 700;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}
</style>
