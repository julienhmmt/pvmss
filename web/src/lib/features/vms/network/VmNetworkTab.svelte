<script lang="ts">
	import { getVmDetailContext, type VmNetworkInterface } from '../detail.svelte';
	import EditNetworkInterfaceDialog from './EditNetworkInterfaceDialog.svelte';
	import { m } from '$lib/paraglide/messages.js';

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

<section aria-labelledby="network-heading">
	<h2 id="network-heading" class="text-lg font-semibold">{m['vms.network.heading']()}</h2>
	<p class="text-sm text-muted-foreground">{m['vms.network.description']()}</p>

	{#if store.hardwareLoading}
		<p class="mt-3 text-sm text-muted-foreground" role="status">{m['vms.network.loading']()}</p>
	{:else if store.hardwareError}
		<p class="mt-3 text-sm text-destructive" role="alert">{store.hardwareError}</p>
	{:else if store.entity?.networkInterfaces?.length}
		<div class="mt-4 overflow-x-auto rounded-md border border-border">
			<table class="min-w-full text-left text-sm">
				<thead class="bg-muted/40 text-xs text-muted-foreground">
					<tr>
						<th class="px-3 py-2">{m['vms.network.columnInterface']()}</th>
						<th class="px-3 py-2">{m['vms.network.columnBridge']()}</th>
						<th class="px-3 py-2">{m['vms.network.columnModel']()}</th>
						<th class="px-3 py-2">{m['vms.network.columnMac']()}</th>
						<th class="px-3 py-2">{m['vms.network.columnVlan']()}</th>
						<th class="px-3 py-2">{m['vms.network.columnRate']()}</th>
						<th class="px-3 py-2">{m['vms.network.columnIps']()}</th>
						<th class="px-3 py-2">{m['common.actions']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each store.entity.networkInterfaces as nic (nic.index)}
						<tr class="border-t border-border" data-testid={`vm-nic-${nic.index}`}>
							<td class="px-3 py-2 font-medium">net{nic.index}</td>
							<td class="px-3 py-2 text-muted-foreground">{nic.bridge}</td>
							<td class="px-3 py-2 text-muted-foreground">{nic.model}</td>
							<td class="px-3 py-2">
								<button
									type="button"
									class="rounded border border-border px-2 py-0.5 text-xs hover:bg-muted"
									onclick={() => copy(nic.mac)}
									data-testid={`vm-nic-mac-copy-${nic.index}`}
								>
									{copied === nic.mac ? m['common.copied']() : nic.mac}
								</button>
							</td>
							<td class="px-3 py-2 text-muted-foreground">{nic.vlan ?? m['common.dash']()}</td>
							<td class="px-3 py-2 text-muted-foreground">
								{nic.rateMbps ? m['vms.network.rateValue']({ rate: nic.rateMbps }) : m['common.dash']()}
							</td>
							<td class="px-3 py-2">
								{#if nic.ipAddresses.length}
									<div class="flex flex-wrap gap-1">
										{#each nic.ipAddresses as ip (ip)}
											<button
												type="button"
												class="rounded border border-border px-2 py-0.5 text-xs hover:bg-muted"
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
							<td class="px-3 py-2">
								<button
									type="button"
									class="rounded-md border border-border px-2 py-1 text-xs hover:bg-muted disabled:opacity-50"
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
		<p
			class="mt-3 rounded-md border border-dashed border-border px-3 py-4 text-sm text-muted-foreground"
		>
			{m['vms.network.empty']()}
		</p>
	{/if}
	{#if store.writeError}
		<p class="mt-2 text-sm text-destructive" role="alert">{store.writeError}</p>
	{/if}
</section>

<EditNetworkInterfaceDialog bind:open={editOpen} iface={editTarget} />
