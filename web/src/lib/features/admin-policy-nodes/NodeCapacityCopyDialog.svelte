<script lang="ts">
	import { SvelteSet } from 'svelte/reactivity';
	import type { NodeCapacity, NodeCapacityExportSection, NodeCapacityImportOutcome } from './policyNodes.svelte';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Dialog from '#lib/shared/ui/Dialog.svelte';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		source: NodeCapacity | null;
		nodes: NodeCapacity[];
		saving: boolean;
		saveError: string | null;
		onImport: (sections: NodeCapacityExportSection[], overwrite: boolean) => Promise<NodeCapacityImportOutcome[] | null>;
		onClose: () => void;
	}

	let { source, nodes, saving, saveError, onImport, onClose }: Props = $props();

	let open = $state(false);
	let failures = $state.raw<NodeCapacityImportOutcome[]>([]);
	const targets = new SvelteSet<string>();

	$effect(() => {
		open = source !== null;
		targets.clear();
		failures = [];
	});

	const others = $derived(nodes.filter((node) => node.node !== source?.node));

	function toggle(node: string): void {
		if (targets.has(node)) targets.delete(node);
		else targets.add(node);
	}

	// Copy rides the import endpoint: one section for the cluster on screen,
	// overwrite on since the admin picked each target explicitly.
	async function submit(): Promise<void> {
		if (!source) return;
		const { maxVms, maxVcpus, maxRamGb, maxDiskGb } = source;
		const items = [...targets].map((node) => ({ node, maxVms, maxVcpus, maxRamGb, maxDiskGb }));
		const outcomes = await onImport([{ cluster: null, nodes: items }], true);
		if (outcomes === null) return;
		failures = outcomes.filter((o) => o.result !== 'updated' && o.result !== 'unchanged');
		if (failures.length === 0) onClose();
	}
</script>

<Dialog bind:open size="md" labelledBy="node-capacity-copy-title" {onClose}>
	<h2 id="node-capacity-copy-title" class="mb-4 text-lg font-medium">{m['policy.nodesCopyTitle']({ node: source?.node ?? '' })}</h2>
	<form onsubmit={(e) => { e.preventDefault(); void submit(); }} class="space-y-4">
		{#if saveError}<Alert>{saveError}</Alert>{/if}
		{#each failures as f (f.node)}<Alert>{f.node}: {f.error ?? f.result}</Alert>{/each}
		{#if source}
			<p class="text-sm text-muted-foreground">
				{source.maxVms} VM, {source.maxVcpus} vCPU, {source.maxRamGb} GB RAM, {source.maxDiskGb} GB
			</p>
		{/if}
		<fieldset>
			<legend class="mb-2 text-sm font-medium">{m['policy.nodesCopyTargets']({ count: targets.size })}</legend>
			<ul class="max-h-60 space-y-1 overflow-y-auto">
				{#each others as node (node.node)}
					<li>
						<label class="flex items-center gap-2 text-sm">
							<input type="checkbox" checked={targets.has(node.node)} onchange={() => toggle(node.node)} />
							<span class="font-mono">{node.node}</span>
						</label>
					</li>
				{/each}
			</ul>
		</fieldset>
		<div class="flex justify-end gap-2 pt-2">
			<Button variant="secondary" onclick={onClose}>{m['common.cancel']()}</Button>
			<Button type="submit" disabled={saving || targets.size === 0}>{saving ? m['common.saving']() : m['policy.nodesCopy']()}</Button>
		</div>
	</form>
</Dialog>
