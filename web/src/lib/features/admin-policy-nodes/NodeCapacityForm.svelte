<script lang="ts">
	import type { NodeCapacity, NodeCapacityPatch } from './policyNodes.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Button from '$lib/shared/ui/Button.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';

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
<form class="mt-6 grid gap-4" onsubmit={(event) => { event.preventDefault(); onSave({ ...values }); }}>
	<FormField label={m['policy.maxVms']()} required>
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} type="number" min={0} bind:value={values.maxVms} required />
		{/snippet}
	</FormField>
	<FormField label={m['policy.maxVcpus']()} required>
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} type="number" min={0} bind:value={values.maxVcpus} required />
		{/snippet}
	</FormField>
	<FormField label={m['policy.maxRam']()} required>
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} type="number" min={0} bind:value={values.maxRamGb} required />
		{/snippet}
	</FormField>
	<FormField label={m['policy.maxNodeDisk']()} required>
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} type="number" min={0} bind:value={values.maxDiskGb} required />
		{/snippet}
	</FormField>
	<div class="flex justify-end gap-2 pt-2">
		<Button variant="secondary" onclick={onClose}>{m['policy.cancel']()}</Button>
		<Button type="submit" disabled={saving}>{saving ? m['policy.saving']() : m['policy.saveCapacity']()}</Button>
	</div>
</form>
