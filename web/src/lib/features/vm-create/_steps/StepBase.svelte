<script lang="ts">
	import { getVmCreateContext } from '../create.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';

	// Base step (FR-011): name, source type (ISO vs template, US2/issue-02),
	// node (hidden for template source, D2b), extra tags (pvmss is added
	// server-side, FR-006), and an optional ISO or template — all choices
	// from the approved catalog.
	const form = getVmCreateContext();

	const catalogTags = $derived(form.catalog?.tags ?? []);
	const selected = $derived(new Set(form.selectedTags()));

	// ISOs are node-local (US1): only show ISOs that are on the selected node,
	// same pattern as storage filtering in StepDisk.
	const isosOnNode = $derived(
		(form.catalog?.isos ?? []).filter((iso) => iso.node === form.node)
	);

	// US2/issue-02: templates available in the catalog. The node is derived
	// from the template server-side (D2b), so no node filtering here.
	const templates = $derived(form.catalog?.templates ?? []);

	// US2/issue-02 D2b: hide the node selector when a template is selected —
	// the clone stays on the template's node.
	const showNodeSelector = $derived(form.sourceType !== 'template');

	// Select binds to string values; templateId is a number. Derive the
	// string representation from the form state. One-way binding only —
	// updates flow through onTemplateChange, which writes form.templateId
	// (the source of this derived value). bind:value would attempt to write
	// back to a read-only $derived, which Svelte 5 forbids.
	let templateIdStr = $derived(form.templateId === 0 ? '' : String(form.templateId));

	function onTemplateChange(event: Event): void {
		const value = (event.currentTarget as HTMLSelectElement).value;
		const vmid = value === '' ? 0 : Number(value);
		form.templateId = vmid;

		// D2b: the clone stays on the template's node. Set form.node so
		// StepNetwork's bridge dropdown filters by the correct node.
		if (vmid !== 0) {
			const tmpl = templates.find((t) => t.vmid === vmid);
			if (tmpl) {
				form.node = tmpl.node;
			}
		}
	}
</script>

<div class="grid gap-4">
	<FormField label={m['vms.create.name']()} required>
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} bind:value={form.name} required placeholder="web-03" />
		{/snippet}
	</FormField>

	<FormField label={m['vms.create.source']()} hint={m['vms.create.sourceHelp']()}>
		{#snippet children({ id, describedBy, invalid })}
			<Select
				{id}
				{describedBy}
				{invalid}
				bind:value={form.sourceType}
				options={[
					{ value: 'iso', label: m['vms.create.sourceIso']() },
					{ value: 'template', label: m['vms.create.sourceTemplate']() }
				]}
			/>
		{/snippet}
	</FormField>

	{#if showNodeSelector}
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
	{/if}

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

	{#if form.sourceType === 'template'}
		<FormField label={m['vms.create.template']()} required hint={m['vms.create.templateHelp']()}>
			{#snippet children({ id, describedBy, invalid })}
				<Select
					{id}
					{describedBy}
					{invalid}
					value={templateIdStr}
					onchange={onTemplateChange}
					placeholder={m['vms.create.chooseTemplate']()}
					options={templates.map((tmpl) => ({
						value: String(tmpl.vmid),
						label: tmpl.name !== '' ? `${tmpl.name} (VMID ${tmpl.vmid})` : `VMID ${tmpl.vmid}`
					}))}
				/>
			{/snippet}
		</FormField>
	{:else}
		<FormField label={m['vms.create.iso']()} hint={m['common.optional']()}>
			{#snippet children({ id, describedBy, invalid })}
				<Select
					{id}
					{describedBy}
					{invalid}
					bind:value={form.isoFile}
					options={isosOnNode.map((iso) => ({ value: iso.file, label: iso.file }))}
				/>
			{/snippet}
		</FormField>
	{/if}
</div>
