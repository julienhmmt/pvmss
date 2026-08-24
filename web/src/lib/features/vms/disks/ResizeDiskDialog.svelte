<script lang="ts">
	import { getVmDetailContext, type VmDisk } from '../detail.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages.js';

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
	<h2 id="resize-disk-title" class="mb-4 text-lg font-semibold">{m['vms.disks.resizeDialogTitle']({ disk: disk?.key ?? 'disk' })}</h2>
	<form
		class="grid gap-3"
		onsubmit={(event) => {
			event.preventDefault();
			void submit();
		}}
	>
		<FormField label={m['vms.disks.resizeNewSize']()} required error={store.diskError}>
			{#snippet children({ id, describedBy, invalid })}
				<TextField
					{id}
					{describedBy}
					{invalid}
					type="number"
					min={(disk?.sizeGB ?? 0) + 1}
					value={sizeGB}
					oninput={(e: Event & { currentTarget: HTMLInputElement }) => (sizeGB = e.currentTarget.value)}
					required
					data-testid="resize-disk-size"
				/>
			{/snippet}
		</FormField>
		<div class="mt-2 flex justify-end gap-2">
			<Button type="button" variant="ghost" onclick={close}>{m['common.cancel']()}</Button>
			<Button
				type="submit"
				loading={store.diskInFlight}
				data-testid="resize-disk-submit"
			>
				{store.diskInFlight ? m['common.resizing']() : m['vms.disks.resize']()}
			</Button>
		</div>
	</form>
</Dialog>
