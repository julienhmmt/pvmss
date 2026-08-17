<script lang="ts">
	import Button from '$lib/shared/ui/Button.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
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
</script>

{#if showForm}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
		role="dialog"
		aria-modal="true"
	>
		<div class="w-full max-w-lg rounded-xl border border-border bg-card p-6 text-card-foreground shadow-card">
			<h2 class="mb-4 text-lg font-semibold">{editingId ? m['admin.cloudinit.editTemplate']() : m['admin.cloudinit.newTemplateForm']()}</h2>
			<form onsubmit={(e) => { e.preventDefault(); onSubmit(); }} class="grid gap-4">
				<FormField label={m['admin.cloudinit.labelField']()} required>
					{#snippet children({ id, describedBy, invalid })}
						<input
							{id}
							aria-describedby={describedBy}
							aria-invalid={invalid ? 'true' : undefined}
							type="text"
							class="pv-input"
							value={label}
							oninput={(e) => onLabelChange(e.currentTarget.value)}
							required
						/>
					{/snippet}
				</FormField>
				<FormField label={m['admin.cloudinit.contentField']()} required>
					{#snippet children({ id, describedBy, invalid })}
						<textarea
							{id}
							aria-describedby={describedBy}
							aria-invalid={invalid ? 'true' : undefined}
							class="pv-input font-mono text-xs"
							rows="12"
							value={content}
							oninput={(e) => onContentChange(e.currentTarget.value)}
							required
						></textarea>
					{/snippet}
				</FormField>
				<div class="flex justify-end gap-2 pt-2">
					<Button variant="ghost" onclick={onCancel}>{m['common.cancel']()}</Button>
					<Button type="submit" disabled={saving}>
						{saving ? m['common.saving']() : editingId ? m['common.save']() : m['common.create']()}
					</Button>
				</div>
			</form>
		</div>
	</div>
{/if}
