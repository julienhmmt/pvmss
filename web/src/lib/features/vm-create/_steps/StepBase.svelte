<script lang="ts">
	import { getVmCreateContext } from '../create.svelte';

	// Base step (FR-011): name, node, extra tags (pvmss is added server-side,
	// FR-006), and an optional ISO — all choices from the approved catalog.
	const form = getVmCreateContext();
	const inputClass = 'rounded-md border border-input bg-background px-3 py-2';
</script>

<div class="grid gap-4">
	<label class="grid gap-1 text-sm font-medium">
		Name
		<input class={inputClass} bind:value={form.name} required placeholder="web-03" />
	</label>

	<label class="grid gap-1 text-sm font-medium">
		Node
		<select class={inputClass} bind:value={form.node} required>
			<option value="" disabled>Choose a node</option>
			{#each form.catalog?.nodes ?? [] as node (node)}
				<option value={node}>{node}</option>
			{/each}
		</select>
	</label>

	<label class="grid gap-1 text-sm font-medium">
		Tags <span class="text-xs font-normal text-muted-foreground">comma-separated, in addition to the mandatory pvmss tag</span>
		<input class={inputClass} bind:value={form.tagsInput} placeholder="team-web, prod" />
	</label>

	<label class="grid gap-1 text-sm font-medium">
		ISO <span class="text-xs font-normal text-muted-foreground">optional</span>
		<select class={inputClass} bind:value={form.isoFile}>
			<option value="">None</option>
			{#each form.catalog?.isos ?? [] as iso (iso.file)}
				<option value={iso.file}>{iso.file}</option>
			{/each}
		</select>
	</label>
</div>
