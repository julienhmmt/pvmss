<script lang="ts">
	import { onMount } from 'svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { themeStore } from '$lib/stores/theme.svelte';
	import { Toaster } from '$lib/components/ui/sonner';
	import '../app.css';

	let { children } = $props();

	onMount(async () => {
		themeStore.init();
		await auth.exchange();
	});
</script>

{#if auth.initialized}
	{@render children()}
{:else}
	<div class="flex h-screen items-center justify-center">
		<p class="text-muted-foreground">Loading...</p>
	</div>
{/if}
<Toaster />
