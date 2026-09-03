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

	// Never offer a source that leads to an empty required dropdown: without
	// approved templates the template path is a dead end, so the selector
	// omits it and any stale 'template' selection falls back to ISO.
	const hasTemplates = $derived(templates.length > 0);

	const sourceOptions = $derived(
		hasTemplates
			? [
					{ value: 'iso', label: m['vms.create.sourceIso']() },
					{ value: 'template', label: m['vms.create.sourceTemplate']() }
				]
			: [{ value: 'iso', label: m['vms.create.sourceIso']() }]
	);

	$effect(() => {
		if (!hasTemplates && form.sourceType === 'template') {
			form.sourceType = 'iso';
		}
	});

	// US2/issue-02 D2b: hide the node selector when a template is selected —
	// the clone stays on the template's node.
	const showNodeSelector = $derived(form.sourceType !== 'template');

	// Issue 04: the template's disk is the clone source — Proxmox cannot
	// shrink it, so the disk size may never drop below the template's size.
	// Switching the source away from template clears the floor.
	let templateMinRaised = $state(false);

	$effect(() => {
		if (form.sourceType !== 'template') {
			form.templateMinDiskGB = 0;
			templateMinRaised = false;
		}
	});

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
		templateMinRaised = false;

		// D2b: the clone stays on the template's node. Set form.node so
		// StepNetwork's bridge dropdown filters by the correct node.
		if (vmid !== 0) {
			const tmpl = templates.find((t) => t.vmid === vmid);
			if (tmpl) {
				form.node = tmpl.node;
				if (form.diskSizeGB < tmpl.diskSizeGB) {
					form.diskSizeGB = tmpl.diskSizeGB;
					templateMinRaised = true;
				}
				form.templateMinDiskGB = tmpl.diskSizeGB;
			}
		} else {
			form.templateMinDiskGB = 0;
		}
	}
	// Issue 04: group templates by node and carry the facts that matter in
	// the label. The shared Select has no optgroup support, so the template
	// picker is a native select — keyboard type-ahead for free.
	const templateGroups = $derived(
		[...new Set(templates.map((tmpl) => tmpl.node))].sort().map((node) => ({
			node,
			templates: templates.filter((tmpl) => tmpl.node === node)
		}))
	);

	function templateLabel(tmpl: typeof templates[number]): string {
		const name = tmpl.name !== '' ? tmpl.name : `VMID ${tmpl.vmid}`;
		const cloudInit = tmpl.cloudInitCapable ? ` · ${m['admin.templates.cloudInit']()}` : '';
		return `${name} · ${tmpl.diskSizeGB} GB${cloudInit}`;
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
				options={sourceOptions}
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
				<select
					{id}
					class="pv-input pv-select"
					aria-describedby={describedBy}
					aria-invalid={invalid ? 'true' : undefined}
					value={templateIdStr}
					onchange={onTemplateChange}
				>
					<option value="" disabled>{m['vms.create.chooseTemplate']()}</option>
					{#each templateGroups as group (group.node)}
						<optgroup label={group.node}>
							{#each group.templates as tmpl (tmpl.vmid)}
								<option value={String(tmpl.vmid)}>{templateLabel(tmpl)}</option>
							{/each}
						</optgroup>
					{/each}
				</select>
				{#if templateMinRaised}
					<p class="mt-1 text-xs text-muted-foreground" data-testid="template-min-raised">
						{m['vms.create.templateMinRaised']({ min: form.templateMinDiskGB })}
					</p>
				{/if}
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
