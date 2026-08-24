<script lang="ts">
	import Button from '$lib/shared/ui/Button.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Textarea from '$lib/shared/ui/Textarea.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		showForm: boolean;
		editingId: string | null;
		label: string;
		content: string;
		saving: boolean;
		onLabelChange: (value: string) => void;
		onContentChange: (value: string) => void;
		onCancel: () => void;
		onSubmit: () => void;
	}

	let {
		showForm,
		editingId,
		label,
		content,
		saving,
		onLabelChange,
		onContentChange,
		onCancel,
		onSubmit
	}: Props = $props();

	const MAX_CONTENT_LENGTH = 16384;
</script>

<Dialog open={showForm} size="lg" labelledBy="cloudinit-template-form-title" onClose={onCancel}>
	<h2 id="cloudinit-template-form-title" class="mb-4 text-lg font-semibold">{editingId ? m['admin.cloudinit.editTemplate']() : m['admin.cloudinit.newTemplateForm']()}</h2>
	<form onsubmit={(e) => { e.preventDefault(); onSubmit(); }} class="grid gap-4">
		<FormField label={m['admin.cloudinit.labelField']()} required>
			{#snippet children({ id, describedBy, invalid })}
				<TextField
					{id}
					{describedBy}
					{invalid}
					value={label}
					oninput={(e: Event & { currentTarget: HTMLInputElement | HTMLTextAreaElement }) => onLabelChange(e.currentTarget.value)}
					required
				/>
			{/snippet}
		</FormField>
		<FormField label={m['admin.cloudinit.contentField']()} required>
			{#snippet children({ id, describedBy, invalid })}
				<Textarea
					{id}
					{describedBy}
					{invalid}
					value={content}
					oninput={(e: Event & { currentTarget: HTMLInputElement | HTMLTextAreaElement }) => onContentChange(e.currentTarget.value)}
					rows={12}
					required
					mono
					maxLength={MAX_CONTENT_LENGTH}
				/>
				<p class="text-right text-xs text-muted-foreground">
					{m['admin.cloudinit.contentCounter']({ used: content.length, max: MAX_CONTENT_LENGTH })}
				</p>
			{/snippet}
		</FormField>
		<div class="flex justify-end gap-2 pt-2">
			<Button variant="ghost" onclick={onCancel}>{m['common.cancel']()}</Button>
			<Button type="submit" disabled={saving}>
				{saving ? m['common.saving']() : editingId ? m['common.save']() : m['common.create']()}
			</Button>
		</div>
	</form>
</Dialog>
