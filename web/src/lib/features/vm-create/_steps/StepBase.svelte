<script lang="ts">
	import { getVmCreateContext } from '../create.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';

	// Base step (FR-011): name, node, extra tags (pvmss is added server-side,
	// FR-006), and an optional ISO — all choices from the approved catalog.
	const form = getVmCreateContext();

	const catalogTags = $derived(form.catalog?.tags ?? []);
	const selected = $derived(new Set(form.selectedTags()));
</script>

<div class="grid gap-4">
	<FormField label={m['vms.create.name']()} required>
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} bind:value={form.name} required placeholder="web-03" />
		{/snippet}
	</FormField>

	<FormField
		label={m['vms.create.node']()}
		required
		hint={form.catalog ? m['vms.create.nodeInCluster']({ cluster: form.clusterDisplayName() }) : undefined}
	>
		{#snippet children({ id, describedBy, invalid })}
			<Select
				{id}
				{describedBy}
				{invalid}
				bind:value={form.node}
				placeholder={m['vms.create.chooseNode']()}
				options={form.catalog?.nodes ?? []}
			/>
		{/snippet}
	</FormField>

	<FormField label={m['vms.create.tags']()} hint={m['vms.create.tagsHelp']()}>
		{#if catalogTags.length === 0}
			<p class="text-sm text-muted-foreground">{m['vms.create.tagsNoneAvailable']()}</p>
		{:else}
			<div class="flex flex-wrap gap-2" role="group" aria-label={m['vms.create.tags']()}>
				{#each catalogTags as tag (tag.name)}
					{@const isSelected = selected.has(tag.name)}
					<button
						type="button"
						aria-pressed={isSelected}
						onclick={() => form.toggleTag(tag.name)}
						class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background {isSelected
							? 'border-transparent bg-primary text-primary-foreground'
							: 'border-border bg-muted text-muted-foreground hover:bg-muted/80'}"
					>
						<span class="h-2 w-2 rounded-full" style="background-color: {tag.color}" aria-hidden="true"></span>
						{tag.name}
					</button>
				{/each}
			</div>
		{/if}
	</FormField>

	<FormField label={m['vms.create.iso']()} hint={m['common.optional']()}>
		{#snippet children({ id, describedBy, invalid })}
			<Select
				{id}
				{describedBy}
				{invalid}
				bind:value={form.isoFile}
				options={form.catalog?.isos.map((iso) => ({ value: iso.file, label: iso.file })) ?? []}
			/>
		{/snippet}
	</FormField>
</div>
