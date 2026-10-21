<script lang="ts">
	import { getVmCreateContext } from '../create.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import FormField from '#lib/shared/ui/FormField.svelte';
	import Select from '#lib/shared/ui/Select.svelte';
	import Tooltip from '#lib/shared/ui/Tooltip.svelte';
	import Button from '#lib/shared/ui/Button.svelte';

	interface Props {
		showErrors?: boolean;
	}

	let { showErrors = false }: Props = $props();

	// Network step: one or more NICs (US2/D3a - multi-NIC). In simple mode
	// only nics[0] is used; detailed mode allows add/remove up to
	// gabarit.maxNetworkCards. Each NIC's bridge is filtered to the node.
	const form = getVmCreateContext();

	const bridgesOnNode = $derived((form.catalog?.bridges ?? []).filter((bridge) => bridge.node === form.node));
	const maxNetworkCards = $derived(form.catalog?.gabarit?.maxNetworkCards ?? 4);
	const canAddNIC = $derived(
		form.mode === 'detailed' && (maxNetworkCards === 0 || form.nics.length < maxNetworkCards)
	);
	const canRemoveNIC = $derived(form.nics.length > 1);

	const noBridgeOnNodeError = $derived(
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
						<Button variant="outline" size="sm" class="text-destructive" onclick={() => form.removeNIC(index)}>
							<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
								<path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6" />
							</svg>
							{m['vms.create.removeNic']()}
						</Button>
					{/if}
				</div>
			{/if}

			<FormField
				label={m['vms.create.bridge']()}
				required
				error={noBridgeOnNodeError ?? (showErrors && nic.bridge === '' ? m['vms.create.chooseBridge']() : null)}
			>
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
		<Button variant="secondary" size="sm" class="justify-self-start" onclick={() => form.addNIC()}>
			<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true">
				<line x1="12" y1="5" x2="12" y2="19" />
				<line x1="5" y1="12" x2="19" y2="12" />
			</svg>
			{m['vms.create.addNic']()}
		</Button>
	{/if}
</div>
