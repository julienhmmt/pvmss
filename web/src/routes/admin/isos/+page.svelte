<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import {
		setAdminCatalogContext,
		type ISOSortColumn
	} from '$lib/features/admin-catalog/admin-catalog.svelte';
	import IsosTable from '$lib/features/admin-catalog/IsosTable.svelte';
	import IsosTableToolbar from '$lib/features/admin-catalog/IsosTableToolbar.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
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

	function handleToggle(node: string, storage: string, file: string, enabled: boolean): void {
		void performToggle(node, storage, file, enabled);
	}

	async function performToggle(node: string, storage: string, file: string, enabled: boolean): Promise<void> {
		try {
			await store.toggleISO(node, storage, file, enabled);
			toast.success(
				enabled ? m['admin.isos.enabledSuccess']({ file, node }) : m['admin.isos.disabledSuccess']({ file, node })
			);
		} catch {
			toast.error(m['admin.catalog.toggleIsoError']());
		}
	}

	function handleSort(column: ISOSortColumn): void {
		store.setISOSort(column);
	}

	function handleRemove(node: string, storage: string, file: string): void {
		void performRemove(node, storage, file);
	}

	async function performRemove(node: string, storage: string, file: string): Promise<void> {
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

<PageHeader title={m['admin.isos.heading']()}>
	{#snippet actions()}
		<ClusterSelector
			options={store.clusterOptions}
			value={store.cluster}
			onChange={(value) => store.setCluster(value)}
			id="isos-cluster"
		/>
	{/snippet}
</PageHeader>

{#if store.loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton columns={5} />
{:else if store.error}
	<Alert>{store.error}</Alert>
{:else}
	<div role="status" aria-live="polite" class="sr-only">
		{m['admin.isos.isosLoaded']({ count: store.filteredIsos.length })}
	</div>

	{#if store.toggleError}
		<Alert class="mb-4">{store.toggleError}</Alert>
	{/if}

	{#if store.isos.length === 0}
		<EmptyState
			title={m['admin.isos.emptyTitle']()}
			description={m['admin.isos.emptyDescription']()}
		>
			{#snippet actions()}
				<Button
					variant="secondary"
					size="sm"
					onclick={() => goto(resolve('/admin/clusters'))}
				>
					{m['admin.isos.emptyAction']()}
				</Button>
			{/snippet}
		</EmptyState>
	{:else if store.filteredIsos.length === 0}
		<EmptyState
			title={m['admin.isos.noMatchTitle']()}
			description={m['admin.isos.noMatchDescription']()}
		>
			{#snippet actions()}
				<Button
					variant="secondary"
					size="sm"
					onclick={() => store.resetISOFilters()}
				>
					{m['admin.isos.resetFilters']()}
				</Button>
			{/snippet}
		</EmptyState>
	{:else}
		<TableCard>
			{#snippet toolbar()}
				<IsosTableToolbar {store} />
			{/snippet}
			<IsosTable
				isos={store.filteredIsos}
				toggling={store.toggling}
				onToggle={handleToggle}
				onRemove={handleRemove}
				sortBy={store.isoSortBy}
				sortDir={store.isoSortDir}
				onSort={handleSort}
			/>
		</TableCard>
	{/if}
{/if}
