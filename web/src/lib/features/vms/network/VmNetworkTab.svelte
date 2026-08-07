<script lang="ts">
	import { getVmDetailContext, type VmNetworkInterface } from '../detail.svelte';
	import EditNetworkInterfaceDialog from './EditNetworkInterfaceDialog.svelte';

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
	<h2 id="network-heading" class="text-lg font-semibold">Network</h2>
	<p class="text-sm text-muted-foreground">Bridges must be from the approved catalog.</p>

	{#if store.hardwareLoading}
		<p class="mt-3 text-sm text-muted-foreground" role="status">Loading hardware options…</p>
	{:else if store.hardwareError}
		<p class="mt-3 text-sm text-destructive" role="alert">{store.hardwareError}</p>
	{:else if store.entity?.networkInterfaces?.length}
		<div class="mt-4 overflow-x-auto rounded-md border border-border">
			<table class="min-w-full text-left text-sm">
				<thead class="bg-muted/40 text-xs text-muted-foreground">
					<tr>
						<th class="px-3 py-2">Interface</th>
						<th class="px-3 py-2">Bridge</th>
						<th class="px-3 py-2">Model</th>
						<th class="px-3 py-2">MAC</th>
						<th class="px-3 py-2">VLAN</th>
						<th class="px-3 py-2">Rate</th>
						<th class="px-3 py-2">IP addresses</th>
						<th class="px-3 py-2">Actions</th>
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
									{copied === nic.mac ? 'Copied' : nic.mac}
								</button>
							</td>
							<td class="px-3 py-2 text-muted-foreground">{nic.vlan ?? '—'}</td>
							<td class="px-3 py-2 text-muted-foreground">
								{nic.rateMbps ? `${nic.rateMbps} MB/s` : '—'}
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
												{copied === ip ? 'Copied' : ip}
											</button>
										{/each}
									</div>
								{:else}
									<span class="text-muted-foreground">—</span>
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
									Edit
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
			No network interfaces.
		</p>
	{/if}
	{#if store.writeError}
		<p class="mt-2 text-sm text-destructive" role="alert">{store.writeError}</p>
	{/if}
</section>

<EditNetworkInterfaceDialog bind:open={editOpen} iface={editTarget} />
