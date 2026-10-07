<script lang="ts">
	/**
	 * TemplatePicker - grouped select for approved Proxmox templates.
	 * Used by both the simple wizard and the detailed wizard's Base step.
	 */
	import { getVmCreateContext } from './create.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';

	interface Props {
		error?: string | null;
	}

	let { error = null }: Props = $props();

	const form = getVmCreateContext();

	const templates = $derived(form.catalog?.templates ?? []);

	// Issue 04: the template's disk is the clone source - Proxmox cannot
	// shrink it, so the disk size may never drop below the template's size.
	let templateMinRaised = $state(false);

	// Select binds to string values; templateId is a number. Derive the
	// string representation from the form state. One-way binding only - 
	// updates flow through onTemplateChange, which writes form.templateId.
	const templateIdStr = $derived(form.templateId === 0 ? '' : String(form.templateId));

	function onTemplateChange(event: Event): void {
		const value = (event.currentTarget as HTMLSelectElement).value;
		const vmid = value === '' ? 0 : Number(value);
		form.templateId = vmid;
		templateMinRaised = false;

		// D2b: the clone stays on the template's node. Set form.node so
		// downstream storage/bridge selects filter by the correct node.
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

	// Issue 04: carry the node as `group` so Select emits one <optgroup> per
	// node (sorted), and carry the facts that matter in the label.
	const options = $derived(
		[...templates]
			.sort((a, b) => a.node.localeCompare(b.node))
			.map((tmpl) => ({
				value: String(tmpl.vmid),
				label: templateLabel(tmpl),
				group: tmpl.node
			}))
	);

	function templateLabel(tmpl: (typeof templates)[number]): string {
		const name = tmpl.name !== '' ? tmpl.name : `VMID ${tmpl.vmid}`;
		const cloudInit = tmpl.cloudInitCapable ? ` · ${m['admin.templates.cloudInit']()}` : '';
		return `${name} · ${tmpl.diskSizeGB} GB${cloudInit}`;
	}
</script>

<FormField label={m['vms.create.template']()} required hint={m['vms.create.templateHelp']()} {error}>
	{#snippet children({ id, describedBy, invalid })}
		<Select
			{id}
			{describedBy}
			{invalid}
			value={templateIdStr}
			{options}
			onchange={onTemplateChange}
			placeholder={m['vms.create.chooseTemplate']()}
			required
		/>
		{#if templateMinRaised}
			<p class="mt-1 text-xs text-muted-foreground" data-testid="template-min-raised">
				{m['vms.create.templateMinRaised']({ min: form.templateMinDiskGB })}
			</p>
		{/if}
	{/snippet}
</FormField>
