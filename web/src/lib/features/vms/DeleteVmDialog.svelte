<script lang="ts">
	import { getVmDetailContext } from './detail.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getVmDetailContext();

	interface Props {
		open?: boolean;
	}

	let { open = $bindable(false) }: Props = $props();

	function close(): void {
		open = false;
	}

	async function confirm(): Promise<void> {
		await store.delete();
	}
</script>

<Dialog bind:open labelledBy="delete-vm-title" onClose={close}>
	<h2 id="delete-vm-title" class="mb-2 text-lg font-semibold">
		{#if store.entity}{m['vms.deleteDialogTitle']({ name: store.entity.name })}{:else}{m['vms.deleteDialogTitle']({ name: '' })}{/if}
	</h2>
	<p class="mb-4 text-sm text-muted-foreground">
		{m['vms.deleteDialogConfirm']()}
	</p>

	{#if store.deleteError}
		<p role="alert" class="mb-4 text-sm text-destructive" data-testid="vm-delete-error">
			{store.deleteError}
		</p>
	{/if}

	<div class="flex justify-end gap-2">
		<button
			type="button"
			class="rounded-md border border-border bg-background px-4 py-2 text-sm font-medium hover:bg-muted"
			onclick={close}
			data-testid="vm-delete-cancel"
		>
			{m['common.cancel']()}
		</button>
		<button
			type="button"
			class="rounded-md bg-destructive px-4 py-2 text-sm font-medium text-destructive-foreground hover:bg-destructive/90 disabled:cursor-not-allowed disabled:opacity-50"
			disabled={store.deleteInFlight}
			onclick={confirm}
			data-testid="vm-delete-confirm"
		>
			{store.deleteInFlight ? m['common.deleting']() : m['common.deletePermanently']()}
		</button>
	</div>
</Dialog>
