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
	import { contextLabel } from './context-label';
	import { untrack } from 'svelte';
	import { getTaskTrayContext } from '#lib/features/tasks/tasks.svelte.js';
	import { getToastContext } from '#lib/shared/ui/toast.svelte.js';
	import { getChromeContext } from './chrome.svelte';
	import MenuIcon from '#lib/shared/ui/icons/MenuIcon.svelte';
	import SearchIcon from '#lib/shared/ui/icons/SearchIcon.svelte';
	import Logo from '#lib/shared/ui/Logo.svelte';
	import { m } from '#lib/paraglide/messages.js';

	const tray = getTaskTrayContext();
	const chrome = getChromeContext();
	const toast = getToastContext();
	const context = $derived(contextLabel(page.url.pathname));
	// The palette shortcut as the user's keyboard writes it.
	const paletteKeys = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.userAgent) ? '⌘K' : 'Ctrl K';

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
	<button
		type="button"
		class="inline-flex h-9 w-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
		aria-label={m['palette.open']()}
		onclick={() => chrome.openPalette()}
		data-testid="palette-open-mobile"
	>
		<SearchIcon />
	</button>
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
	<button
		type="button"
		class="inline-flex h-9 items-center gap-2 rounded-lg border border-border px-3 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
		onclick={() => chrome.openPalette()}
		data-testid="palette-open"
	>
		<SearchIcon class="h-4 w-4" />
		{m['palette.open']()}
		<kbd class="rounded border border-border bg-muted px-1.5 font-mono text-2xs">{paletteKeys}</kbd>
	</button>
</header>
