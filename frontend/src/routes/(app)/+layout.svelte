<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { fade } from 'svelte/transition';

	let { children } = $props();

	$effect(() => {
		if (auth.initialized && !auth.username) {
			window.location.href = '/';
		}
	});
</script>

{#if auth.initialized && auth.username}
	<!-- Single stable render branch: `in:fade` animates only on mount and keeps
	     `children` in one fragment. Branching on a `firstRender` flag (the old
	     approach) moved children between {#if}/{:else} fragments, which destroys
	     and recreates the page component — remounting it and firing onMount twice
	     (e.g. opening two VNC sessions for the console). -->
	<div in:fade={{ duration: 150 }}>
		{@render children()}
	</div>
{/if}
