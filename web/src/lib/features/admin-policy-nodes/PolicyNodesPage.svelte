<script lang="ts">
	import NodeCapacityDialog from './NodeCapacityDialog.svelte';
	import type { NodeCapacity, NodeCapacityPatch } from './policyNodes.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import SortableHeader from '$lib/shared/ui/SortableHeader.svelte';

	type SortColumn = 'node' | 'maxVms' | 'maxVcpus' | 'maxRamGb' | 'maxDiskGb' | 'usedVms' | 'usedVcpus' | 'usedRamGb' | 'physicalVcpus' | 'physicalRamGb';

	interface Props {
		nodes: NodeCapacity[];
		loading: boolean;
		error: string | null;
		saving: boolean;
		saveError: string | null;
		onLoad: () => void;
		onSave: (node: string, patch: NodeCapacityPatch) => void;
		sortBy: SortColumn;
		sortDir: 'asc' | 'desc';
		onSort: (column: SortColumn) => void;
	}

	let { nodes, loading, error, saving, saveError, onLoad, onSave, sortBy, sortDir, onSort }: Props = $props();
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

<PageHeader title={m['policy.nodeTitle']()} description={m['policy.nodeDescription']()} titleId="node-policy-title" />

<section aria-labelledby="node-policy-title">
	{#if loading}
		<div role="status" aria-live="polite" class="sr-only">{m['policy.loading']()}</div>
		<TableSkeleton columns={5} />
	{:else if error}
		<div class="space-y-3" role="alert"><p class="text-destructive">{error}</p><Button variant="secondary" onclick={onLoad}>{m['policy.retry']()}</Button></div>
	{:else}
		{#if saveError}<p role="alert" class="mb-4 text-sm text-destructive">{saveError}</p>{/if}
		<div class="overflow-x-auto rounded-lg border border-border">
			<table class="w-full min-w-[760px] text-sm">
				<caption class="sr-only">{m['policy.nodeTitle']()}</caption>
				<thead class="bg-muted/50 text-left">
					<tr>
						<SortableHeader text={m['policy.node']()} column="node" activeColumn={sortBy} {sortDir} onSort={handleSort} />
						<SortableHeader text={m['policy.capacity']()} column="maxVms" activeColumn={sortBy} {sortDir} onSort={handleSort} />
						<SortableHeader text={m['policy.usage']()} column="usedVms" activeColumn={sortBy} {sortDir} onSort={handleSort} />
						<SortableHeader text={m['policy.physical']()} column="physicalVcpus" activeColumn={sortBy} {sortDir} onSort={handleSort} />
						<th scope="col" class="px-4 py-3 font-medium">{m['policy.actions']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each nodes as node (node.node)}
						<tr class="border-t border-border">
							<th scope="row" class="px-4 py-3 text-left font-mono">{node.node}</th>
							<td class="px-4 py-3">{node.maxVms} / {node.maxVcpus} / {node.maxRamGb} / {node.maxDiskGb}</td>
							<td class="px-4 py-3">{node.usedVms} VMs · {node.usedVcpus} vCPU · {node.usedRamGb} GB</td>
							<td class="px-4 py-3">{node.physicalVcpus} vCPU · {node.physicalRamGb} GB</td>
							<td class="px-4 py-3"><Button variant="secondary" size="sm" label={`${m['policy.edit']()} ${node.node}`} onclick={() => openEditor(node)}>{m['policy.edit']()}</Button></td>
						</tr>
					{:else}
						<tr><td colspan={5} class="p-0">
							<EmptyState title={m['policy.noNodes']()} />
						</td></tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>

<NodeCapacityDialog node={selected} open={dialogOpen} {saving} onClose={closeEditor} onSave={saveSelected} />
