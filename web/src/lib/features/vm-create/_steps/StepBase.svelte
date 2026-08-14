<script lang="ts">
	import { getVmCreateContext } from '../create.svelte';
	import { m } from '$lib/paraglide/messages.js';

	// Base step (FR-011): name, node, extra tags (pvmss is added server-side,
	// FR-006), and an optional ISO — all choices from the approved catalog.
	const form = getVmCreateContext();
	const inputClass = 'rounded-md border border-input bg-background px-3 py-2';
</script>

<div class="grid gap-4">
	<label class="grid gap-1 text-sm font-medium">
		{m['vms.create.name']()}
		<input class={inputClass} bind:value={form.name} required placeholder="web-03" />
	</label>

	<label class="grid gap-1 text-sm font-medium">
		{m['vms.create.node']()}
		<select class={inputClass} bind:value={form.node} required>
			<option value="" disabled>{m['vms.create.chooseNode']()}</option>
			{#each form.catalog?.nodes ?? [] as node (node)}
				<option value={node}>{node}</option>
			{/each}
		</select>
	</label>

	<label class="grid gap-1 text-sm font-medium">
		{m['vms.create.tags']()} <span class="text-xs font-normal text-muted-foreground">{m['vms.create.tagsHelp']()}</span>
		<input class={inputClass} bind:value={form.tagsInput} placeholder="team-web, prod" />
	</label>

	<label class="grid gap-1 text-sm font-medium">
		{m['vms.create.iso']()} <span class="text-xs font-normal text-muted-foreground">{m['common.optional']()}</span>
		<select class={inputClass} bind:value={form.isoFile}>
			<option value="">{m['common.none']()}</option>
			{#each form.catalog?.isos ?? [] as iso (iso.file)}
				<option value={iso.file}>{iso.file}</option>
			{/each}
		</select>
	</label>
</div>
