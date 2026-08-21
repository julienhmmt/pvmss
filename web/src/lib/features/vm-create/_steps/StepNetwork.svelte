<script lang="ts">
	import { getVmCreateContext } from '../create.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import Tooltip from '$lib/shared/ui/Tooltip.svelte';

	// Network step: one initial NIC (multi-NIC is T07) — an approved bridge
	// plus a device model.
	const form = getVmCreateContext();

	const bridgesOnNode = $derived((form.catalog?.bridges ?? []).filter((bridge) => bridge.node === form.node));
	const bridgeError = $derived(
		form.node !== '' && bridgesOnNode.length === 0 ? m['vms.create.noBridgeOnNode']({ node: form.node }) : null
	);
</script>

<div class="grid gap-4">
	{#if form.node !== '' && form.catalog}
		<p class="text-sm text-muted-foreground">
			{m['vms.create.selectedContext']({ node: form.node, cluster: form.clusterDisplayName() })}
		</p>
	{/if}

	<FormField label={m['vms.create.bridge']()} required error={bridgeError}>
		{#snippet children({ id, describedBy, invalid })}
			<Select
				{id}
				{describedBy}
				{invalid}
				bind:value={form.bridge}
				placeholder={m['vms.create.chooseBridge']()}
				options={bridgesOnNode.map((bridge) => ({
					value: bridge.name,
					label: m['vms.create.optionWithLocation']({ name: bridge.name, node: bridge.node, cluster: form.clusterDisplayName() })
				}))}
			/>
		{/snippet}
	</FormField>

	<FormField label={m['vms.create.model']()}>
		{#snippet children({ id, describedBy, invalid })}
			<div class="flex items-center gap-2">
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
				<Tooltip text={m['vms.create.networkModelTooltip']()}>
					<span
						class="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-semibold text-muted-foreground"
						aria-hidden="true"
					>
						?
					</span>
				</Tooltip>
			</div>
		{/snippet}
	</FormField>
</div>
