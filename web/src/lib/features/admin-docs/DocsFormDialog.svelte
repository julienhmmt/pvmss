<script lang="ts">
	import Button from '$lib/shared/ui/Button.svelte';
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

	const inputClass = 'w-full rounded-md border bg-background px-3 py-2 text-sm';
</script>

{#if showForm}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
		role="dialog"
		aria-modal="true"
	>
		<div class="w-full max-w-2xl rounded-lg bg-background p-6 shadow-lg">
			<h2 class="mb-4 text-lg font-medium">
				{editing ? m['docs.editPage']() : m['docs.newPage']()}
			</h2>
			<form onsubmit={(e) => { e.preventDefault(); onSave(); }} class="space-y-4">
				<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
					<div>
						<label for="docs-title" class="mb-1 block text-sm font-medium">
							{m['docs.titleField']()}
						</label>
						<input
							id="docs-title"
							type="text"
							class={inputClass}
							value={title}
							oninput={(e) => onTitleChange(e.currentTarget.value)}
							required
						/>
					</div>
					<div>
						<label for="docs-slug" class="mb-1 block text-sm font-medium">
							{m['docs.slug']()}
						</label>
						<input
							id="docs-slug"
							type="text"
							class={`${inputClass} font-mono text-xs`}
							value={slug}
							oninput={(e) => onSlugChange(e.currentTarget.value)}
							disabled={editing?.isSystem ?? false}
						/>
						{#if editing?.isSystem}
							<p class="mt-1 text-xs text-muted-foreground">{m['docs.systemProtected']()}</p>
						{/if}
					</div>
				</div>
				<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
					<div>
						<label for="docs-lang" class="mb-1 block text-sm font-medium">
							{m['docs.language']()}
						</label>
						<select
							id="docs-lang"
							class={inputClass}
							value={lang}
							onchange={(e) => onLangChange(e.currentTarget.value)}
							disabled={editing?.isSystem ?? false}
						>
							<option value="en">en</option>
							<option value="fr">fr</option>
						</select>
					</div>
					<div>
						<label for="docs-category" class="mb-1 block text-sm font-medium">
							{m['docs.category']()}
						</label>
						<input
							id="docs-category"
							type="text"
							class={inputClass}
							value={category}
							oninput={(e) => onCategoryChange(e.currentTarget.value)}
							required
						/>
					</div>
					<div>
						<label for="docs-audience" class="mb-1 block text-sm font-medium">
							{m['docs.audience']()}
						</label>
						<select
							id="docs-audience"
							class={inputClass}
							value={audience}
							onchange={(e) => onAudienceChange(e.currentTarget.value as 'user' | 'admin')}
						>
							<option value="user">{m['docs.audienceUser']()}</option>
							<option value="admin">{m['docs.audienceAdmin']()}</option>
						</select>
					</div>
				</div>
				<div class="flex items-center gap-2">
					<input
						id="docs-enabled"
						type="checkbox"
						class="h-4 w-4 rounded border-border"
						checked={enabled}
						onchange={(e) => onEnabledChange(e.currentTarget.checked)}
					/>
					<label for="docs-enabled" class="text-sm font-medium">{m['docs.enabled']()}</label>
				</div>
				<div>
					<label for="docs-body" class="mb-1 block text-sm font-medium">
						{m['docs.body']()}
					</label>
					<textarea
						id="docs-body"
						class={`${inputClass} font-mono text-xs`}
						rows="14"
						value={bodyMd}
						oninput={(e) => onBodyChange(e.currentTarget.value)}
						required
					></textarea>
				</div>
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
