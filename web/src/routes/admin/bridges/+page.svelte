<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import {
		setAdminCatalogContext,
		type BridgeSortColumn
	} from '$lib/features/admin-catalog/admin-catalog.svelte';
	import BridgesTable from '$lib/features/admin-catalog/BridgesTable.svelte';
	import BridgesTableToolbar from '$lib/features/admin-catalog/BridgesTableToolbar.svelte';
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

	function handleToggle(node: string, name: string, enabled: boolean): void {
		void performToggle(node, name, enabled);
	}

	async function performToggle(node: string, name: string, enabled: boolean): Promise<void> {
		try {
			await store.toggleBridge(node, name, enabled);
			toast.success(
				enabled ? m['admin.bridges.enabledSuccess']({ name, node }) : m['admin.bridges.disabledSuccess']({ name, node })
			);
		} catch {
			toast.error(m['admin.catalog.toggleBridgeError']());
		}
	}

	function handleSort(column: BridgeSortColumn): void {
		store.setBridgeSort(column);
	}

	function handleRemove(node: string, name: string): void {
		void performRemove(node, name);
	}

	async function performRemove(node: string, name: string): Promise<void> {
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

{#if store.loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton columns={5} />
{:else if store.error}
	<p role="alert" class="text-destructive">{store.error}</p>
{:else}
	<div role="status" aria-live="polite" class="sr-only">
		{m['admin.bridges.bridgesLoaded']({ count: store.filteredBridges.length })}
	</div>

	{#if store.toggleError}
		<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">
			{store.toggleError}
		</p>
	{/if}

	{#if store.bridges.length === 0}
		<EmptyState
			title={m['admin.bridges.emptyTitle']()}
			description={m['admin.bridges.emptyDescription']()}
		>
			{#snippet actions()}
				<Button
					variant="secondary"
					size="sm"
					onclick={() => goto(resolve('/admin/clusters'))}
				>
					{m['admin.bridges.emptyAction']()}
				</Button>
			{/snippet}
		</EmptyState>
	{:else if store.filteredBridges.length === 0}
		<EmptyState
			title={m['admin.bridges.noMatchTitle']()}
			description={m['admin.bridges.noMatchDescription']()}
		>
			{#snippet actions()}
				<Button
					variant="secondary"
					size="sm"
					onclick={() => store.resetBridgeFilters()}
				>
					{m['admin.bridges.resetFilters']()}
				</Button>
			{/snippet}
		</EmptyState>
	{:else}
		<TableCard>
			{#snippet toolbar()}
				<BridgesTableToolbar {store} />
			{/snippet}
			<BridgesTable
				bridges={store.filteredBridges}
				toggling={store.toggling}
				onToggle={handleToggle}
				onRemove={handleRemove}
				sortBy={store.bridgeSortBy}
				sortDir={store.bridgeSortDir}
				onSort={handleSort}
			/>
		</TableCard>
	{/if}
{/if}
