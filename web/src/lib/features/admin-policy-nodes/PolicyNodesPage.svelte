<script lang="ts">
	import NodeCapacityDialog from './NodeCapacityDialog.svelte';
	import type { NodeCapacity, NodeCapacityPatch } from './policyNodes.svelte';
	import type { ClusterOption } from '$lib/shared/clusters';
	import { m } from '$lib/paraglide/messages.js';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import TableCard from '$lib/shared/ui/TableCard.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import TableHeader from '$lib/shared/ui/TableHeader.svelte';

	type SortColumn = 'node' | 'maxVms' | 'maxVcpus' | 'maxRamGb' | 'maxDiskGb' | 'usedVms' | 'usedVcpus' | 'usedRamGb' | 'physicalVcpus' | 'physicalRamGb';

	interface Props {
		nodes: NodeCapacity[];
		loading: boolean;
		error: string | null;
		errorCode: string | null;
		saving: boolean;
		saveError: string | null;
		clusterOptions: ClusterOption[];
		cluster: string;
		onClusterChange: (value: string) => void;
		onLoad: () => void;
		onRetry: () => void;
		onSave: (node: string, patch: NodeCapacityPatch) => void;
		sortBy: SortColumn;
		sortDir: 'asc' | 'desc';
		onSort: (column: SortColumn) => void;
	}

	let { nodes, loading, error, errorCode, saving, saveError, clusterOptions, cluster, onClusterChange, onLoad, onRetry, onSave, sortBy, sortDir, onSort }: Props = $props();
	let selected = $state<NodeCapacity | null>(null);
	let dialogOpen = $state(false);

	function openEditor(node: NodeCapacity): void {
		selected = node;
		dialogOpen = true;
	}

	function closeEditor(): void {
		dialogOpen = false;
		selected = null;
	}

	function saveSelected(patch: NodeCapacityPatch): void {
		if (selected !== null) onSave(selected.node, patch);
	}

	function handleSort(column: string): void {
		onSort(column as SortColumn);
	}
</script>

<svelte:head><title>{m['policy.nodeTitle']()} — PVMSS</title></svelte:head>

<PageHeader title={m['policy.nodeTitle']()} description={m['policy.nodeDescription']()} titleId="node-policy-title">
	{#snippet actions()}
		<ClusterSelector options={clusterOptions} value={cluster} onChange={onClusterChange} id="policy-nodes-cluster" />
	{/snippet}
</PageHeader>

<section aria-labelledby="node-policy-title">
	{#if loading}
		<div role="status" aria-live="polite" class="sr-only">{m['policy.loading']()}</div>
		<TableSkeleton columns={5} />
	{:else if errorCode === 'inventory_not_ready'}
		<EmptyState
			title={m['policy.clusterUnreachableTitle']()}
			description={m['policy.clusterUnreachableDescription']()}
		>
			{#snippet actions()}
				<Button onclick={onRetry}>{m['policy.clusterUnreachableRetry']()}</Button>
			{/snippet}
		</EmptyState>
	{:else if error}
		<div class="space-y-3" role="alert"><p class="text-destructive">{error}</p><Button variant="secondary" onclick={onLoad}>{m['policy.retry']()}</Button></div>
	{:else}
		{#if saveError}<Alert class="mb-4">{saveError}</Alert>{/if}
		<TableCard>
			<table class="pv-table pv-responsive-table">
				<caption class="sr-only">{m['policy.nodeTitle']()}</caption>
				<thead>
					<tr>
						<TableHeader text={m['policy.node']()} column="node" activeColumn={sortBy} {sortDir} onSort={handleSort} />
						<TableHeader text={m['policy.capacity']()} column="maxVms" activeColumn={sortBy} {sortDir} onSort={handleSort} />
						<TableHeader text={m['policy.usage']()} column="usedVms" activeColumn={sortBy} {sortDir} onSort={handleSort} />
						<TableHeader text={m['policy.physical']()} column="physicalVcpus" activeColumn={sortBy} {sortDir} onSort={handleSort} />
						<th scope="col" class="font-medium">{m['policy.actions']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each nodes as node (node.node)}
						<tr class="group transition-colors hover:bg-muted/40">
							<th scope="row" class="text-left font-mono" data-label={m['policy.node']()}>{node.node}</th>
							<td data-label={m['policy.capacity']()}>{node.maxVms} / {node.maxVcpus} / {node.maxRamGb} / {node.maxDiskGb}</td>
							<td data-label={m['policy.usage']()}>{node.usedVms} VMs · {node.usedVcpus} vCPU · {node.usedRamGb} GB</td>
							<td data-label={m['policy.physical']()}>{node.physicalVcpus} vCPU · {node.physicalRamGb} GB</td>
							<td data-label={m['policy.actions']()}><Button variant="secondary" size="sm" label={`${m['policy.edit']()} ${node.node}`} onclick={() => openEditor(node)}>{m['policy.edit']()}</Button></td>
						</tr>
					{:else}
						<tr><td colspan={5} class="p-0">
							<EmptyState title={m['policy.noNodes']()} />
						</td></tr>
					{/each}
				</tbody>
			</table>
		</TableCard>
	{/if}
</section>

<NodeCapacityDialog node={selected} open={dialogOpen} {saving} onClose={closeEditor} onSave={saveSelected} />
