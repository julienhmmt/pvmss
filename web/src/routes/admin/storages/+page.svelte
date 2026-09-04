<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import {
		setAdminCatalogContext,
		type StorageSortColumn
	} from '$lib/features/admin-catalog/admin-catalog.svelte';
	import StoragesTable from '$lib/features/admin-catalog/StoragesTable.svelte';
	import StoragesTableToolbar from '$lib/features/admin-catalog/StoragesTableToolbar.svelte';
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
		void store.loadAll();
	});

	function handleToggle(name: string, node: string, enabled: boolean): void {
		void performToggle(name, node, enabled);
	}

	async function performToggle(name: string, node: string, enabled: boolean): Promise<void> {
		try {
			await store.toggleStorage(name, node, enabled);
			toast.success(
				enabled ? m['admin.storages.enabledSuccess']({ name, node }) : m['admin.storages.disabledSuccess']({ name, node })
			);
		} catch {
			toast.error(m['admin.catalog.toggleStorageError']());
		}
	}

	function handleSort(column: StorageSortColumn): void {
		store.setStorageSort(column);
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

{#if store.loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton columns={5} />
{:else if store.error}
	<p role="alert" class="text-destructive">{store.error}</p>
{:else}
	<div role="status" aria-live="polite" class="sr-only">
		{m['admin.storages.storagesLoaded']({ count: store.filteredStorageCount })}
	</div>

	{#if store.toggleError}
		<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">
			{store.toggleError}
		</p>
	{/if}

	{#if store.storages.length === 0}
		<EmptyState
			title={m['admin.storages.emptyTitle']()}
			description={m['admin.storages.emptyDescription']()}
		>
			{#snippet actions()}
				<Button
					variant="secondary"
					size="sm"
					onclick={() => goto(resolve('/admin/clusters'))}
				>
					{m['admin.storages.emptyAction']()}
				</Button>
			{/snippet}
		</EmptyState>
	{:else if store.filteredStorages.length === 0}
		<EmptyState
			title={m['admin.storages.noMatchTitle']()}
			description={m['admin.storages.noMatchDescription']()}
		>
			{#snippet actions()}
				<Button
					variant="secondary"
					size="sm"
					onclick={() => store.resetStorageFilters()}
				>
					{m['admin.storages.resetFilters']()}
				</Button>
			{/snippet}
		</EmptyState>
	{:else}
		<TableCard>
			{#snippet toolbar()}
				<StoragesTableToolbar {store} />
			{/snippet}
			<StoragesTable
				storages={store.filteredStorages}
				toggling={store.toggling}
				onToggle={handleToggle}
				sortBy={store.storageSortBy}
				sortDir={store.storageSortDir}
				onSort={handleSort}
			/>
		</TableCard>
	{/if}
{/if}
