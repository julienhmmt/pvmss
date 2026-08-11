<script lang="ts">
	import { onMount } from 'svelte';
	import { setAdminCatalogContext } from '$lib/features/admin-catalog/admin-catalog.svelte';
	import NodesTable from '$lib/features/admin-catalog/NodesTable.svelte';

	const store = setAdminCatalogContext();

	onMount(() => {
		void store.loadAll();
	});
</script>

<svelte:head>
	<title>Admin — Nodes — PVMSS</title>
</svelte:head>

<section class="mx-auto w-full max-w-5xl px-4 py-8">
	<h1 class="mb-6 text-2xl font-semibold tracking-tight">Nodes</h1>

	{#if store.loading}
		<p role="status" aria-live="polite" class="text-muted-foreground">Loading…</p>
	{:else if store.error}
		<p role="alert" class="text-destructive">{store.error}</p>
	{:else}
		<div role="status" aria-live="polite" class="sr-only">{store.nodes.length} nodes loaded</div>

		{#if store.toggleError}
			<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">
				{store.toggleError}
			</p>
		{/if}

		<NodesTable
			nodes={store.nodes}
			toggling={store.toggling}
			onToggle={(name, enabled) => void store.toggleNode(name, enabled)}
		/>
	{/if}
</section>
