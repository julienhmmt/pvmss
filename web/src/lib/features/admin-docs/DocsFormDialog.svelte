<script lang="ts">
	import Button from '$lib/shared/ui/Button.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import Checkbox from '$lib/shared/ui/Checkbox.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import type { AdminDocPage } from './docs.svelte';

	interface Props {
		showForm: boolean;
		editing: AdminDocPage | null;
		title: string;
		slug: string;
		lang: string;
		category: string;
		audience: 'user' | 'admin';
		enabled: boolean;
		bodyMd: string;
		saving: boolean;
		onTitleChange: (value: string) => void;
		onSlugChange: (value: string) => void;
		onLangChange: (value: string) => void;
		onCategoryChange: (value: string) => void;
		onAudienceChange: (value: 'user' | 'admin') => void;
		onEnabledChange: (value: boolean) => void;
		onBodyChange: (value: string) => void;
		onCancel: () => void;
		onSave: () => void;
		onSaveAndView: () => void;
	}

	let {
		showForm,
		editing,
		title,
		slug,
		lang,
		category,
		audience,
		enabled,
		bodyMd,
		saving,
		onTitleChange,
		onSlugChange,
		onLangChange,
		onCategoryChange,
		onAudienceChange,
		onEnabledChange,
		onBodyChange,
		onCancel,
		onSave,
		onSaveAndView
	}: Props = $props();
</script>

{#if showForm}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
		role="dialog"
		aria-modal="true"
	>
		<div class="w-full max-w-2xl rounded-xl border border-border bg-card p-6 text-card-foreground shadow-card">
			<h2 class="mb-4 text-lg font-semibold">
				{editing ? m['docs.editPage']() : m['docs.newPage']()}
			</h2>
			<form onsubmit={(e) => { e.preventDefault(); onSave(); }} class="grid gap-4">
				<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
					<FormField label={m['docs.titleField']()} required>
						{#snippet children({ id, describedBy, invalid })}
							<input
								{id}
								aria-describedby={describedBy}
								aria-invalid={invalid ? 'true' : undefined}
								type="text"
								class="pv-input"
								value={title}
								oninput={(e) => onTitleChange(e.currentTarget.value)}
								required
							/>
						{/snippet}
					</FormField>
					<FormField label={m['docs.slug']()} hint={editing?.isSystem ? m['docs.systemProtected']() : undefined}>
						{#snippet children({ id, describedBy, invalid })}
							<input
								{id}
								aria-describedby={describedBy}
								aria-invalid={invalid ? 'true' : undefined}
								type="text"
								class="pv-input font-mono text-xs"
								value={slug}
								oninput={(e) => onSlugChange(e.currentTarget.value)}
								disabled={editing?.isSystem ?? false}
							/>
						{/snippet}
					</FormField>
				</div>
				<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
					<FormField label={m['docs.language']()}>
						{#snippet children({ id, describedBy, invalid })}
							<select
								{id}
								aria-describedby={describedBy}
								aria-invalid={invalid ? 'true' : undefined}
								class="pv-input pv-select"
								value={lang}
								onchange={(e) => onLangChange(e.currentTarget.value)}
								disabled={editing?.isSystem ?? false}
							>
								<option value="en">en</option>
								<option value="fr">fr</option>
							</select>
						{/snippet}
					</FormField>
					<FormField label={m['docs.category']()} required>
						{#snippet children({ id, describedBy, invalid })}
							<input
								{id}
								aria-describedby={describedBy}
								aria-invalid={invalid ? 'true' : undefined}
								type="text"
								class="pv-input"
								value={category}
								oninput={(e) => onCategoryChange(e.currentTarget.value)}
								required
							/>
						{/snippet}
					</FormField>
					<FormField label={m['docs.audience']()}>
						{#snippet children({ id, describedBy, invalid })}
							<select
								{id}
								aria-describedby={describedBy}
								aria-invalid={invalid ? 'true' : undefined}
								class="pv-input pv-select"
								value={audience}
								onchange={(e) => onAudienceChange(e.currentTarget.value as 'user' | 'admin')}
							>
								<option value="user">{m['docs.audienceUser']()}</option>
								<option value="admin">{m['docs.audienceAdmin']()}</option>
							</select>
						{/snippet}
					</FormField>
				</div>
				<Checkbox
					label={m['docs.enabled']()}
					checked={enabled}
					onToggle={(checked) => onEnabledChange(checked)}
				/>
				<FormField label={m['docs.body']()} required>
					{#snippet children({ id, describedBy, invalid })}
						<textarea
							{id}
							aria-describedby={describedBy}
							aria-invalid={invalid ? 'true' : undefined}
							class="pv-input font-mono text-xs"
							rows="14"
							value={bodyMd}
							oninput={(e) => onBodyChange(e.currentTarget.value)}
							required
						></textarea>
					{/snippet}
				</FormField>
				<div class="flex justify-end gap-2 pt-2">
					<Button variant="ghost" onclick={onCancel}>{m['docs.cancel']()}</Button>
					<Button type="submit" disabled={saving}>
						{saving ? `${m['docs.save']()}…` : editing ? m['docs.save']() : m['docs.create']()}
					</Button>
					{#if editing}
						<Button variant="secondary" onclick={onSaveAndView} disabled={saving}>
							{m['docs.saveAndView']()}
						</Button>
					{/if}
				</div>
			</form>
		</div>
	</div>
{/if}
