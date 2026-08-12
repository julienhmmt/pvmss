<script lang="ts">
	import type { Snippet } from 'svelte';
	import { onDestroy, onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import '../app.css';
	import { setTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { setSessionContext } from '$lib/features/auth/session.svelte';
	import { get } from '$lib/shared/api/client';
	import { setLocaleContext } from '$lib/features/chrome/locale.svelte';
	import { setThemeContext } from '$lib/features/chrome/theme.svelte';
	import { setStatusContext } from '$lib/features/chrome/status.svelte';
	import Navbar from '$lib/features/chrome/Navbar.svelte';
	import LanguageSwitcher from '$lib/features/chrome/LanguageSwitcher.svelte';
	import ThemeToggle from '$lib/features/chrome/ThemeToggle.svelte';
	import StatusBanner from '$lib/features/chrome/StatusBanner.svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();

	// The task tray is global (FR-015): one instance for the whole shell,
	// mounted in the navbar so task progress survives in-app navigation.
	const tray = setTaskTrayContext();
	onDestroy(() => tray.destroy());

	// The session context powers the admin nav link visibility (FR-008):
	// non-admins never see the link, so they don't click into a 403. The
	// server-side RequireAdmin middleware remains the real guard.
	const session = setSessionContext();
	onMount(() => session.load());

	// T19: locale and theme preferences are client-side state, instantiated
	// once here and provided via context (constitution VII — no module
	// singletons). init() reads localStorage and applies the active value
	// before the first interactive chrome renders.
	const locale = setLocaleContext();
	const theme = setThemeContext();
	const status = setStatusContext();
	onMount(() => {
		locale.init();
		theme.init();
		status.start();
	});
	onDestroy(() => status.stop());

	// T14: the public version string is shown in the footer for every visitor,
	// authenticated or not (X17/FR-015). Fetched once on mount from the
	// unauthenticated /api/v1/public/version endpoint.
	let version = $state<string | null>(null);
	onMount(async () => {
		try {
			const result = await get<{ version: string }>('/api/v1/public/version');
			version = result.version;
		} catch {
			// Version is informational — a failure leaves the footer silent.
		}
	});
</script>

<div class="flex min-h-screen flex-col bg-background text-foreground">
	<Navbar>
		{#snippet languageSwitcher()}
			<LanguageSwitcher />
		{/snippet}
		{#snippet themeToggle()}
			<ThemeToggle />
		{/snippet}
		{#snippet statusBanner()}
			<StatusBanner />
		{/snippet}
	</Navbar>
	<main class="flex flex-1 flex-col items-center justify-center">
		{@render children()}
	</main>
	<footer class="border-t border-border py-3 text-center text-xs text-muted-foreground">
		{#if version}PVMSS {version}{/if}
	</footer>
</div>
