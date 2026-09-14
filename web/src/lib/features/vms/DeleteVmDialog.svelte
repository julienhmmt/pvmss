<script lang="ts">
	import { getVmDetailContext } from './detail.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getVmDetailContext();
	const toast = getToastContext();

	interface Props {
		open?: boolean;
	}

	let { open = $bindable(false) }: Props = $props();

	/** True once the server reported the VM is running and the user must
	 * explicitly confirm a force-stop before the destroy proceeds. */
	let needsForceStop = $state(false);

	function close(): void {
		open = false;
	}

	// Reset the force-stop step whenever the dialog closes.
	$effect(() => {
		if (!open) needsForceStop = false;
	});

	async function confirm(): Promise<void> {
		const vmName = store.entity?.name ?? '';
		await store.delete(needsForceStop);
		if (store.deleteError) {
			if (store.deleteErrorCode === 'vm_running' && !needsForceStop) {
				// The VM is running - switch to the force-stop confirmation step
				// rather than showing a generic error toast.
				needsForceStop = true;
				return;
			}
			toast.error(m['toast.vmDeleteFailed']({ error: store.deleteError }));
		} else if (store.deleted) {
			toast.success(m['toast.vmDeleted']({ name: vmName }));
		}
	}
</script>

<Dialog bind:open labelledBy="delete-vm-title" onClose={close}>
	<h2 id="delete-vm-title" class="mb-4 text-lg font-semibold">
		{#if store.entity}{m['vms.deleteDialogTitle']({ name: store.entity.name })}{:else}{m['vms.deleteDialogTitle']({ name: '' })}{/if}
	</h2>

	{#if needsForceStop}
		<p class="mb-4 text-sm text-destructive" data-testid="vm-delete-running-warning">
			{m['vms.deleteDialogRunningWarning']()}
		</p>
		<p class="mb-4 text-sm text-muted-foreground">
			{m['vms.deleteDialogForceConfirm']()}
		</p>
	{:else}
		<p class="mb-4 text-sm text-muted-foreground">
			{m['vms.deleteDialogConfirm']()}
		</p>
	{/if}

	{#if store.deleteError && !(store.deleteErrorCode === 'vm_running' && needsForceStop)}
		<Alert data-testid="vm-delete-error" class="mb-4">{store.deleteError}</Alert>
	{/if}

	<div class="flex justify-end gap-2">
		<Button variant="ghost" onclick={close} data-testid="vm-delete-cancel">
			{m['common.cancel']()}
		</Button>
		<Button
			variant="destructive"
			disabled={store.deleteInFlight}
			onclick={confirm}
			data-testid="vm-delete-confirm"
		>
			{#if store.deleteInFlight}
				{m['common.deleting']()}
			{:else if needsForceStop}
				{m['vms.deleteDialogForceStopAndDelete']()}
			{:else}
				{m['common.deletePermanently']()}
			{/if}
		</Button>
	</div>
</Dialog>
