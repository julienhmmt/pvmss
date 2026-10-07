<script lang="ts">
	import { onMount } from 'svelte';
	import { setAdminCatalogContext, type AdminNode } from '$lib/features/admin-catalog/admin-catalog.svelte';
	import CatalogListStates from '$lib/features/admin-catalog/CatalogListStates.svelte';
	import CatalogToolbar from '$lib/features/admin-catalog/CatalogToolbar.svelte';
	import NodesTable from '$lib/features/admin-catalog/NodesTable.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import ConfirmDialog from '$lib/shared/ui/ConfirmDialog.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import TableCard from '$lib/shared/ui/TableCard.svelte';
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

	async function handleRemove(name: string): Promise<void> {
		try {
			await store.removeNode(name);
			toast.success(m['admin.nodes.removeSuccess']({ name }));
		} catch {
			toast.error(m['admin.catalog.removeNodeError']());
		}
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

<CatalogListStates
	loading={store.loading}
	error={store.error}
	toggleError={store.toggleError}
	columns={6}
	totalCount={store.nodes.length}
	filteredCount={store.filteredNodes.length}
	loadedLabel={m['admin.nodes.nodesLoaded']({ count: store.filteredNodes.length })}
	emptyTitle={m['admin.nodes.emptyTitle']()}
	emptyDescription={m['admin.nodes.emptyDescription']()}
	emptyActionLabel={m['admin.nodes.emptyAction']()}
	noMatchTitle={m['admin.nodes.noMatchTitle']()}
	noMatchDescription={m['admin.nodes.noMatchDescription']()}
	resetLabel={m['admin.nodes.resetFilters']()}
	onResetFilters={() => store.resetNodeFilters()}
>
	<TableCard>
		{#snippet toolbar()}
			<CatalogToolbar {store} kind="nodes" />
		{/snippet}
		<NodesTable
			clusterKey={store.cluster}
			nodes={store.filteredNodes}
			toggling={store.toggling}
			sortBy={store.nodeSortBy}
			sortDir={store.nodeSortDir}
			onToggle={(name, e) => handleToggle(name, e)}
			onRemove={(name) => void handleRemove(name)}
			onSort={(column) => store.setNodeSort(column)}
		/>
	</TableCard>
</CatalogListStates>

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
