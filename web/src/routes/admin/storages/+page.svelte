<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { setAdminCatalogContext } from '$lib/features/admin-catalog/admin-catalog.svelte';
	import CatalogListStates from '$lib/features/admin-catalog/CatalogListStates.svelte';
	import CatalogToolbar from '$lib/features/admin-catalog/CatalogToolbar.svelte';
	import StoragesTable from '$lib/features/admin-catalog/StoragesTable.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import TableCard from '$lib/shared/ui/TableCard.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = setAdminCatalogContext();
	const toast = getToastContext();

	onMount(() => {
		// Deep links from dashboard alerts carry ?cluster= and ?search= hints;
		// assigning them before loadAll keeps the first (and only) fetch.
		const clusterHint = page.url.searchParams.get('cluster');
		const searchHint = page.url.searchParams.get('search');
		if (clusterHint) store.cluster = clusterHint;
		if (searchHint) store.storageSearch = searchHint;
		void store.loadAll();
	});

	async function handleToggle(name: string, node: string, enabled: boolean): Promise<void> {
		try {
			await store.toggleStorage(name, node, enabled);
			toast.success(
				enabled ? m['admin.storages.enabledSuccess']({ name, node }) : m['admin.storages.disabledSuccess']({ name, node })
			);
		} catch {
			toast.error(m['admin.catalog.toggleStorageError']());
		}
	}

	async function handleRemove(name: string, node: string): Promise<void> {
		try {
			await store.removeStorage(name, node);
			toast.success(m['admin.storages.removeSuccess']({ name, node }));
		} catch {
			toast.error(m['admin.catalog.removeStorageError']());
		}
	}
</script>

<svelte:head>
	<title>{m['admin.storages.title']()}</title>
</svelte:head>

<PageHeader title={m['admin.storages.heading']()}>
	{#snippet actions()}
		<ClusterSelector
			options={store.clusterOptions}
			value={store.cluster}
			onChange={(value) => store.setCluster(value)}
			id="storages-cluster"
		/>
	{/snippet}
</PageHeader>

<CatalogListStates
	loading={store.loading}
	error={store.error}
	toggleError={store.toggleError}
	columns={5}
	totalCount={store.storages.length}
	filteredCount={store.filteredStorages.length}
	loadedLabel={m['admin.storages.storagesLoaded']({ count: store.filteredStorageCount })}
	emptyTitle={m['admin.storages.emptyTitle']()}
	emptyDescription={m['admin.storages.emptyDescription']()}
	emptyActionLabel={m['admin.storages.emptyAction']()}
	noMatchTitle={m['admin.storages.noMatchTitle']()}
	noMatchDescription={m['admin.storages.noMatchDescription']()}
	resetLabel={m['admin.storages.resetFilters']()}
	onResetFilters={() => store.resetStorageFilters()}
>
	<TableCard>
		{#snippet toolbar()}
			<CatalogToolbar {store} kind="storages" />
		{/snippet}
		<StoragesTable
			storages={store.filteredStorages}
			toggling={store.toggling}
			onToggle={(name, node, e) => void handleToggle(name, node, e)}
			onRemove={(name, node) => void handleRemove(name, node)}
			sortBy={store.storageSortBy}
			sortDir={store.storageSortDir}
			onSort={(column) => store.setStorageSort(column)}
		/>
	</TableCard>
</CatalogListStates>
