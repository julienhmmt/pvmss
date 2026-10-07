<script lang="ts">
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import TemplateEditForm from './TemplateEditForm.svelte';
	import type { AdminTemplate, AdminTemplatePatch } from './admin-catalog.svelte';

	interface Props {
		template: AdminTemplate | null;
		open: boolean;
		saving: boolean;
		onClose: () => void;
		onSave: (patch: AdminTemplatePatch) => void;
	}

	let { template, open, saving, onClose, onSave }: Props = $props();
</script>

<Dialog {open} labelledBy="template-edit-title" {onClose}>
	{#if template !== null}
		{#key `${template.vmid}:${template.node}:${template.diskStorage}:${template.diskSizeGB}:${template.diskBus}:${template.cloudInitCapable}`}
			<TemplateEditForm {template} {saving} {onClose} {onSave} />
		{/key}
	{/if}
</Dialog>
