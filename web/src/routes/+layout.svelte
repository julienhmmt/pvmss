<script lang="ts">
	import type { Snippet } from 'svelte';
	import { onDestroy, onMount } from 'svelte';
	import { page } from '$app/state';
	import { onNavigate } from '$app/navigation';
	import '../app.css';
	import { setTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { setSessionContext } from '$lib/features/auth/session.svelte';
	import { isPublicPath } from '$lib/features/auth/public-routes';
	import AuthRequired from '$lib/features/auth/AuthRequired.svelte';
	import { get } from '$lib/shared/api/client';
	import { setLocaleContext } from '$lib/features/chrome/locale.svelte';
	import { setThemeContext } from '$lib/features/chrome/theme.svelte';
	import { setStatusContext } from '$lib/features/chrome/status.svelte';
	import { setChromeContext } from '$lib/features/chrome/chrome.svelte';
	import Sidebar from '$lib/features/chrome/Sidebar.svelte';
	import CapabilitiesPanel from '$lib/features/chrome/CapabilitiesPanel.svelte';
	import AppHeader from '$lib/features/chrome/AppHeader.svelte';
	import HeaderLite from '$lib/features/chrome/HeaderLite.svelte';
	import StatusBanner from '$lib/features/chrome/StatusBanner.svelte';
	import ShortcutsDialog from '$lib/features/chrome/ShortcutsDialog.svelte';
	import Toaster from '$lib/shared/ui/Toaster.svelte';
	import { setToastContext } from '$lib/shared/ui/toast.svelte';
	import ClusterDownOverlay from '$lib/shared/ui/ClusterDownOverlay.svelte';

	import { m } from '$lib/paraglide/messages.js';

	let { children }: { children: Snippet } = $props();

	const tray = setTaskTrayContext();
	onDestroy(() => tray.destroy());

	const session = setSessionContext();
	let routeChecked = $state(false);
	onMount(async () => {
		await session.load();
		routeChecked = true;
	});

	const locale = setLocaleContext();
	const theme = setThemeContext();
	const status = setStatusContext();
	const chrome = setChromeContext();
	setToastContext();
	onMount(() => {
		locale.init();
		theme.init();
		status.start();
	});
	onDestroy(() => status.stop());

	if (typeof onNavigate === 'function' && typeof document !== 'undefined' && 'startViewTransition' in document) {
		onNavigate((navigation) =>
			new Promise((resolve) => {
				void document.startViewTransition(async () => {
					resolve();
					await navigation.complete;
				});
			})
		);
	}

	let version = $state<string | null>(null);
	const githubUrl = 'https://github.com/julienhmmt/pvmss';
	const websiteUrl = 'https://j.hommet.net/pvmss';
	onMount(async () => {
		try {
			const result = await get<{ version: string }>('/api/v1/public/version');
			version = result.version;
		} catch {
			// Version is informational — a failure leaves the footer silent.
		}
	});

	// Pages that can still function (or are not cluster-dependent) when all
	// clusters are unreachable. Everything else is covered by ClusterDownOverlay.
	const clusterIndependentPaths = new Set([
		'/',
		'/about',
		'/docs',
		'/login',
		'/profile/tokens',
		'/admin/clusters',
		'/admin/settings',
		'/admin/appinfo',
		'/admin/docs',
		'/admin/tags',
		'/admin/profiles'
	]);

	// Treat dynamic docs as cluster-independent; all other /admin/* pages need
	// at least one reachable cluster.
	function isClusterIndependent(path: string): boolean {
		if (clusterIndependentPaths.has(path)) return true;
		return path.startsWith('/docs/');
	}

	// T035: force the sidebar drawer closed when the viewport crosses 900px
	// upward so desktop cannot get stuck "closed" (data-model.md).
	let mql: MediaQueryList | null = null;
	function onViewportChange(event: MediaQueryListEvent): void {
		if (event.matches) chrome.closeSidebar();
	}
	onMount(() => {
		mql = window.matchMedia('(min-width: 900px)');
		mql.addEventListener('change', onViewportChange);
	});
	onDestroy(() => mql?.removeEventListener('change', onViewportChange));
	onMount(() => {
		window.addEventListener('keydown', handleGlobalShortcut);
		return () => window.removeEventListener('keydown', handleGlobalShortcut);
	});

	function handleGlobalShortcut(event: KeyboardEvent): void {
		if (event.ctrlKey || event.altKey || event.metaKey) return;
		const target = event.target as HTMLElement | null;
		if (target === null) return;
		const isTyping =
			target instanceof HTMLInputElement ||
			target instanceof HTMLTextAreaElement ||
			target instanceof HTMLSelectElement ||
			target.isContentEditable;
		if (isTyping) return;

		if (event.key === '/') {
			event.preventDefault();
			const searchInput = document.querySelector<HTMLInputElement>('input[type="search"], [data-search="true"]');
			searchInput?.focus();
		} else if (event.key === '?') {
			event.preventDefault();
			shortcutsOpen = true;
		} else if (event.key.toLowerCase() === 'r') {
			event.preventDefault();
			window.location.reload();
		}
	}

	const signedIn = $derived(session.principal !== null);
	const hasRouteError = $derived(page.error !== null);
	const isHomePage = $derived(page.route.id === '/');
	const isAboutPage = $derived(page.route.id === '/about');
	const isLoginPage = $derived(page.route.id === '/login');
	const showCapabilitiesPanel = $derived(signedIn && !isHomePage && !isAboutPage);
	let shortcutsOpen = $state(false);

	let sidebarDialog: HTMLDialogElement | null = $state(null);

	$effect(() => {
		if (chrome.sidebarOpen) {
			sidebarDialog?.showModal();
		} else {
			sidebarDialog?.close();
		}
	});
</script>

{#if signedIn}
	<div class="min-h-screen bg-background text-foreground">
		<a
			href="#main-content"
			class="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-[100] focus:rounded-md focus:bg-primary focus:px-4 focus:py-2 focus:text-sm focus:font-medium focus:text-primary-foreground"
			data-testid="skip-to-content"
		>
			{m['chrome.skipToContent']()}
		</a>
		<div class="flex min-h-screen">
			<div class="sticky top-0 hidden h-screen min-[900px]:flex">
				<Sidebar />
			</div>

			<dialog
				bind:this={sidebarDialog}
				id="app-sidebar-drawer"
				onclose={() => chrome.closeSidebar()}
				aria-label={m['chrome.sidebar.ariaLabel']()}
				class="drawer-slide-in-left fixed left-0 top-0 m-0 h-full w-[260px] max-w-full border-0 p-0 min-[900px]:hidden"
			>
				<Sidebar />
			</dialog>

			<div class="flex min-w-0 flex-1 flex-col">
				<AppHeader />
				<main id="main-content" class="flex-1 p-7">
					<div class="mx-auto max-w-[1180px]">
						{#if status.allClustersDown && !isClusterIndependent(page.url.pathname)}
							<ClusterDownOverlay>
								{@render children()}
							</ClusterDownOverlay>
						{:else}
							{@render children()}
						{/if}
					</div>
				</main>
				{#if showCapabilitiesPanel}
					<div class="px-7 pb-4">
						<div class="mx-auto max-w-[1180px]">
							<CapabilitiesPanel />
						</div>
					</div>
				{/if}
				<footer class="flex items-center justify-between gap-4 border-t border-border px-7 py-3 text-xs text-muted-foreground-subtle">
					{#if version}PVMSS {version}{/if}
					<div class="flex items-center gap-4">
						<a href={githubUrl} target="_blank" rel="noopener noreferrer" class="hover:text-foreground hover:underline">
							{m['chrome.footer.github']()}
						</a>
						<a href={websiteUrl} target="_blank" rel="noopener noreferrer" class="hover:text-foreground hover:underline">
							{m['chrome.footer.website']()}
						</a>
					</div>
				</footer>
			</div>
		</div>
	</div>
{:else}
	<div class="flex min-h-screen flex-col bg-background text-foreground">
		<a
			href="#main-content"
			class="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-[100] focus:rounded-md focus:bg-primary focus:px-4 focus:py-2 focus:text-sm focus:font-medium focus:text-primary-foreground"
			data-testid="skip-to-content"
		>
			{m['chrome.skipToContent']()}
		</a>
		<StatusBanner />
		{#if !isLoginPage}
			<HeaderLite />
		{/if}
		<main
			id="main-content"
			class="flex flex-1 flex-col {isLoginPage ? 'h-full' : 'items-center justify-center p-6'}"
		>
			{#if hasRouteError}
				{@render children()}
			{:else if !routeChecked && !isPublicPath(page.url.pathname)}
				<p role="status" aria-live="polite" class="text-muted-foreground">{m['common.loading']()}</p>
			{:else if !isPublicPath(page.url.pathname)}
				<AuthRequired />
			{:else}
				{@render children()}
			{/if}
		</main>
		{#if !isLoginPage}
			<footer class="border-t border-border py-3 text-center text-xs text-muted-foreground-subtle">
				{#if version}PVMSS {version}{/if}
				<div class="mt-1 flex items-center justify-center gap-4">
					<a href={githubUrl} target="_blank" rel="noopener noreferrer" class="hover:text-foreground hover:underline">
						{m['chrome.footer.github']()}
					</a>
					<a href={websiteUrl} target="_blank" rel="noopener noreferrer" class="hover:text-foreground hover:underline">
						{m['chrome.footer.website']()}
					</a>
				</div>
			</footer>
		{/if}
	</div>
{/if}

{#if signedIn}
	<ShortcutsDialog bind:open={shortcutsOpen} onClose={() => (shortcutsOpen = false)} />
{/if}

<Toaster />

<style>
	dialog::backdrop {
		background-color: rgba(0, 0, 0, 0.4);
	}
</style>
