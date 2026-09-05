<script lang="ts">
	import { getVmDetailContext, type VmDisk } from '../detail.svelte';
	import AddDiskDialog from './AddDiskDialog.svelte';
	import ResizeDiskDialog from './ResizeDiskDialog.svelte';
	import DeleteDiskDialog from './DeleteDiskDialog.svelte';
	import VmCdromCard from './VmCdromCard.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Button from '$lib/shared/ui/Button.svelte';

	const store = getVmDetailContext();

	let addOpen = $state(false);
	let resizeOpen = $state(false);
	let resizeTarget = $state<VmDisk | null>(null);
	let deleteOpen = $state(false);
	let deleteTarget = $state<VmDisk | null>(null);

	function openResize(disk: VmDisk): void {
		resizeTarget = disk;
		resizeOpen = true;
	}

	function openDelete(disk: VmDisk): void {
		deleteTarget = disk;
		deleteOpen = true;
	}
</script>

<section class="rounded-xl border border-border bg-card p-6 shadow-card" aria-labelledby="disks-heading">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h2 id="disks-heading" class="text-lg font-semibold">{m['vms.disks.heading']()}</h2>
			<p class="mt-1 text-sm text-muted-foreground">
				{m['vms.disks.description']()}
			</p>
		</div>
		<Button
			disabled={store.diskInFlight}
			onclick={() => (addOpen = true)}
			data-testid="vm-disk-add-open"
		>
			{m['vms.disks.addButton']()}
		</Button>
	</div>

	{#if store.hardwareLoading}
		<p class="mt-5 text-sm text-muted-foreground" role="status">{m['vms.disks.loading']()}</p>
	{:else if store.hardwareError}
		<Alert class="mt-5">{store.hardwareError}</Alert>
	{:else if store.entity?.disks?.length}
		<div class="mt-5 overflow-x-auto">
			<table class="pv-table pv-responsive-table">
				<thead>
					<tr class="border-b border-border text-xs text-muted-foreground">
						<th class="font-medium">{m['vms.disks.tableDisk']()}</th>
						<th class="font-medium">{m['vms.disks.tableStorage']()}</th>
						<th class="font-medium">{m['vms.disks.tableSize']()}</th>
						<th class="font-medium">{m['common.actions']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each store.entity.disks as disk (disk.key)}
						<tr class="border-b border-border last:border-b-0">
							<td class="font-medium" data-label={m['vms.disks.tableDisk']()}>
								<span class="font-mono">{disk.key}</span>
								{#if disk.isBoot}
									<span class="ml-2 rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
										{m['common.boot']()}
									</span>
								{/if}
							</td>
							<td class="font-mono text-muted-foreground" data-label={m['vms.disks.tableStorage']()}>
								{disk.storage}
							</td>
							<td class="font-mono" data-label={m['vms.disks.tableSize']()}>{disk.sizeGB} GB</td>
							<td data-label={m['common.actions']()}>
								<div class="flex flex-wrap items-center gap-2">
									<button
										type="button"
										class="rounded-lg border border-border px-2.5 py-1 text-xs font-medium hover:bg-muted disabled:opacity-50"
										disabled={store.diskInFlight}
										onclick={() => openResize(disk)}
										data-testid={`vm-disk-resize-open-${disk.key}`}
									>
										{m['vms.disks.resize']()}
									</button>
									<button
										type="button"
										class="rounded-lg border border-destructive/30 px-2.5 py-1 text-xs font-medium text-destructive hover:bg-destructive/10 disabled:opacity-50"
										disabled={store.diskInFlight || disk.isBoot}
										onclick={() => openDelete(disk)}
										data-testid={`vm-disk-delete-open-${disk.key}`}
									>
										{disk.isBoot ? m['common.protected']() : m['common.delete']()}
									</button>
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{:else}
		<EmptyState
			title={m['vms.disks.empty']()}
			class="mt-5 rounded-xl border border-dashed border-border"
		>
			{#snippet actions()}
				<Button onclick={() => (addOpen = true)}>{m['vms.disks.addButton']()}</Button>
			{/snippet}
		</EmptyState>
	{/if}
	{#if store.diskError}
		<Alert class="mt-3">{store.diskError}</Alert>
	{/if}
</section>

<section class="mt-4" aria-label={m['vms.disks.cdromSection']()}>
	<VmCdromCard />
</section>

<AddDiskDialog bind:open={addOpen} />
<ResizeDiskDialog bind:open={resizeOpen} disk={resizeTarget} />
<DeleteDiskDialog bind:open={deleteOpen} disk={deleteTarget} />
