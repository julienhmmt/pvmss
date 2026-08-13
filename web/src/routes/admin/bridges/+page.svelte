<script lang="ts">
	import { onMount } from 'svelte';
	import { setAdminCatalogContext } from '$lib/features/admin-catalog/admin-catalog.svelte';
	import BridgesTable from '$lib/features/admin-catalog/BridgesTable.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';

	const store = setAdminCatalogContext();

	onMount(() => {
		void store.loadAll();
	});
</script>

<svelte:head>
	<title>Admin — Bridges — PVMSS</title>
</svelte:head>

<PageHeader title="Bridges">
	{#snippet actions()}
		<ClusterSelector options={store.clusterOptions} value={store.cluster} onChange={(value) => store.setCluster(value)} id="bridges-cluster" />
	{/snippet}
</PageHeader>

{#if store.loading}
	<p role="status" aria-live="polite" class="text-muted-foreground">Loading…</p>
{:else if store.error}
	<p role="alert" class="text-destructive">{store.error}</p>
{:else}
	<div role="status" aria-live="polite" class="sr-only">{store.bridges.length} bridges loaded</div>

	{#if store.toggleError}
		<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">
			{store.toggleError}
		</p>
	{/if}

	<BridgesTable
		bridges={store.bridges}
		toggling={store.toggling}
		onToggle={(name, enabled) => void store.toggleBridge(name, enabled)}
	/>
{/if}
