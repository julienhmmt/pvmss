<script lang="ts">
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
		<div class="w-full max-w-lg rounded-lg bg-background p-6 shadow-lg">
			<h2 class="mb-4 text-lg font-medium">{editingId ? 'Edit template' : 'New template'}</h2>
			<form onsubmit={(e) => { e.preventDefault(); onSubmit(); }} class="space-y-4">
				<div>
					<label for="cit-label" class="mb-1 block text-sm font-medium">Label</label>
					<input
						id="cit-label"
						type="text"
						class="w-full rounded-md border bg-background px-3 py-2 text-sm"
						value={label}
						oninput={(e) => onLabelChange(e.currentTarget.value)}
						required
					/>
				</div>
				<div>
					<label for="cit-content" class="mb-1 block text-sm font-medium">
						Content (must start with <code>#cloud-config</code>)
					</label>
					<textarea
						id="cit-content"
						class="w-full rounded-md border bg-background px-3 py-2 font-mono text-xs"
						rows="12"
						value={content}
						oninput={(e) => onContentChange(e.currentTarget.value)}
						required
					></textarea>
				</div>
				<div class="flex justify-end gap-2 pt-2">
					<button
						type="button"
						class="rounded-md px-4 py-2 text-sm text-muted-foreground hover:text-foreground"
						onclick={onCancel}
					>
						Cancel
					</button>
					<button
						type="submit"
						class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
						disabled={saving}
					>
						{saving ? 'Saving…' : editingId ? 'Save' : 'Create'}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}
