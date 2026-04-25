<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { fade } from 'svelte/transition';

	let { children } = $props();
	let firstRender = $state(true);

	$effect(() => {
		if (auth.initialized && !auth.username) {
			window.location.href = '/';
		}
	});

	$effect(() => {
		if (auth.initialized && auth.username) {
			firstRender = false;
		}
	});
</script>

{#if auth.initialized && auth.username}
	{#if firstRender}
		<div transition:fade={{ duration: 150 }}>
			{@render children()}
		</div>
	{:else}
		{@render children()}
	{/if}
{/if}
