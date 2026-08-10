<script lang="ts">
	import { getVmDetailContext, type VmDisk } from '../detail.svelte';
	import AddDiskDialog from './AddDiskDialog.svelte';
	import ResizeDiskDialog from './ResizeDiskDialog.svelte';
	import DeleteDiskDialog from './DeleteDiskDialog.svelte';
	import VmCdromCard from './VmCdromCard.svelte';

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
			<h2 id="disks-heading" class="text-lg font-semibold">Disks</h2>
			<p class="text-sm text-muted-foreground">
				Boot disks are protected. Growth is available while the VM runs.
			</p>
		</div>
		<button
			type="button"
			class="rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
			disabled={store.diskInFlight}
			onclick={() => (addOpen = true)}
			data-testid="vm-disk-add-open"
		>
			Add disk
		</button>
	</div>

	{#if store.hardwareLoading}
		<p class="mt-3 text-sm text-muted-foreground" role="status">Loading hardware options…</p>
	{:else if store.hardwareError}
		<p class="mt-3 text-sm text-destructive" role="alert">{store.hardwareError}</p>
	{:else if store.entity?.disks?.length}
		<div class="mt-4 overflow-x-auto rounded-md border border-border">
			<table class="min-w-full text-left text-sm">
				<thead class="bg-muted/40 text-xs text-muted-foreground">
					<tr>
						<th class="px-3 py-2">Disk</th>
						<th class="px-3 py-2">Storage</th>
						<th class="px-3 py-2">Size</th>
						<th class="px-3 py-2">Actions</th>
					</tr>
				</thead>
				<tbody>
					{#each store.entity.disks as disk (disk.key)}
						<tr class="border-t border-border">
							<td class="px-3 py-2 font-medium">{disk.key}{disk.isBoot ? ' · boot' : ''}</td>
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
										Resize
									</button>
									<button
										type="button"
										class="rounded-md border border-destructive/30 px-2 py-1 text-xs text-destructive hover:bg-destructive/10 disabled:opacity-50"
										disabled={store.diskInFlight || disk.isBoot}
										onclick={() => openDelete(disk)}
										data-testid={`vm-disk-delete-open-${disk.key}`}
									>
										{disk.isBoot ? 'Protected' : 'Delete'}
									</button>
								</div>
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
			No disks attached.
		</p>
	{/if}
	{#if store.diskError}
		<p class="mt-2 text-sm text-destructive" role="alert">{store.diskError}</p>
	{/if}
</section>

<section class="mt-6" aria-label="CD-ROM">
	<VmCdromCard />
</section>

<AddDiskDialog bind:open={addOpen} />
<ResizeDiskDialog bind:open={resizeOpen} disk={resizeTarget} />
<DeleteDiskDialog bind:open={deleteOpen} disk={deleteTarget} />
