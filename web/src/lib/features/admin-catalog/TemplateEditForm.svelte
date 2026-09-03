<script lang="ts">
	import type { AdminTemplate, AdminTemplatePatch } from './admin-catalog.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Button from '$lib/shared/ui/Button.svelte';
	import Checkbox from '$lib/shared/ui/Checkbox.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';

	interface Props {
		template: AdminTemplate;
		saving: boolean;
		onClose: () => void;
		onSave: (patch: AdminTemplatePatch) => void;
	}

	let { template, saving, onClose, onSave }: Props = $props();
	// This form intentionally captures the server snapshot when the keyed form mounts.
	// svelte-ignore state_referenced_locally
	let values = $state<AdminTemplatePatch>({
		node: template.node,
		name: template.name,
		cloudInitCapable: template.cloudInitCapable,
		diskStorage: template.diskStorage,
		diskSizeGB: template.diskSizeGB,
		diskBus: template.diskBus
	});
</script>

<h2 id="template-edit-title" class="text-lg font-semibold">{m['admin.templates.editTitle']({ vmid: template.vmid })}</h2>
<p class="mt-2 text-sm text-muted-foreground">{m['admin.templates.overrideHint']()}</p>
<form class="mt-5 grid gap-4" onsubmit={(event) => { event.preventDefault(); onSave({ ...values }); }}>
	<FormField label={m['admin.templates.fieldNode']()} required>
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} bind:value={values.node} required />
		{/snippet}
	</FormField>
	<FormField label={m['admin.templates.fieldName']()} required>
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} bind:value={values.name} required />
		{/snippet}
	</FormField>
	<FormField label={m['admin.templates.fieldDiskStorage']()} required>
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} bind:value={values.diskStorage} required />
		{/snippet}
	</FormField>
	<FormField label={m['admin.templates.fieldDiskSizeGB']()} required>
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} type="number" min={0} bind:value={values.diskSizeGB} required />
		{/snippet}
	</FormField>
	<FormField label={m['admin.templates.fieldDiskBus']()} required>
		{#snippet children({ id, describedBy, invalid })}
			<TextField {id} {describedBy} {invalid} bind:value={values.diskBus} required />
		{/snippet}
	</FormField>
	<Checkbox
		label={m['admin.templates.fieldCloudInitCapable']()}
		checked={values.cloudInitCapable}
		onToggle={(checked) => (values.cloudInitCapable = checked)}
	/>
	<div class="flex justify-end gap-2 pt-2">
		<Button variant="secondary" onclick={onClose}>{m['policy.cancel']()}</Button>
		<Button type="submit" disabled={saving}>{saving ? m['policy.saving']() : m['common.save']()}</Button>
	</div>
</form>
