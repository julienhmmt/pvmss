<script lang="ts">
	import type { Snippet } from 'svelte';
	import { onDestroy, onMount } from 'svelte';
	import '../app.css';
	import { setTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { setSessionContext } from '$lib/features/auth/session.svelte';
	import { get } from '$lib/shared/api/client';
	import { setLocaleContext } from '$lib/features/chrome/locale.svelte';
	import { setThemeContext } from '$lib/features/chrome/theme.svelte';
	import { setStatusContext } from '$lib/features/chrome/status.svelte';
	import { setChromeContext } from '$lib/features/chrome/chrome.svelte';
	import Sidebar from '$lib/features/chrome/Sidebar.svelte';
	import AppHeader from '$lib/features/chrome/AppHeader.svelte';
	import HeaderLite from '$lib/features/chrome/HeaderLite.svelte';
	import Toaster from '$lib/shared/ui/Toaster.svelte';
	import { setToastContext } from '$lib/shared/ui/toast.svelte';
	import { trapFocus } from '$lib/shared/ui/focus-trap';
	import { m } from '$lib/paraglide/messages.js';

	let { children }: { children: Snippet } = $props();

	const tray = setTaskTrayContext();
	onDestroy(() => tray.destroy());

	const session = setSessionContext();
	onMount(() => session.load());

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

	let version = $state<string | null>(null);
	onMount(async () => {
		try {
			const result = await get<{ version: string }>('/api/v1/public/version');
			version = result.version;
		} catch {
			// Version is informational — a failure leaves the footer silent.
		}
	});

	// T035: force the sidebar drawer closed when the viewport crosses 900px
	// upward so desktop cannot get stuck "closed" (data-model.md).
	let mql: MediaQueryList | null = null;
	function onViewportChange(event: MediaQueryListEvent): void {
		if (event.matches) chrome.closeSidebarOnDesktop();
	}
	onMount(() => {
		mql = window.matchMedia('(min-width: 900px)');
		mql.addEventListener('change', onViewportChange);
	});
	onDestroy(() => mql?.removeEventListener('change', onViewportChange));

	function closeSidebarOnEscape(event: KeyboardEvent): void {
		if (event.key === 'Escape' && chrome.sidebarOpen) {
			event.preventDefault();
			chrome.closeSidebar();
		}
	}

	const signedIn = $derived(session.principal !== null);
</script>

{#if signedIn}
	<div class="min-h-screen bg-background text-foreground">
		<div class="flex min-h-screen">
			<div class="sticky top-0 hidden h-screen min-[900px]:flex">
				<Sidebar {version} />
			</div>

			{#if chrome.sidebarOpen}
				<div
					class="fixed inset-0 z-40 bg-black/40 min-[900px]:hidden"
					role="presentation"
					onclick={() => chrome.closeSidebar()}
					onkeydown={closeSidebarOnEscape}
					data-testid="sidebar-backdrop"
				></div>
				<div
					id="app-sidebar-drawer"
					use:trapFocus
					tabindex="-1"
					role="dialog"
					aria-modal="true"
					aria-label={m['chrome.sidebar.ariaLabel']()}
					class="drawer-slide-in-left fixed left-0 top-0 z-50 h-full min-[900px]:hidden motion-reduce:transition-none"
					onkeydown={closeSidebarOnEscape}
				>
					<Sidebar {version} />
				</div>
			{/if}

			<div class="flex min-w-0 flex-1 flex-col">
				<AppHeader />
				<main class="flex-1 p-7">
					<div class="max-w-[1180px]">
						{@render children()}
					</div>
				</main>
				<footer class="border-t border-border px-7 py-3 text-xs text-muted-foreground-subtle">
					{#if version}PVMSS {version}{/if}
				</footer>
			</div>
		</div>
	</div>
{:else}
	<div class="flex min-h-screen flex-col bg-background text-foreground">
		<HeaderLite />
		<main class="flex flex-1 flex-col items-center justify-center p-6">
			{@render children()}
		</main>
		<footer class="border-t border-border py-3 text-center text-xs text-muted-foreground-subtle">
			{#if version}PVMSS {version}{/if}
		</footer>
	</div>
{/if}

<Toaster />
