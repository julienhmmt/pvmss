<script lang="ts">
	import { getVmDetailContext } from '../detail.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
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
		<FormField label={m['vms.disks.addBus']()} required>
			{#snippet children({ id, describedBy, invalid })}
				<Select
					{id}
					{describedBy}
					{invalid}
					value={bus}
					onchange={(e: Event & { currentTarget: HTMLSelectElement }) => (bus = e.currentTarget.value as typeof bus)}
					options={[
						{ value: 'scsi', label: 'SCSI' },
						{ value: 'virtio', label: 'VirtIO' },
						{ value: 'sata', label: 'SATA' },
						{ value: 'ide', label: 'IDE' }
					]}
					required
				/>
			{/snippet}
		</FormField>
		<FormField label={m['vms.disks.addStorage']()} required error={store.diskError}>
			{#snippet children({ id, describedBy, invalid })}
				<Select
					{id}
					{describedBy}
					{invalid}
					bind:value={storage}
					placeholder={m['vms.disks.addSelectStorage']()}
					options={(store.hardwareOptions?.storages ?? []).map((option) => ({
						value: option.storage,
						label: `${option.storage} · ${option.node}`
					}))}
					required
					data-testid="add-disk-storage"
				/>
			{/snippet}
		</FormField>
		<FormField label={m['vms.disks.addSize']()} required>
			{#snippet children({ id, describedBy, invalid })}
				<TextField
					{id}
					{describedBy}
					{invalid}
					type="number"
					min={1}
					value={sizeGB}
					oninput={(e: Event & { currentTarget: HTMLInputElement }) => (sizeGB = e.currentTarget.value)}
					required
					data-testid="add-disk-size"
				/>
			{/snippet}
		</FormField>
		<div class="mt-2 flex justify-end gap-2">
			<Button type="button" variant="ghost" onclick={close}>{m['common.cancel']()}</Button>
			<Button
				type="submit"
				loading={store.diskInFlight}
				data-testid="add-disk-submit"
			>
				{store.diskInFlight ? m['common.adding']() : m['vms.disks.addButton']()}
			</Button>
		</div>
	</form>
</Dialog>
