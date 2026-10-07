<script lang="ts">
	import { onMount } from 'svelte';
	import { setAdminCatalogContext } from '$lib/features/admin-catalog/admin-catalog.svelte';
	import CatalogFilesTable from '$lib/features/admin-catalog/CatalogFilesTable.svelte';
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
		void store.loadImages();
	});

	async function handleToggle(node: string, storage: string, file: string, enabled: boolean): Promise<void> {
		try {
			await store.toggleImage(node, storage, file, enabled);
			toast.success(
				enabled ? m['admin.images.enabledSuccess']({ file, node }) : m['admin.images.disabledSuccess']({ file, node })
			);
		} catch {
			toast.error(m['admin.images.toggleError']());
		}
	}

	async function handleRemove(node: string, storage: string, file: string): Promise<void> {
		try {
			await store.removeImage(node, storage, file);
			toast.success(m['admin.images.removeSuccess']({ file, node }));
		} catch {
			toast.error(m['admin.images.removeError']());
		}
	}
</script>

<svelte:head>
	<title>{m['admin.images.title']()}</title>
</svelte:head>

<PageHeader title={m['admin.images.heading']()}>
	{#snippet actions()}
		<ClusterSelector
			options={store.clusterOptions}
			value={store.cluster}
			onChange={(value) => store.setCluster(value)}
			id="images-cluster"
		/>
	{/snippet}
</PageHeader>

<CatalogListStates
	loading={store.loading}
	error={store.error}
	toggleError={store.toggleError}
	columns={5}
	totalCount={store.images.length}
	filteredCount={store.filteredImages.length}
	loadedLabel={m['admin.images.imagesLoaded']({ count: store.filteredImages.length })}
	emptyTitle={m['admin.images.emptyTitle']()}
	emptyDescription={m['admin.images.emptyDescription']()}
	emptyActionLabel={m['admin.images.emptyAction']()}
	noMatchTitle={m['admin.images.noMatchTitle']()}
	noMatchDescription={m['admin.images.noMatchDescription']()}
	resetLabel={m['admin.images.resetFilters']()}
	onResetFilters={() => store.resetImageFilters()}
>
	<TableCard>
		{#snippet toolbar()}
			<CatalogToolbar {store} kind="images" />
		{/snippet}
		<CatalogFilesTable
			kind="image"
			files={store.filteredImages}
			toggling={store.toggling}
			onToggle={(n, s, f, e) => void handleToggle(n, s, f, e)}
			onRemove={(n, s, f) => void handleRemove(n, s, f)}
			sortBy={store.imageSortBy}
			sortDir={store.imageSortDir}
			onSort={(column) => store.setImageSort(column)}
		/>
	</TableCard>
</CatalogListStates>
