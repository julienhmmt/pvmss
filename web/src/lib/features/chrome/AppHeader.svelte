<script lang="ts">
	/**
	 * AppHeader — Layer B sticky translucent header. Docs link, Activity button
	 * with a task-count badge + slide-over drawer, logout, language switcher,
	 * theme toggle, and the status banner above. No search field (spec forbids
	 * it, research.md R6). A menu button opens the sidebar drawer below 900px.
	 */
	import { untrack } from 'svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { getChromeContext } from './chrome.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import LanguageSwitcher from './LanguageSwitcher.svelte';
	import ThemeToggle from './ThemeToggle.svelte';
	import StatusBanner from './StatusBanner.svelte';
	import MenuIcon from '$lib/shared/ui/icons/MenuIcon.svelte';
	import CloseIcon from '$lib/shared/ui/icons/CloseIcon.svelte';
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
			// surfaces outside the activity drawer too (FR-019). untrack:
			// push() reads toast.items — without untrack that read gets
			// captured as a dependency of THIS effect, and the write inside
			// push() re-triggers it, looping until the region's own item cap
			// silently truncates the flood.
			untrack(() => toast.push({ variant: tray.toast!.kind, message: tray.toast!.message }));
		}
	});

	function onActivityKeydown(event: KeyboardEvent): void {
		if (event.key === 'Escape' && chrome.activityOpen) {
			event.preventDefault();
			chrome.closeActivity();
		}
	}
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

		<button
			type="button"
			class="relative inline-flex h-9 items-center rounded-lg px-3 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
			aria-label={tray.tasks.length > 0
				? m['chrome.header.activityWithCount']({ count: tray.tasks.length })
				: m['chrome.header.activity']()}
			aria-expanded={chrome.activityOpen}
			aria-controls="activity-drawer"
			onclick={() => chrome.toggleActivity()}
			data-testid="activity-button"
		>
			{m['chrome.header.activity']()}
			{#if tray.tasks.length > 0}
				<span
					class="absolute -right-1 -top-1 inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-semibold text-primary-foreground"
					aria-hidden="true"
				>
					{tray.tasks.length}
				</span>
			{/if}
		</button>

		<LanguageSwitcher />
		<ThemeToggle />
	</nav>
</header>

{#if chrome.activityOpen}
	<div
		class="fixed inset-0 z-40 bg-black/40"
		role="presentation"
		onclick={() => chrome.closeActivity()}
		onkeydown={onActivityKeydown}
		data-testid="activity-backdrop"
	></div>
	<div
		id="activity-drawer"
		class="drawer-slide-in fixed right-0 top-0 z-50 flex h-full w-80 max-w-[85vw] flex-col gap-4 border-l border-border bg-card p-5 text-card-foreground shadow-card motion-reduce:transition-none"
		role="dialog"
		aria-modal="true"
		aria-label={m['activity.ariaLabel']()}
		tabindex="-1"
		onkeydown={onActivityKeydown}
		data-testid="activity-drawer"
	>
		<div class="flex items-center justify-between">
			<h2 class="text-sm font-semibold">{m['activity.heading']()}</h2>
			<button
				type="button"
				class="inline-flex h-8 w-8 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
				aria-label={m['common.close']()}
				onclick={() => chrome.closeActivity()}
			>
				<CloseIcon />
			</button>
		</div>
		{#if tray.tasks.length === 0}
			<p class="text-sm text-muted-foreground">{m['activity.empty']()}</p>
		{:else}
			<ul class="flex flex-col gap-2 overflow-y-auto">
				{#each tray.tasks as task (task.upid)}
					<li class="rounded-lg border border-border-subtle p-3 text-sm">
						<p class="font-medium">{task.name}</p>
						<p class="font-mono text-xs text-muted-foreground">{task.upid}</p>
					</li>
				{/each}
			</ul>
		{/if}
		<div aria-live="polite" class="mt-auto">
			{#if tray.toast !== null}
				<div
					role="status"
					class="rounded-lg border px-3 py-2 text-sm {tray.toast.kind === 'success'
						? 'border-border bg-background text-foreground'
						: 'border-destructive bg-background text-destructive'}"
				>
					{tray.toast.message}
					<button type="button" class="ml-3 text-xs underline" onclick={() => tray.clearToast()}>
						{m['common.dismiss']()}
					</button>
				</div>
			{/if}
		</div>
	</div>
{/if}
