<script lang="ts">
	import { getVmDetailContext, type VmNetworkInterface } from '../detail.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getVmDetailContext();

	interface Props {
		open?: boolean;
		iface: VmNetworkInterface | null;
	}

	let { open = $bindable(false), iface }: Props = $props();

	let bridge = $state('');
	let model = $state('');
	let vlan = $state('');
	let rateMbps = $state('');

	$effect(() => {
		if (iface) {
			bridge = iface.bridge;
			model = iface.model;
			vlan = iface.vlan === null ? '' : String(iface.vlan);
			rateMbps = iface.rateMbps === null ? '' : String(iface.rateMbps);
		}
	});

	function close(): void {
		open = false;
	}

	async function submit(): Promise<void> {
		if (!iface || !store.entity?.networkInterfaces || !bridge) return;
		const interfaces = store.entity.networkInterfaces.map((nic) =>
			nic.index === iface.index
				? {
						index: nic.index,
						bridge,
						model,
						vlan: vlan === '' ? null : Number(vlan),
						rateMbps: rateMbps === '' ? null : Number(rateMbps)
					}
				: { index: nic.index, bridge: nic.bridge, model: nic.model, vlan: nic.vlan, rateMbps: nic.rateMbps }
		);
		await store.updateNetwork(interfaces);
		if (!store.writeError) close();
	}
</script>

<Dialog bind:open labelledBy="edit-nic-title" onClose={close}>
	<h2 id="edit-nic-title" class="mb-4 text-lg font-semibold">{m['vms.network.editTitle']({ index: iface?.index ?? 0 })}</h2>
	<form
		class="grid gap-3"
		onsubmit={(event) => {
			event.preventDefault();
			void submit();
		}}
	>
		<FormField label={m['vms.network.bridge']()} required error={store.writeError}>
			{#snippet children({ id, describedBy, invalid })}
				<Select
					{id}
					{describedBy}
					{invalid}
					bind:value={bridge}
					placeholder={m['vms.network.selectBridge']()}
					options={(store.hardwareOptions?.bridges ?? []).map((option) => ({
						value: option.bridge,
						label: `${option.bridge} · ${option.node}`
					}))}
					required
					data-testid="edit-nic-bridge"
				/>
			{/snippet}
		</FormField>
		<FormField label={m['vms.network.model']()} required>
			{#snippet children({ id, describedBy, invalid })}
				<Select
					{id}
					{describedBy}
					{invalid}
					bind:value={model}
					options={[
						{ value: 'virtio', label: 'VirtIO' },
						{ value: 'e1000', label: 'E1000' },
						{ value: 'rtl8139', label: 'RTL8139' },
						{ value: 'vmxnet3', label: 'VMXNET3' }
					]}
					required
				/>
			{/snippet}
		</FormField>
		<FormField label={m['vms.network.vlan']()} hint={m['common.optional']()}>
			{#snippet children({ id, describedBy, invalid })}
				<TextField
					{id}
					{describedBy}
					{invalid}
					type="number"
					min={1}
					max={4094}
					value={vlan}
					oninput={(e: Event & { currentTarget: HTMLInputElement }) => (vlan = e.currentTarget.value)}
					data-testid="edit-nic-vlan"
				/>
			{/snippet}
		</FormField>
		<FormField label={m['vms.network.rateLimit']()} hint={m['common.optional']()}>
			{#snippet children({ id, describedBy, invalid })}
				<TextField
					{id}
					{describedBy}
					{invalid}
					type="number"
					min={1}
					value={rateMbps}
					oninput={(e: Event & { currentTarget: HTMLInputElement }) => (rateMbps = e.currentTarget.value)}
					data-testid="edit-nic-rate"
				/>
			{/snippet}
		</FormField>
		<div class="mt-2 flex justify-end gap-2">
			<Button type="button" variant="ghost" onclick={close}>{m['common.cancel']()}</Button>
			<Button
				type="submit"
				loading={store.networkInFlight}
				disabled={!bridge}
				data-testid="edit-nic-submit"
			>
				{store.networkInFlight ? m['common.saving']() : m['common.save']()}
			</Button>
		</div>
	</form>
</Dialog>
