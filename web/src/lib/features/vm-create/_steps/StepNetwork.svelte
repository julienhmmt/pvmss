<script lang="ts">
	import { getVmCreateContext } from '../create.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import Tooltip from '$lib/shared/ui/Tooltip.svelte';

	// Network step: one or more NICs (US2/D3a — multi-NIC). In simple mode
	// only nics[0] is used; detailed mode allows add/remove up to
	// gabarit.maxNetworkCards. Each NIC's bridge is filtered to the node.
	const form = getVmCreateContext();

	const bridgesOnNode = $derived((form.catalog?.bridges ?? []).filter((bridge) => bridge.node === form.node));
	const maxNetworkCards = $derived(form.catalog?.gabarit?.maxNetworkCards ?? 4);
	const canAddNIC = $derived(form.mode === 'detailed' && form.nics.length < maxNetworkCards);
	const canRemoveNIC = $derived(form.nics.length > 1);

	const bridgeError = $derived(
		form.node !== '' && bridgesOnNode.length === 0 ? m['vms.create.noBridgeOnNode']({ node: form.node }) : null
	);

	const networkModels = [
		{ value: 'virtio', label: 'virtio' },
		{ value: 'e1000', label: 'e1000' },
		{ value: 'rtl8139', label: 'rtl8139' }
	];
</script>

<div class="grid gap-4">
	{#if form.node !== '' && form.catalog}
		<p class="text-sm text-muted-foreground">
			{m['vms.create.selectedContext']({ node: form.node, cluster: form.clusterDisplayName() })}
		</p>
	{/if}

	{#each form.nics as nic, index (index)}
		<div class="grid gap-4 rounded-lg border border-border p-4">
			{#if form.mode === 'detailed'}
				<div class="flex items-center justify-between">
					<span class="text-sm font-medium text-foreground">{m['vms.create.nicLabel']({ index })}</span>
					{#if canRemoveNIC}
						<button
							type="button"
							class="text-sm text-destructive hover:underline"
							onclick={() => form.removeNIC(index)}
						>
							{m['vms.create.removeNic']()}
						</button>
					{/if}
				</div>
			{/if}

			<FormField label={m['vms.create.bridge']()} required error={bridgeError}>
				{#snippet children({ id, describedBy, invalid })}
					<Select
						{id}
						{describedBy}
						{invalid}
						bind:value={nic.bridge}
						placeholder={m['vms.create.chooseBridge']()}
						options={bridgesOnNode.map((bridge) => ({
							value: bridge.name,
							label: bridge.comment
								? m['vms.create.optionWithLocationAndComment']({
										name: bridge.name,
										node: bridge.node,
										cluster: form.clusterDisplayName(),
										comment: bridge.comment
									})
								: m['vms.create.optionWithLocation']({ name: bridge.name, node: bridge.node, cluster: form.clusterDisplayName() })
						}))}
					/>
				{/snippet}
			</FormField>

			<FormField label={m['vms.create.model']()}>
				{#snippet children({ id, describedBy, invalid })}
					<div class="flex items-center gap-2">
						<Select {id} {describedBy} {invalid} bind:value={nic.model} options={networkModels} />
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
	{/each}

	{#if canAddNIC}
		<button type="button" class="justify-self-start text-sm text-primary hover:underline" onclick={() => form.addNIC()}>
			{m['vms.create.addNic']()}
		</button>
	{/if}
</div>
