<script lang="ts">
	import { getVmCreateContext, type VmSource } from '../create.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import TemplatePicker from '../TemplatePicker.svelte';
	import ImagePicker from '../ImagePicker.svelte';
	import ImageCloudInitFields from '../ImageCloudInitFields.svelte';

	// Base step (FR-011): name, source type (ISO vs template vs cloud image,
	// US2/issue-02), node (hidden for template source, D2b), extra tags
	// (pvmss is added server-side, FR-006), and the source — all choices
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

	// Cloud images available in the catalog (image mode). Like ISOs they are
	// node-local; the picker groups them by node.
	const images = $derived(form.catalog?.images ?? []);

	// Never offer a source that leads to an empty required dropdown: without
	// approved templates the template path is a dead end, so the selector
	// omits it and any stale 'template' selection falls back to ISO. Same for
	// cloud images.
	const hasTemplates = $derived(templates.length > 0);
	const hasImages = $derived(images.length > 0);

	const sourceOptions = $derived([
		{ value: 'iso', label: m['vms.create.sourceIso']() },
		...(hasTemplates ? [{ value: 'template', label: m['vms.create.sourceTemplate']() }] : []),
		...(hasImages ? [{ value: 'image', label: m['vms.create.sourceImage']() }] : [])
	]);

	$effect(() => {
		if (!hasTemplates && form.sourceType === 'template') {
			form.sourceType = 'iso';
		}
		if (!hasImages && form.sourceType === 'image') {
			form.sourceType = 'iso';
		}
	});

	// US2/issue-02 D2b: hide the node selector when a template is selected —
	// the clone stays on the template's node. Image mode keeps it: like an
	// ISO, an image is node-local and the server restricts candidates to the
	// nodes holding it.
	const showNodeSelector = $derived(form.sourceType !== 'template');

	// Issue 04: the template's disk is the clone source — Proxmox cannot
	// shrink it, so the disk size may never drop below the template's size.
	// Switching the source away from template clears the floor.
	$effect(() => {
		if (form.sourceType !== 'template') {
			form.templateMinDiskGB = 0;
		}
	});
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
				value={form.sourceType}
				onchange={(event: Event) => form.setSourceType((event.currentTarget as HTMLSelectElement).value as VmSource)}
				options={sourceOptions}
			/>
			{#if form.catalog && !hasTemplates}
				<p class="mt-1 text-xs text-muted-foreground" data-testid="no-templates-hint">
					{m['vms.create.noTemplatesAvailable']()}
				</p>
			{/if}
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
		<TemplatePicker />
	{:else if form.sourceType === 'image'}
		<ImagePicker />
		<ImageCloudInitFields />
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
