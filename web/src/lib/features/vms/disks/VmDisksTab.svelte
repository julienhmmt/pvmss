<script lang="ts">
	import { getVmDetailContext, type VmDisk } from '../detail.svelte';
	import AddDiskDialog from './AddDiskDialog.svelte';
	import ResizeDiskDialog from './ResizeDiskDialog.svelte';
	import DeleteDiskDialog from './DeleteDiskDialog.svelte';
	import VmCdromCard from './VmCdromCard.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';

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

<section aria-labelledby="disks-heading">
	<div class="flex flex-wrap items-baseline justify-between gap-2">
		<div>
			<h2 id="disks-heading" class="text-lg font-semibold">{m['vms.disks.heading']()}</h2>
			<p class="text-sm text-muted-foreground">
				{m['vms.disks.description']()}
			</p>
		</div>
		<button
			type="button"
			class="rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
			disabled={store.diskInFlight}
			onclick={() => (addOpen = true)}
			data-testid="vm-disk-add-open"
		>
			{m['vms.disks.addButton']()}
		</button>
	</div>

	{#if store.hardwareLoading}
		<p class="mt-3 text-sm text-muted-foreground" role="status">{m['vms.disks.loading']()}</p>
	{:else if store.hardwareError}
		<p class="mt-3 text-sm text-destructive" role="alert">{store.hardwareError}</p>
	{:else if store.entity?.disks?.length}
		<div class="mt-4 overflow-x-auto rounded-md border border-border">
			<table class="min-w-full text-left text-sm">
				<thead class="bg-muted/40 text-xs text-muted-foreground">
					<tr>
						<th class="px-3 py-2">{m['vms.disks.tableDisk']()}</th>
						<th class="px-3 py-2">{m['vms.disks.tableStorage']()}</th>
						<th class="px-3 py-2">{m['vms.disks.tableSize']()}</th>
						<th class="px-3 py-2">{m['common.actions']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each store.entity.disks as disk (disk.key)}
						<tr class="border-t border-border">
							<td class="px-3 py-2 font-medium">{disk.key}{disk.isBoot ? ` · ${m['common.boot']()}` : ''}</td>
							<td class="px-3 py-2 text-muted-foreground">{disk.storage}</td>
							<td class="px-3 py-2">{disk.sizeGB} GB</td>
							<td class="px-3 py-2">
								<div class="flex flex-wrap items-center gap-2">
									<button
										type="button"
										class="rounded-md border border-border px-2 py-1 text-xs hover:bg-muted disabled:opacity-50"
										disabled={store.diskInFlight}
										onclick={() => openResize(disk)}
										data-testid={`vm-disk-resize-open-${disk.key}`}
									>
										{m['vms.disks.resize']()}
									</button>
									<button
										type="button"
										class="rounded-md border border-destructive/30 px-2 py-1 text-xs text-destructive hover:bg-destructive/10 disabled:opacity-50"
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
			class="mt-3 rounded-md border border-dashed border-border py-4"
		>
			{#snippet actions()}
				<button
					type="button"
					class="rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
					onclick={() => (addOpen = true)}
				>
					{m['vms.disks.addButton']()}
				</button>
			{/snippet}
		</EmptyState>
	{/if}
	{#if store.diskError}
		<p class="mt-2 text-sm text-destructive" role="alert">{store.diskError}</p>
	{/if}
</section>

<section class="mt-6" aria-label={m['vms.disks.cdromSection']()}>
	<VmCdromCard />
</section>

<AddDiskDialog bind:open={addOpen} />
<ResizeDiskDialog bind:open={resizeOpen} disk={resizeTarget} />
<DeleteDiskDialog bind:open={deleteOpen} disk={deleteTarget} />
