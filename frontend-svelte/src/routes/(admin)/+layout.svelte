<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import AppShell from '$lib/components/layout/AppShell.svelte';
	import AdminSidebar from '$lib/components/layout/AdminSidebar.svelte';

	let { children } = $props();

	$effect(() => {
		if (auth.initialized && !auth.isAdmin) {
			window.location.href = '/login';
		}
	});
</script>

{#if auth.isAdmin}
	<AppShell>
		{#snippet sidebar()}
			<AdminSidebar />
		{/snippet}
		{@render children()}
	</AppShell>
{/if}
