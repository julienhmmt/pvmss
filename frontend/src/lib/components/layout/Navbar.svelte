<script lang="ts">
	import { page } from '$app/stores';
	import { onMount, onDestroy } from 'svelte';
	
	import { auth } from '$lib/stores/auth.svelte';
	import { logout } from '$lib/api/auth';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Sheet from '$lib/components/ui/sheet';
	import { Badge } from '$lib/components/ui/badge';
	import ThemeToggle from './ThemeToggle.svelte';
	import { t } from 'svelte-i18n';
	import { setLocale } from '$lib/i18n';
	import { notifications } from '$lib/stores/notifications.svelte';
	import type { NavLink, KeyboardShortcut } from '$lib/types/navbar';
	import {
		HouseIcon,
		PlusSquareIcon,
		MagnifyingGlassIcon,
		BookOpenIcon,
		GearSixIcon,
		SignInIcon,
		SignOutIcon,
		ListIcon,
		CaretDownIcon,
		GlobeIcon,
		BellIcon
	} from 'phosphor-svelte';

	let mobileOpen = $state(false);
	let notificationOpen = $state(false);
	let langDropdownOpen = $state(false);
	let userDropdownOpen = $state(false);
	let skipLinkFocused = $state(false);

	// Refs for keyboard navigation (not reactive, just DOM references)
	let mobileSheetContent: HTMLElement;

	// Keyboard shortcuts
	const keyboardShortcuts: KeyboardShortcut[] = [
		{
			key: 'k',
			macKey: 'k',
			ctrlKey: true,
			description: 'Search VMs',
			action: () => navigate('/search')
		},
		{
			key: 'b',
			macKey: 'b',
			ctrlKey: true,
			description: 'Create VM',
			action: () => navigate('/vm/create')
		},
		{
			key: 'Escape',
			description: 'Close dropdowns/modals',
			action: () => {
				mobileOpen = false;
				notificationOpen = false;
				langDropdownOpen = false;
				userDropdownOpen = false;
			}
		}
	];

	const navLinks = $derived<NavLink[]>([
		{ href: '/', icon: HouseIcon, label: $t('nav.home'), authRequired: false },
		{ href: '/home', icon: HouseIcon, label: $t('nav.myVms'), authRequired: true },
		{ href: '/vm/create', icon: PlusSquareIcon, label: $t('nav.createVm'), authRequired: true },
		{ href: '/search', icon: MagnifyingGlassIcon, label: $t('nav.searchVm'), authRequired: true },
		{ href: '/docs/user', icon: BookOpenIcon, label: $t('nav.documentation'), authRequired: false }
	].filter(link => !link.authRequired || auth.username));

	function isActive(href: string, pathname: string): boolean {
		if (href === '/') return pathname === '/';
		return pathname.startsWith(href);
	}

	function navigate(url: string) {
		window.location.href = url;
	}

	async function handleLogout() {
		try {
			await logout();
		} finally {
			auth.clear();
			window.location.href = '/login';
		}
	}

	// Keyboard event handler
	function handleKeyboard(event: KeyboardEvent) {
		// Check for keyboard shortcuts
		const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
		const modifierKey = isMac ? event.metaKey : event.ctrlKey;

		for (const shortcut of keyboardShortcuts) {
			if (
				event.key.toLowerCase() === shortcut.key.toLowerCase() &&
				!!modifierKey === !!shortcut.ctrlKey &&
				!!event.shiftKey === !!shortcut.shiftKey &&
				!!event.altKey === !!shortcut.altKey
			) {
				event.preventDefault();
				shortcut.action();
				return;
			}
		}

		// Handle Escape key to close dropdowns
		if (event.key === 'Escape') {
			mobileOpen = false;
			notificationOpen = false;
			langDropdownOpen = false;
			userDropdownOpen = false;
		}

		// Focus trap for mobile menu
		if (mobileOpen && event.key === 'Tab' && mobileSheetContent) {
			const focusableElements = mobileSheetContent.querySelectorAll(
				'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
			);
			if (focusableElements.length === 0) return;
			const firstElement = focusableElements[0] as HTMLElement;
			const lastElement = focusableElements[focusableElements.length - 1] as HTMLElement;

			if (event.shiftKey && document.activeElement === firstElement) {
				event.preventDefault();
				lastElement.focus();
			} else if (!event.shiftKey && document.activeElement === lastElement) {
				event.preventDefault();
				firstElement.focus();
			}
		}
	}

	// Skip to content handler
	function handleSkipToContent() {
		const mainContent = document.querySelector('main');
		if (mainContent) {
			mainContent.setAttribute('tabindex', '-1');
			mainContent.focus();
			mainContent.removeAttribute('tabindex');
		}
	}

	// Lifecycle hooks
	onMount(() => {
		window.addEventListener('keydown', handleKeyboard);
	});

	onDestroy(() => {
		window.removeEventListener('keydown', handleKeyboard);
	});
