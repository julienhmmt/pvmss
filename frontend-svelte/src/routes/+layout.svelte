<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { auth } from '$lib/stores/auth.svelte';
	import { themeStore } from '$lib/stores/theme.svelte';
	import { Toaster } from '$lib/components/ui/sonner';
	import Navbar from '$lib/components/layout/Navbar.svelte';
	import '$lib/i18n';
	import '../app.css';

	let { children } = $props();

	onMount(() => {
		themeStore.init();
		// Run auth in background, don't wait
		auth.exchange().catch(err => {
			console.error('Auth exchange failed:', err);
		});
	});
</script>

{#if $page.url.pathname !== '/login' && $page.url.pathname !== '/console'}
	<Navbar />
	<div class="pt-14">
		{@render children()}
	</div>
{:else}
	{@render children()}
{/if}
<Toaster />
