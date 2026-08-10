<script lang="ts">
	import { getVmDetailContext, type VmNetworkInterface } from '../detail.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';

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
	<h2 id="edit-nic-title" class="mb-4 text-lg font-semibold">Edit net{iface?.index ?? ''}</h2>
	<form
		class="grid gap-3"
		onsubmit={(event) => {
			event.preventDefault();
			void submit();
		}}
	>
		<label class="grid gap-1 text-sm">
			Bridge
			<select
				class="rounded-md border border-border bg-background px-2 py-2"
				bind:value={bridge}
				data-testid="edit-nic-bridge"
			>
				<option value="" disabled>Select approved bridge</option>
				{#each store.hardwareOptions?.bridges ?? [] as option (option.bridge)}
					<option value={option.bridge}>{option.bridge} · {option.node}</option>
				{/each}
			</select>
		</label>
		<label class="grid gap-1 text-sm">
			Model
			<select class="rounded-md border border-border bg-background px-2 py-2" bind:value={model}>
				<option value="virtio">VirtIO</option>
				<option value="e1000">E1000</option>
				<option value="rtl8139">RTL8139</option>
				<option value="vmxnet3">VMXNET3</option>
			</select>
		</label>
		<label class="grid gap-1 text-sm">
			VLAN (optional)
			<input
				class="rounded-md border border-border bg-background px-2 py-2"
				type="number"
				min="1"
				max="4094"
				bind:value={vlan}
				data-testid="edit-nic-vlan"
			/>
		</label>
		<label class="grid gap-1 text-sm">
			Rate limit, MB/s (optional)
			<input
				class="rounded-md border border-border bg-background px-2 py-2"
				type="number"
				min="1"
				bind:value={rateMbps}
				data-testid="edit-nic-rate"
			/>
		</label>
		{#if store.writeError}
			<p role="alert" class="text-sm text-destructive">{store.writeError}</p>
		{/if}
		<div class="mt-2 flex justify-end gap-2">
			<button
				type="button"
				class="rounded-md border border-border px-4 py-2 text-sm hover:bg-muted"
				onclick={close}
			>
				Cancel
			</button>
			<button
				type="submit"
				class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
				disabled={!bridge || store.networkInFlight}
				data-testid="edit-nic-submit"
			>
				{store.networkInFlight ? 'Saving…' : 'Save'}
			</button>
		</div>
	</form>
</Dialog>
