<script lang="ts">
	import NodeCapacityDialog from './NodeCapacityDialog.svelte';
	import type { NodeCapacity, NodeCapacityPatch } from './policyNodes.svelte';
	import { resolveAdminPolicyCopy } from '$lib/i18n/admin-policy';

	interface Props {
		nodes: NodeCapacity[];
		loading: boolean;
		error: string | null;
		saving: boolean;
		saveError: string | null;
		onLoad: () => void;
		onSave: (node: string, patch: NodeCapacityPatch) => void;
	}

	let { nodes, loading, error, saving, saveError, onLoad, onSave }: Props = $props();
	let selected = $state<NodeCapacity | null>(null);
	let dialogOpen = $state(false);
	const copy = resolveAdminPolicyCopy();

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
</script>

<svelte:head><title>{copy.nodeTitle} — PVMSS</title></svelte:head>

<section class="mx-auto w-full max-w-6xl px-4 py-8" aria-labelledby="node-policy-title">
	<div class="mb-8">
		<h1 id="node-policy-title" class="text-2xl font-semibold tracking-tight">{copy.nodeTitle}</h1>
		<p class="mt-2 max-w-2xl text-sm text-muted-foreground">{copy.nodeDescription}</p>
	</div>
	{#if loading}
		<p role="status" aria-live="polite" class="text-muted-foreground">{copy.loading}</p>
	{:else if error}
		<div class="space-y-3" role="alert"><p class="text-destructive">{error}</p><button type="button" class="rounded-md border px-3 py-2 text-sm" onclick={onLoad}>{copy.retry}</button></div>
	{:else}
		{#if saveError}<p role="alert" class="mb-4 text-sm text-destructive">{saveError}</p>{/if}
		<div class="overflow-x-auto rounded-lg border">
			<table class="w-full min-w-[760px] text-sm">
				<caption class="sr-only">{copy.nodeTitle}</caption>
				<thead class="bg-muted/50 text-left"><tr><th scope="col" class="px-4 py-3">{copy.node}</th><th scope="col" class="px-4 py-3">{copy.capacity}</th><th scope="col" class="px-4 py-3">{copy.usage}</th><th scope="col" class="px-4 py-3">{copy.physical}</th><th scope="col" class="px-4 py-3">{copy.actions}</th></tr></thead>
				<tbody>
					{#each nodes as node (node.node)}
						<tr class="border-t">
							<th scope="row" class="px-4 py-3 text-left font-mono">{node.node}</th>
							<td class="px-4 py-3">{node.maxVms} / {node.maxVcpus} / {node.maxRamGb} / {node.maxDiskGb}</td>
							<td class="px-4 py-3">{node.usedVms} VMs · {node.usedVcpus} vCPU · {node.usedRamGb} GB</td>
							<td class="px-4 py-3">{node.physicalVcpus} vCPU · {node.physicalRamGb} GB</td>
							<td class="px-4 py-3"><button type="button" class="rounded-md border px-3 py-2 text-sm hover:bg-muted" onclick={() => openEditor(node)}>{copy.edit}</button></td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>

<NodeCapacityDialog node={selected} open={dialogOpen} {saving} onClose={closeEditor} onSave={saveSelected} />
