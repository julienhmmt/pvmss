<script lang="ts">
	import { getVmDetailContext, type VmDisk } from '../detail.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages.js';

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
	<h2 id="delete-disk-title" class="mb-4 text-lg font-semibold">{m['vms.disks.deleteDialogTitle']({ disk: disk?.key ?? 'disk' })}</h2>
	{#if disk?.isBoot}
		<p class="mb-4 text-sm text-warning" role="alert" data-testid="delete-disk-boot-warning">
			{m['vms.disks.deleteBootWarning']()}
		</p>
	{:else}
		<p class="mb-4 text-sm text-muted-foreground">
			{m['vms.disks.deleteWarning']()}
		</p>
	{/if}
	{#if store.diskError}
		<Alert class="mb-4">{store.diskError}</Alert>
	{/if}
	<div class="flex justify-end gap-2">
		<Button variant="ghost" onclick={close} data-testid="delete-disk-cancel">
			{m['common.cancel']()}
		</Button>
		<Button
			variant="destructive"
			disabled={store.diskInFlight || disk?.isBoot === true}
			onclick={confirm}
			data-testid="delete-disk-confirm"
		>
			{store.diskInFlight ? m['common.deleting']() : m['common.deletePermanently']()}
		</Button>
	</div>
</Dialog>
