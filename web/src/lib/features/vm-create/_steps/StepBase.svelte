<script lang="ts">
	import { getVmCreateContext } from '../create.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';

	// Base step (FR-011): name, node, extra tags (pvmss is added server-side,
	// FR-006), and an optional ISO — all choices from the approved catalog.
	const form = getVmCreateContext();
</script>

<div class="grid gap-4">
	<FormField label={m['vms.create.name']()} required>
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} bind:value={form.name} required placeholder="web-03" />
		{/snippet}
	</FormField>

	<FormField label={m['vms.create.node']()} required>
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
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} bind:value={form.tagsInput} placeholder="team-web, prod" />
		{/snippet}
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
