<script lang="ts">
	import { onMount } from 'svelte';
	import { setAdminCatalogContext } from '$lib/features/admin-catalog/admin-catalog.svelte';
	import StoragesTable from '$lib/features/admin-catalog/StoragesTable.svelte';

	const store = setAdminCatalogContext();

	onMount(() => {
		void store.loadAll();
	});
</script>

<svelte:head>
	<title>Admin — Storages — PVMSS</title>
</svelte:head>

<section class="mx-auto w-full max-w-5xl px-4 py-8">
	<h1 class="mb-6 text-2xl font-semibold tracking-tight">Storages</h1>

	{#if store.loading}
		<p role="status" aria-live="polite" class="text-muted-foreground">Loading…</p>
	{:else if store.error}
		<p role="alert" class="text-destructive">{store.error}</p>
	{:else}
		<div role="status" aria-live="polite" class="sr-only">{store.storages.length} storages loaded</div>

		{#if store.toggleError}
			<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">
				{store.toggleError}
			</p>
		{/if}

		<StoragesTable
			storages={store.storages}
			toggling={store.toggling}
			onToggle={(name, node, enabled) => void store.toggleStorage(name, node, enabled)}
		/>
	{/if}
</section>
