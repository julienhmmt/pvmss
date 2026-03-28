<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';

	let { children } = $props();

	$effect(() => {
		if (auth.initialized && !auth.username) {
			window.location.href = '/login';
		}
	});
</script>

{#if auth.initialized && auth.username}
	{@render children()}
{/if}
