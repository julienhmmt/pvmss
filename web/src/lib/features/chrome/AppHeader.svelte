<script lang="ts">
	/**
	 * AppHeader — Layer B sticky translucent header. Docs link, logout,
	 * language switcher, theme toggle, and the status banner above. No search
	 * field (spec forbids it, research.md R6). A menu button opens the sidebar
	 * drawer below 900px.
	 *
	 * Task progress surfaces only through the global toast region: the effect
	 * below mirrors the task-tray toast into it (FR-019). The old Activity
	 * drawer was removed as unusable — toasts are the single notification
	 * channel.
	 */
	import { untrack } from 'svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { getChromeContext } from './chrome.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import LanguageSwitcher from './LanguageSwitcher.svelte';
	import ThemeToggle from './ThemeToggle.svelte';
	import StatusBanner from './StatusBanner.svelte';
	import MenuIcon from '$lib/shared/ui/icons/MenuIcon.svelte';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';

	const tray = getTaskTrayContext();
	const chrome = getChromeContext();
	const toast = getToastContext();

	let dismissTimer: ReturnType<typeof setTimeout> | null = null;
	$effect(() => {
		if (tray.toast !== null) {
			if (dismissTimer !== null) clearTimeout(dismissTimer);
			dismissTimer = setTimeout(() => tray.clearToast(), 5000);
			// Mirror the task-tray toast into the global toast region so it
			// surfaces as a notification (FR-019). untrack: push() reads
			// toast.items — without untrack that read gets captured as a
			// dependency of THIS effect, and the write inside push()
			// re-triggers it, looping until the region's own item cap
			// silently truncates the flood.
			untrack(() => toast.push({ variant: tray.toast!.kind, message: tray.toast!.message }));
		}
	});
</script>

<StatusBanner />

<header
	class="sticky top-0 z-30 flex h-14 items-center gap-2 border-b border-border bg-background/80 px-5 backdrop-blur-md"
	aria-label={m['chrome.header.ariaLabel']()}
>
	<button
		type="button"
		class="inline-flex h-9 w-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring lg:hidden"
		aria-label={m['chrome.sidebar.drawerOpen']()}
		aria-expanded={chrome.sidebarOpen}
		aria-controls="app-sidebar-drawer"
		onclick={() => chrome.openSidebar()}
		data-testid="sidebar-menu-button"
	>
		<MenuIcon />
	</button>

	<div class="flex-1"></div>

	<nav class="flex items-center gap-1.5" aria-label={m['nav.language']()}>
		<a
			href={resolve('/docs')}
			class="inline-flex h-9 items-center rounded-lg px-3 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
		>
			{m['chrome.header.docs']()}
		</a>

		<LanguageSwitcher />
		<ThemeToggle />
	</nav>
</header>
