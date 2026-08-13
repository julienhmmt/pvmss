<script lang="ts">
	import { onMount } from 'svelte';
	import { setAdminCatalogContext } from '$lib/features/admin-catalog/admin-catalog.svelte';
	import IsosTable from '$lib/features/admin-catalog/IsosTable.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';

	const store = setAdminCatalogContext();

	onMount(() => {
		void store.loadAll();
	});
</script>

<svelte:head>
	<title>Admin — ISOs — PVMSS</title>
</svelte:head>

<PageHeader title="ISOs">
	{#snippet actions()}
		<ClusterSelector options={store.clusterOptions} value={store.cluster} onChange={(value) => store.setCluster(value)} id="isos-cluster" />
	{/snippet}
</PageHeader>

{#if store.loading}
	<div role="status" aria-live="polite" class="sr-only">Loading…</div>
	<TableSkeleton columns={5} />
{:else if store.error}
	<p role="alert" class="text-destructive">{store.error}</p>
{:else}
	<div role="status" aria-live="polite" class="sr-only">{store.isos.length} ISOs loaded</div>

	{#if store.toggleError}
		<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">
			{store.toggleError}
		</p>
	{/if}

	<IsosTable
		isos={store.isos}
		toggling={store.toggling}
		onToggle={(storage, file, enabled) => void store.toggleISO(storage, file, enabled)}
	/>
{/if}
