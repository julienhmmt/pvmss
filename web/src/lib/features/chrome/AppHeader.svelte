<script lang="ts">
	/**
	 * AppHeader — minimal mobile-only bar. Holds the menu button and brand.
	 * Docs, language and theme controls have moved into the sidebar. The
	 * task-tray toast mirroring effect remains here because AppHeader is
	 * mounted on every viewport (hidden on desktop via CSS).
	 */
	import { untrack } from 'svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { getChromeContext } from './chrome.svelte';
	import MenuIcon from '$lib/shared/ui/icons/MenuIcon.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const tray = getTaskTrayContext();
	const chrome = getChromeContext();
	const toast = getToastContext();

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

	<span class="text-sm font-bold tracking-tight text-foreground">{m['chrome.sidebar.brand']()}</span>

	<div class="flex-1"></div>
</header>
