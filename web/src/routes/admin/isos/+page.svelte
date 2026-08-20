<script lang="ts">
	import { onMount } from 'svelte';
	import { setAdminCatalogContext } from '$lib/features/admin-catalog/admin-catalog.svelte';
	import IsosTable from '$lib/features/admin-catalog/IsosTable.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = setAdminCatalogContext();

	onMount(() => {
		void store.loadAll();
	});
</script>

<svelte:head>
	<title>{m['admin.isos.title']()}</title>
</svelte:head>

<PageHeader title={m['admin.isos.heading']()}>
	{#snippet actions()}
		<ClusterSelector options={store.clusterOptions} value={store.cluster} onChange={(value) => store.setCluster(value)} id="isos-cluster" />
	{/snippet}
</PageHeader>

{#if store.loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton columns={5} />
{:else if store.error}
	<p role="alert" class="text-destructive">{store.error}</p>
{:else}
	<div role="status" aria-live="polite" class="sr-only">{m['admin.isos.isosLoaded']({ count: store.isos.length })}</div>

	{#if store.toggleError}
		<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">
			{store.toggleError}
		</p>
	{/if}

	<div class="fade-in">
		<IsosTable
			isos={store.isos}
			toggling={store.toggling}
			onToggle={(node, storage, file, enabled) => void store.toggleISO(node, storage, file, enabled)}
		/>
	</div>
{/if}
