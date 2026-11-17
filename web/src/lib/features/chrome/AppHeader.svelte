<script lang="ts">
	/**
	 * AppHeader - the mobile top bar, plus shell effects:
	 *  - below 900px, the bar that opens the sidebar drawer (menu + brand);
	 *  - from 900px, nothing: search and the bell live in the sidebar.
	 * The task-tray toast mirroring effect lives here because AppHeader is
	 * mounted on every viewport.
	 */
	import { untrack } from 'svelte';
	import { getTaskTrayContext } from '#lib/features/tasks/tasks.svelte.js';
	import { getToastContext } from '#lib/shared/ui/toast.svelte.js';
	import { getChromeContext } from './chrome.svelte';
	import MenuIcon from '#lib/shared/ui/icons/MenuIcon.svelte';
	import SearchIcon from '#lib/shared/ui/icons/SearchIcon.svelte';
	import Logo from '#lib/shared/ui/Logo.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { afterNavigate } from '$app/navigation';
	import { getSessionContext } from '#lib/features/auth/session.svelte.js';
	import { fetchMyQuotaRequest } from '#lib/features/quota-requests/quota-requests.js';
	import { NotificationsState, quotaNotification } from '#lib/features/notifications/notifications.svelte.js';
	import NotificationBell from '#lib/features/notifications/NotificationBell.svelte';

	const tray = getTaskTrayContext();
	const chrome = getChromeContext();
	const toast = getToastContext();
	const session = getSessionContext();

	// Pool users only: admins see pending work through the sidebar chips.
	const notifications = $derived(
		session.principal && !session.isAdmin ? new NotificationsState(session.principal.username) : null
	);
	async function checkQuotaDecision(): Promise<void> {
		if (notifications === null) return;
		try {
			notifications.add(quotaNotification(await fetchMyQuotaRequest()));
		} catch {
			// Best effort: the bell simply shows nothing new.
		}
	}
	// Cheap enough to re-ask on every navigation; it is how a decision reaches a user mid-session.
	afterNavigate(() => void checkQuotaDecision());
	// The first navigation can land before the session does.
	$effect(() => {
		if (notifications) void untrack(() => checkQuotaDecision());
	});
	$effect(() => {
		chrome.notifications = notifications;
	});

	let dismissTimer: ReturnType<typeof setTimeout> | null = null;
	$effect(() => {
		if (tray.toast !== null) {
			if (dismissTimer !== null) clearTimeout(dismissTimer);
			dismissTimer = setTimeout(() => tray.clearToast(), 5000);
			const settled = tray.toast;
			untrack(() => {
				toast.push({ variant: settled.kind, message: settled.message });
				notifications?.add({ id: crypto.randomUUID(), kind: settled.kind, message: settled.message, at: new Date().toISOString() });
			});
		}
	});
</script>

<header
	class="sticky top-0 z-30 flex h-14 items-center gap-2 border-b border-border bg-background px-5 min-[900px]:hidden"
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
	{#if notifications}
		<NotificationBell state={notifications} />
	{/if}
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

