<script lang="ts">
	import { onMount } from 'svelte';
	import { setAdminCatalogContext } from '$lib/features/admin-catalog/admin-catalog.svelte';
	import NodesTable from '$lib/features/admin-catalog/NodesTable.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';

	const store = setAdminCatalogContext();

	onMount(() => {
		void store.loadAll();
	});
</script>

<svelte:head>
	<title>Admin — Nodes — PVMSS</title>
</svelte:head>

<PageHeader title="Nodes">
	{#snippet actions()}
		<ClusterSelector options={store.clusterOptions} value={store.cluster} onChange={(value) => store.setCluster(value)} id="nodes-cluster" />
	{/snippet}
</PageHeader>

{#if store.loading}
	<div role="status" aria-live="polite" class="sr-only">Loading…</div>
	<TableSkeleton columns={6} />
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
