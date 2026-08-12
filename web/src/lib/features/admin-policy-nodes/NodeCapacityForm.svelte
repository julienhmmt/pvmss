<script lang="ts">
	import type { NodeCapacity, NodeCapacityPatch } from './policyNodes.svelte';
	import { resolveAdminPolicyCopy } from '$lib/i18n/admin-policy';

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
	const copy = resolveAdminPolicyCopy();
</script>

<h2 id="node-capacity-title" class="text-lg font-semibold">{copy.edit}: {node.node}</h2>
<p class="mt-2 text-sm text-muted-foreground">{copy.noCap}</p>
<form class="mt-5 grid gap-4" onsubmit={(event) => { event.preventDefault(); onSave({ ...values }); }}>
	<label class="grid gap-1 text-sm" for="node-max-vms">{copy.maxVms}<input id="node-max-vms" type="number" min="0" bind:value={values.maxVms} required class="rounded-md border bg-background px-3 py-2" /></label>
	<label class="grid gap-1 text-sm" for="node-max-vcpus">{copy.maxVcpus}<input id="node-max-vcpus" type="number" min="0" bind:value={values.maxVcpus} required class="rounded-md border bg-background px-3 py-2" /></label>
	<label class="grid gap-1 text-sm" for="node-max-ram">{copy.maxRam}<input id="node-max-ram" type="number" min="0" bind:value={values.maxRamGb} required class="rounded-md border bg-background px-3 py-2" /></label>
	<label class="grid gap-1 text-sm" for="node-max-disk">{copy.maxNodeDisk}<input id="node-max-disk" type="number" min="0" bind:value={values.maxDiskGb} required class="rounded-md border bg-background px-3 py-2" /></label>
	<div class="flex justify-end gap-2 pt-2">
		<button type="button" class="rounded-md border px-4 py-2 text-sm" onclick={onClose}>{copy.cancel}</button>
		<button type="submit" class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground" disabled={saving}>{saving ? copy.saving : copy.saveCapacity}</button>
	</div>
</form>
