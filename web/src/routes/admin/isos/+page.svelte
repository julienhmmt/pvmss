<script lang="ts">
	import { onMount } from 'svelte';
	import { setAdminCatalogContext } from '#lib/features/admin-catalog/admin-catalog.svelte.js';
	import CatalogFilesTable from '#lib/features/admin-catalog/CatalogFilesTable.svelte';
	import CatalogListStates from '#lib/features/admin-catalog/CatalogListStates.svelte';
	import CatalogToolbar from '#lib/features/admin-catalog/CatalogToolbar.svelte';
	import ClusterSelector from '#lib/shared/ui/ClusterSelector.svelte';
	import PageHeader from '#lib/shared/ui/PageHeader.svelte';
	import Pagination from '#lib/shared/ui/Pagination.svelte';
	import { loadPageSize, paginate } from '#lib/shared/ui/paginate.js';
	import TableCard from '#lib/shared/ui/TableCard.svelte';
	import { getToastContext } from '#lib/shared/ui/toast.svelte.js';
	import { m } from '#lib/paraglide/messages.js';

	const store = setAdminCatalogContext();
	const toast = getToastContext();
	let page = $state(1);
	let size = $state(loadPageSize('isos'));
	const visible = $derived(paginate(store.filteredIsos, page, size));

	onMount(() => {
		void store.loadAll();
	});

	async function handleToggle(node: string, storage: string, file: string, enabled: boolean): Promise<void> {
		try {
			await store.toggleISO(node, storage, file, enabled);
			toast.success(
				enabled ? m['admin.isos.enabledSuccess']({ file, node }) : m['admin.isos.disabledSuccess']({ file, node })
			);
		} catch {
			toast.error(m['admin.catalog.toggleIsoError']());
		}
	}

	async function handleRemove(node: string, storage: string, file: string): Promise<void> {
		try {
			await store.removeISO(node, storage, file);
			toast.success(m['admin.isos.removeSuccess']({ file, node }));
		} catch {
			toast.error(m['admin.catalog.removeIsoError']());
		}
	}
</script>

<svelte:head>
	<title>{m['admin.isos.title']()}</title>
</svelte:head>

<PageHeader title={m['admin.isos.heading']()} eyebrow={m['chrome.adminNav.resources']()}>
	{#snippet actions()}
		<ClusterSelector
			options={store.clusterOptions}
			value={store.cluster}
			onChange={(value) => store.setCluster(value)}
			id="isos-cluster"
		/>
	{/snippet}
</PageHeader>

<CatalogListStates
	loading={store.loading}
	error={store.error}
	toggleError={store.toggleError}
	columns={5}
	totalCount={store.isos.length}
	filteredCount={store.filteredIsos.length}
	loadedLabel={m['admin.isos.isosLoaded']({ count: store.filteredIsos.length })}
	emptyTitle={m['admin.isos.emptyTitle']()}
	emptyDescription={m['admin.isos.emptyDescription']()}
	emptyActionLabel={m['admin.isos.emptyAction']()}
	noMatchTitle={m['admin.isos.noMatchTitle']()}
	noMatchDescription={m['admin.isos.noMatchDescription']()}
	resetLabel={m['admin.isos.resetFilters']()}
	onResetFilters={() => store.resetISOFilters()}
>
	<TableCard>
		{#snippet toolbar()}
			<CatalogToolbar {store} kind="isos" />
		{/snippet}
		<CatalogFilesTable
			kind="iso"
			files={visible}
			toggling={store.toggling}
			onToggle={(n, s, f, e) => void handleToggle(n, s, f, e)}
			onRemove={(n, s, f) => void handleRemove(n, s, f)}
			sortBy={store.isoSortBy}
			sortDir={store.isoSortDir}
			onSort={(column) => store.setISOSort(column)}
		/>
		{#snippet footer()}
			<Pagination total={store.filteredIsos.length} bind:page bind:size name="isos" />
		{/snippet}
	</TableCard>
</CatalogListStates>
