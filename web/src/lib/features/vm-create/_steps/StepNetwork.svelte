<script lang="ts">
	import { getVmCreateContext } from '../create.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';

	// Network step: one initial NIC (multi-NIC is T07) — an approved bridge
	// plus a device model.
	const form = getVmCreateContext();
</script>

<div class="grid gap-4">
	<FormField label={m['vms.create.bridge']()} required>
		{#snippet children({ id, describedBy, invalid })}
			<Select
				{id}
				{describedBy}
				{invalid}
				bind:value={form.bridge}
				placeholder={m['vms.create.chooseBridge']()}
				options={form.catalog?.bridges ?? []}
			/>
		{/snippet}
	</FormField>

	<FormField label={m['vms.create.model']()}>
		{#snippet children({ id, describedBy, invalid })}
			<Select
				{id}
				{describedBy}
				{invalid}
				bind:value={form.networkModel}
				options={[
					{ value: 'virtio', label: 'virtio' },
					{ value: 'e1000', label: 'e1000' },
					{ value: 'rtl8139', label: 'rtl8139' }
				]}
			/>
		{/snippet}
	</FormField>
</div>
