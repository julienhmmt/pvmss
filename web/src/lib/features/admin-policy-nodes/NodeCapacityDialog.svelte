<script lang="ts">
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import NodeCapacityForm from './NodeCapacityForm.svelte';
	import type { NodeCapacity, NodeCapacityPatch } from './policyNodes.svelte';

	interface Props {
		node: NodeCapacity | null;
		open: boolean;
		saving: boolean;
		onClose: () => void;
		onSave: (patch: NodeCapacityPatch) => void;
	}

	let { node, open, saving, onClose, onSave }: Props = $props();
</script>

<Dialog {open} labelledBy="node-capacity-title" {onClose}>
	{#if node !== null}
		{#key `${node.node}:${node.maxVms}:${node.maxVcpus}:${node.maxRamGb}:${node.maxDiskGb}`}
			<NodeCapacityForm {node} {saving} {onClose} {onSave} />
		{/key}
	{/if}
</Dialog>
