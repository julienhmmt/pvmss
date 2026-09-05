<script lang="ts">
	import { getVmDetailContext, type VmNetworkInterface } from '../detail.svelte';
	import EditNetworkInterfaceDialog from './EditNetworkInterfaceDialog.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';

	const store = getVmDetailContext();

	let editOpen = $state(false);
	let editTarget = $state<VmNetworkInterface | null>(null);
	let copied = $state('');

	function openEdit(iface: VmNetworkInterface): void {
		editTarget = iface;
		editOpen = true;
	}

	async function copy(value: string): Promise<void> {
		await navigator.clipboard.writeText(value);
		copied = value;
		setTimeout(() => {
			if (copied === value) copied = '';
		}, 1500);
	}
</script>

<section class="rounded-xl border border-border bg-card p-6 shadow-card" aria-labelledby="network-heading">
	<h2 id="network-heading" class="text-lg font-semibold">{m['vms.network.heading']()}</h2>
	<p class="mt-1 text-sm text-muted-foreground">{m['vms.network.description']()}</p>

	{#if store.hardwareLoading}
		<p class="mt-5 text-sm text-muted-foreground" role="status">{m['vms.network.loading']()}</p>
	{:else if store.hardwareError}
		<Alert class="mt-5">{store.hardwareError}</Alert>
	{:else if store.entity?.networkInterfaces?.length}
		<div class="mt-5 overflow-x-auto">
			<table class="pv-table pv-responsive-table">
				<thead>
					<tr class="border-b border-border text-xs text-muted-foreground">
						<th class="font-medium">{m['vms.network.columnInterface']()}</th>
						<th class="font-medium">{m['vms.network.columnBridge']()}</th>
						<th class="font-medium">{m['vms.network.columnModel']()}</th>
						<th class="font-medium">{m['vms.network.columnMac']()}</th>
						<th class="font-medium">{m['vms.network.columnVlan']()}</th>
						<th class="font-medium">{m['vms.network.columnRate']()}</th>
						<th class="font-medium">{m['vms.network.columnIps']()}</th>
						<th class="font-medium">{m['common.actions']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each store.entity.networkInterfaces as nic (nic.index)}
						<tr class="border-b border-border last:border-b-0" data-testid={`vm-nic-${nic.index}`}>
							<td class="font-medium font-mono" data-label={m['vms.network.columnInterface']()}>
								net{nic.index}
							</td>
							<td class="font-mono text-muted-foreground" data-label={m['vms.network.columnBridge']()}>
								{nic.bridge}
							</td>
							<td class="font-mono text-muted-foreground" data-label={m['vms.network.columnModel']()}>
								{nic.model}
							</td>
							<td data-label={m['vms.network.columnMac']()}>
								<button
									type="button"
									class="rounded-md border border-border bg-muted/40 px-2 py-0.5 font-mono text-xs hover:bg-muted"
									onclick={() => copy(nic.mac)}
									data-testid={`vm-nic-mac-copy-${nic.index}`}
								>
									{copied === nic.mac ? m['common.copied']() : nic.mac}
								</button>
							</td>
							<td class="font-mono text-muted-foreground" data-label={m['vms.network.columnVlan']()}>
								{nic.vlan ?? m['common.dash']()}
							</td>
							<td class="font-mono text-muted-foreground" data-label={m['vms.network.columnRate']()}>
								{nic.rateMbps ? m['vms.network.rateValue']({ rate: nic.rateMbps }) : m['common.dash']()}
							</td>
							<td data-label={m['vms.network.columnIps']()}>
								{#if nic.ipAddresses.length}
									<div class="flex flex-wrap gap-1">
										{#each nic.ipAddresses as ip (ip)}
											<button
												type="button"
												class="rounded-md border border-border bg-muted/40 px-2 py-0.5 font-mono text-xs hover:bg-muted"
												onclick={() => copy(ip)}
											>
												{copied === ip ? m['common.copied']() : ip}
											</button>
										{/each}
									</div>
								{:else}
									<span class="text-muted-foreground">{m['common.dash']()}</span>
								{/if}
							</td>
							<td data-label={m['common.actions']()}>
								<button
									type="button"
									class="rounded-lg border border-border px-2.5 py-1 text-xs font-medium hover:bg-muted disabled:opacity-50"
									disabled={store.networkInFlight}
									onclick={() => openEdit(nic)}
									data-testid={`vm-nic-edit-open-${nic.index}`}
								>
									{m['common.edit']()}
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{:else}
		<EmptyState
			title={m['vms.network.empty']()}
			class="mt-5 rounded-xl border border-dashed border-border"
		/>
	{/if}
	{#if store.writeError}
		<Alert class="mt-3">{store.writeError}</Alert>
	{/if}
</section>

<EditNetworkInterfaceDialog bind:open={editOpen} iface={editTarget} />
