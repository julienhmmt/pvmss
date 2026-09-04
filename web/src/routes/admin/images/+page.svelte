<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import {
		setAdminCatalogContext,
		type ImageSortColumn
	} from '$lib/features/admin-catalog/admin-catalog.svelte';
	import ImagesTable from '$lib/features/admin-catalog/ImagesTable.svelte';
	import ImagesTableToolbar from '$lib/features/admin-catalog/ImagesTableToolbar.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import TableCard from '$lib/shared/ui/TableCard.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = setAdminCatalogContext();
	const toast = getToastContext();

	onMount(() => {
		void store.loadImages();
	});

	function handleToggle(node: string, storage: string, file: string, enabled: boolean): void {
		void performToggle(node, storage, file, enabled);
	}

	async function performToggle(node: string, storage: string, file: string, enabled: boolean): Promise<void> {
		try {
			await store.toggleImage(node, storage, file, enabled);
			toast.success(
				enabled ? m['admin.images.enabledSuccess']({ file, node }) : m['admin.images.disabledSuccess']({ file, node })
			);
		} catch {
			toast.error(m['admin.images.toggleError']());
		}
	}

	function handleSort(column: ImageSortColumn): void {
		store.setImageSort(column);
	}

	function handleRemove(node: string, storage: string, file: string): void {
		void performRemove(node, storage, file);
	}

	async function performRemove(node: string, storage: string, file: string): Promise<void> {
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

{#if store.loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton columns={5} />
{:else if store.error}
	<p role="alert" class="text-destructive">{store.error}</p>
{:else}
	<div role="status" aria-live="polite" class="sr-only">
		{m['admin.images.imagesLoaded']({ count: store.filteredImages.length })}
	</div>

	{#if store.toggleError}
		<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">
			{store.toggleError}
		</p>
	{/if}

	{#if store.images.length === 0}
		<EmptyState
			title={m['admin.images.emptyTitle']()}
			description={m['admin.images.emptyDescription']()}
		>
			{#snippet actions()}
				<Button
					variant="secondary"
					size="sm"
					onclick={() => goto(resolve('/admin/clusters'))}
				>
					{m['admin.images.emptyAction']()}
				</Button>
			{/snippet}
		</EmptyState>
	{:else if store.filteredImages.length === 0}
		<EmptyState
			title={m['admin.images.noMatchTitle']()}
			description={m['admin.images.noMatchDescription']()}
		>
			{#snippet actions()}
				<Button
					variant="secondary"
					size="sm"
					onclick={() => store.resetImageFilters()}
				>
					{m['admin.images.resetFilters']()}
				</Button>
			{/snippet}
		</EmptyState>
	{:else}
		<TableCard>
			{#snippet toolbar()}
				<ImagesTableToolbar {store} />
			{/snippet}
			<ImagesTable
				images={store.filteredImages}
				toggling={store.toggling}
				onToggle={handleToggle}
				onRemove={handleRemove}
				sortBy={store.imageSortBy}
				sortDir={store.imageSortDir}
				onSort={handleSort}
			/>
		</TableCard>
	{/if}
{/if}
