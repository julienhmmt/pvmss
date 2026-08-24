<script lang="ts">
	import Button from '$lib/shared/ui/Button.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Textarea from '$lib/shared/ui/Textarea.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
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

<Dialog open={showForm} size="xl" labelledBy="docs-form-title" onClose={onCancel}>
	<h2 id="docs-form-title" class="mb-4 text-lg font-semibold">
		{editing ? m['docs.editPage']() : m['docs.newPage']()}
	</h2>
	<form onsubmit={(e) => { e.preventDefault(); onSave(); }} class="grid gap-4">
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
			<FormField label={m['docs.titleField']()} required>
				{#snippet children({ id, describedBy, invalid })}
					<TextField
						{id}
						{describedBy}
						{invalid}
						value={title}
						oninput={(e: Event & { currentTarget: HTMLInputElement | HTMLTextAreaElement }) => onTitleChange(e.currentTarget.value)}
						required
					/>
				{/snippet}
			</FormField>
			<FormField label={m['docs.slug']()} hint={editing?.isSystem ? m['docs.systemProtected']() : undefined}>
				{#snippet children({ id, describedBy, invalid })}
					<TextField
						{id}
						{describedBy}
						{invalid}
						value={slug}
						oninput={(e: Event & { currentTarget: HTMLInputElement | HTMLTextAreaElement }) => onSlugChange(e.currentTarget.value)}
						disabled={editing?.isSystem ?? false}
						class="font-mono text-xs"
					/>
				{/snippet}
			</FormField>
		</div>
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
			<FormField label={m['docs.language']()}>
				{#snippet children({ id, describedBy, invalid })}
					<Select
						{id}
						{describedBy}
						{invalid}
						value={lang}
						onchange={(e: Event & { currentTarget: HTMLSelectElement }) => onLangChange(e.currentTarget.value)}
						disabled={editing?.isSystem ?? false}
						options={[
							{ value: 'en', label: 'en' },
							{ value: 'fr', label: 'fr' }
						]}
					/>
				{/snippet}
			</FormField>
			<FormField label={m['docs.category']()} required>
				{#snippet children({ id, describedBy, invalid })}
					<TextField
						{id}
						{describedBy}
						{invalid}
						value={category}
						oninput={(e: Event & { currentTarget: HTMLInputElement | HTMLTextAreaElement }) => onCategoryChange(e.currentTarget.value)}
						required
					/>
				{/snippet}
			</FormField>
			<FormField label={m['docs.audience']()}>
				{#snippet children({ id, describedBy, invalid })}
					<Select
						{id}
						{describedBy}
						{invalid}
						value={audience}
						onchange={(e: Event & { currentTarget: HTMLSelectElement }) => onAudienceChange(e.currentTarget.value as 'user' | 'admin')}
						options={[
							{ value: 'user', label: m['docs.audienceUser']() },
							{ value: 'admin', label: m['docs.audienceAdmin']() }
						]}
					/>
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
				<Textarea
					{id}
					{describedBy}
					{invalid}
					value={bodyMd}
					oninput={(e: Event & { currentTarget: HTMLInputElement | HTMLTextAreaElement }) => onBodyChange(e.currentTarget.value)}
					rows={14}
					required
					mono
				/>
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
</Dialog>
