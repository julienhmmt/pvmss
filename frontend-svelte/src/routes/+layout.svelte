<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { auth } from '$lib/stores/auth.svelte';
	import { themeStore } from '$lib/stores/theme.svelte';
	import { Toaster } from '$lib/components/ui/sonner';
	import Navbar from '$lib/components/layout/Navbar.svelte';
	import { isLoading } from 'svelte-i18n';
	import '$lib/i18n';
	import '../app.css';

	let { children } = $props();

	onMount(async () => {
		themeStore.init();
		await auth.exchange();
	});
</script>

{#if !$isLoading && auth.initialized}
	{#if $page.url.pathname !== '/login' && $page.url.pathname !== '/console'}
		<Navbar />
		<div class="pt-14">
			{@render children()}
		</div>
	{:else}
		{@render children()}
	{/if}
{:else}
	<div class="flex h-screen items-center justify-center">
		<p class="text-muted-foreground">Loading...</p>
	</div>
{/if}
<Toaster />
