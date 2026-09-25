<script lang="ts">
	/**
	 * AppHeader - two render modes in one component:
	 *  - below 900px, the bar that opens the sidebar drawer (menu + brand);
	 *  - from 900px, the slim context header (DESIGN.md §5): "Workspace /
	 *    <current screen>" on the left, a private-workspace marker on the
	 *    right. It is a breadcrumb-style line, never a second <h1>.
	 * The task-tray toast mirroring effect lives here because AppHeader is
	 * mounted on every viewport.
	 */
	import { page } from '$app/state';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { contextLabel } from './context-label';
	import { untrack } from 'svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { getChromeContext } from './chrome.svelte';
	import MenuIcon from '$lib/shared/ui/icons/MenuIcon.svelte';
	import Logo from '$lib/shared/ui/Logo.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const tray = getTaskTrayContext();
	const chrome = getChromeContext();
	const toast = getToastContext();
	const session = getSessionContext();
	const context = $derived(contextLabel(page.url.pathname));

	let dismissTimer: ReturnType<typeof setTimeout> | null = null;
	$effect(() => {
		if (tray.toast !== null) {
			if (dismissTimer !== null) clearTimeout(dismissTimer);
			dismissTimer = setTimeout(() => tray.clearToast(), 5000);
			untrack(() => toast.push({ variant: tray.toast!.kind, message: tray.toast!.message }));
		}
	});
</script>

<header
	class="sticky top-0 z-30 flex h-14 items-center gap-2 border-b border-border bg-background/80 px-5 backdrop-blur-md min-[900px]:hidden"
	aria-label={m['chrome.header.ariaLabel']()}
>
	<button
		type="button"
		class="inline-flex h-9 w-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
		aria-label={m['chrome.sidebar.drawerOpen']()}
		aria-expanded={chrome.sidebarOpen}
		aria-controls="app-sidebar-drawer"
		onclick={() => chrome.openSidebar()}
		data-testid="sidebar-menu-button"
	>
		<MenuIcon />
	</button>

	<Logo class="text-foreground" />

	<div class="flex-1"></div>
</header>

<header
	class="sticky top-0 z-30 hidden h-[66px] items-center justify-between gap-4 border-b border-border/60 bg-background/80 px-11 backdrop-blur-md min-[900px]:flex"
	aria-label={m['chrome.context.ariaLabel']()}
	data-testid="context-header"
>
	<p class="flex min-w-0 items-center gap-2 text-sm">
		<span class="text-muted-foreground">{context.section}</span>
		{#if context.screen}
			<span class="text-muted-foreground-subtle" aria-hidden="true">/</span>
			<span class="truncate font-medium text-foreground" data-testid="context-screen">{context.screen}</span>
		{/if}
	</p>
	{#if !session.isAdmin}
		<span class="inline-flex items-center gap-1.5 rounded-full border border-border bg-card px-2.5 py-0.5 text-xs text-muted-foreground">
			<svg viewBox="0 0 24 24" class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
				<rect x="3" y="11" width="18" height="11" rx="2" />
				<path d="M7 11V7a5 5 0 0 1 10 0v4" />
			</svg>
			{m['chrome.context.privateWorkspace']()}
		</span>
	{/if}
</header>
