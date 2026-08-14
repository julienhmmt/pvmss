<script lang="ts">
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import { VmSnapshotsStore, type VmSnapshot } from './snapshots.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		open?: boolean;
		store: VmSnapshotsStore;
		snapshot: VmSnapshot | null;
	}

	let { open = $bindable(false), store, snapshot }: Props = $props();

	async function confirm(): Promise<void> {
		if (snapshot === null) return;
		const deleted = await store.delete(snapshot.name);
		if (deleted) open = false;
	}

	function close(): void {
		store.clearError();
		open = false;
	}
</script>

<Dialog bind:open labelledBy="delete-snapshot-title" onClose={close}>
	<h2 id="delete-snapshot-title" class="mb-2 text-lg font-semibold">{m['vms.snapshots.deleteDialogTitle']()}</h2>
	<p class="mb-4 text-sm text-muted-foreground">{m['vms.snapshots.deleteConfirm']({ name: snapshot?.name ?? '' })}</p>
	{#if store.error}<p role="alert" class="mb-4 text-sm text-destructive" data-testid="snapshot-delete-error">{store.error}</p>{/if}
	<div class="flex justify-end gap-2">
		<button type="button" class="rounded-md border border-border px-4 py-2 text-sm hover:bg-muted" onclick={close} data-testid="snapshot-delete-cancel">{m['common.cancel']()}</button>
		<button type="button" class="rounded-md bg-destructive px-4 py-2 text-sm font-medium text-destructive-foreground hover:bg-destructive/90 disabled:opacity-50" disabled={store.inFlight || snapshot === null} onclick={() => void confirm()} data-testid="snapshot-delete-confirm">
			{store.inFlight ? m['common.deleting']() : m['vms.snapshots.delete']()}
		</button>
	</div>
</Dialog>