</script>

<nav class="pv-navbar">
	<!-- Skip to content link for keyboard users -->
	<a
		href="#main-content"
		class="pv-skip-link"
		class:pv-skip-link--focused={skipLinkFocused}
		onfocus={() => (skipLinkFocused = true)}
		onblur={() => (skipLinkFocused = false)}
		onclick={(e) => {
			e.preventDefault();
			handleSkipToContent();
		}}
	>
		Skip to content
	</a>

	<div class="pv-navbar-inner">
		<!-- Brand -->
		<a href="/" class="pv-navbar-brand">
			<div class="pv-navbar-brand-text">
				<span class="pv-navbar-brand-title">Proxmox</span>
				<span class="pv-navbar-brand-subtitle">VM Self Service</span>
			</div>
		</a>

		<!-- Loading skeleton -->
		{#if !auth.initialized}
			<div class="pv-navbar-skeleton hidden md:flex">
				<div class="pv-skeleton-link"></div>
				<div class="pv-skeleton-link"></div>
				<div class="pv-skeleton-link"></div>
			</div>
		{:else}
			<!-- Desktop navigation -->
			<div class="pv-navbar-links hidden md:flex">
				{#each navLinks as link, i (i)}
					{@const active = isActive((link as NavLink).href, $page.url.pathname)}
					{@const linkData = link as NavLink}
					<a
						href={linkData.href}
						class="pv-navbar-link {active ? 'pv-navbar-link--active' : ''}"
					>
						<linkData.icon weight={active ? 'fill' : 'regular'} class="h-4 w-4 flex-shrink-0" />
						{linkData.label}
					</a>
				{/each}
			</div>
		{/if}

		<div class="flex-1"></div>

		<!-- Right side -->
		<div class="flex items-center gap-1">
			<!-- Language selector -->
			<div class="hidden items-center sm:flex">
				<DropdownMenu.Root open={langDropdownOpen} onOpenChange={(open) => (langDropdownOpen = open)}>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<button
								class="pv-navbar-icon-btn"
								{...props}
								aria-label="Language"
								aria-haspopup="true"
								aria-expanded={langDropdownOpen}
							>
								<GlobeIcon class="h-4 w-4" />
							</button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end" class="w-36">
						<DropdownMenu.Item
							onclick={() => {
								setLocale('fr');
								langDropdownOpen = false;
							}}
						>
							🇫🇷 {$t('nav.languageFr')}
						</DropdownMenu.Item>
						<DropdownMenu.Item
							onclick={() => {
								setLocale('en');
								langDropdownOpen = false;
							}}
						>
							🇬🇧 {$t('nav.languageEn')}
						</DropdownMenu.Item>
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			</div>

			<!-- Theme toggle -->
			<ThemeToggle />

			<!-- Vertical divider -->
			<div class="pv-navbar-divider hidden sm:block"></div>

			<!-- Notifications -->
			{#if auth.initialized && auth.username}
				<DropdownMenu.Root open={notificationOpen} onOpenChange={(open) => (notificationOpen = open)}>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<button
								class="pv-navbar-icon-btn pv-navbar-icon-btn--relative"
								{...props}
								aria-label="Notifications"
								aria-haspopup="true"
								aria-expanded={notificationOpen}
							>
								<BellIcon class="h-4 w-4" />
								{#if notifications.unreadCount > 0}
									<Badge
										variant="destructive"
										class="pv-notification-badge"
									>
										{notifications.unreadCount}
									</Badge>
								{/if}
							</button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end" class="w-80">
						<DropdownMenu.Label>Notifications</DropdownMenu.Label>
						<DropdownMenu.Separator />
						{#if notifications.notifications.length === 0}
							<DropdownMenu.Item disabled>No notifications</DropdownMenu.Item>
						{:else}
							{#each notifications.notifications as notification}
								<div
									class="pv-notification-item {notification.read ? '' : 'pv-notification-item--unread'}"
									role="button"
									tabindex="0"
									onclick={() => notifications.markAsRead(notification.id)}
									onkeydown={(e) => {
										if (e.key === 'Enter' || e.key === ' ') {
											e.preventDefault();
											notifications.markAsRead(notification.id);
										}
									}}
								>
									<div class="pv-notification-item-content">
										<div class="pv-notification-item-title">{notification.title}</div>
										<div class="pv-notification-item-message">{notification.message}</div>
									</div>
								</div>
							{/each}
							<DropdownMenu.Separator />
							<DropdownMenu.Item onclick={() => notifications.markAllAsRead()}>
								Mark all as read
							</DropdownMenu.Item>
						{/if}
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			{/if}

			<!-- Sign in button (unauthenticated) -->
			{#if auth.initialized && !auth.username}
				<a href="/login" class="pv-navbar-signin-btn">
					<SignInIcon class="h-4 w-4" />
					<span class="hidden sm:inline">{$t('landing.signIn')}</span>
				</a>
			{:else if !auth.initialized}
				<!-- Loading skeleton for auth state -->
				<div class="pv-skeleton-avatar"></div>
			{/if}

			<!-- User dropdown (authenticated) -->
			{#if auth.initialized && auth.username}
				<DropdownMenu.Root open={userDropdownOpen} onOpenChange={(open) => (userDropdownOpen = open)}>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<button
								class="pv-navbar-user-btn"
								{...props}
								aria-label="User menu"
								aria-haspopup="true"
								aria-expanded={userDropdownOpen}
							>
								<div class="pv-navbar-avatar">
									{auth.username.slice(0, 1).toUpperCase()}
								</div>
								<span class="hidden sm:inline">{auth.username}</span>
								<CaretDownIcon class="h-3 w-3 opacity-50" />
							</button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end" class="w-52">
						<DropdownMenu.Label class="font-normal">
							<div class="flex flex-col space-y-0.5">
								<p class="text-sm font-semibold leading-none">{auth.username}</p>
								<p class="text-muted-foreground text-xs leading-none">{auth.isAdmin ? $t('nav.administrator') : $t('nav.user')}</p>
							</div>
						</DropdownMenu.Label>
						<DropdownMenu.Separator />
						{#if auth.isAdmin}
							<DropdownMenu.Item
								onclick={() => {
									navigate('/admin/');
									userDropdownOpen = false;
								}}
							>
								<GearSixIcon class="h-4 w-4" />
								{$t('common.admin')}
							</DropdownMenu.Item>
							<DropdownMenu.Separator />
						{/if}
						<DropdownMenu.Item
							class="text-destructive focus:text-destructive"
							onclick={handleLogout}
						>
							<SignOutIcon class="h-4 w-4" />
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
							<button
								class="pv-navbar-icon-btn"
								{...props}
								aria-label={$t('nav.menu')}
								aria-expanded={mobileOpen}
							>
								<ListIcon class="h-5 w-5" />
							</button>
						{/snippet}
					</Sheet.Trigger>
					<Sheet.Content side="left" class="w-72 p-0">
						<div bind:this={mobileSheetContent} class="flex flex-col h-full">
							<div class="flex items-center gap-3 border-b px-5 py-4">
								<div class="pv-navbar-logo">PV</div>
								<span class="text-base font-bold tracking-tight">PVMSS</span>
							</div>
							<div class="flex flex-col gap-0.5 p-3 flex-1 overflow-y-auto">
								{#each navLinks as link, i (i)}
									{@const active = isActive((link as NavLink).href, $page.url.pathname)}
									{@const linkData = link as NavLink}
									<a
										href={linkData.href}
										class="pv-sidebar-item {active ? 'pv-sidebar-item--active' : ''}"
										onclick={() => (mobileOpen = false)}
									>
										<span class="pv-sidebar-icon-wrap {active ? 'pv-sidebar-icon-wrap--active' : ''}">
											<linkData.icon weight={active ? 'fill' : 'regular'} class="h-4 w-4" />
										</span>
										{linkData.label}
									</a>
								{/each}

							<div class="border-border my-2 border-t"></div>

							<button
								class="pv-sidebar-item"
								onclick={() => {
									setLocale('fr');
									mobileOpen = false;
								}}
							>
								<span class="pv-sidebar-icon-wrap">🇫🇷</span>
								{$t('nav.languageFr')}
							</button>
							<button
								class="pv-sidebar-item"
								onclick={() => {
									setLocale('en');
									mobileOpen = false;
								}}
							>
								<span class="pv-sidebar-icon-wrap">🇬🇧</span>
								{$t('nav.languageEn')}
							</button>

							{#if auth.initialized && auth.username}
							<div class="border-border my-2 border-t"></div>

							{#if auth.isAdmin}
								<a
									href="/admin/"
									class="pv-sidebar-item"
									onclick={() => (mobileOpen = false)}
								>
									<span class="pv-sidebar-icon-wrap"><GearSixIcon class="h-4 w-4" /></span>
									{$t('common.admin')}
								</a>
							{/if}
							<button
								class="pv-sidebar-item text-destructive"
								onclick={handleLogout}
							>
								<span class="pv-sidebar-icon-wrap"><SignOutIcon class="h-4 w-4" /></span>
								{$t('common.logout')}
							</button>
						{:else if auth.initialized}
							<div class="border-border my-2 border-t"></div>

							<a
								href="/login"
								class="pv-sidebar-item"
								onclick={() => (mobileOpen = false)}
							>
								<span class="pv-sidebar-icon-wrap"><SignInIcon class="h-4 w-4" /></span>
								{$t('landing.signIn')}
							</a>
						{/if}
					</div>
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

	:global(.pv-navbar-brand-text) {
		display: flex;
		flex-direction: column;
		line-height: 1.1;
	}

	:global(.pv-navbar-brand-title) {
		font-size: 0.95rem;
		font-weight: 700;
		letter-spacing: -0.01em;
		color: var(--foreground);
	}

	:global(.pv-navbar-brand-subtitle) {
		font-size: 0.75rem;
		font-weight: 500;
		letter-spacing: 0.01em;
		color: var(--muted-foreground);
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

	:global(.pv-navbar-signin-btn) {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 12px;
		border-radius: 8px;
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--foreground);
		text-decoration: none;
		border: 1px solid var(--border);
		transition: background 0.12s, color 0.12s;
	}

	:global(.pv-navbar-signin-btn:hover) {
		background: var(--accent);
	}

	/* Skip to content link */
	:global(.pv-skip-link) {
		position: absolute;
		top: -100%;
		left: 50%;
		transform: translateX(-50%);
		background: var(--background);
		color: var(--foreground);
		padding: 8px 16px;
		border-radius: 4px;
		text-decoration: none;
		font-size: 0.875rem;
		font-weight: 500;
		z-index: 100;
		transition: top 0.2s;
	}

	:global(.pv-skip-link--focused) {
		top: 8px;
	}

	/* Loading skeleton */
	:global(.pv-navbar-skeleton) {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	:global(.pv-skeleton-link) {
		height: 32px;
		width: 80px;
		border-radius: 8px;
		background: var(--muted);
		animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
	}

	:global(.pv-skeleton-avatar) {
		width: 34px;
		height: 34px;
		border-radius: 8px;
		background: var(--muted);
		animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
	}

	@keyframes pulse {
		0%, 100% {
			opacity: 1;
		}
		50% {
			opacity: 0.5;
		}
	}

	/* Notification badge */
	:global(.pv-navbar-icon-btn--relative) {
		position: relative;
	}

	:global(.pv-notification-badge) {
		position: absolute;
		top: -2px;
		right: -2px;
		min-width: 16px;
		height: 16px;
		padding: 0 4px;
		font-size: 0.625rem;
		font-weight: 600;
		border-radius: 8px;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	/* Notification items */
	:global(.pv-notification-item) {
		padding: 8px 12px;
		cursor: pointer;
	}

	:global(.pv-notification-item--unread) {
		background: var(--accent);
	}

	:global(.pv-notification-item-content) {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	:global(.pv-notification-item-title) {
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--foreground);
	}

	:global(.pv-notification-item-message) {
		font-size: 0.75rem;
		color: var(--muted-foreground);
	}
</style>
