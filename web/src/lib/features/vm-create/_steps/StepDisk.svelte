<script lang="ts">
	import { getVmCreateContext } from '../create.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';

	// Disk step: one initial disk (multi-disk is T07) — an approved storage on
	// the chosen node, plus a size within the technical ceiling.
	const form = getVmCreateContext();

	const storagesOnNode = $derived(
		(form.catalog?.storages ?? []).filter((storage) => storage.node === form.node)
	);
	const storageError = $derived(
		form.node !== '' && storagesOnNode.length === 0
			? m['vms.create.noStorageOnNode']({ node: form.node })
			: null
	);

	const maxDiskGB = $derived(form.catalog?.gabarit?.maxDiskPerVMGB ?? 2048);
	const diskError = $derived(
		Number.isInteger(form.diskSizeGB) && form.diskSizeGB >= 1 && form.diskSizeGB <= maxDiskGB
			? null
			: m['vms.create.diskOutOfRange']({ min: 1, max: maxDiskGB })
	);
</script>

<div class="grid gap-4">
	{#if form.node !== '' && form.catalog}
		<p class="text-sm text-muted-foreground">
			{m['vms.create.selectedContext']({ node: form.node, cluster: form.catalog.cluster })}
		</p>
	{/if}

	<FormField label={m['vms.create.storage']()} required error={storageError}>
		{#snippet children({ id, describedBy, invalid })}
			<Select
				{id}
				{describedBy}
				{invalid}
				bind:value={form.diskStorage}
				placeholder={m['vms.create.chooseStorage']()}
				options={storagesOnNode.map((storage) => storage.name)}
			/>
		{/snippet}
	</FormField>

	<FormField label={m['vms.create.size']()} hint={m['vms.create.diskLimitHint']({ min: 1, max: maxDiskGB })} error={diskError} required>
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} type="number" min={1} max={maxDiskGB} bind:value={form.diskSizeGB} required />
		{/snippet}
	</FormField>
</div>
