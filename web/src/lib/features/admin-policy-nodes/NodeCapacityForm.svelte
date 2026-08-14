<script lang="ts">
	import type { NodeCapacity, NodeCapacityPatch } from './policyNodes.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Button from '$lib/shared/ui/Button.svelte';

	interface Props {
		node: NodeCapacity;
		saving: boolean;
		onClose: () => void;
		onSave: (patch: NodeCapacityPatch) => void;
	}

	let { node, saving, onClose, onSave }: Props = $props();
	// This form intentionally captures the server snapshot when the keyed form mounts.
	// svelte-ignore state_referenced_locally
	let values = $state<NodeCapacityPatch>({ maxVms: node.maxVms, maxVcpus: node.maxVcpus, maxRamGb: node.maxRamGb, maxDiskGb: node.maxDiskGb });
</script>

<h2 id="node-capacity-title" class="text-lg font-semibold">{m['policy.edit']()}: {node.node}</h2>
<p class="mt-2 text-sm text-muted-foreground">{m['policy.noCap']()}</p>
<form class="mt-5 grid gap-4" onsubmit={(event) => { event.preventDefault(); onSave({ ...values }); }}>
	<label class="grid gap-1 text-sm" for="node-max-vms">{m['policy.maxVms']()}<input id="node-max-vms" type="number" min="0" bind:value={values.maxVms} required class="rounded-md border bg-background px-3 py-2" /></label>
	<label class="grid gap-1 text-sm" for="node-max-vcpus">{m['policy.maxVcpus']()}<input id="node-max-vcpus" type="number" min="0" bind:value={values.maxVcpus} required class="rounded-md border bg-background px-3 py-2" /></label>
	<label class="grid gap-1 text-sm" for="node-max-ram">{m['policy.maxRam']()}<input id="node-max-ram" type="number" min="0" bind:value={values.maxRamGb} required class="rounded-md border bg-background px-3 py-2" /></label>
	<label class="grid gap-1 text-sm" for="node-max-disk">{m['policy.maxNodeDisk']()}<input id="node-max-disk" type="number" min="0" bind:value={values.maxDiskGb} required class="rounded-md border bg-background px-3 py-2" /></label>
	<div class="flex justify-end gap-2 pt-2">
		<Button variant="secondary" onclick={onClose}>{m['policy.cancel']()}</Button>
		<Button type="submit" disabled={saving}>{saving ? m['policy.saving']() : m['policy.saveCapacity']()}</Button>
	</div>
</form>
