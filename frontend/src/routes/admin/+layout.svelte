<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import AppShell from '$lib/components/layout/AppShell.svelte';
	import AdminSidebar from '$lib/components/layout/AdminSidebar.svelte';
	import Footer from '$lib/components/layout/Footer.svelte';

	let { children } = $props();

	$effect(() => {
		if (auth.initialized && !auth.isAdmin) {
			goto('/');
		}
	});
</script>

{#if auth.isAdmin}
	<div class="flex flex-col min-h-[calc(100svh-3.5rem)]">
		<AppShell>
			{#snippet sidebar()}
				<AdminSidebar />
			{/snippet}
			{@render children()}
		</AppShell>
		<Footer />
	</div>
{/if}
