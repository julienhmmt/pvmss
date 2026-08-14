<script lang="ts">
	import { getVmCreateContext } from '../create.svelte';
	import { m } from '$lib/paraglide/messages.js';

	// Disk step: one initial disk (multi-disk is T07) — an approved storage on
	// the chosen node, plus a size within the technical ceiling.
	const form = getVmCreateContext();
	const inputClass = 'rounded-md border border-input bg-background px-3 py-2';

	const storagesOnNode = $derived(
		(form.catalog?.storages ?? []).filter((storage) => storage.node === form.node)
	);
</script>

<div class="grid gap-4">
	<label class="grid gap-1 text-sm font-medium">
		{m['vms.create.storage']()}
		<select class={inputClass} bind:value={form.diskStorage} required>
			<option value="" disabled>{m['vms.create.chooseStorage']()}</option>
			{#each storagesOnNode as storage (storage.name)}
				<option value={storage.name}>{storage.name}</option>
			{/each}
		</select>
		{#if form.node !== '' && storagesOnNode.length === 0}
			<span class="text-xs text-destructive">{m['vms.create.noStorageOnNode']({ node: form.node })}</span>
		{/if}
	</label>

	<label class="grid gap-1 text-sm font-medium">
		{m['vms.create.size']()} <span class="text-xs font-normal text-muted-foreground">{m['vms.create.sizeRange']()}</span>
		<input class={inputClass} type="number" min="1" max="2048" bind:value={form.diskSizeGB} required />
	</label>
</div>
