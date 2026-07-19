<script lang="ts">
	import { page } from '$app/stores';
	import { auth } from '$lib/stores/auth.svelte';
	import { t } from 'svelte-i18n';
	import { setLocale } from '$lib/i18n';
	import type { NavLink } from '$lib/types/navbar';
	import { GearSixIcon, SignInIcon, SignOutIcon } from 'phosphor-svelte';

	interface Props {
		navLinks: NavLink[];
		onClose: () => void;
		onLogout: () => Promise<void>;
	}

	let { navLinks, onClose, onLogout }: Props = $props();

	function isActive(href: string, pathname: string): boolean {
		if (href === '/') return pathname === '/';
		return pathname.startsWith(href);
	}

	function handleLogout() {
		onLogout();
		onClose();
	}
</script>

<div class="pv-mobile-menu">
	<div class="pv-mobile-menu-header">
		<div class="pv-navbar-logo">PV</div>
		<span class="pv-mobile-menu-title">PVMSS</span>
	</div>
	<div class="pv-mobile-menu-content">
		{#each navLinks as link, i (i)}
			{@const active = isActive(link.href, $page.url.pathname)}
			<a
				href={link.href}
				class="pv-sidebar-item {active ? 'pv-sidebar-item--active' : ''}"
				onclick={onClose}
			>
				<span class="pv-sidebar-icon-wrap {active ? 'pv-sidebar-icon-wrap--active' : ''}">
					<link.icon weight={active ? 'fill' : 'regular'} class="h-4 w-4" />
				</span>
				{link.label}
			</a>
		{/each}

		<div class="pv-sidebar-divider"></div>

		<button
			class="pv-sidebar-item"
			onclick={() => {
				setLocale('fr');
				onClose();
			}}
		>
			<span class="pv-sidebar-icon-wrap">🇫🇷</span>
			{$t('nav.languageFr')}
		</button>
		<button
			class="pv-sidebar-item"
			onclick={() => {
				setLocale('en');
				onClose();
			}}
		>
			<span class="pv-sidebar-icon-wrap">🇬🇧</span>
			{$t('nav.languageEn')}
		</button>

		{#if auth.initialized && auth.username}
			<div class="pv-sidebar-divider"></div>

			{#if auth.isAdmin}
				<a
					href="/admin/"
					class="pv-sidebar-item"
					onclick={onClose}
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
			<div class="pv-sidebar-divider"></div>

			<a
				href="/login"
				class="pv-sidebar-item"
				onclick={onClose}
			>
				<span class="pv-sidebar-icon-wrap"><SignInIcon class="h-4 w-4" /></span>
				{$t('landing.signIn')}
			</a>
		{/if}
	</div>
</div>

<style>
	.pv-mobile-menu {
		display: flex;
		flex-direction: column;
		height: 100%;
	}

	.pv-mobile-menu-header {
		display: flex;
		align-items: center;
		gap: 12px;
		border-bottom: 1px solid var(--border);
		padding: 16px 20px;
	}

	.pv-mobile-menu-title {
		font-size: 1rem;
		font-weight: 700;
		letter-spacing: -0.025em;
	}

	.pv-mobile-menu-content {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding: 12px;
		flex: 1;
		overflow-y: auto;
	}

	.pv-sidebar-item {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 10px 12px;
		border-radius: 8px;
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--foreground);
		text-decoration: none;
		transition: background 0.12s, color 0.12s;
		cursor: pointer;
		border: none;
		background: none;
		width: 100%;
		text-align: left;
	}

	.pv-sidebar-item:hover {
		background: var(--accent);
		color: var(--accent-foreground);
	}

	.pv-sidebar-item--active {
		color: hsl(var(--blaze-orange-600));
		background: hsl(var(--blaze-orange-50));
		font-weight: 600;
	}

	:global(.dark .pv-sidebar-item--active) {
		background: hsl(var(--blaze-orange-950) / 0.5);
		color: hsl(var(--blaze-orange-400));
	}

	.pv-sidebar-icon-wrap {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		border-radius: 6px;
		background: var(--muted);
		color: var(--muted-foreground);
		flex-shrink: 0;
	}

	.pv-sidebar-icon-wrap--active {
		background: hsl(var(--blaze-orange-600));
		color: white;
	}

	.pv-sidebar-divider {
		border-top: 1px solid var(--border);
		margin: 8px 0;
	}
</style>
