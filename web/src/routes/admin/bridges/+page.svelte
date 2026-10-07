<script lang="ts">
	import { onMount } from 'svelte';
	import { setAdminCatalogContext } from '$lib/features/admin-catalog/admin-catalog.svelte';
	import BridgesTable from '$lib/features/admin-catalog/BridgesTable.svelte';
	import CatalogListStates from '$lib/features/admin-catalog/CatalogListStates.svelte';
	import CatalogToolbar from '$lib/features/admin-catalog/CatalogToolbar.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import TableCard from '$lib/shared/ui/TableCard.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = setAdminCatalogContext();
	const toast = getToastContext();

	onMount(() => {
		void store.loadAll();
	});

	async function handleToggle(node: string, name: string, enabled: boolean): Promise<void> {
		try {
			await store.toggleBridge(node, name, enabled);
			toast.success(
				enabled ? m['admin.bridges.enabledSuccess']({ name, node }) : m['admin.bridges.disabledSuccess']({ name, node })
			);
		} catch {
			toast.error(m['admin.catalog.toggleBridgeError']());
		}
	}

	async function handleRemove(node: string, name: string): Promise<void> {
		try {
			await store.removeBridge(node, name);
			toast.success(m['admin.bridges.removeSuccess']({ name, node }));
		} catch {
			toast.error(m['admin.catalog.removeBridgeError']());
		}
	}
</script>

<svelte:head>
	<title>{m['admin.bridges.title']()}</title>
</svelte:head>

<PageHeader title={m['admin.bridges.heading']()}>
	{#snippet actions()}
		<ClusterSelector
			options={store.clusterOptions}
			value={store.cluster}
			onChange={(value) => store.setCluster(value)}
			id="bridges-cluster"
		/>
	{/snippet}
</PageHeader>

<CatalogListStates
	loading={store.loading}
	error={store.error}
	toggleError={store.toggleError}
	columns={5}
	totalCount={store.bridges.length}
	filteredCount={store.filteredBridges.length}
	loadedLabel={m['admin.bridges.bridgesLoaded']({ count: store.filteredBridges.length })}
	emptyTitle={m['admin.bridges.emptyTitle']()}
	emptyDescription={m['admin.bridges.emptyDescription']()}
	emptyActionLabel={m['admin.bridges.emptyAction']()}
	noMatchTitle={m['admin.bridges.noMatchTitle']()}
	noMatchDescription={m['admin.bridges.noMatchDescription']()}
	resetLabel={m['admin.bridges.resetFilters']()}
	onResetFilters={() => store.resetBridgeFilters()}
>
	<TableCard>
		{#snippet toolbar()}
			<CatalogToolbar {store} kind="bridges" />
		{/snippet}
		<BridgesTable
			bridges={store.filteredBridges}
			toggling={store.toggling}
			onToggle={(n, name, e) => void handleToggle(n, name, e)}
			onRemove={(n, name) => void handleRemove(n, name)}
			sortBy={store.bridgeSortBy}
			sortDir={store.bridgeSortDir}
			onSort={(column) => store.setBridgeSort(column)}
		/>
	</TableCard>
</CatalogListStates>
