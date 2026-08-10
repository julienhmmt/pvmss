<script lang="ts">
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import { VmSnapshotsStore, type VmSnapshot } from './snapshots.svelte';

	interface Props {
		open?: boolean;
		store: VmSnapshotsStore;
		snapshot: VmSnapshot | null;
	}

	let { open = $bindable(false), store, snapshot }: Props = $props();

	async function confirm(): Promise<void> {
		if (snapshot === null) return;
		const rolledBack = await store.rollback(snapshot.name);
		if (rolledBack) open = false;
	}

	function close(): void {
		store.clearError();
		open = false;
	}
</script>

<Dialog bind:open labelledBy="rollback-snapshot-title" onClose={close}>
	<h2 id="rollback-snapshot-title" class="mb-2 text-lg font-semibold">Restore snapshot?</h2>
	<p class="mb-4 text-sm text-muted-foreground">
		Restoring “{snapshot?.name}” means everything captured after this snapshot is lost. This cannot be undone.
	</p>
	{#if store.error}<p role="alert" class="mb-4 text-sm text-destructive" data-testid="snapshot-rollback-error">{store.error}</p>{/if}
	<div class="flex justify-end gap-2">
		<button type="button" class="rounded-md border border-border px-4 py-2 text-sm hover:bg-muted" onclick={close} data-testid="snapshot-rollback-cancel">Cancel</button>
		<button type="button" class="rounded-md bg-destructive px-4 py-2 text-sm font-medium text-destructive-foreground hover:bg-destructive/90 disabled:opacity-50" disabled={store.inFlight || snapshot === null} onclick={() => void confirm()} data-testid="snapshot-rollback-confirm">
			{store.inFlight ? 'Restoring…' : 'Restore snapshot'}
		</button>
	</div>
</Dialog>
