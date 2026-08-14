<script lang="ts">
	import { getVmDetailContext } from '../detail.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getVmDetailContext();

	interface Props {
		open?: boolean;
	}

	let { open = $bindable(false) }: Props = $props();

	let bus = $state<'virtio' | 'scsi' | 'sata' | 'ide'>('scsi');
	let storage = $state('');
	let sizeGB = $state('10');

	function close(): void {
		open = false;
	}

	async function submit(): Promise<void> {
		const size = Number(sizeGB);
		if (!storage || !Number.isInteger(size) || size < 1) return;
		if (await store.addDisk(bus, storage, size)) close();
	}
</script>

<Dialog bind:open labelledBy="add-disk-title" onClose={close}>
	<h2 id="add-disk-title" class="mb-4 text-lg font-semibold">{m['vms.disks.addDialogTitle']()}</h2>
	<form
		class="grid gap-3"
		onsubmit={(event) => {
			event.preventDefault();
			void submit();
		}}
	>
		<label class="grid gap-1 text-sm">
			{m['vms.disks.addBus']()}
			<select class="rounded-md border border-border bg-background px-2 py-2" bind:value={bus}>
				<option value="scsi">SCSI</option>
				<option value="virtio">VirtIO</option>
				<option value="sata">SATA</option>
				<option value="ide">IDE</option>
			</select>
		</label>
		<label class="grid gap-1 text-sm">
			{m['vms.disks.addStorage']()}
			<select
				class="rounded-md border border-border bg-background px-2 py-2"
				bind:value={storage}
				data-testid="add-disk-storage"
			>
				<option value="" disabled>{m['vms.disks.addSelectStorage']()}</option>
				{#each store.hardwareOptions?.storages ?? [] as option (option.storage)}
					<option value={option.storage}>{option.storage} · {option.node}</option>
				{/each}
			</select>
		</label>
		<label class="grid gap-1 text-sm">
			{m['vms.disks.addSize']()}
			<input
				class="rounded-md border border-border bg-background px-2 py-2"
				type="number"
				min="1"
				bind:value={sizeGB}
				data-testid="add-disk-size"
			/>
		</label>
		{#if store.diskError}
			<p role="alert" class="text-sm text-destructive">{store.diskError}</p>
		{/if}
		<div class="mt-2 flex justify-end gap-2">
			<button
				type="button"
				class="rounded-md border border-border px-4 py-2 text-sm hover:bg-muted"
				onclick={close}
			>
				{m['common.cancel']()}
			</button>
			<button
				type="submit"
				class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
				disabled={store.diskInFlight}
				data-testid="add-disk-submit"
			>
				{store.diskInFlight ? m['common.adding']() : m['vms.disks.addButton']()}
			</button>
		</div>
	</form>
</Dialog>
