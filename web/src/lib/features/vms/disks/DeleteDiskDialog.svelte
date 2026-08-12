<script lang="ts">
	import { getVmDetailContext, type VmDisk } from '../detail.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';

	const store = getVmDetailContext();

	interface Props {
		open?: boolean;
		disk: VmDisk | null;
	}

	let { open = $bindable(false), disk }: Props = $props();

	function close(): void {
		open = false;
	}

	async function confirm(): Promise<void> {
		if (!disk || disk.isBoot) return;
		if (await store.deleteDisk(disk.key)) close();
	}
</script>

<Dialog bind:open labelledBy="delete-disk-title" onClose={close}>
	<h2 id="delete-disk-title" class="mb-2 text-lg font-semibold">Delete {disk?.key ?? 'disk'}?</h2>
	{#if disk?.isBoot}
		<p class="mb-4 text-sm text-warning" role="alert" data-testid="delete-disk-boot-warning">
			This is the boot disk — it cannot be deleted while it boots the VM.
		</p>
	{:else}
		<p class="mb-4 text-sm text-muted-foreground">
			This permanently destroys the disk. There is no undo.
		</p>
	{/if}
	{#if store.diskError}
		<p role="alert" class="mb-4 text-sm text-destructive">{store.diskError}</p>
	{/if}
	<div class="flex justify-end gap-2">
		<button
			type="button"
			class="rounded-md border border-border px-4 py-2 text-sm hover:bg-muted"
			onclick={close}
			data-testid="delete-disk-cancel"
		>
			Cancel
		</button>
		<button
			type="button"
			class="rounded-md bg-destructive px-4 py-2 text-sm font-medium text-destructive-foreground disabled:cursor-not-allowed disabled:opacity-50"
			disabled={store.diskInFlight || disk?.isBoot}
			onclick={confirm}
			data-testid="delete-disk-confirm"
		>
			{store.diskInFlight ? 'Deleting…' : 'Delete permanently'}
		</button>
	</div>
</Dialog>
