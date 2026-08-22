<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import {
		setAdminCatalogContext,
		type AdminNode,
		type NodeSortColumn
	} from '$lib/features/admin-catalog/admin-catalog.svelte';
	import NodesTable from '$lib/features/admin-catalog/NodesTable.svelte';
	import NodeTableToolbar from '$lib/features/admin-catalog/NodeTableToolbar.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import ConfirmDialog from '$lib/shared/ui/ConfirmDialog.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = setAdminCatalogContext();
	const toast = getToastContext();

	onMount(() => {
		void store.loadAll();
	});

	let disableDialogOpen = $state(false);
	let pendingNode: AdminNode | null = $state(null);

	function handleToggle(name: string, enabled: boolean): void {
		const node = store.nodes.find((n) => n.name === name) ?? null;
		if (!node) return;

		if (!enabled && node.vmCount > 0) {
			pendingNode = node;
			disableDialogOpen = true;
			return;
		}

		void performToggle(name, enabled);
	}

	async function performToggle(name: string, enabled: boolean): Promise<void> {
		try {
			await store.toggleNode(name, enabled);
			toast.success(
				enabled ? m['admin.nodes.enabledSuccess']({ name }) : m['admin.nodes.disabledSuccess']({ name })
			);
		} catch {
			toast.error(m['admin.catalog.toggleNodeError']());
		} finally {
			disableDialogOpen = false;
			pendingNode = null;
		}
	}

	function handleDialogConfirm(): void {
		if (pendingNode) {
			void performToggle(pendingNode.name, false);
		}
	}

	function handleDialogClose(): void {
		disableDialogOpen = false;
		pendingNode = null;
	}

	function handleSort(column: NodeSortColumn): void {
		store.setNodeSort(column);
	}
</script>

<svelte:head>
	<title>{m['admin.nodes.title']()}</title>
</svelte:head>

<PageHeader title={m['admin.nodes.heading']()}>
	{#snippet actions()}
		<ClusterSelector
			options={store.clusterOptions}
			value={store.cluster}
			onChange={(value) => store.setCluster(value)}
			id="nodes-cluster"
		/>
	{/snippet}
</PageHeader>

{#if store.loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton columns={6} />
{:else if store.error}
	<p role="alert" class="text-destructive">{store.error}</p>
{:else}
	<div role="status" aria-live="polite" class="sr-only">
		{m['admin.nodes.nodesLoaded']({ count: store.filteredNodes.length })}
	</div>

	<NodeTableToolbar {store} />

	{#if store.toggleError}
		<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">
			{store.toggleError}
		</p>
	{/if}

	{#if store.nodes.length === 0}
		<EmptyState
			title={m['admin.nodes.emptyTitle']()}
			description={m['admin.nodes.emptyDescription']()}
		>
			{#snippet actions()}
				<Button
					variant="secondary"
					size="sm"
					onclick={() => goto(resolve('/admin/clusters'))}
				>
					{m['admin.nodes.emptyAction']()}
				</Button>
			{/snippet}
		</EmptyState>
	{:else if store.filteredNodes.length === 0}
		<EmptyState
			title={m['admin.nodes.noMatchTitle']()}
			description={m['admin.nodes.noMatchDescription']()}
		>
			{#snippet actions()}
				<Button
					variant="secondary"
					size="sm"
					onclick={() => store.resetNodeFilters()}
				>
					{m['admin.nodes.resetFilters']()}
				</Button>
			{/snippet}
		</EmptyState>
	{:else}
		<div class="fade-in">
			<NodesTable
				nodes={store.filteredNodes}
				toggling={store.toggling}
				sortBy={store.nodeSortBy}
				sortDir={store.nodeSortDir}
				onToggle={handleToggle}
				onSort={handleSort}
			/>
		</div>
	{/if}
{/if}

<ConfirmDialog
	open={disableDialogOpen}
	title={m['admin.nodes.disableTitle']({ name: pendingNode?.name ?? '' })}
	message={m['admin.nodes.disableMessage']({ count: pendingNode?.vmCount ?? 0 })}
	confirmLabel={m['admin.nodes.disableConfirm']()}
	cancelLabel={m['common.cancel']()}
	confirming={store.toggling === `node:${pendingNode?.name ?? ''}`}
	onConfirm={handleDialogConfirm}
	onClose={handleDialogClose}
/>
