<script lang="ts">
	import { getVmDetailContext, type VmDisk } from '../detail.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';

	const store = getVmDetailContext();

	interface Props {
		open?: boolean;
		disk: VmDisk | null;
	}

	let { open = $bindable(false), disk }: Props = $props();

	let sizeGB = $state('');

	$effect(() => {
		if (disk) sizeGB = String(disk.sizeGB + 1);
	});

	function close(): void {
		open = false;
	}

	async function submit(): Promise<void> {
		if (!disk) return;
		const size = Number(sizeGB);
		if (!Number.isInteger(size) || size <= disk.sizeGB) return;
		if (await store.resizeDisk(disk.key, size)) close();
	}
</script>

<Dialog bind:open labelledBy="resize-disk-title" onClose={close}>
	<h2 id="resize-disk-title" class="mb-4 text-lg font-semibold">Resize {disk?.key ?? 'disk'}</h2>
	<form
		class="grid gap-3"
		onsubmit={(event) => {
			event.preventDefault();
			void submit();
		}}
	>
		<label class="grid gap-1 text-sm">
			New size (GB)
			<input
				class="rounded-md border border-border bg-background px-2 py-2"
				type="number"
				min={(disk?.sizeGB ?? 0) + 1}
				bind:value={sizeGB}
				data-testid="resize-disk-size"
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
				Cancel
			</button>
			<button
				type="submit"
				class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
				disabled={store.diskInFlight}
				data-testid="resize-disk-submit"
			>
				{store.diskInFlight ? 'Resizing…' : 'Resize'}
			</button>
		</div>
	</form>
</Dialog>
