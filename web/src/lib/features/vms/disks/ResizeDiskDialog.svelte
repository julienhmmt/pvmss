<script lang="ts">
	import { getVmDetailContext, type VmDisk } from '../detail.svelte';
	import Dialog from '#lib/shared/ui/Dialog.svelte';
	import FormField from '#lib/shared/ui/FormField.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import { m } from '#lib/paraglide/messages.js';

	const store = getVmDetailContext();

	interface Props {
		open?: boolean;
		disk: VmDisk | null;
	}

	let { open = $bindable(false), disk }: Props = $props();

	let sizeGB = $state('');

	const minSizeGB = $derived((disk?.sizeGB ?? 0) + 1);
	const maxDiskGB = $derived.by(() => {
		const configured = store.hardwareOptions?.limits.maxDiskPerVMGB;
		return configured !== undefined && configured > 0 ? configured : 2048;
	});
	const sizeNum = $derived(Number(sizeGB));
	const sizeError = $derived(
		sizeGB === '' || (Number.isInteger(sizeNum) && sizeNum >= minSizeGB && sizeNum <= maxDiskGB)
			? null
			: m['vms.create.diskOutOfRange']({ min: minSizeGB, max: maxDiskGB })
	);

	$effect(() => {
		if (disk) sizeGB = String(disk.sizeGB + 1);
	});

	function close(): void {
		open = false;
	}

	async function submit(): Promise<void> {
		if (!disk || sizeError !== null || !Number.isInteger(sizeNum) || sizeNum <= disk.sizeGB) return;
		if (await store.resizeDisk(disk.key, sizeNum)) close();
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
		<FormField
			label={m['vms.disks.resizeNewSize']()}
			hint={m['vms.create.diskLimitHint']({ min: minSizeGB, max: maxDiskGB })}
			required
			error={sizeError ?? store.diskError}
		>
			{#snippet children({ id, describedBy, invalid })}
				<TextField
					{id}
					{describedBy}
					{invalid}
					type="number"
					min={minSizeGB}
					max={maxDiskGB}
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
